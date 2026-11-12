package service

import (
	"context"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// EventDetail is everything the entries, arrange and results pages show
// for one event.
type EventDetail struct {
	Rules   domain.EventRules `json:"rules"`
	Teams   []TeamView        `json:"teams"`
	Event   domain.Event      `json:"event"`
	Entries []EntryView       `json:"entries"`
	Rounds  []RoundView       `json:"rounds"`
}

// EntryView is one entered athlete.
type EntryView struct {
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
}

// RoundView is a round with its heats and overall order.
type RoundView struct {
	ID        string          `json:"id"`
	Stage     domain.Stage    `json:"stage"`
	Label     string          `json:"label"`
	Ordinal   int             `json:"ordinal"`
	Advance   *domain.Advance `json:"advance"`
	Heats     []HeatView      `json:"heats"`
	Standings []Standing      `json:"standings"`
	Complete  bool            `json:"complete"` // every placed athlete has a result
}

// HeatView is a heat; Base is its fingerprint when loaded.
type HeatView struct {
	ID          string     `json:"id"`
	Ordinal     int        `json:"ordinal"`
	Name        string     `json:"name"`
	Label       string     `json:"label"`
	ScheduledAt string     `json:"scheduledAt"`
	Slots       []SlotView `json:"slots"`
	KeepReason  string     `json:"keepReason"` // "" = free to change
	Base        string     `json:"base"`
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
	Status   domain.Status `json:"status"`
	Mark     *int64        `json:"mark"`
	MarkText string        `json:"markText"`
	Rank     *int          `json:"rank"`
	Qualify  string        `json:"qualify"`
	Attempts []string      `json:"attempts"`
	Bars     []BarView     `json:"bars"`
	Note     string        `json:"note"`
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
	athletes map[string]domain.Athlete // the whole meet's
	units    map[string]domain.Unit
	rounds   []domain.Round
}

func (s *Service) loadEvent(ctx context.Context, eventID string) (*eventState, error) {
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
	athletes, err := s.store.ListAthletes(ctx, st.event.MeetID)
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
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return st.view(), nil
}

func (st *eventState) view() *EventDetail {
	kind := st.event.Kind
	placed := map[string]bool{}
	rounds := make([]RoundView, len(st.rounds))
	for i, r := range st.rounds {
		rv := RoundView{ID: r.ID, Stage: r.Stage, Label: r.Label(), Ordinal: r.Ordinal, Advance: r.Advance,
			Heats: make([]HeatView, len(r.Heats))}
		for j, h := range r.Heats {
			hv := HeatView{ID: h.ID, Ordinal: h.Ordinal, Name: h.Name, Label: domain.HeatLabel(h.Ordinal, h.Name),
				ScheduledAt: h.ScheduledAt, KeepReason: h.Facts().KeepReason(), Base: arrange.Fingerprint(h),
				Slots: make([]SlotView, len(h.Slots))}
			for k, sl := range h.Slots {
				placed[sl.CompetitorID()] = true
				hv.Slots[k] = SlotView{Position: sl.Position, AthleteID: sl.AthleteID, TeamID: sl.TeamID, CompetitorID: sl.CompetitorID(), Result: resultView(kind, sl.Result)}
			}
			rv.Heats[j] = hv
		}
		rv.Standings, rv.Complete = standings(kind, r)
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
			v.SeedText = scoring.FormatMark(kind, *e.SeedMark)
		}
		if e.TeamID != "" {
			v.GenderMismatch = mismatch[e.TeamID]
		}
		entries = append(entries, v)
	}
	return &EventDetail{Event: st.event, Rules: st.event.Rules(), Teams: teams, Entries: entries, Rounds: rounds}
}

func resultView(kind domain.EventKind, r *domain.Result) *ResultView {
	if r == nil {
		return nil
	}
	v := &ResultView{Status: r.Status, Mark: r.Mark, Rank: r.Rank, Qualify: r.Qualify, Note: r.Note,
		Attempts: make([]string, len(r.Attempts)), Bars: make([]BarView, len(r.Bars))}
	if r.Mark != nil {
		v.MarkText = scoring.FormatMark(kind, *r.Mark)
	}
	for i, a := range r.Attempts {
		v.Attempts[i] = scoring.FormatAttempt(kind, a)
	}
	for i, b := range r.Bars {
		v.Bars[i] = BarView{Height: b.Height, HeightText: scoring.FormatMark(kind, b.Height), Trials: b.Trials}
	}
	return v
}

// ranked turns a slot's result into scoring input.
func ranked(key string, r *domain.Result) scoring.Ranked {
	out := scoring.Ranked{Key: key, Status: r.Status, Attempts: r.Attempts, Bars: r.Bars}
	if r.Mark != nil {
		out.Mark = *r.Mark
	}
	return out
}

// standings orders a round's results across its heats. complete is true
// when the round has results for everyone placed in it.
func standings(kind domain.EventKind, r domain.Round) (list []Standing, complete bool) {
	var all []scoring.Ranked
	list = []Standing{}
	placed := 0
	for _, h := range r.Heats {
		for _, sl := range h.Slots {
			placed++
			if sl.Result == nil {
				continue
			}
			all = append(all, ranked(sl.CompetitorID(), sl.Result))
			row := Standing{AthleteID: sl.AthleteID, TeamID: sl.TeamID, CompetitorID: sl.CompetitorID(), HeatLabel: domain.HeatLabel(h.Ordinal, h.Name),
				Position: sl.Position, Status: sl.Result.Status, Qualify: sl.Result.Qualify}
			if sl.Result.Mark != nil {
				row.MarkText = scoring.FormatMark(kind, *sl.Result.Mark)
			}
			list = append(list, row)
		}
	}
	places := scoring.Rank(kind, all)
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
	return list, placed > 0 && len(list) == placed
}

// rescore recomputes what follows from the marks of a round held in
// memory: each result's place inside its heat, and Q / q once the round is
// complete and has an advancement rule. It replaces the Result values it
// changes, so callers can compare pointers' contents with what is stored.
func rescore(kind domain.EventKind, advance *domain.Advance, heats []domain.Heat) {
	complete := true
	placed := 0
	finishers := make([][]arrange.Finisher, len(heats))
	for i, h := range heats {
		var list []scoring.Ranked
		for _, sl := range h.Slots {
			placed++
			if sl.Result == nil {
				complete = false
				continue
			}
			list = append(list, ranked(sl.CompetitorID(), sl.Result))
		}
		places := scoring.Rank(kind, list)
		for k, sl := range h.Slots {
			if sl.Result == nil {
				continue
			}
			r := *sl.Result
			r.Rank, r.Qualify = nil, ""
			f := arrange.Finisher{ID: sl.CompetitorID(), Status: r.Status}
			if p, ok := places[sl.CompetitorID()]; ok && r.Mark != nil {
				r.Rank, f.Rank, f.Mark = &p, p, *r.Mark
			}
			heats[i].Slots[k].Result = &r
			finishers[i] = append(finishers[i], f)
		}
	}
	if advance == nil || !complete || placed == 0 {
		return
	}
	q := arrange.Qualify(kind, finishers, advance.ByPlace, advance.ByMark)
	for i, h := range heats {
		for k, sl := range h.Slots {
			if sl.Result != nil {
				heats[i].Slots[k].Result.Qualify = q[sl.CompetitorID()]
			}
		}
	}
}
