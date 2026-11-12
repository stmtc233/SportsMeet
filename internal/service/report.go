package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Report kinds.
const (
	ReportStartList = "startlist" // 秩序册
	ReportResults   = "results"   // 成绩册
)

// RenderReport returns a printable HTML document for the meet. eventIDs
// limits it to some events (in the meet's order); empty means all.
func (s *Service) RenderReport(ctx context.Context, kind, meetID string, eventIDs []string) (string, error) {
	book, err := s.reportBook(ctx, meetID, eventIDs)
	if err != nil {
		return "", err
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	book.NewPage = prefs.PrintNewPage
	switch kind {
	case ReportStartList:
		return report.StartList(*book)
	case ReportResults:
		return report.Results(*book)
	}
	return "", invalid("报表类型不正确")
}

func (s *Service) reportBook(ctx context.Context, meetID string, eventIDs []string) (*report.Book, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}
	st := &eventState{athletes: map[string]domain.Athlete{}, units: map[string]domain.Unit{}}
	for _, a := range athletes {
		st.athletes[a.ID] = a
	}
	for _, u := range units {
		st.units[u.ID] = u
	}
	seeds := map[string]*int64{}
	for _, e := range entries {
		seeds[e.EventID+"|"+e.AthleteID] = e.SeedMark
	}
	only := map[string]bool{}
	for _, id := range eventIDs {
		only[id] = true
	}

	book := &report.Book{Meet: meet.Name, Dates: meet.StartDate, Location: meet.Location}
	if meet.EndDate != "" && meet.EndDate != meet.StartDate {
		book.Dates += " – " + meet.EndDate
	}
	for _, e := range events {
		if len(only) > 0 && !only[e.ID] {
			continue
		}
		date := e.Date
		if date == "" {
			date = meet.StartDate
		}
		ev := report.Event{Name: e.Name, Date: date, Timed: e.Kind.Timed()}
		line := func(sl domain.Slot) report.Line {
			a := st.athletes[sl.AthleteID]
			l := report.Line{Position: sl.Position, Bib: a.Bib, Name: a.Name, Unit: st.unitName(a)}
			if seed := seeds[e.ID+"|"+a.ID]; seed != nil {
				l.Seed = scoring.FormatMark(e.Kind, *seed)
			}
			if r := sl.Result; r != nil {
				l.Mark, l.Qualify = r.Status.Label(), r.Qualify
				if r.Status == domain.StatusOK && r.Mark != nil {
					l.Mark = scoring.FormatMark(e.Kind, *r.Mark)
				}
				if r.Rank != nil {
					l.Rank = strconv.Itoa(*r.Rank)
				}
			}
			return l
		}
		for _, r := range rounds[e.ID] {
			rr := report.Round{Label: r.Label(), ByHeat: r.Advance != nil}
			if r.Advance != nil {
				rr.Advance = advanceText(*r.Advance)
			}
			bySlot := map[string]report.Line{}
			for _, h := range r.Heats {
				heat := report.Heat{Label: domain.HeatLabel(h.Ordinal, h.Name), Time: heatTime(h.ScheduledAt, date)}
				for _, sl := range h.Slots {
					l := line(sl)
					heat.Lines = append(heat.Lines, l)
					l.Heat = heat.Label
					bySlot[sl.AthleteID] = l
				}
				rr.Heats = append(rr.Heats, heat)
			}
			list, _ := standings(e.Kind, r)
			for _, row := range list {
				l := bySlot[row.AthleteID]
				l.Rank = ""
				if row.Rank != nil {
					l.Rank = strconv.Itoa(*row.Rank)
				}
				if len(r.Heats) < 2 {
					l.Heat = ""
				}
				rr.Standings = append(rr.Standings, l)
			}
			ev.Rounds = append(ev.Rounds, rr)
		}
		book.Events = append(book.Events, ev)
	}
	return book, nil
}

func advanceText(a domain.Advance) string {
	var parts []string
	if a.ByPlace > 0 {
		parts = append(parts, fmt.Sprintf("各组前 %d 名", a.ByPlace))
	}
	if a.ByMark > 0 {
		parts = append(parts, fmt.Sprintf("其余成绩最好的 %d 名", a.ByMark))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "与") + "晋级"
}

// heatTime shortens a scheduled time to HH:mm when it is on the event's
// day.
func heatTime(scheduledAt, eventDate string) string {
	day, clock, ok := strings.Cut(scheduledAt, "T")
	if !ok {
		return ""
	}
	if day == eventDate {
		return clock
	}
	return day + " " + clock
}
