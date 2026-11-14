package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	entered  map[string]int // event id → people entered; a team counts its members
	minRest  int
}

func newMeetPlan(meet domain.Meet, events []domain.Event, rounds map[string][]domain.Round, teams []domain.Team, athletes []domain.Athlete, entries []domain.Entry, minRest int) *meetPlan {
	p := &meetPlan{meet: meet, rounds: rounds, minRest: minRest, eventBy: map[string]domain.Event{},
		teams: map[string]domain.Team{}, athletes: map[string]domain.Athlete{}, entered: map[string]int{}}
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
	for _, en := range entries {
		if en.TeamID != "" {
			p.entered[en.EventID] += len(p.teams[en.TeamID].MemberIDs)
		} else {
			p.entered[en.EventID]++
		}
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
	rounds, err := s.store.LoadPlanRounds(ctx, sc)
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
	entered, err := s.store.EntryPeople(ctx, sc)
	if err != nil {
		return nil, err
	}
	prefs, err := s.MeetSettings(ctx, meetID)
	if err != nil {
		return nil, err
	}
	plan := newMeetPlan(*meet, events, rounds, teams, athletes, nil, prefs.MinRestMinutes)
	plan.entered = entered
	return plan, nil
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
	// Text names the heat in full: 2026-11-12 09:00 男子100m 决赛 第1组.
	Text string `json:"text"`
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
	ref := ScheduleHeatRef{EventID: place.event.ID, EventName: place.event.Name, RoundID: place.round.ID, RoundLabel: place.round.Label(),
		HeatID: h.ID, HeatLabel: domain.HeatLabel(h.Ordinal, h.Name), TimeText: domain.ScheduledText(at, "")}
	ref.Text = strings.TrimSpace(strings.Join([]string{ref.TimeText, ref.EventName, ref.RoundLabel, ref.HeatLabel}, " "))
	return ref
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
	StartAt   string `json:"startAt"`
	StartText string `json:"startText"`
	HeatCount int    `json:"heatCount"`
	// People is how many athletes the round has, a team counting its
	// members: those placed in its heats once it has heats; for an event's
	// first round without heats, those entered (PeopleEntered); else 0.
	People        int             `json:"people"`
	PeopleEntered bool            `json:"peopleEntered"`
	Heats         []TimetableHeat `json:"heats"`
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
	HeatStart
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
		for ri, r := range p.rounds[e.ID] {
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
					HeatStart: HeatStartOf(times.EffectiveAt, h.StartedAt, day),
					EndText:   domain.ScheduledText(times.EndAt, day), KeepReason: h.Facts().KeepReason(), People: people,
					Conflicts: []HeatConflict{}})
			}
			if len(r.Heats) == 0 && ri == 0 {
				tr.People, tr.PeopleEntered = p.entered[e.ID], true
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
	// Unscheduled: rounds that have heats first, then those without.
	sort.SliceStable(t.Unscheduled, func(i, j int) bool {
		return t.Unscheduled[i].HeatCount > 0 && t.Unscheduled[j].HeatCount == 0
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

// HeatTimeEdit changes one heat's own time and expected duration only (MCP
// set_heat_time); its name, slots and results stay.
type HeatTimeEdit struct {
	EventID         string `json:"eventId"`
	HeatID          string `json:"heatId"`
	Base            string `json:"base" jsonschema:"get_event 里该组的 base（组指纹）；组在此之后有任何变化即返回 E_STALE"`
	ScheduledAt     string `json:"scheduledAt" jsonschema:"组单独的本地时间，YYYY-MM-DDTHH:mm，无时区；空字符串表示沿用轮次开始时间"`
	DurationMinutes *int   `json:"durationMinutes" jsonschema:"预计用时，1 至 1440 分钟；null 表示不填"`
}

// HeatEditResult is what an edit leaves: the heat's new fingerprint, the
// warnings its time raises and the conflicts it is now in. Warnings and
// conflicts never block the write.
type HeatEditResult struct {
	Base      string         `json:"base"`
	Warnings  []string       `json:"warnings"`
	Conflicts []HeatConflict `json:"conflicts"`
}

// EditHeat saves a heat's name, time and duration the way the arrange
// page saves them: as a board save of that one heat, its slots unchanged.
// A heat changed since it was shown returns ErrStale.
func (s *Service) EditHeat(ctx context.Context, src store.Source, in HeatEdit) (*HeatEditResult, error) {
	return s.editHeat(ctx, src, in.EventID, in.HeatID, in.Base, &in.Name, in.ScheduledAt, in.DurationMinutes)
}

// SetHeatTime is EditHeat keeping the heat's name.
func (s *Service) SetHeatTime(ctx context.Context, src store.Source, in HeatTimeEdit) (*HeatEditResult, error) {
	return s.editHeat(ctx, src, in.EventID, in.HeatID, in.Base, nil, in.ScheduledAt, in.DurationMinutes)
}

func (s *Service) editHeat(ctx context.Context, src store.Source, eventID, heatID, base string, name *string, at string, minutes *int) (*HeatEditResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	var round *domain.Round
	var heat *domain.Heat
	for i := range st.rounds {
		for j := range st.rounds[i].Heats {
			if st.rounds[i].Heats[j].ID == heatID {
				round, heat = &st.rounds[i], &st.rounds[i].Heats[j]
			}
		}
	}
	if heat == nil {
		return nil, invalid("组不存在或已删除")
	}
	if arrange.Fingerprint(*heat) != base {
		return nil, ErrStale
	}
	save := HeatSave{ID: heat.ID, Base: base, Name: heat.Name, ScheduledAt: at, DurationMinutes: minutes, Slots: []SlotSave{}}
	if name != nil {
		save.Name = *name
	}
	for _, sl := range heat.Slots {
		save.Slots = append(save.Slots, SlotSave{Position: sl.Position, AthleteID: sl.AthleteID, TeamID: sl.TeamID})
	}
	rs := RoundSave{RoundID: round.ID, Order: []string{}, Changed: []HeatSave{save}}
	for _, other := range round.Heats {
		rs.Order = append(rs.Order, other.ID)
	}
	if err := s.saveBoard(ctx, src, BoardSave{EventID: eventID, Rounds: []RoundSave{rs}}); err != nil {
		return nil, err
	}
	p, err := s.loadMeetPlan(ctx, st.event.MeetID)
	if err != nil {
		return nil, err
	}
	out := &HeatEditResult{Warnings: []string{}, Conflicts: []HeatConflict{}}
	for _, r := range p.rounds[eventID] {
		for _, h := range r.Heats {
			if h.ID != heatID {
				continue
			}
			out.Base = arrange.Fingerprint(h)
			effective, _ := domain.EffectiveScheduled(r, h)
			if domain.OutsideMeet(effective, p.meet) {
				out.Warnings = append(out.Warnings, "时间"+outsideMeetWarning)
			}
		}
	}
	if list := byHeat(p.conflictViews(p.conflicts(nil), nil))[heatID]; list != nil {
		out.Conflicts = list
	}
	return out, nil
}

// DraftConflictRequest is the arrange page's draft of one event: every
// heat of every round as it would be saved (deleted heats left out).
type DraftConflictRequest struct {
	EventID string      `json:"eventId"`
	Heats   []DraftTime `json:"heats"`
}

// DraftTime is one draft heat: Key is its id, or the page's key for a heat
// not saved yet; Competitors are athlete ids, or team ids in a team event.
type DraftTime struct {
	Key             string   `json:"key"`
	RoundID         string   `json:"roundId"`
	Name            string   `json:"name"`
	ScheduledAt     string   `json:"scheduledAt"`
	DurationMinutes *int     `json:"durationMinutes"`
	Competitors     []string `json:"competitors"`
}

// DraftConflicts checks the meet's saved times with this event's heats
// replaced by the draft, so unsaved changes count, and lists the conflicts
// of the draft heats by key. It reads only; heats are labelled by their
// place in the draft round, as the page shows them.
func (s *Service) DraftConflicts(ctx context.Context, in DraftConflictRequest) ([]HeatConflicts, error) {
	meetID, err := s.meetOfEvent(ctx, in.EventID)
	if err != nil {
		return nil, err
	}
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	e := p.eventBy[in.EventID]
	rounds := append([]domain.Round(nil), p.rounds[in.EventID]...)
	index := map[string]int{}
	for i := range rounds {
		index[rounds[i].ID] = i
		rounds[i].Heats = nil
	}
	keys := []string{}
	for _, d := range in.Heats {
		i, ok := index[d.RoundID]
		if !ok || d.Key == "" {
			continue // a round deleted meanwhile; the page reloads
		}
		h := domain.Heat{ID: d.Key, RoundID: d.RoundID, Ordinal: len(rounds[i].Heats) + 1, Name: d.Name, DurationMinutes: d.DurationMinutes}
		if domain.ValidScheduled(d.ScheduledAt) {
			h.ScheduledAt = d.ScheduledAt
		}
		if !domain.ValidDuration(h.DurationMinutes) {
			h.DurationMinutes = nil
		}
		for n, id := range d.Competitors {
			sl := domain.Slot{Position: n + 1, AthleteID: id}
			if e.IsTeam() {
				sl = domain.Slot{Position: n + 1, TeamID: id}
			}
			h.Slots = append(h.Slots, sl)
		}
		rounds[i].Heats = append(rounds[i].Heats, h)
		keys = append(keys, d.Key)
	}
	p.rounds[in.EventID] = rounds
	grouped := byHeat(p.conflictViews(p.conflicts(nil), nil))
	out := []HeatConflicts{}
	for _, k := range keys {
		if list := grouped[k]; len(list) > 0 {
			out = append(out, HeatConflicts{HeatID: k, Conflicts: list})
		}
	}
	return out, nil
}
