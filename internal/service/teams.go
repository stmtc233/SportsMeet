package service

import (
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// TeamInput is a complete team definition. Empty MemberIDs means an undecided lineup.
type TeamInput struct {
	Name      string   `json:"name"`
	UnitID    string   `json:"unitId,omitempty"`
	MemberIDs []string `json:"memberIds"`
}

type TeamView struct {
	GroupMismatch bool   `json:"groupMismatch"`
	SizeWarning   string `json:"sizeWarning"`
	domain.Team
	UnitName       string   `json:"unitName"`
	MemberNames    []string `json:"memberNames"`
	Entered        bool     `json:"entered"`
	Placed         bool     `json:"placed"`
	KeepReason     string   `json:"keepReason"`
	GenderMismatch bool     `json:"genderMismatch"`
}

func (st *eventState) teamViews() []TeamView {
	out := make([]TeamView, 0, len(st.teamList))
	for _, t := range st.teamList {
		v := TeamView{GroupMismatch: st.groupMismatch(t.ID), SizeWarning: st.event.TeamSizeWarning(len(t.MemberIDs)), Team: t, UnitName: st.unitName(t.UnitID), MemberNames: []string{}, Entered: st.entered[t.ID] != nil}
		for _, id := range t.MemberIDs {
			a := st.athletes[id]
			v.MemberNames = append(v.MemberNames, a.Name)
			v.GenderMismatch = v.GenderMismatch || (a.Gender != "" && st.event.Gender != domain.GenderMixed && a.Gender != st.event.Gender)
		}
		for _, r := range st.rounds {
			for _, h := range r.Heats {
				for _, sl := range h.Slots {
					if sl.TeamID == t.ID {
						v.Placed = true
						if h.Facts().KeepReason() != "" {
							v.KeepReason = h.Facts().KeepReason()
						}
					}
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// competitor returns display/arrangement data for the event's athlete or team.
type competitor struct {
	ID, Name, UnitID, Bib string
	Gender                domain.Gender
}

func (st *eventState) competitor(id string) (competitor, bool) {
	if !st.event.IsTeam() {
		a, ok := st.athletes[id]
		return competitor{ID: a.ID, Name: a.Name, UnitID: a.UnitID, Bib: a.Bib, Gender: a.Gender}, ok
	}
	t, ok := st.teams[id]
	return competitor{ID: t.ID, Name: t.Name, UnitID: t.UnitID}, ok
}
func (st *eventState) competitorName(id string) string { a, _ := st.competitor(id); return a.Name }
func (st *eventState) slot(pos int, id string, r *domain.Result) domain.Slot {
	sl := domain.Slot{Position: pos, Result: r}
	if st.event.IsTeam() {
		sl.TeamID = id
	} else {
		sl.AthleteID = id
	}
	return sl
}
func insertSlot(heatID string, sl domain.Slot) sqlite.Statement {
	return store.Stmt(`INSERT INTO slots(heat_id,position,athlete_id,team_id) VALUES (?,?,?,?)`, heatID, sl.Position, nullable(sl.AthleteID), nullable(sl.TeamID))
}
func insertEntry(eventID, athleteID, teamID string, seed any) sqlite.Statement {
	return store.Stmt(`INSERT INTO entries(event_id,athlete_id,team_id,seed_mark) VALUES (?,?,?,?)`, eventID, nullable(athleteID), nullable(teamID), seed)
}
func insertTeam(t domain.Team) []sqlite.Statement {
	out := []sqlite.Statement{store.Stmt(`INSERT INTO teams(id,event_id,name,unit_id) VALUES (?,?,?,?)`, t.ID, t.EventID, t.Name, nullable(t.UnitID))}
	for i, id := range t.MemberIDs {
		out = append(out, store.Stmt(`INSERT INTO team_members(team_id,leg,athlete_id) VALUES (?,?,?)`, t.ID, i+1, id))
	}
	return out
}

// SaveTeam edits a relay roster. Any scored heat locks its lineup, including unscored teams in that heat.
func (s *Service) SaveTeam(ctx context.Context, src store.Source, eventID, id string, in TeamInput) (*domain.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if !st.event.IsTeam() {
		return nil, invalid("请在队伍项目里添加队伍")
	}
	t := domain.Team{ID: id, EventID: eventID, Name: strings.TrimSpace(in.Name), UnitID: in.UnitID, MemberIDs: append([]string{}, in.MemberIDs...)}
	if t.Name == "" {
		return nil, invalid("请填写队名")
	}
	if t.UnitID != "" {
		if _, ok := st.units[t.UnitID]; !ok {
			return nil, invalid("参赛单位不属于本赛事")
		}
	}
	var old *domain.Team
	for _, o := range st.teamList {
		if o.ID == id {
			old = &o
			continue
		}
		if o.UnitID == t.UnitID && domain.NormalizeText(o.Name) == domain.NormalizeText(t.Name) {
			return nil, invalid("已有同名队伍")
		}
		for _, m := range t.MemberIDs {
			if slices.Contains(o.MemberIDs, m) {
				return nil, invalid("「%s」已在本项目其他队伍", st.athletes[m].Name)
			}
		}
	}
	if id != "" && old == nil {
		return nil, invalid("队伍不存在或已删除")
	}
	seen := map[string]bool{}
	for _, m := range t.MemberIDs {
		a, ok := st.athletes[m]
		if !ok {
			return nil, invalid("运动员不属于本赛事")
		}
		if seen[m] {
			if st.event.Kind == domain.KindRelay {
				return nil, invalid("「%s」重复担任棒次", a.Name)
			}
			return nil, invalid("「%s」在队内重复", a.Name)
		}
		seen[m] = true
	}
	if old != nil && !slices.Equal(old.MemberIDs, t.MemberIDs) {
		for _, v := range st.teamViews() {
			if v.ID == id && v.KeepReason != "" {
				if st.event.Kind == domain.KindRelay {
					return nil, invalid("队伍已有比赛记录，不能改棒次")
				}
				return nil, invalid("已有比赛记录，不能改名单")
			}
		}
	}
	action := "team.created"
	var stmts []sqlite.Statement
	if old == nil {
		t.ID = newID()
		stmts = insertTeam(t)
	} else {
		action = "team.updated"
		stmts = []sqlite.Statement{store.Stmt(`UPDATE teams SET name=?,unit_id=? WHERE id=?`, t.Name, nullable(t.UnitID), id), store.Stmt(`DELETE FROM team_members WHERE team_id=?`, id)}
		stmts = append(stmts, insertTeam(t)[1:]...)
	}
	err = s.write(ctx, store.Audit{Source: src, Action: action, MeetID: st.event.MeetID, EventID: eventID, Message: t.Name, Detail: map[string]any{"before": old, "after": t}}, Change{Kind: "teams", MeetID: st.event.MeetID, EventID: eventID}, stmts...)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTeam requires all placements to be removed first; results cannot be orphaned.
func (s *Service) DeleteTeam(ctx context.Context, src store.Source, eventID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	t, ok := st.teams[id]
	if !ok {
		return invalid("队伍不存在或已删除")
	}
	for _, v := range st.teamViews() {
		if v.ID == id && v.Placed {
			return invalid("队伍已排进组，请先移出")
		}
	}
	return s.write(ctx, store.Audit{Source: src, Action: "team.deleted", MeetID: st.event.MeetID, EventID: eventID, Message: t.Name, Detail: t}, Change{Kind: "teams", MeetID: st.event.MeetID, EventID: eventID}, store.Stmt(`DELETE FROM teams WHERE id=?`, id))
}

func (st *eventState) competitorLabel() string {
	if st.event.IsTeam() {
		return "队伍"
	}
	return "运动员"
}

// TeamWarnings validates the draft roster using the same event and athlete data as saves.
func (s *Service) TeamWarnings(ctx context.Context, eventID string, memberIDs []string) ([]string, error) {
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if warning := st.event.TeamSizeWarning(len(memberIDs)); warning != "" {
		warnings = append(warnings, warning)
	}
	groupMismatch := false
	mismatch := false
	for _, id := range memberIDs {
		a, ok := st.athletes[id]
		if !ok {
			return nil, invalid("运动员不属于本赛事")
		}
		groupMismatch = groupMismatch || (st.event.GroupID != "" && a.GroupID != "" && st.event.GroupID != a.GroupID)
		mismatch = mismatch || (a.Gender != "" && st.event.Gender != domain.GenderMixed && a.Gender != st.event.Gender)
	}
	if groupMismatch {
		warnings = append(warnings, "成员组别与项目不符")
	}
	if mismatch {
		warnings = append(warnings, "成员性别与项目不符")
	}
	return warnings, nil
}
