package examplemeet

import (
	"fmt"
	"slices"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/scoring"
)

type roundPlan struct {
	day, minute, duration int
	stage                 string
	byPlace, byMark       int
}

type discipline struct {
	name, kind, start, distance string
	from, to                    int
	team                        bool
	base                        int64
	stages                      []roundPlan
}

func disciplines(c int) []discipline {
	return []discipline{
		{name: "100m", kind: "track", start: "lanes", from: 0, to: 4, base: 12300,
			stages: []roundPlan{{0, 480 + c*20, 3, "heat", 2, 4}, {0, 810 + c*10, 3, "semifinal", 3, 2}, {1, 480 + c*5, 3, "", 0, 0}}},
		{name: "200m", kind: "track", start: "lanes", from: 0, to: 4, base: 25400,
			stages: []roundPlan{{1, 540 + c*20, 3, "heat", 1, 2}, {2, 480 + c*5, 3, "", 0, 0}}},
		{name: "400m", kind: "track", start: "lanes", from: 4, to: 8, base: 56800,
			stages: []roundPlan{{0, 600 + c*20, 3, "heat", 1, 2}, {1, 660 + c*5, 3, "", 0, 0}}},
		{name: "800m", kind: "track", start: "break", from: 4, to: 8, base: 138000,
			stages: []roundPlan{{day: 1, minute: 780 + c*40, duration: 6}}},
		{name: "1500m", kind: "track", start: "group", from: 8, to: 10, base: 290000,
			stages: []roundPlan{{day: 0, minute: 870 + c*20, duration: 9}}},
		{name: "3000m", kind: "track", start: "group", from: 8, to: 10, base: 630000,
			stages: []roundPlan{{day: 2, minute: 600 + c*30, duration: 14}}},
		{name: "跳远", kind: "distance", distance: "jump", from: 10, to: 12, base: 5100,
			stages: []roundPlan{{day: 0, minute: 480 + c*60, duration: 25}}},
		{name: "铅球", kind: "distance", distance: "throw", from: 10, to: 12, base: 8800,
			stages: []roundPlan{{day: 1, minute: 480 + c*60, duration: 25}}},
		{name: "跳高", kind: "height", from: 12, to: 14, base: 1450,
			stages: []roundPlan{{day: 1, minute: 480 + c*75, duration: 35}}},
		{name: "1分钟跳绳", kind: "count", from: 12, to: 14, base: 140,
			stages: []roundPlan{{day: 0, minute: 780 + c*20, duration: 8}}},
		{name: "4×100m", kind: "relay", start: "lanes", team: true, from: 0, base: 51000,
			stages: []roundPlan{{2, 510 + c*10, 4, "heat", 3, 2}, {2, 840 + c*5, 4, "", 0, 0}}},
		{name: "集体技巧", kind: "score", team: true, from: 10, base: 4200,
			stages: []roundPlan{{day: 2, minute: 540 + c*15, duration: 12}}},
	}
}

func eventConfig(e interchange.Event) domain.Event {
	return domain.Event{Kind: domain.EventKind(e.Kind), CompetitionConfig: e.CompetitionConfig}
}

