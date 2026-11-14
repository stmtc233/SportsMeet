package service

import (
	"context"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Overview is the 总览 page: where the meet stands in its workflow, the
// rounds still to be held, and its size.
type Overview struct {
	Steps []OverviewStep `json:"steps"`
	// Next lists the rounds with a planned time that are not finished,
	// earliest first, at most overviewNext of them.
	Next      []OverviewRound `json:"next"`
	NextTotal int             `json:"nextTotal"` // all such rounds
	Units     int             `json:"units"`
	Athletes  int             `json:"athletes"`
	Groups    int             `json:"groups"`
}

// Step states.
const (
	StepTodo  = "todo"
	StepDoing = "doing"
	StepDone  = "done"
)

// OverviewStep is one stage of the workflow: Done of Total, its state, and
// what is worth a look (Warnings). Extra carries the counts a stage shows
// besides Done and Total.
type OverviewStep struct {
	Key      string `json:"key"` // events | entries | arrange | schedule | results
	State    string `json:"state"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Warnings int    `json:"warnings"`
	Extra    int    `json:"extra"`
}

// OverviewRound is a round still to be held.
type OverviewRound struct {
	EventID    string `json:"eventId"`
	EventName  string `json:"eventName"`
	RoundLabel string `json:"roundLabel"`
	StartText  string `json:"startText"`
	Heats      int    `json:"heats"`
	Started    bool   `json:"started"` // some result is in
}

const overviewNext = 6

// stepState is todo before anything is done, done when all is, doing
// between.
func stepState(done, total int) string {
	switch {
	case done == 0 || total == 0:
		return StepTodo
	case done >= total:
		return StepDone
	}
	return StepDoing
}

// roundFinished says every slot of the round's heats has a result that is
// not a height draft; a round without heats is not finished.
func roundFinished(r domain.Round) (finished, started bool) {
	finished = len(r.Heats) > 0
	for _, h := range r.Heats {
		if len(h.Slots) == 0 {
			finished = false
		}
		for _, sl := range h.Slots {
			if sl.Result == nil || sl.Result.HeightPending {
				finished = false
			}
			if sl.Result != nil {
				started = true
			}
		}
	}
	return finished, started
}

// Overview reads the meet once and sums up each stage:
//   - events: the individual and team events and the combineds (Extra);
//   - entries: units with an entry or a team of all units, Warnings the
//     athletes and events over a 报名限制;
//   - arrange: events with entries whose first round has heats;
//   - schedule: rounds of those events with a planned time, Warnings the
//     athlete time conflicts;
//   - results: events whose final is finished, Extra those with results
//     but not finished.
func (s *Service) Overview(ctx context.Context, meetID string) (*Overview, error) {
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	summary, err := s.EntrySummary(ctx, meetID)
	if err != nil {
		return nil, err
	}
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := &Overview{Steps: []OverviewStep{}, Next: []OverviewRound{}, Units: len(summary.Units), Athletes: len(p.athletes), Groups: len(groups)}

	events, combineds := 0, map[string]bool{}
	for _, e := range p.events {
		if e.CombinedID != "" {
			combineds[e.CombinedID] = true
		} else {
			events++
		}
	}
	out.Steps = append(out.Steps, OverviewStep{Key: "events", State: stepState(events+len(combineds), events+len(combineds)), Done: events, Total: events, Extra: len(combineds)})

	entered, over := 0, 0
	for _, u := range summary.Units {
		if u.Entries > 0 || u.Teams > 0 {
			entered++
		}
		over += u.Over
	}
	out.Steps = append(out.Steps, OverviewStep{Key: "entries", State: stepState(entered, len(summary.Units)), Done: entered, Total: len(summary.Units), Warnings: over})

	arranged, withEntries := 0, 0
	timed, rounds := 0, 0
	finished, started := 0, 0
	var next []OverviewRound
	var nextAt []string
	for _, e := range p.events {
		rs := p.rounds[e.ID]
		if p.entered[e.ID] == 0 && (len(rs) == 0 || len(rs[0].Heats) == 0) {
			continue
		}
		withEntries++
		if len(rs) > 0 && len(rs[0].Heats) > 0 {
			arranged++
		}
		eventStarted := false
		for _, r := range rs {
			rounds++
			at := domain.EarliestScheduled([]domain.Round{r})
			if at != "" {
				timed++
			}
			done, began := roundFinished(r)
			eventStarted = eventStarted || began
			if r.Stage == domain.StageFinal && done {
				finished++
			}
			if !done && at != "" {
				next = append(next, OverviewRound{EventID: e.ID, EventName: e.Name, RoundLabel: r.Label(),
					StartText: domain.ScheduledText(at, p.meet.StartDate), Heats: len(r.Heats), Started: began})
				nextAt = append(nextAt, at)
			}
		}
		if eventStarted {
			started++
		}
	}
	conflicts := len(p.conflicts(nil))
	resultsState := stepState(finished, withEntries)
	if resultsState == StepTodo && started > 0 {
		resultsState = StepDoing
	}
	out.Steps = append(out.Steps,
		OverviewStep{Key: "arrange", State: stepState(arranged, withEntries), Done: arranged, Total: withEntries},
		OverviewStep{Key: "schedule", State: stepState(timed, rounds), Done: timed, Total: rounds, Warnings: conflicts},
		OverviewStep{Key: "results", State: resultsState, Done: finished, Total: withEntries, Extra: started - finished},
	)

	order := make([]int, len(next))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return nextAt[order[a]] < nextAt[order[b]] })
	out.NextTotal = len(next)
	for _, i := range order[:min(len(order), overviewNext)] {
		out.Next = append(out.Next, next[i])
	}
	return out, nil
}
