package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// meetPlan is one meet as the timetable, the conflict check and
// multi-round scheduling read it, loaded in a fixed number of queries
// (docs/agents/storage.md「查询」).
type meetPlan struct {
	meet     domain.Meet
	events   []domain.Event // schedule order (domain.ScheduleOrder)
	eventBy  map[string]domain.Event
	rounds   map[string][]domain.Round
	teams    map[string]domain.Team
	athletes map[string]domain.Athlete
	minRest  int
}

func newMeetPlan(meet domain.Meet, events []domain.Event, rounds map[string][]domain.Round, teams []domain.Team, athletes []domain.Athlete, minRest int) *meetPlan {
	p := &meetPlan{meet: meet, rounds: rounds, minRest: minRest, eventBy: map[string]domain.Event{},
		teams: map[string]domain.Team{}, athletes: map[string]domain.Athlete{}}
	p.events = domain.ScheduleOrder(events, rounds, meet.StartDate)
	for _, e := range events {
		p.eventBy[e.ID] = e
	}
	for _, t := range teams {
		p.teams[t.ID] = t
	}
	for _, a := range athletes {
		p.athletes[a.ID] = a
	}
	return p
}

func (s *Service) loadMeetPlan(ctx context.Context, meetID string) (*meetPlan, error) {
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
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return nil, err
	}
	return newMeetPlan(*meet, events, rounds, teams, athletes, prefs.MinRestMinutes), nil
}

// meetOfEvent is the meet an event belongs to.
func (s *Service) meetOfEvent(ctx context.Context, eventID string) (string, error) {
	events, err := s.store.ListEvents(ctx, store.Scope{EventID: eventID})
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", invalid("项目不存在或已删除")
	}
	return events[0].MeetID, nil
}

// heatPlace is where a heat sits in the meet.
type heatPlace struct {
	event domain.Event
	round domain.Round
	heat  domain.Heat
}

// planned is a heat's time and duration as a plan would leave them.
type planned struct {
	at      string
	minutes *int
}

// heatsByID indexes every heat of the meet.
func (p *meetPlan) heatsByID() map[string]heatPlace {
	out := map[string]heatPlace{}
	for _, e := range p.events {
		for _, r := range p.rounds[e.ID] {
			for _, h := range r.Heats {
				out[h.ID] = heatPlace{e, r, h}
			}
		}
	}
	return out
}

// people lists who competes in a heat: its athletes, a team's members.
func (p *meetPlan) people(h domain.Heat) []string {
	var out []string
	for _, sl := range h.Slots {
		if sl.TeamID != "" {
			out = append(out, p.teams[sl.TeamID].MemberIDs...)
		} else if sl.AthleteID != "" {
			out = append(out, sl.AthleteID)
		}
	}
	return out
}

// conflicts runs the check over the whole meet; changed overrides some
// heats' own time and duration (a schedule preview).
func (p *meetPlan) conflicts(changed map[string]planned) []arrange.Conflict {
	var heats []arrange.TimedHeat
	for _, e := range p.events {
		for _, r := range p.rounds[e.ID] {
			for _, h := range r.Heats {
				if c, ok := changed[h.ID]; ok {
					h.ScheduledAt, h.DurationMinutes = c.at, c.minutes
				}
				at, _ := domain.EffectiveScheduled(r, h)
				heats = append(heats, arrange.TimedHeat{HeatID: h.ID, Start: at, Minutes: h.DurationMinutes, Athletes: p.people(h)})
			}
		}
	}
	return arrange.Conflicts(heats, p.minRest)
}

// ScheduleHeatRef names a heat for a conflict line.
type ScheduleHeatRef struct {
	EventID    string `json:"eventId"`
	EventName  string `json:"eventName"`
	RoundID    string `json:"roundId"`
	RoundLabel string `json:"roundLabel"`
	HeatID     string `json:"heatId"`
	HeatLabel  string `json:"heatLabel"`
	// TimeText is the heat's effective time with its date.
	TimeText string `json:"timeText"`
}

// ConflictView is one athlete in two heats too close together.
type ConflictView struct {
	AthleteID   string          `json:"athleteId"`
	AthleteName string          `json:"athleteName"`
	Overlap     bool            `json:"overlap"`
	GapMinutes  int             `json:"gapMinutes"` // negative when the heats overlap
	Text        string          `json:"text"`       // 时间重叠 / 间隔 5 分钟
	First       ScheduleHeatRef `json:"first"`
	Second      ScheduleHeatRef `json:"second"`
}

// HeatConflict is a conflict as one of its heats shows it.
type HeatConflict struct {
	AthleteID   string          `json:"athleteId"`
	AthleteName string          `json:"athleteName"`
	Text        string          `json:"text"`
	Other       ScheduleHeatRef `json:"other"`
}

// HeatConflicts lists one heat's conflicts.
type HeatConflicts struct {
	HeatID    string         `json:"heatId"`
	Conflicts []HeatConflict `json:"conflicts"`
}

func conflictText(c arrange.Conflict) string {
	if c.Overlap {
		return "时间重叠"
	}
	return fmt.Sprintf("间隔 %d 分钟", c.GapMinutes)
}

