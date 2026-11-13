package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type GroupChangeRequest struct {
	MeetID      string   `json:"meetId"`
	GroupID     string   `json:"groupId"`
	EventIDs    []string `json:"eventIds"`
	CombinedIDs []string `json:"combinedIds"`
}
type GroupAssignment struct {
	Kind     string   `json:"kind"` // unit | athlete
	ID       string   `json:"id"`
	GroupIDs []string `json:"groupIds"`
}
type GroupAssignmentsRequest struct {
	MeetID      string            `json:"meetId"`
	Assignments []GroupAssignment `json:"assignments"`
}
type GroupChangeLine struct {
	Key    string   `json:"key"`
	Name   string   `json:"name"`
	Before []string `json:"before"`
	After  []string `json:"after"`
}
type GroupReview struct {
	PreviewHash string            `json:"previewHash"`
	Changes     []GroupChangeLine `json:"changes"`
	Before      []TeamScoreBoard  `json:"before"`
	After       []TeamScoreBoard  `json:"after"`
	Warnings    []string          `json:"warnings"`
}

func groupNames(groups []domain.CompetitionGroup, ids []string) []string {
	out := []string{}
	for _, g := range groups {
		if slices.Contains(ids, g.ID) {
			out = append(out, g.Name)
		}
	}
	return out
}

// groupHash covers the request and everything the preview shows; a change that
// alters the review makes the confirmation stale.
func groupHash(req any, p *GroupReview) (string, error) {
	b, err := json.Marshal(struct {
		Request  any
		Changes  []GroupChangeLine
		Before   []TeamScoreBoard
		After    []TeamScoreBoard
		Warnings []string
	}{req, p.Changes, p.Before, p.After, p.Warnings})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// prepareGroupChange builds the statements; review adds the team score boards and hash.
func (s *Service) prepareGroupChange(ctx context.Context, req GroupChangeRequest, review bool) (*GroupReview, []sqlite.Statement, error) {
	st, err := s.loadTeamScoreState(ctx, req.MeetID)
	if err != nil {
		return nil, nil, err
	}
	if req.GroupID != "" {
		if err := s.validateGroupIDs(ctx, req.MeetID, []string{req.GroupID}); err != nil {
			return nil, nil, err
		}
	}
	label := ""
	for _, g := range st.groups {
		if g.ID == req.GroupID {
			label = g.Name
		}
	}
	p := &GroupReview{Changes: []GroupChangeLine{}, Warnings: []string{}}
	if review {
		if p.Before, err = st.boards(TeamScoreFilter{MeetID: req.MeetID}, st.rules); err != nil {
			return nil, nil, err
		}
	}
	stmts := []sqlite.Statement{}
	selected := map[string]bool{}
	selectedParents := map[string]bool{}
	for _, id := range req.EventIDs {
		e, ok := st.combined.events[id]
		if id == "" || !ok || e.CombinedID != "" || selected[id] {
			return nil, nil, invalid("请选择本赛事的普通项目")
		}
		selected[id] = true
	}
	for _, id := range req.CombinedIDs {
		if id == "" || selectedParents[id] {
			return nil, nil, invalid("全能重复或不存在")
		}
		found := false
		for i, parent := range st.combined.parents {
			if parent.Combined.ID == id {
				found = true
				selectedParents[id] = true
				if parent.Combined.GroupID != req.GroupID {
					p.Changes = append(p.Changes, GroupChangeLine{Key: "combined:" + parent.Combined.ID, Name: parent.Combined.Name, Before: groupNames(st.groups, []string{parent.Combined.GroupID}), After: groupNames(st.groups, []string{req.GroupID})})
					st.combined.parents[i].Combined.GroupID = req.GroupID
					st.combined.parents[i].Combined.GroupLabel = label
					stmts = append(stmts, store.Stmt(`UPDATE combined_events SET group_id=?,group_label=? WHERE id=?`, nullable(req.GroupID), label, id))
				}
				for _, c := range parent.Components {
					selected[c.EventID] = true
				}
			}
		}
		if !found {
			return nil, nil, invalid("全能不属于本赛事")
		}
	}
	if len(selected) == 0 {
		return nil, nil, invalid("请选择项目或全能")
	}
	parents := map[string]string{}
	for _, p := range st.combined.parents {
		parents[p.Combined.ID] = p.Combined.Name
	}
	for id, e := range st.combined.events {
		if !selected[id] || e.GroupID == req.GroupID {
			continue
		}
		if e.CombinedID == "" {
			p.Changes = append(p.Changes, GroupChangeLine{Key: "event:" + e.ID, Name: e.Name, Before: groupNames(st.groups, []string{e.GroupID}), After: groupNames(st.groups, []string{req.GroupID})})
		}
		auto, next := domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel), domain.ComposeEventName(e.Gender, e.Discipline, label)
		if e.CombinedID != "" {
			auto = parents[e.CombinedID] + " · " + auto
			next = parents[e.CombinedID] + " · " + next
		}
		if e.Name == auto {
			e.Name = next
		}
		e.GroupID, e.GroupLabel = req.GroupID, label
		st.combined.events[id] = e
		stmts = append(stmts, store.Stmt(`UPDATE events SET group_id=?,group_label=?,name=? WHERE id=?`, nullable(req.GroupID), label, e.Name, id))
	}
	// Check identities after applying the entire request; never merge records.
	keys := map[string]bool{}
	for _, e := range st.combined.events {
		key := e.CombinedID + "|" + domain.EventKey(e.Gender, e.Discipline, e.GroupID)
		if keys[key] {
			return nil, nil, invalid("调整后存在相同项目")
		}
		keys[key] = true
	}
	keys = map[string]bool{}
	for _, c := range st.combined.parents {
		key := domain.CombinedKey(c.Combined.Gender, c.Combined.Name, c.Combined.GroupID)
		if keys[key] {
			return nil, nil, invalid("调整后存在相同全能")
		}
		keys[key] = true
	}
	for i, t := range st.targets {
		if selected[t.EventID] || selectedParents[t.CombinedID] {
			st.targets[i].GroupID = req.GroupID
			st.targets[i].GroupLabel = label
			if t.EventID != "" {
				st.targets[i].Name = st.combined.events[t.EventID].Name
			}
		}
	}
	p.Warnings = groupEntryWarnings(st, selected)
	slices.SortFunc(p.Changes, func(a, b GroupChangeLine) int { return strings.Compare(a.Key, b.Key) })
	p.Warnings = slices.Compact(p.Warnings)
	return s.finishGroupReview(st, req.MeetID, req, p, stmts, review)
}

