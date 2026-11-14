package service

import (
	"context"
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Entry limits are only pointed out: no save is refused for exceeding them
// (docs/agents/domain.md「报名限制」).

// EntriesPlan is what MCP add_entries previews: the entries and the limits
// they would exceed, shown and never blocking.
type EntriesPlan struct {
	Entries  []EntryInput `json:"entries"`
	Warnings []string     `json:"warnings"`
}

// CombinedEntriesPlan is the same for a combined's full roster.
type CombinedEntriesPlan struct {
	AthleteIDs []string `json:"athleteIds"`
	Warnings   []string `json:"warnings"`
}

// entryLimitState is a meet's limits with its entries as the limits count them.
type entryLimitState struct {
	limits  domain.EntryLimits
	entries []domain.LimitEntry
}

// loadEntryLimits reads the meet's limits and, when any is set, its entries.
func (s *Service) loadEntryLimits(ctx context.Context, meet *domain.Meet) (*entryLimitState, error) {
	st := &entryLimitState{limits: meet.EntryLimits}
	if !st.limits.Any() {
		return st, nil
	}
	var err error
	st.entries, err = s.store.LimitEntries(ctx, meet.ID)
	return st, err
}

func (st *entryLimitState) check() domain.EntryLimitReport {
	return domain.CheckEntryLimits(st.limits, st.entries)
}

// limitItem is what an event counts as: its combined for a component.
func limitItem(e domain.Event) string {
	if e.CombinedID != "" {
		return e.CombinedID
	}
	return e.ID
}

// athleteOverText says how far an athlete is over: "报 3 项，限报 2 项".
func (st *entryLimitState) athleteOverText(n int) string {
	return fmt.Sprintf("报 %d 项，限报 %d 项", n, st.limits.AthleteEvents)
}

// unitOverText says how far a unit is over in one item.
func (st *entryLimitState) unitOverText(n int, team bool) string {
	if team {
		return fmt.Sprintf("报 %d 队，限报 %d 队", n, st.limits.UnitTeams)
	}
	return fmt.Sprintf("报 %d 人，限报 %d 人", n, st.limits.UnitAthletes)
}

// focusWarnings lists, in the given order, the athletes of ids over the
// per-athlete limit, then the units of ids over the limit in item.
func (st *entryLimitState) focusWarnings(item string, team bool, ids []string, athleteName, unitName func(string) string, unitOf func(string) string) []string {
	report := st.check()
	out := []string{}
	if !team {
		for _, id := range ids {
			if n := report.Athletes[id]; n > 0 {
				out = append(out, fmt.Sprintf("「%s」%s", athleteName(id), st.athleteOverText(n)))
			}
		}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		u := unitOf(id)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if n := report.Units[domain.UnitItem{Item: item, UnitID: u}]; n > 0 {
			out = append(out, fmt.Sprintf("「%s」%s", unitName(u), st.unitOverText(n, team)))
		}
	}
	return out
}

// EntryLimitWarnings says which limits entering ids (athletes, or teams of
// a team event) into the event would exceed. It writes nothing.
func (s *Service) EntryLimitWarnings(ctx context.Context, eventID string, ids []string) ([]string, error) {
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	meet, err := s.GetMeet(ctx, st.event.MeetID)
	if err != nil {
		return nil, err
	}
	limits, err := s.loadEntryLimits(ctx, meet)
	if err != nil || !limits.limits.Any() {
		return []string{}, err
	}
	item := limitItem(st.event)
	team := st.event.IsTeam()
	known := []string{}
	for _, id := range ids {
		c, ok := st.competitor(id)
		if !ok || st.entered[id] != nil {
			continue
		}
		known = append(known, id)
		e := domain.LimitEntry{Item: item, UnitID: c.UnitID}
		if team {
			e.TeamID = id
		} else {
			e.AthleteID = id
		}
		limits.entries = append(limits.entries, e)
	}
	unitOf := func(id string) string { c, _ := st.competitor(id); return c.UnitID }
	return limits.focusWarnings(item, team, known, st.competitorName, st.unitName, unitOf), nil
}

// CombinedLimitWarnings says which limits the combined's full new roster
// would exceed, for the athletes on it. It writes nothing.
func (s *Service) CombinedLimitWarnings(ctx context.Context, combinedID string, athleteIDs []string) ([]string, error) {
	rows, err := s.store.Query(ctx, `SELECT meet_id FROM combined_events WHERE id = ?`, combinedID)
	if err != nil {
		return nil, err
	}
	if len(rows.Values) == 0 {
		return nil, invalid("全能不存在或已删除")
	}
	meet, err := s.GetMeet(ctx, rows.Values[0][0].(string))
	if err != nil {
		return nil, err
	}
	limits, err := s.loadEntryLimits(ctx, meet)
	if err != nil || !limits.limits.Any() {
		return []string{}, err
	}
	athletes, err := s.store.ListAthletes(ctx, meet.ID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meet.ID)
	if err != nil {
		return nil, err
	}
	byID := map[string]domain.Athlete{}
	for _, a := range athletes {
		byID[a.ID] = a
	}
	unitNames := unitNameMap(units)
	kept := limits.entries[:0]
	for _, e := range limits.entries {
		if e.Item != combinedID {
			kept = append(kept, e)
		}
	}
	limits.entries = kept
	known := []string{}
	for _, id := range athleteIDs {
		if a, ok := byID[id]; ok {
			known = append(known, id)
			limits.entries = append(limits.entries, domain.LimitEntry{Item: combinedID, AthleteID: id, UnitID: a.UnitID})
		}
	}
	return limits.focusWarnings(combinedID, false, known,
		func(id string) string { return byID[id].Name },
		func(id string) string { return unitNames[id] },
		func(id string) string { return byID[id].UnitID }), nil
}

// unitNameMap names units as lists show them: the short name when there is one.
func unitNameMap(units []domain.Unit) map[string]string {
	out := map[string]string{}
	for _, u := range units {
		out[u.ID] = u.Name
		if u.ShortName != "" {
			out[u.ID] = u.ShortName
		}
	}
	return out
}

// entryLimitFlags marks an event's entries that are over a limit.
func (s *Service) entryLimitFlags(ctx context.Context, meet *domain.Meet, event domain.Event, entries []EntryView) error {
	limits, err := s.loadEntryLimits(ctx, meet)
	if err != nil || !limits.limits.Any() {
		return err
	}
	report := limits.check()
	item := limitItem(event)
	for i := range entries {
		e := &entries[i]
		if e.TeamID == "" {
			e.OverEvents = report.Athletes[e.AthleteID] > 0
		}
		e.OverUnit = e.UnitID != "" && report.Units[domain.UnitItem{Item: item, UnitID: e.UnitID}] > 0
	}
	return nil
}

// overLimitAthletes returns the athletes of a meet over the per-athlete limit.
func (s *Service) overLimitAthletes(ctx context.Context, meetID string) (map[string]int, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	limits, err := s.loadEntryLimits(ctx, meet)
	if err != nil || limits.limits.AthleteEvents == 0 {
		return map[string]int{}, err
	}
	return limits.check().Athletes, nil
}