func (p *meetPlan) ref(place heatPlace, changed map[string]planned) ScheduleHeatRef {
	h := place.heat
	if c, ok := changed[h.ID]; ok {
		h.ScheduledAt = c.at
	}
	at, _ := domain.EffectiveScheduled(place.round, h)
	return ScheduleHeatRef{EventID: place.event.ID, EventName: place.event.Name, RoundID: place.round.ID, RoundLabel: place.round.Label(),
		HeatID: h.ID, HeatLabel: domain.HeatLabel(h.Ordinal, h.Name), TimeText: domain.ScheduledText(at, "")}
}

func (p *meetPlan) conflictViews(list []arrange.Conflict, changed map[string]planned) []ConflictView {
	heats := p.heatsByID()
	out := make([]ConflictView, 0, len(list))
	for _, c := range list {
		out = append(out, ConflictView{AthleteID: c.AthleteID, AthleteName: p.athletes[c.AthleteID].Name, Overlap: c.Overlap,
			GapMinutes: c.GapMinutes, Text: conflictText(c),
			First: p.ref(heats[c.First], changed), Second: p.ref(heats[c.Second], changed)})
	}
	return out
}

// byHeat groups conflict lines under each heat they involve.
func byHeat(list []ConflictView) map[string][]HeatConflict {
	out := map[string][]HeatConflict{}
	for _, c := range list {
		out[c.First.HeatID] = append(out[c.First.HeatID], HeatConflict{AthleteID: c.AthleteID, AthleteName: c.AthleteName, Text: c.Text, Other: c.Second})
		out[c.Second.HeatID] = append(out[c.Second.HeatID], HeatConflict{AthleteID: c.AthleteID, AthleteName: c.AthleteName, Text: c.Text, Other: c.First})
	}
	return out
}

// ConflictList is a meet's athlete time conflicts and the minimum rest
// they were checked with.
type ConflictList struct {
	MinRestMinutes int            `json:"minRestMinutes"`
	Conflicts      []ConflictView `json:"conflicts"`
}

// MeetConflicts lists the meet's athlete time conflicts with the minimum
// rest from the settings. They are warnings only: no save is blocked.
func (s *Service) MeetConflicts(ctx context.Context, meetID string) (*ConflictList, error) {
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	return &ConflictList{MinRestMinutes: p.minRest, Conflicts: p.conflictViews(p.conflicts(nil), nil)}, nil
}

// EventConflicts lists the conflicts of an event's heats, for the arrange
// page's heat cards.
func (s *Service) EventConflicts(ctx context.Context, eventID string) ([]HeatConflicts, error) {
	meetID, err := s.meetOfEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	grouped := byHeat(p.conflictViews(p.conflicts(nil), nil))
	out := []HeatConflicts{}
	for _, r := range p.rounds[eventID] {
		for _, h := range r.Heats {
			if list := grouped[h.ID]; len(list) > 0 {
				out = append(out, HeatConflicts{HeatID: h.ID, Conflicts: list})
			}
		}
	}
	return out, nil
}

// Timetable is a meet's competition schedule: its rounds by day and
// effective time, those with no time at all apart, and every conflict.
type Timetable struct {
	MinRestMinutes int              `json:"minRestMinutes"`
	Days           []TimetableDay   `json:"days"`
	Unscheduled    []TimetableRound `json:"unscheduled"`
	Conflicts      []ConflictView   `json:"conflicts"`
}

// TimetableDay is one day's rounds, by their first time.
type TimetableDay struct {
	Day    string           `json:"day"` // YYYY-MM-DD
	Rounds []TimetableRound `json:"rounds"`
}

// TimetableRound is a round in the schedule with its heats in order.
type TimetableRound struct {
	EventID    string `json:"eventId"`
	EventName  string `json:"eventName"`
	RoundID    string `json:"roundId"`
	RoundLabel string `json:"roundLabel"`
	// Day is the day the round is listed on ("" when unscheduled); texts
	// are relative to it.
	Day string `json:"day"`
	// ScheduledAt is the round's start, "" = unset; ScheduledText shows it.
	ScheduledAt   string `json:"scheduledAt"`
	ScheduledText string `json:"scheduledText"`
	// StartAt is the earliest planned time of the round (its start or a
	// heat's own time); StartText shows it.
	StartAt   string          `json:"startAt"`
	StartText string          `json:"startText"`
	HeatCount int             `json:"heatCount"`
	People    int             `json:"people"` // athletes placed; a team counts its members
	Heats     []TimetableHeat `json:"heats"`
}

// TimetableHeat is one heat of the schedule.
type TimetableHeat struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Name            string `json:"name"`
	Base            string `json:"base"` // fingerprint, for editing it from the schedule
	ScheduledAt     string `json:"scheduledAt"`
	DurationMinutes *int   `json:"durationMinutes"`
	HeatTimes
	EndText    string         `json:"endText"` // EndAt as the times are shown; "" = none
	KeepReason string         `json:"keepReason"`
	People     int            `json:"people"`
	Conflicts  []HeatConflict `json:"conflicts"`
}

