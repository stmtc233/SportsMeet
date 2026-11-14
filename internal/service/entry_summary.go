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
	// then one for the units of no group when there are any.
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
	Units    int    `json:"units"`
	Athletes int    `json:"athletes"`
	Entries  int    `json:"entries"`
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
		none := 0
		for _, u := range units {
			if len(u.GroupIDs) == 0 {
				none++
			}
		}
		for _, g := range groups {
			n := 0
			for _, u := range units {
				if slices.Contains(u.GroupIDs, g.ID) {
					n++
				}
			}
			out.Groups = append(out.Groups, GroupEntries{ID: g.ID, Name: g.Name, Units: n})
		}
		if none > 0 {
			out.Groups = append(out.Groups, GroupEntries{Name: entryform.NoGroup, Units: none})
		}
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