// finishEvent uses the real ranking and advancement functions; no fabricated
// places or qualifiers can diverge from what the application would calculate.
func finishEvent(e *interchange.Event, spec discipline, category int) {
	entries := slices.Clone(e.Entries)
	for roundIndex, plan := range spec.stages {
		stage := plan.stage
		if stage == "" {
			stage = "final"
		}
		r := interchange.Round{ID: fmt.Sprintf("%s-r%d", e.ID, roundIndex+1), Stage: stage, Ordinal: roundIndex + 1,
			ScheduledAt: ptr(wallTime(plan.day, plan.minute)), Heats: []interchange.Heat{}}
		if stage != "final" {
			r.Advance = &interchange.Advance{ByPlace: plan.byPlace, ByMark: plan.byMark}
		}
		capacity := 12
		if spec.start == "lanes" || spec.start == "break" {
			capacity = 8
		}
		for i, entry := range entries {
			if i%capacity == 0 {
				h := interchange.Heat{ID: fmt.Sprintf("%s-h%d", r.ID, len(r.Heats)+1), Ordinal: len(r.Heats) + 1,
					DurationMinutes: ptr(plan.duration), Slots: []interchange.Slot{}}
				minute := plan.minute + len(r.Heats)*plan.duration
				if len(r.Heats) > 0 {
					h.ScheduledAt = ptr(wallTime(plan.day, minute))
				}
				h.StartedAt = ptr(wallTime(plan.day, minute+1))
				if spec.name == "100m" || spec.name == "200m" {
					h.Wind = ptr(8 + category)
				}
				r.Heats = append(r.Heats, h)
			}
			h := &r.Heats[len(r.Heats)-1]
			h.Slots = append(h.Slots, interchange.Slot{Position: i%capacity + 1, AthleteID: entry.AthleteID, TeamID: entry.TeamID,
				Result: makeResult(spec, category, roundIndex, i, len(entries))})
		}
		var all []scoring.Ranked
		for _, h := range r.Heats {
			all = append(all, ranked(h)...)
		}
		order := scoring.RankEvent(eventConfig(*e), all)
		var heats [][]arrange.Finisher
		for hi := range r.Heats {
			h := &r.Heats[hi]
			ranks := scoring.RankEvent(eventConfig(*e), ranked(*h))
			var finishers []arrange.Finisher
			for si := range h.Slots {
				slot := &h.Slots[si]
				key := competitor(*slot)
				if place := ranks[key]; place > 0 {
					slot.Result.Rank = ptr(place)
				}
				finishers = append(finishers, arrange.Finisher{ID: key, Status: domain.Status(slot.Result.Status), Rank: ranks[key], Order: order[key]})
			}
			heats = append(heats, finishers)
		}
		if r.Advance != nil {
			qualifiers := arrange.Qualify(heats, plan.byPlace, plan.byMark)
			var next []interchange.Entry
			for hi := range r.Heats {
				for si := range r.Heats[hi].Slots {
					slot := &r.Heats[hi].Slots[si]
					if q := qualifiers[competitor(*slot)]; q != "" {
						slot.Result.Qualify = ptr(q)
						next = append(next, interchange.Entry{AthleteID: slot.AthleteID, TeamID: slot.TeamID})
					}
				}
			}
			// Snake the ranked qualifiers across the next round's heats.
			slices.SortStableFunc(next, func(a, b interchange.Entry) int { return order[entryKey(a)] - order[entryKey(b)] })
			entries = make([]interchange.Entry, len(next))
			n := (len(next) + capacity - 1) / capacity
			for i, entry := range next {
				row, col := i/n, i%n
				if row%2 == 1 {
					col = n - 1 - col
				}
				// Qualifying counts are multiples of eight for these rounds.
				entries[col*capacity+row] = entry
			}
		}
		e.Rounds = append(e.Rounds, r)
	}
	e.Date = ptr(days[spec.stages[0].day])
}

func wallTime(day, minute int) string {
	return fmt.Sprintf("%sT%02d:%02d", days[day], minute/60, minute%60)
}
func entryKey(e interchange.Entry) string {
	if e.TeamID != "" {
		return e.TeamID
	}
	return e.AthleteID
}
func competitor(s interchange.Slot) string {
	if s.TeamID != "" {
		return s.TeamID
	}
	return s.AthleteID
}

func ranked(h interchange.Heat) []scoring.Ranked {
	var out []scoring.Ranked
	for _, s := range h.Slots {
		r := s.Result
		row := scoring.Ranked{Key: competitor(s), Status: domain.Status(r.Status)}
		if r.Mark != nil {
			row.Mark = *r.Mark
		}
		for _, a := range r.Attempts {
			row.Attempts = append(row.Attempts, domain.Attempt{Outcome: a.Outcome, Mark: a.Mark, Wind: a.Wind})
		}
		for _, b := range r.Bars {
			row.Bars = append(row.Bars, domain.Bar{Height: b.Height, Trials: b.Trials})
		}
		out = append(out, row)
	}
	return out
}