func (s *Service) finishGroupReview(st *teamScoreState, meetID string, req any, p *GroupReview, stmts []sqlite.Statement, review bool) (*GroupReview, []sqlite.Statement, error) {
	if !review {
		return p, stmts, nil
	}
	var err error
	if p.After, err = st.boards(TeamScoreFilter{MeetID: meetID}, st.rules); err != nil {
		return nil, nil, err
	}
	p.PreviewHash, err = groupHash(req, p)
	return p, stmts, err
}
func groupEntryWarnings(st *teamScoreState, selected map[string]bool) []string {
	athletes := map[string]domain.Athlete{}
	for _, a := range st.combined.athletes {
		athletes[a.ID] = a
	}
	teams := map[string]domain.Team{}
	for _, t := range st.teams {
		teams[t.ID] = t
	}
	for _, en := range st.entries {
		if selected != nil && !selected[en.EventID] {
			continue
		}
		gid := st.combined.events[en.EventID].GroupID
		if gid == "" {
			continue
		}
		ids := []string{en.AthleteID}
		if en.TeamID != "" {
			ids = teams[en.TeamID].MemberIDs
		}
		for _, id := range ids {
			if a := athletes[id]; a.GroupID != "" && a.GroupID != gid {
				return []string{"存在跨组报名"}
			}
		}
	}
	return []string{}
}

func (s *Service) PreviewGroupChange(ctx context.Context, req GroupChangeRequest) (*GroupReview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.prepareGroupChange(ctx, req, true)
	return p, err
}

// ApplyGroupChange commits a confirmed preview (MCP).
func (s *Service) ApplyGroupChange(ctx context.Context, src store.Source, req GroupChangeRequest, hash string, confirmed bool) (*GroupReview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, stmts, err := s.prepareGroupChange(ctx, req, true)
	if err != nil {
		return nil, err
	}
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	if hash != p.PreviewHash {
		return nil, ErrStale
	}
	return p, s.writeGroupChange(ctx, src, req.MeetID, p, stmts)
}

// ChangeGroup moves events and combined events to a group right away (UI).
func (s *Service) ChangeGroup(ctx context.Context, src store.Source, req GroupChangeRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, stmts, err := s.prepareGroupChange(ctx, req, false)
	if err != nil {
		return err
	}
	return s.writeGroupChange(ctx, src, req.MeetID, p, stmts)
}

func (s *Service) writeGroupChange(ctx context.Context, src store.Source, meetID string, p *GroupReview, stmts []sqlite.Statement) error {
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx, store.Audit{Source: src, Action: "group.projects", MeetID: meetID, Message: "调整项目组别", Detail: p.Changes}, Change{Kind: "groups", MeetID: meetID}, stmts...)
}

