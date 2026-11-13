package service

import (
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type GroupInput struct {
	Name string `json:"name"`
}
type GroupView struct {
	domain.CompetitionGroup
	Events   int `json:"events"`
	Units    int `json:"units"`
	Athletes int `json:"athletes"`
}

func (s *Service) ListGroups(ctx context.Context, meetID string) ([]GroupView, error) {
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	parents, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := []GroupView{}
	for _, g := range groups {
		v := GroupView{CompetitionGroup: g}
		for _, e := range events {
			if e.GroupID == g.ID && e.CombinedID == "" {
				v.Events++
			}
		}
		for _, c := range parents {
			if c.Combined.GroupID == g.ID {
				v.Events++
			}
		}
		for _, u := range units {
			if slices.Contains(u.GroupIDs, g.ID) {
				v.Units++
			}
		}
		for _, a := range athletes {
			if a.GroupID == g.ID {
				v.Athletes++
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) validateGroupIDs(ctx context.Context, meetID string, ids []string) error {
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] || !slices.ContainsFunc(groups, func(g domain.CompetitionGroup) bool { return g.ID == id }) {
			return invalid("组别不存在、重复或不属于本赛事")
		}
		seen[id] = true
	}
	return nil
}

// Name-based calls are a compatibility path, interned into the same catalog.
// The returned creation statement joins the caller's transaction.
func (s *Service) resolveGroup(ctx context.Context, meetID, id, label string) (string, string, []sqlite.Statement, error) {
	label = strings.TrimSpace(label)
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return "", "", nil, err
	}
	for _, g := range groups {
		if id != "" && g.ID == id || id == "" && label != "" && domain.NormalizeText(g.Name) == domain.NormalizeText(label) {
			if id != "" && label != "" && domain.NormalizeText(label) != domain.NormalizeText(g.Name) {
				return "", "", nil, invalid("组别名称与所选组别不符")
			}
			return g.ID, g.Name, nil, nil
		}
	}
	if id != "" {
		return "", "", nil, invalid("组别不属于本赛事")
	}
	if label == "" {
		return "", "", nil, nil
	}
	g := domain.CompetitionGroup{ID: newID(), MeetID: meetID, Name: label, Ordinal: nextGroupOrdinal(groups)}
	return g.ID, g.Name, []sqlite.Statement{insertGroup(g)}, nil
}

func nextGroupOrdinal(groups []domain.CompetitionGroup) int {
	next := 1
	for _, g := range groups {
		next = max(next, g.Ordinal+1)
	}
	return next
}

func insertGroup(g domain.CompetitionGroup) sqlite.Statement {
	return store.Stmt(`INSERT INTO meet_groups(id,meet_id,name,name_key,ordinal) VALUES (?,?,?,?,?)`, g.ID, g.MeetID, g.Name, domain.NormalizeText(g.Name), g.Ordinal)
}

func (s *Service) SaveGroup(ctx context.Context, src store.Source, meetID, id string, in GroupInput) (*domain.CompetitionGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalid("请填写组别名称")
	}
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, err
	}
	g := domain.CompetitionGroup{ID: id, MeetID: meetID, Name: name, Ordinal: nextGroupOrdinal(groups)}
	found := id == ""
	for _, old := range groups {
		if old.ID == id {
			g.Ordinal = old.Ordinal
			found = true
		} else if domain.NormalizeText(old.Name) == domain.NormalizeText(name) {
			return nil, invalid("已有同名组别")
		}
	}
	if !found {
		return nil, invalid("组别不存在或已删除")
	}
	stmts := []sqlite.Statement{}
	action := "group.updated"
	if id == "" {
		g.ID = newID()
		action = "group.created"
		stmts = append(stmts, insertGroup(g))
	} else {
		events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
		if err != nil {
			return nil, err
		}
		parents, err := s.store.ListCombined(ctx, meetID)
		if err != nil {
			return nil, err
		}
		parentNames := map[string]string{}
		for _, p := range parents {
			parentNames[p.Combined.ID] = p.Combined.Name
		}
		for _, e := range events {
			if e.GroupID != id {
				continue
			}
			auto := domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)
			next := domain.ComposeEventName(e.Gender, e.Discipline, name)
			if e.CombinedID != "" {
				auto = parentNames[e.CombinedID] + " · " + auto
				next = parentNames[e.CombinedID] + " · " + next
			}
			if e.Name != auto {
				next = e.Name
			}
			stmts = append(stmts, store.Stmt(`UPDATE events SET name=?,group_label=? WHERE id=?`, next, name, e.ID))
		}
		stmts = append(stmts, store.Stmt(`UPDATE combined_events SET group_label=? WHERE group_id=?`, name, id), store.Stmt(`UPDATE meet_groups SET name=?,name_key=? WHERE id=?`, name, domain.NormalizeText(name), id))
	}
	err = s.write(ctx, store.Audit{Source: src, Action: action, MeetID: meetID, Message: name, Detail: g}, Change{Kind: "groups", MeetID: meetID}, stmts...)
	return &g, err
}

// DeleteGroup removes a group; what referred to it becomes ungrouped in the
// same transaction, the way ChangeGroup moves events (results stay as they are).
func (s *Service) DeleteGroup(ctx context.Context, src store.Source, meetID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := s.ListGroups(ctx, meetID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(groups, func(g GroupView) bool { return g.ID == id })
	if i < 0 {
		return invalid("组别不存在或已删除")
	}
	g := groups[i]
	stmts := []sqlite.Statement{}
	if g.Events > 0 {
		st, err := s.loadCombinedMeet(ctx, meetID)
		if err != nil {
			return err
		}
		req := GroupChangeRequest{MeetID: meetID, EventIDs: []string{}, CombinedIDs: []string{}}
		for _, e := range st.events {
			if e.GroupID == id && e.CombinedID == "" {
				req.EventIDs = append(req.EventIDs, e.ID)
			}
		}
		for _, p := range st.parents {
			if p.Combined.GroupID == id {
				req.CombinedIDs = append(req.CombinedIDs, p.Combined.ID)
			}
		}
		if _, stmts, err = s.prepareGroupChange(ctx, req, false); err != nil {
			return err
		}
	}
	stmts = append(stmts,
		store.Stmt(`DELETE FROM unit_groups WHERE group_id=?`, id),
		store.Stmt(`UPDATE athletes SET group_id=NULL WHERE group_id=?`, id),
		store.Stmt(`DELETE FROM meet_groups WHERE id=? AND meet_id=?`, id, meetID))
	return s.write(ctx, store.Audit{Source: src, Action: "group.deleted", MeetID: meetID, Message: g.Name}, Change{Kind: "groups", MeetID: meetID}, stmts...)
}

func (s *Service) ReorderGroups(ctx context.Context, src store.Source, meetID string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return err
	}
	if len(ids) != len(groups) {
		return ErrStale
	}
	if err := s.validateGroupIDs(ctx, meetID, ids); err != nil {
		return err
	}
	stmts := []sqlite.Statement{}
	for i, id := range ids {
		stmts = append(stmts, store.Stmt(`UPDATE meet_groups SET ordinal=? WHERE id=?`, i+1, id))
	}
	return s.write(ctx, store.Audit{Source: src, Action: "group.reordered", MeetID: meetID, Message: "调整组别顺序", Detail: ids}, Change{Kind: "groups", MeetID: meetID}, stmts...)
}
