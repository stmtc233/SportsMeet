package service

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// renderEntries reads entries rather than positions: arranging or advancing a
// competitor does not change how many times they appear in the table.
func (s *Service) renderEntries(ctx context.Context, meetID string, eventIDs []string) (string, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return "", err
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return "", err
	}
	parents, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return "", err
	}
	chosen := map[string]bool{}
	for _, id := range eventIDs {
		chosen[id] = true
	}
	selected := func(id string) bool { return len(chosen) == 0 || chosen[id] }
	// An explicit empty scope matches nothing, including unknown selections.
	sc.EventIDs = []string{}
	for _, e := range events {
		if e.CombinedID == "" && selected(e.ID) {
			sc.EventIDs = append(sc.EventIDs, e.ID)
		}
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return "", err
	}
	teams, err := s.store.ListTeams(ctx, sc)
	if err != nil {
		return "", err
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return "", err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return "", err
	}
	overviews, err := s.store.EventOverviews(ctx, meetID)
	if err != nil {
		return "", err
	}
	rounds := map[string][]domain.Round{}
	for id, overview := range overviews {
		rounds[id] = overview.TimingRounds
	}
	st := &eventState{teams: map[string]domain.Team{}, athletes: map[string]domain.Athlete{}, units: map[string]domain.Unit{}}
	for _, a := range athletes {
		st.athletes[a.ID] = a
	}
	for _, u := range units {
		st.units[u.ID] = u
	}
	for _, t := range teams {
		st.teams[t.ID] = t
	}
	byEvent := map[string][]domain.Entry{}
	for _, entry := range entries {
		byEvent[entry.EventID] = append(byEvent[entry.EventID], entry)
	}
	book := report.Book{Meet: meet.Name, Dates: domain.DateRangeText(meet.StartDate, meet.EndDate), Location: meet.Location,
		PrintedAt: domain.RecordedText(s.store.Now(), s.zone), NewPage: prefs.PrintNewPage}
	for _, e := range domain.ScheduleOrder(events, rounds, meet.StartDate) {
		if e.CombinedID != "" || !selected(e.ID) {
			continue
		}
		st.event = e
		named := printedTeamNames(e, st)
		ev := report.Event{Name: e.Name, Team: e.IsTeam(), NameLabel: competitorHeading(e, len(named) > 0)}
		for i, entry := range byEvent[e.ID] {
			a, _ := st.competitor(entry.CompetitorID())
			line := report.Line{Position: i + 1, Name: a.Name, Bib: a.Bib, Unit: st.unitName(a.UnitID)}
			if entry.TeamID != "" {
				if !named[entry.TeamID] {
					line.Name = ""
				}
				line.Members = st.memberLabels(entry.TeamID)
			}
			if entry.SeedMark != nil {
				line.Seed = scoring.FormatEventMark(e, *entry.SeedMark)
				ev.Seeded = true
			}
			ev.Entries = append(ev.Entries, line)
		}
		ev.Count = countText(len(ev.Entries), ev.Team)
		book.Events = append(book.Events, ev)
	}
	for _, p := range parents {
		if !selected(p.Combined.ID) {
			continue
		}
		ev := report.Event{Name: combinedTitle(p.Combined), NameLabel: "姓名", Count: countText(len(p.AthleteIDs), false)}
		for i, id := range p.AthleteIDs {
			a := st.athletes[id]
			ev.Entries = append(ev.Entries, report.Line{Position: i + 1, Name: a.Name, Bib: a.Bib, Unit: st.unitName(a.UnitID)})
		}
		book.Events = append(book.Events, ev)
	}
	return report.Entries(book)
}
