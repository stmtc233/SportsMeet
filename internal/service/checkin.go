package service

import (
	"context"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// CheckInTargets is what the 检录单 can print: the meet's events in the
// list's order with their rounds that have heats, and the days on which
// heats are planned.
type CheckInTargets struct {
	Days   []string       `json:"days"` // YYYY-MM-DD, ascending
	Events []CheckInEvent `json:"events"`
}

// CheckInEvent is an event with its rounds that have heats.
type CheckInEvent struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Rounds []CheckInRound `json:"rounds"`
}

// CheckInRound is a round that has heats.
type CheckInRound struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	HeatCount int    `json:"heatCount"`
	// Days are the days its heats are planned on, ascending; empty when none is.
	Days []string `json:"days"`
}

// CheckInRequest selects the 检录单's heats: those of RoundIDs (empty = every
// round), and with Day only those planned on that day, by time.
type CheckInRequest struct {
	MeetID   string   `json:"meetId"`
	RoundIDs []string `json:"roundIds"`
	Day      string   `json:"day"` // YYYY-MM-DD, "" = every heat
}

// checkInData is a meet as the 检录单 reads it, in a fixed number of queries.
type checkInData struct {
	meet   domain.Meet
	events []domain.Event // list order (domain.ScheduleOrder)
	rounds map[string][]domain.Round
	state  *eventState // athletes, units and teams; event set per event
}

func (s *Service) loadCheckIn(ctx context.Context, meetID string) (*checkInData, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}
	teams, err := s.store.ListTeams(ctx, sc)
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
	return &checkInData{meet: *meet, events: domain.ScheduleOrder(events, rounds, meet.StartDate), rounds: rounds, state: st}, nil
}

// CheckInTargets lists the rounds the 检录单 can print and the days to filter by.
func (s *Service) CheckInTargets(ctx context.Context, meetID string) (*CheckInTargets, error) {
	d, err := s.loadCheckIn(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := &CheckInTargets{Days: []string{}, Events: []CheckInEvent{}}
	all := map[string]bool{}
	for _, e := range d.events {
		ev := CheckInEvent{ID: e.ID, Name: e.Name, Rounds: []CheckInRound{}}
		for _, r := range d.rounds[e.ID] {
			if len(r.Heats) == 0 {
				continue
			}
			cr := CheckInRound{ID: r.ID, Label: r.Label(), HeatCount: len(r.Heats), Days: []string{}}
			seen := map[string]bool{}
			for _, h := range r.Heats {
				at, _ := domain.EffectiveScheduled(r, h)
				if day := domain.ScheduledDay(at); day != "" && !seen[day] {
					seen[day], all[day] = true, true
					cr.Days = append(cr.Days, day)
				}
			}
			sort.Strings(cr.Days)
			ev.Rounds = append(ev.Rounds, cr)
		}
		if len(ev.Rounds) > 0 {
			out.Events = append(out.Events, ev)
		}
	}
	for day := range all {
		out.Days = append(out.Days, day)
	}
	sort.Strings(out.Days)
	return out, nil
}

// sheetHeat is one heat a per-heat sheet prints.
type sheetHeat struct {
	event domain.Event
	round domain.Round
	heat  domain.Heat
	at    string // effective planned time
}

// sheetHeats selects a request's heats and leaves d.state on no particular
// event: in the event list's order, or with a day by planned time.
func (d *checkInData) sheetHeats(req CheckInRequest) ([]sheetHeat, error) {
	if req.Day != "" && !domain.ValidDate(req.Day) {
		return nil, invalid("日期格式不正确")
	}
	only := map[string]bool{}
	for _, id := range req.RoundIDs {
		only[id] = true
	}
	var list []sheetHeat
	for _, e := range d.events {
		for _, r := range d.rounds[e.ID] {
			if len(only) > 0 && !only[r.ID] {
				continue
			}
			for _, h := range r.Heats {
				at, _ := domain.EffectiveScheduled(r, h)
				if req.Day != "" && domain.ScheduledDay(at) != req.Day {
					continue
				}
				list = append(list, sheetHeat{e, r, h, at})
			}
		}
	}
	if req.Day != "" {
		sort.SliceStable(list, func(i, j int) bool { return list[i].at < list[j].at })
	}
	return list, nil
}

// sheetLines are a heat's competitors by position; teams list their members
// with bibs.
func (d *checkInData) sheetLines(e domain.Event, h domain.Heat) []report.Line {
	st := d.state
	st.event = e
	lines := []report.Line{}
	for _, sl := range h.Slots {
		c, ok := st.competitor(sl.CompetitorID())
		if !ok {
			continue
		}
		l := report.Line{Position: sl.Position, Bib: c.Bib, Name: c.Name, Unit: st.unitName(c.UnitID)}
		if sl.TeamID != "" {
			for i, id := range st.teams[sl.TeamID].MemberIDs {
				a := st.athletes[id]
				label := teamMemberLabel(e, i, a.Name)
				if a.Bib != "" {
					label += " " + a.Bib
				}
				l.Members = append(l.Members, label)
			}
		}
		lines = append(lines, l)
	}
	return lines
}

func rosterLabel(e domain.Event) string {
	if e.Kind == domain.KindRelay {
		return "棒次"
	}
	return "成员"
}

// RenderCheckIn returns the 检录单 as a printable HTML document: one sheet per
// heat, in the event list's order, or with a day by planned time; the
// settings say where a new page starts (sheetBreaks). The
// check-in time is the heat's effective planned time minus the settings'
// lead (domain.CheckInAt); a heat without a time prints it blank.
func (s *Service) RenderCheckIn(ctx context.Context, req CheckInRequest) (string, error) {
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	d, err := s.loadCheckIn(ctx, req.MeetID)
	if err != nil {
		return "", err
	}
	list, err := d.sheetHeats(req)
	if err != nil {
		return "", err
	}
	book := report.CheckInBook{Meet: d.meet.Name, PrintedAt: domain.RecordedText(s.store.Now(), s.zone), ShowCheckIn: prefs.CheckInLeadMinutes > 0}
	breaks := sheetBreaks(list, prefs.PrintCheckInBreak)
	for i, t := range list {
		e := t.event
		book.Sheets = append(book.Sheets, report.CheckInSheet{Break: breaks[i], Event: e.Name, Round: t.round.Label(), Heat: domain.HeatLabel(t.heat.Ordinal, t.heat.Name),
			Time: domain.ScheduledText(t.at, ""), CheckIn: domain.ScheduledText(domain.CheckInAt(t.at, prefs.CheckInLeadMinutes), domain.ScheduledDay(t.at)),
			PositionLabel: e.Rules().PositionLabel, Team: e.IsTeam(), RosterLabel: rosterLabel(e), Lines: d.sheetLines(e, t.heat)})
	}
	return report.CheckIn(book)
}