// Schedule returns the meet's timetable. It reads the meet once, so its
// queries do not grow with the heats.
func (s *Service) Schedule(ctx context.Context, meetID string) (*Timetable, error) {
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	t := p.timetable(nil)
	t.Conflicts = p.conflictViews(p.conflicts(nil), nil)
	grouped := byHeat(t.Conflicts)
	attach := func(list []TimetableRound) {
		for i := range list {
			for j := range list[i].Heats {
				if c := grouped[list[i].Heats[j].ID]; c != nil {
					list[i].Heats[j].Conflicts = c
				}
			}
		}
	}
	for i := range t.Days {
		attach(t.Days[i].Rounds)
	}
	attach(t.Unscheduled)
	return t, nil
}

// timetable lays the meet's rounds out by day; only, when not empty,
// keeps some events (the printed 秩序册's selection).
func (p *meetPlan) timetable(only map[string]bool) *Timetable {
	t := &Timetable{MinRestMinutes: p.minRest, Days: []TimetableDay{}, Unscheduled: []TimetableRound{}, Conflicts: []ConflictView{}}
	type placed struct {
		round TimetableRound
		order int
	}
	var timed []placed
	for order, e := range p.events {
		if len(only) > 0 && !only[e.ID] {
			continue
		}
		for _, r := range p.rounds[e.ID] {
			start := domain.EarliestScheduled([]domain.Round{r})
			day := domain.ScheduledDay(start)
			tr := TimetableRound{EventID: e.ID, EventName: e.Name, RoundID: r.ID, RoundLabel: r.Label(), Day: day,
				ScheduledAt: r.ScheduledAt, ScheduledText: domain.ScheduledText(r.ScheduledAt, day),
				StartAt: start, StartText: domain.ScheduledText(start, day), HeatCount: len(r.Heats), Heats: []TimetableHeat{}}
			for _, h := range r.Heats {
				times := HeatTimesOf(r.ScheduledAt, h.ScheduledAt, h.DurationMinutes, day)
				people := len(p.people(h))
				tr.People += people
				tr.Heats = append(tr.Heats, TimetableHeat{ID: h.ID, Label: domain.HeatLabel(h.Ordinal, h.Name), Name: h.Name,
					Base: arrange.Fingerprint(h), ScheduledAt: h.ScheduledAt, DurationMinutes: h.DurationMinutes, HeatTimes: times,
					EndText: domain.ScheduledText(times.EndAt, day), KeepReason: h.Facts().KeepReason(), People: people,
					Conflicts: []HeatConflict{}})
			}
			if day == "" {
				t.Unscheduled = append(t.Unscheduled, tr)
				continue
			}
			timed = append(timed, placed{tr, order})
		}
	}
	// By first time, then as the event list orders them, then by round.
	sort.SliceStable(timed, func(i, j int) bool {
		if timed[i].round.StartAt != timed[j].round.StartAt {
			return timed[i].round.StartAt < timed[j].round.StartAt
		}
		return timed[i].order < timed[j].order
	})
	for _, pl := range timed {
		if n := len(t.Days); n == 0 || t.Days[n-1].Day != pl.round.Day {
			t.Days = append(t.Days, TimetableDay{Day: pl.round.Day, Rounds: []TimetableRound{}})
		}
		last := &t.Days[len(t.Days)-1]
		last.Rounds = append(last.Rounds, pl.round)
	}
	return t
}

// HeatEdit changes one heat's custom name, own time and expected duration
// from the schedule page; Base is the fingerprint it was shown with.
type HeatEdit struct {
	EventID         string `json:"eventId"`
	HeatID          string `json:"heatId"`
	Base            string `json:"base"`
	Name            string `json:"name"`
	ScheduledAt     string `json:"scheduledAt"`
	DurationMinutes *int   `json:"durationMinutes"`
}

// EditHeat saves a heat's name, time and duration the way the arrange
// page saves them: as a board save of that one heat, its slots unchanged.
// A heat changed since it was shown returns ErrStale.
func (s *Service) EditHeat(ctx context.Context, src store.Source, in HeatEdit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, in.EventID)
	if err != nil {
		return err
	}
	for _, r := range st.rounds {
		for _, h := range r.Heats {
			if h.ID != in.HeatID {
				continue
			}
			if arrange.Fingerprint(h) != in.Base {
				return ErrStale
			}
			rs := RoundSave{RoundID: r.ID, Order: []string{}, Changed: []HeatSave{{ID: h.ID, Base: in.Base, Name: in.Name,
				ScheduledAt: in.ScheduledAt, DurationMinutes: in.DurationMinutes, Slots: []SlotSave{}}}}
			for _, other := range r.Heats {
				rs.Order = append(rs.Order, other.ID)
			}
			for _, sl := range h.Slots {
				rs.Changed[0].Slots = append(rs.Changed[0].Slots, SlotSave{Position: sl.Position, AthleteID: sl.AthleteID, TeamID: sl.TeamID})
			}
			return s.saveBoard(ctx, src, BoardSave{EventID: in.EventID, Rounds: []RoundSave{rs}})
		}
	}
	return invalid("组不存在或已删除")
}
