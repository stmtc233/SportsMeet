package service

import (
	"context"
	"sort"
	"time"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// EventDetail is everything the entries, arrange and results pages show
// for one event.
type EventDetail struct {
	// Day is the day the event is shown on (domain.PlannedDay): the day of
	// its earliest planned time, else its date, else the meet's start.
	Day     string            `json:"day"`
	Rules   domain.EventRules `json:"rules"`
	Teams   []TeamView        `json:"teams"`
	Event   domain.Event      `json:"event"`
	Entries []EntryView       `json:"entries"`
	Rounds  []RoundView       `json:"rounds"`
}

// EntryView is one entered athlete.
type EntryView struct {
	GroupMismatch  bool          `json:"groupMismatch"`
	AthleteID      string        `json:"athleteId"`
	Name           string        `json:"name"`
	Gender         domain.Gender `json:"gender"`
	Bib            string        `json:"bib"`
	UnitID         string        `json:"unitId"`
	UnitName       string        `json:"unitName"` // short name when the unit has one
	SeedMark       *int64        `json:"seedMark"`
	SeedText       string        `json:"seedText"`
	Placed         bool          `json:"placed"`         // occupies a slot in this event
	GenderMismatch bool          `json:"genderMismatch"` // differs from the event's gender
	CompetitorID   string        `json:"competitorId"`
	TeamID         string        `json:"teamId,omitempty"`
	// OverEvents: the athlete enters more events than the meet allows.
	OverEvents bool `json:"overEvents"`
	// OverUnit: the unit enters more athletes (teams) here than allowed.
	OverUnit bool `json:"overUnit"`
}

// RoundView is a round with its heats and overall order.
type RoundView struct {
	ID      string          `json:"id"`
	Stage   domain.Stage    `json:"stage"`
	Label   string          `json:"label"`
	Ordinal int             `json:"ordinal"`
	Advance *domain.Advance `json:"advance"`
	// ScheduledAt is when the round starts, "" = unscheduled; ScheduledText
	// shows it as the heats' times are shown.
	ScheduledAt   string     `json:"scheduledAt"`
	ScheduledText string     `json:"scheduledText"`
	Heats         []HeatView `json:"heats"`
	Standings     []Standing `json:"standings"`
	Ties          []TieView  `json:"ties"`
	Complete      bool       `json:"complete"` // every placed competitor has a confirmed result
}

// HeatView is a heat; Base is its fingerprint when loaded.
type HeatView struct {
	ID      string `json:"id"`
	Ordinal int    `json:"ordinal"`
	Name    string `json:"name"`
	Label   string `json:"label"`
	// ScheduledAt is the heat's own time, "" = it follows the round's start.
	ScheduledAt     string `json:"scheduledAt"`
	DurationMinutes *int   `json:"durationMinutes"`
	HeatTimes
	HeatStart
	// ResultsUpdatedText is when the heat's results were last written, to the minute; "" = none.
	ResultsUpdatedText string     `json:"resultsUpdatedText"`
	Slots              []SlotView `json:"slots"`
	KeepReason         string     `json:"keepReason"` // "" = free to change
	Base               string     `json:"base"`
}

// HeatTimes is a heat's plan as the pages show it (docs/agents/domain.md
// 「时间」).
type HeatTimes struct {
	// EffectiveAt is when the heat starts: its own time, else the round's
	// start (ScheduleInherited); "" = unscheduled.
	EffectiveAt       string `json:"effectiveAt"`
	ScheduleInherited bool   `json:"scheduleInherited"`
	// ScheduledText shows EffectiveAt: HH:mm on the event's day, else with the date.
	ScheduledText string `json:"scheduledText"`
	DurationText  string `json:"durationText"` // "5 分钟", "" = not given
	// EndAt is EffectiveAt plus the duration, "" when either is missing.
	EndAt string `json:"endAt"`
}

// HeatTimesOf works out a heat's plan from its round's start and its own
// time and duration; day is the event's shown day.
func HeatTimesOf(roundStart, own string, minutes *int, day string) HeatTimes {
	r, h := domain.Round{ScheduledAt: roundStart}, domain.Heat{ScheduledAt: own, DurationMinutes: minutes}
	at, inherited := domain.EffectiveScheduled(r, h)
	end, _ := domain.HeatEnd(r, h)
	return HeatTimes{EffectiveAt: at, ScheduleInherited: inherited, ScheduledText: domain.ScheduledText(at, day),
		DurationText: domain.DurationText(minutes), EndAt: end}
}

// HeatStart is when a heat actually started, as the pages show it.
type HeatStart struct {
	// StartedAt is the actual start (YYYY-MM-DDTHH:mm), "" = not recorded;
	// StartedText shows it as the planned times are shown.
	StartedAt   string `json:"startedAt"`
	StartedText string `json:"startedText"`
	// StartDeltaText compares it with the effective planned time: 晚 12 分钟,
	// 早 5 分钟, 准时; "" when either is missing.
	StartDeltaText string `json:"startDeltaText"`
}

// HeatStartOf shows a heat's actual start against its effective planned
// time; day is the event's shown day.
func HeatStartOf(effective, started, day string) HeatStart {
	return HeatStart{StartedAt: started, StartedText: domain.ScheduledText(started, day), StartDeltaText: domain.StartDeltaText(effective, started)}
}

// SlotView is a slot with its result.
type SlotView struct {
	Position     int         `json:"position"`
	AthleteID    string      `json:"athleteId"`
	Result       *ResultView `json:"result"`
	CompetitorID string      `json:"competitorId"`
	TeamID       string      `json:"teamId,omitempty"`
}

// ResultView is a result with its texts as the page shows and edits them.
type ResultView struct {
	HeightPending bool          `json:"heightPending,omitempty"`
	Status        domain.Status `json:"status"`
	Mark          *int64        `json:"mark"`
	MarkText      string        `json:"markText"`
	Rank          *int          `json:"rank"`
	Qualify       string        `json:"qualify"`
	Attempts      []string      `json:"attempts"`
	Bars          []BarView     `json:"bars"`
	Note          string        `json:"note"`
}

// BarView is one height of a height result.
type BarView struct {
	Height     int64  `json:"height"`
	HeightText string `json:"heightText"`
	Trials     string `json:"trials"`
}

// Standing is one line of a round's overall order: ranked results first,
// then the others.
type Standing struct {
	Rank         *int          `json:"rank"`
	AthleteID    string        `json:"athleteId"`
	HeatLabel    string        `json:"heatLabel"`
	Position     int           `json:"position"`
	Status       domain.Status `json:"status"`
	MarkText     string        `json:"markText"`
	Qualify      string        `json:"qualify"`
	CompetitorID string        `json:"competitorId"`
	TeamID       string        `json:"teamId,omitempty"`
}

// eventState is one event as loaded for a read or, under the write lock,
// for a write.
type eventState struct {
	teams    map[string]domain.Team
	teamList []domain.Team
	event    domain.Event
	entries  []domain.Entry
	entered  map[string]*domain.Entry
	athletes map[string]domain.Athlete // all meet athletes for roster edits, otherwise participants
	units    map[string]domain.Unit
	rounds   []domain.Round
}

func (s *Service) loadEvent(ctx context.Context, eventID string) (*eventState, error) {
	return s.loadEventRoster(ctx, eventID, true)
}

func (s *Service) loadEventParticipants(ctx context.Context, eventID string) (*eventState, error) {
	return s.loadEventRoster(ctx, eventID, false)
}

func (s *Service) loadEventRoster(ctx context.Context, eventID string, allAthletes bool) (*eventState, error) {
	sc := store.Scope{EventID: eventID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, invalid("项目不存在或已删除")
	}
	st := &eventState{event: events[0], entered: map[string]*domain.Entry{},
		athletes: map[string]domain.Athlete{}, units: map[string]domain.Unit{}}
	if st.entries, err = s.store.ListEntries(ctx, sc); err != nil {
		return nil, err
	}
	for i := range st.entries {
		st.entered[st.entries[i].CompetitorID()] = &st.entries[i]
	}
	var athletes []domain.Athlete
	if allAthletes {
		athletes, err = s.store.ListAthletes(ctx, st.event.MeetID)
	} else {
		athletes, err = s.store.ListParticipants(ctx, sc)
	}
	if err != nil {
		return nil, err
	}
	for _, a := range athletes {
		st.athletes[a.ID] = a
	}
	units, err := s.store.ListUnits(ctx, st.event.MeetID)
	if err != nil {
		return nil, err
	}
	for _, u := range units {
		st.units[u.ID] = u
	}
	rounds, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}
	st.teamList, err = s.store.ListTeams(ctx, sc)
	if err != nil {
		return nil, err
	}
	st.teams = map[string]domain.Team{}
	for _, t := range st.teamList {
		st.teams[t.ID] = t
	}
	st.rounds = rounds[eventID]
	return st, nil
}

