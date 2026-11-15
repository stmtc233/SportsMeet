package service

import (
	"context"
	"slices"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// GroupFollow is how many athletes a change of their unit's groups moves
// into a group ("" = 未分组).
type GroupFollow struct {
	GroupID  string `json:"groupId"`
	Athletes int    `json:"athletes"`
}

// groupFollow is what keeping athletes in their unit's groups adds to a
// write (domain.FollowUnitGroups).
type groupFollow struct {
	stmts   []sqlite.Statement
	lines   []GroupChangeLine
	follows []GroupFollow // athletes moved that the write did not change itself
}

// followGroups applies domain.FollowUnitGroups to units and athletes, the
// state the write leaves, in place. Statements go after the write's own.
func followGroups(groups []domain.CompetitionGroup, units []domain.Unit, athletes []domain.Athlete, changedUnits, changedAthletes map[string]bool) groupFollow {
	adds, moves := domain.FollowUnitGroups(units, athletes, changedUnits, changedAthletes)
	out := groupFollow{stmts: []sqlite.Statement{}, lines: []GroupChangeLine{}, follows: []GroupFollow{}}
	for _, a := range adds {
		for i := range units {
			if u := &units[i]; u.ID == a.UnitID {
				before := groupNames(groups, u.GroupIDs)
				u.GroupIDs = append(slices.Clone(u.GroupIDs), a.GroupID)
				out.lines = append(out.lines, GroupChangeLine{Key: "unit:" + u.ID, Name: u.Name, Before: before, After: groupNames(groups, u.GroupIDs)})
			}
		}
		out.stmts = append(out.stmts, store.Stmt(`INSERT OR IGNORE INTO unit_groups(unit_id,group_id) VALUES (?,?)`, a.UnitID, a.GroupID))
	}
	count := map[string]int{}
	for _, m := range moves {
		for i := range athletes {
			if a := &athletes[i]; a.ID == m.AthleteID {
				a.GroupID = m.To
				out.lines = append(out.lines, GroupChangeLine{Key: "athlete:" + a.ID, Name: a.Name, Before: groupNames(groups, []string{m.From}), After: groupNames(groups, []string{m.To})})
			}
		}
		out.stmts = append(out.stmts, store.Stmt(`UPDATE athletes SET group_id=? WHERE id=?`, nullable(m.To), m.AthleteID))
		if !changedAthletes[m.AthleteID] {
			count[m.To]++
		}
	}
	for _, g := range groups {
		if n := count[g.ID]; n > 0 {
			out.follows = append(out.follows, GroupFollow{GroupID: g.ID, Athletes: n})
		}
	}
	if n := count[""]; n > 0 {
		out.follows = append(out.follows, GroupFollow{GroupID: "", Athletes: n})
	}
	return out
}

// mergeLines adds lines to changes; a line for what changes already lists
// replaces its After.
func mergeLines(changes, lines []GroupChangeLine) []GroupChangeLine {
	for _, l := range lines {
		if i := slices.IndexFunc(changes, func(c GroupChangeLine) bool { return c.Key == l.Key }); i >= 0 {
			changes[i].After = l.After
		} else {
			changes = append(changes, l)
		}
	}
	return changes
}

// unitsWith is the units with u in place of the one of its ID, or added.
func unitsWith(units []domain.Unit, u domain.Unit) []domain.Unit {
	out := slices.Clone(units)
	if i := slices.IndexFunc(out, func(o domain.Unit) bool { return o.ID == u.ID }); i >= 0 {
		out[i] = u
		return out
	}
	return append(out, u)
}

// athletesWith is the athletes with a in place of the one of its ID, or added.
func athletesWith(athletes []domain.Athlete, a domain.Athlete) []domain.Athlete {
	out := slices.Clone(athletes)
	if i := slices.IndexFunc(out, func(o domain.Athlete) bool { return o.ID == a.ID }); i >= 0 {
		out[i] = a
		return out
	}
	return append(out, a)
}

// GroupFollowRequest asks what saving assignments does to the athletes;
// NewGroup is the ID the assignments use for a group not saved yet.
type GroupFollowRequest struct {
	MeetID      string            `json:"meetId"`
	Assignments []GroupAssignment `json:"assignments"`
	NewGroup    string            `json:"newGroup,omitempty"`
}

// GroupAssignmentFollows is how many athletes saving the assignments moves
// into each group with their unit, for the dialogs to say before saving.
// Nothing is checked or written.
func (s *Service) GroupAssignmentFollows(ctx context.Context, req GroupFollowRequest) ([]GroupFollow, error) {
	units, err := s.store.ListUnits(ctx, req.MeetID)
	if err != nil {
		return nil, err
	}
	athletes, err := s.store.ListAthletes(ctx, req.MeetID)
	if err != nil {
		return nil, err
	}
	groups, err := s.store.ListGroups(ctx, req.MeetID)
	if err != nil {
		return nil, err
	}
	if req.NewGroup != "" {
		groups = append(groups, domain.CompetitionGroup{ID: req.NewGroup})
	}
	changedUnits, changedAthletes := map[string]bool{}, map[string]bool{}
	for _, a := range req.Assignments {
		switch a.Kind {
		case "unit":
			for i := range units {
				if units[i].ID == a.ID && !sameGroups(units[i].GroupIDs, a.GroupIDs) {
					units[i].GroupIDs = slices.Clone(a.GroupIDs)
					changedUnits[a.ID] = true
				}
			}
		case "athlete":
			for i := range athletes {
				if athletes[i].ID == a.ID {
					athletes[i].GroupID = ""
					if len(a.GroupIDs) > 0 {
						athletes[i].GroupID = a.GroupIDs[0]
					}
					changedAthletes[a.ID] = true
				}
			}
		}
	}
	return followGroups(groups, units, athletes, changedUnits, changedAthletes).follows, nil
}