func (s *Service) prepareGroupAssignments(ctx context.Context, req GroupAssignmentsRequest, review bool) (*GroupReview, []sqlite.Statement, error) {
	st, err := s.loadTeamScoreState(ctx, req.MeetID)
	if err != nil {
		return nil, nil, err
	}
	p := &GroupReview{Changes: []GroupChangeLine{}, Warnings: []string{}}
	stmts := []sqlite.Statement{}
	seen := map[string]bool{}
	if review {
		if p.Before, err = st.boards(TeamScoreFilter{MeetID: req.MeetID}, st.rules); err != nil {
			return nil, nil, err
		}
	}
	if len(req.Assignments) == 0 {
		return nil, nil, invalid("请选择归属调整")
	}
	for _, a := range req.Assignments {
		if seen[a.Kind+":"+a.ID] {
			return nil, nil, invalid("归属调整重复")
		}
		seen[a.Kind+":"+a.ID] = true
		if err := s.validateGroupIDs(ctx, req.MeetID, a.GroupIDs); err != nil {
			return nil, nil, err
		}
		found := false
		switch a.Kind {
		case "unit":
			for i, u := range st.units {
				if u.ID == a.ID {
					found = true
					p.Changes = append(p.Changes, GroupChangeLine{Key: "unit:" + u.ID, Name: u.Name, Before: groupNames(st.groups, u.GroupIDs), After: groupNames(st.groups, a.GroupIDs)})
					st.units[i].GroupIDs = append([]string{}, a.GroupIDs...)
					stmts = append(stmts, store.Stmt(`DELETE FROM unit_groups WHERE unit_id=?`, u.ID))
					for _, id := range a.GroupIDs {
						stmts = append(stmts, store.Stmt(`INSERT INTO unit_groups(unit_id,group_id) VALUES (?,?)`, u.ID, id))
					}
				}
			}
		case "athlete":
			if len(a.GroupIDs) > 1 {
				return nil, nil, invalid("运动员只能有一个默认组别")
			}
			id := ""
			if len(a.GroupIDs) == 1 {
				id = a.GroupIDs[0]
			}
			for i, v := range st.combined.athletes {
				if v.ID == a.ID {
					found = true
					p.Changes = append(p.Changes, GroupChangeLine{Key: "athlete:" + v.ID, Name: v.Name, Before: groupNames(st.groups, []string{v.GroupID}), After: groupNames(st.groups, a.GroupIDs)})
					st.combined.athletes[i].GroupID = id
					stmts = append(stmts, store.Stmt(`UPDATE athletes SET group_id=? WHERE id=?`, nullable(id), v.ID))
				}
			}
		default:
			return nil, nil, invalid("归属类型不正确")
		}
		if !found {
			return nil, nil, invalid("参赛单位或运动员不属于本赛事")
		}
	}
	p.Warnings = groupEntryWarnings(st, nil)
	return s.finishGroupReview(st, req.MeetID, req, p, stmts, review)
}
func (s *Service) PreviewGroupAssignments(ctx context.Context, req GroupAssignmentsRequest) (*GroupReview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.prepareGroupAssignments(ctx, req, true)
	return p, err
}

// ApplyGroupAssignments commits a confirmed preview (MCP).
func (s *Service) ApplyGroupAssignments(ctx context.Context, src store.Source, req GroupAssignmentsRequest, hash string, confirmed bool) (*GroupReview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, stmts, err := s.prepareGroupAssignments(ctx, req, true)
	if err != nil {
		return nil, err
	}
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	if hash != p.PreviewHash {
		return nil, ErrStale
	}
	return p, s.writeGroupAssignments(ctx, src, req.MeetID, p, stmts)
}

// AssignGroups saves unit and athlete groups right away (UI).
func (s *Service) AssignGroups(ctx context.Context, src store.Source, req GroupAssignmentsRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, stmts, err := s.prepareGroupAssignments(ctx, req, false)
	if err != nil {
		return err
	}
	return s.writeGroupAssignments(ctx, src, req.MeetID, p, stmts)
}

func (s *Service) writeGroupAssignments(ctx context.Context, src store.Source, meetID string, p *GroupReview, stmts []sqlite.Statement) error {
	return s.write(ctx, store.Audit{Source: src, Action: "group.assignments", MeetID: meetID, Message: "调整参赛组别", Detail: p.Changes}, Change{Kind: "groups", MeetID: meetID}, stmts...)
}
