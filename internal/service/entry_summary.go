package service

import (
	"context"
	"slices"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/entryform"
)

// EntrySummary is how far each unit, and each group, has entered: what the
// 报名 page lists to see whose 报名登记表 is still out.
type EntrySummary struct {
	Units []UnitEntries `json:"units"`
	// Groups is empty when the meet has no group; else one row per group,
	// then one for no group when some unit or athlete has none.
	Groups []GroupEntries `json:"groups"`
	// Limited says some 报名限制 is set, so Over means something.
	Limited bool `json:"limited"`
}

// UnitEntries is one unit's athletes, entries and teams.
type UnitEntries struct {
	Unit     domain.Unit `json:"unit"`
	Athletes int         `json:"athletes"`
	// Entries counts individual events and combineds entered, a combined
	// once, the way 报名限制 count them.
	Entries int `json:"entries"`
	Teams   int `json:"teams"` // teams entered
	// Over is how many of the unit's athletes, and of its events, are over
	// a limit.
	Over int `json:"over"`
}

// GroupEntries is one group's units and athletes; ID "" = the units and
// athletes of no group.
type GroupEntries struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Units    int    `json:"units"` // units of the group; of no group for ""
	Athletes int    `json:"athletes"`
	Entries  int    `json:"entries"`
	// Members are the group's units in unit order, each with its athletes
	// of this group: the 报名 page's tree and the 总览 structure. For "" they
	// are the units of no group, then the other units that have athletes of
	// no group, so every athlete shows under some unit.
	Members []GroupMember `json:"members"`
}

// GroupMember is a unit under a group and how many of its athletes are in
// that group.
type GroupMember struct {
	UnitID   string `json:"unitId"`
	Athletes int    `json:"athletes"`
}

// EntrySummary counts the meet's athletes, entries and teams by unit and by
// group, in three queries whatever the meet's size.
func (s *Service) EntrySummary(ctx context.Context, meetID string) (*EntrySummary, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, err
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.LimitEntries(ctx, meetID)
	if err != nil {
		return nil, err
	}
	groupOf := map[string]string{}
	unitOf := map[string]string{}
	byUnit := map[string]*UnitEntries{}
	byGroup := map[string]*GroupEntries{}
	out := &EntrySummary{Units: make([]UnitEntries, len(units)), Groups: []GroupEntries{}, Limited: meet.EntryLimits.Any()}
	for i, u := range units {
		out.Units[i] = UnitEntries{Unit: u}
		byUnit[u.ID] = &out.Units[i]
	}
	if len(groups) > 0 {
		out.Groups = groupMembers(groups, units, athletes)
		for i := range out.Groups {
			byGroup[out.Groups[i].ID] = &out.Groups[i]
		}
	}
	for _, a := range athletes {
		groupOf[a.ID], unitOf[a.ID] = a.GroupID, a.UnitID
		if u := byUnit[a.UnitID]; u != nil {
			u.Athletes++
		}
		if g := byGroup[a.GroupID]; g != nil {
			g.Athletes++
		}
	}
	seen := map[domain.LimitEntry]bool{}
	teams := map[string]bool{}
	for _, e := range entries {
		key := domain.LimitEntry{Item: e.Item, AthleteID: e.AthleteID, TeamID: e.TeamID}
		if seen[key] {
			continue
		}
		seen[key] = true
		u := byUnit[e.UnitID]
		if e.TeamID != "" {
			if u != nil && !teams[e.TeamID] {
				u.Teams++
			}
			teams[e.TeamID] = true
			continue
		}
		if u != nil {
			u.Entries++
		}
		if g := byGroup[groupOf[e.AthleteID]]; g != nil {
			g.Entries++
		}
	}
	report := domain.CheckEntryLimits(meet.EntryLimits, entries)
	for id := range report.Athletes {
		if u := byUnit[unitOf[id]]; u != nil {
			u.Over++
		}
	}
	for k := range report.Units {
		if u := byUnit[k.UnitID]; u != nil {
			u.Over++
		}
	}
	return out, nil
}

// groupMembers lays out each group's units with their athletes of that
// group, then the row of no group when a unit or an athlete has none.
func groupMembers(groups []domain.CompetitionGroup, units []domain.Unit, athletes []domain.Athlete) []GroupEntries {
	order := map[string]int{}
	for i, u := range units {
		order[u.ID] = i
	}
	in := map[[2]string]int{} // group, unit → athletes of the unit in the group
	loose := false
	for _, a := range athletes {
		in[[2]string{a.GroupID, a.UnitID}]++
		loose = loose || a.GroupID == ""
	}
	var out []GroupEntries
	var none []GroupMember
	for _, u := range units {
		if len(u.GroupIDs) == 0 {
			none = append(none, GroupMember{UnitID: u.ID, Athletes: in[[2]string{"", u.ID}]})
		}
	}
	for _, g := range groups {
		row := GroupEntries{ID: g.ID, Name: g.Name, Members: []GroupMember{}}
		for _, u := range units {
			if slices.Contains(u.GroupIDs, g.ID) {
				row.Members = append(row.Members, GroupMember{UnitID: u.ID, Athletes: in[[2]string{g.ID, u.ID}]})
			}
		}
		row.Units = len(row.Members)
		out = append(out, row)
	}
	if len(none) == 0 && !loose {
		return out
	}
	row := GroupEntries{Name: entryform.NoGroup, Units: len(none), Members: none}
	for _, u := range units {
		if n := in[[2]string{"", u.ID}]; n > 0 && len(u.GroupIDs) > 0 {
			row.Members = append(row.Members, GroupMember{UnitID: u.ID, Athletes: n})
		}
	}
	slices.SortStableFunc(row.Members, func(a, b GroupMember) int { return order[a.UnitID] - order[b.UnitID] })
	if row.Members == nil {
		row.Members = []GroupMember{}
	}
	return append(out, row)
}