func (st *eventState) round(id string) *domain.Round {
	for i := range st.rounds {
		if st.rounds[i].ID == id {
			return &st.rounds[i]
		}
	}
	return nil
}

func (st *eventState) unitName(unitID string) string {
	u := st.units[unitID]
	if u.ShortName != "" {
		return u.ShortName
	}
	return u.Name
}

// GetEvent returns an event's detail.
func (s *Service) GetEvent(ctx context.Context, eventID string) (*EventDetail, error) {
	st, err := s.loadEventParticipants(ctx, eventID)
	if err != nil {
		return nil, err
	}
	meet, err := s.store.GetMeet(ctx, st.event.MeetID)
	if err != nil {
		return nil, err
	}
	start := ""
	if meet != nil {
		start = meet.StartDate
	}
	v := st.view(domain.PlannedDay(st.event, st.rounds, start), s.zone)
	if meet != nil {
		if err := s.entryLimitFlags(ctx, meet, st.event, v.Entries); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (st *eventState) view(day string, zone *time.Location) *EventDetail {
	event := st.event
	placed := map[string]bool{}
	rounds := make([]RoundView, len(st.rounds))
	for i, r := range st.rounds {
		rv := RoundView{ID: r.ID, Stage: r.Stage, Label: r.Label(), Ordinal: r.Ordinal, Advance: r.Advance,
			ScheduledAt: r.ScheduledAt, ScheduledText: domain.ScheduledText(r.ScheduledAt, day),
			Heats: make([]HeatView, len(r.Heats))}
		for j, h := range r.Heats {
			times := HeatTimesOf(r.ScheduledAt, h.ScheduledAt, h.DurationMinutes, day)
			hv := HeatView{ID: h.ID, Ordinal: h.Ordinal, Name: h.Name, Label: domain.HeatLabel(h.Ordinal, h.Name),
				ScheduledAt: h.ScheduledAt, DurationMinutes: h.DurationMinutes,
				HeatTimes:          times,
				HeatStart:          HeatStartOf(times.EffectiveAt, h.StartedAt, day),
				ResultsUpdatedText: domain.RecordedText(h.ResultsUpdatedAt, zone),
				KeepReason:         h.Facts().KeepReason(), Base: arrange.Fingerprint(h),
				Slots: make([]SlotView, len(h.Slots))}
			for k, sl := range h.Slots {
				placed[sl.CompetitorID()] = true
				hv.Slots[k] = SlotView{Position: sl.Position, AthleteID: sl.AthleteID, TeamID: sl.TeamID, CompetitorID: sl.CompetitorID(), Result: resultViewEvent(event, sl.Result)}
			}
			rv.Heats[j] = hv
		}
		rv.Standings, rv.Complete = standingsEvent(event, r)
		rv.Ties = roundTies(event, r)
		rounds[i] = rv
	}
	teams := st.teamViews()
	mismatch := map[string]bool{}
	for _, t := range teams {
		mismatch[t.ID] = t.GenderMismatch
	}
	entries := make([]EntryView, 0, len(st.entries))
	for _, e := range st.entries {
		a, ok := st.competitor(e.CompetitorID())
		if !ok {
			continue
		}
		v := EntryView{AthleteID: e.AthleteID, TeamID: e.TeamID, CompetitorID: e.CompetitorID(), Name: a.Name, Gender: a.Gender, Bib: a.Bib, UnitID: a.UnitID,
			UnitName: st.unitName(a.UnitID), SeedMark: e.SeedMark, Placed: placed[a.ID],
			GenderMismatch: a.Gender != "" && st.event.Gender != domain.GenderMixed && a.Gender != st.event.Gender}
		if e.SeedMark != nil {
			v.SeedText = scoring.FormatEventMark(event, *e.SeedMark)
		}
		if e.TeamID != "" {
			v.GenderMismatch = mismatch[e.TeamID]
		}
		v.GroupMismatch = st.groupMismatch(e.CompetitorID())
		entries = append(entries, v)
	}
	return &EventDetail{Day: day, Event: st.event, Rules: st.event.Rules(), Teams: teams, Entries: entries, Rounds: rounds}
}

func resultViewEvent(event domain.Event, r *domain.Result) *ResultView {
	if r == nil {
		return nil
	}
	v := &ResultView{Status: r.Status, Mark: r.Mark, Rank: r.Rank, Qualify: r.Qualify, Note: r.Note,
		HeightPending: r.HeightPending,
		Attempts:      make([]string, len(r.Attempts)), Bars: make([]BarView, len(r.Bars))}
	if r.Mark != nil {
		v.MarkText = scoring.FormatEventMark(event, *r.Mark)
	}
	for i, a := range r.Attempts {
		v.Attempts[i] = scoring.FormatEventAttempt(event, a)
	}
	for i, b := range r.Bars {
		v.Bars[i] = BarView{Height: b.Height, HeightText: scoring.FormatEventMark(event, b.Height), Trials: b.Trials}
	}
	return v
}

// ranked turns a slot's result into scoring input.
func ranked(key string, r *domain.Result) scoring.Ranked {
	out := scoring.Ranked{Key: key, Status: r.Status, Attempts: r.Attempts, Bars: r.Bars, TieBreak: r.TieBreak}
	if r.Mark != nil {
		out.Mark = *r.Mark
	}
	return out
}

// standings orders a round's results across its heats. complete is true
// when the round has results for everyone placed in it.
func standingsEvent(event domain.Event, r domain.Round) (list []Standing, complete bool) {
	var all []scoring.Ranked
	list = []Standing{}
	placed := 0
	pending := false
	for _, h := range r.Heats {
		for _, sl := range h.Slots {
			placed++
			if sl.Result == nil {
				continue
			}
			all = append(all, ranked(sl.CompetitorID(), sl.Result))
			pending = pending || sl.Result.HeightPending
			row := Standing{AthleteID: sl.AthleteID, TeamID: sl.TeamID, CompetitorID: sl.CompetitorID(), HeatLabel: domain.HeatLabel(h.Ordinal, h.Name),
				Position: sl.Position, Status: sl.Result.Status, Qualify: sl.Result.Qualify}
			if sl.Result.Mark != nil {
				row.MarkText = scoring.FormatEventMark(event, *sl.Result.Mark)
			} else if sl.Result.HeightPending && sl.Result.Status == domain.StatusNM {
				row.MarkText = "未过杆"
			}
			list = append(list, row)
		}
	}
	places := scoring.RankEvent(event, all)
	for i := range list {
		if p, ok := places[list[i].CompetitorID]; ok {
			list[i].Rank = &p
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].Rank, list[j].Rank
		switch {
		case a == nil:
			return false
		case b == nil:
			return true
		}
		return *a < *b
	})
	return list, placed > 0 && len(list) == placed && !pending
}

// rescore recomputes what follows from the marks of a round held in
// memory: each result's place inside its heat, and Q / q once the round is
// complete and has an advancement rule. It replaces the Result values it
// changes, so callers can compare pointers' contents with what is stored.
func rescoreEvent(event domain.Event, advance *domain.Advance, heats []domain.Heat) {
	complete := true
	placed := 0
	finishers := make([][]arrange.Finisher, len(heats))
	var all []scoring.Ranked
	for _, h := range heats {
		for _, sl := range h.Slots {
			if sl.Result != nil {
				all = append(all, ranked(sl.CompetitorID(), sl.Result))
			}
		}
	}
	order := scoring.RankEvent(event, all)
	for i, h := range heats {
		var list []scoring.Ranked
		for _, sl := range h.Slots {
			placed++
			if sl.Result == nil {
				complete = false
				continue
			}
			if sl.Result.HeightPending {
				complete = false
			}
			list = append(list, ranked(sl.CompetitorID(), sl.Result))
		}
		places := scoring.RankEvent(event, list)
		for k, sl := range h.Slots {
			if sl.Result == nil {
				continue
			}
			r := *sl.Result
			r.Rank, r.Qualify = nil, ""
			f := arrange.Finisher{ID: sl.CompetitorID(), Status: r.Status}
			if p, ok := places[sl.CompetitorID()]; ok && r.Mark != nil {
				r.Rank, f.Rank, f.Order = &p, p, order[sl.CompetitorID()]
			}
			heats[i].Slots[k].Result = &r
			finishers[i] = append(finishers[i], f)
		}
	}
	if advance == nil || !complete || placed == 0 {
		return
	}
	q := arrange.Qualify(finishers, advance.ByPlace, advance.ByMark)
	for i, h := range heats {
		for k, sl := range h.Slots {
			if sl.Result != nil {
				heats[i].Slots[k].Result.Qualify = q[sl.CompetitorID()]
			}
		}
	}
}

func (st *eventState) groupMismatch(id string) bool {
	if st.event.GroupID == "" {
		return false
	}
	if a, ok := st.athletes[id]; ok {
		return a.GroupID != "" && a.GroupID != st.event.GroupID
	}
	for _, id := range st.teams[id].MemberIDs {
		if a := st.athletes[id]; a.GroupID != "" && a.GroupID != st.event.GroupID {
			return true
		}
	}
	return false
}
