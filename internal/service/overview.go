package service

import (
	"context"
	"slices"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/entryform"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Overview is the 总览 page: where the meet stands in its workflow, the
// rounds still to be held, and its size.
type Overview struct {
	// Phases are the workflow in order: before (赛前), during (比赛) and
	// after (赛后) the competition.
	Phases []OverviewPhase `json:"phases"`
	// Current is the key of the step to work on: the first one neither
	// done nor off; "" when every step is.
	Current string `json:"current"`
	// Next lists the rounds with a planned time that are not finished,
	// earliest first, at most overviewNext of them.
	Next      []OverviewRound `json:"next"`
	NextTotal int             `json:"nextTotal"` // all such rounds
	Units     int             `json:"units"`
	Athletes  int             `json:"athletes"`
	Groups    int             `json:"groups"`
	// Structure is the meet laid out as 组别 → 参赛单位 / 项目: one branch
	// per group in catalog order, then 未分组 (GroupID "") for the units,
	// athletes and events of no group. A meet without groups has the one
	// branch "" with all of them.
	Structure []OverviewBranch `json:"structure"`
}

// OverviewPhase is a part of the workflow and its steps in order.
type OverviewPhase struct {
	Key   string         `json:"key"` // before | during | after
	Steps []OverviewStep `json:"steps"`
}

// OverviewBranch is a group with its units and its events.
type OverviewBranch struct {
	GroupID  string          `json:"groupId"`
	Name     string          `json:"name"`     // "" for the meet without groups
	Athletes int             `json:"athletes"` // athletes of the group
	Units    []OverviewUnit  `json:"units"`
	Events   []OverviewEvent `json:"events"`
}

// OverviewUnit is a unit under a group: its athletes of that group, and
// the unit's 报名进度 as on the 报名 page.
type OverviewUnit struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Athletes int    `json:"athletes"`
	Entered  bool   `json:"entered"` // has an entry or a team
	Over     int    `json:"over"`
}

// OverviewEvent is an event or a combined event (Combined) of a group, in
// the 项目 page's order, named without its group, and how far it got.
type OverviewEvent struct {
	ID       string `json:"id"`
	Combined bool   `json:"combined"`
	Name     string `json:"name"`
	// State: todo (its first round has no heats), arranged, doing (some
	// result is in) or done (the final is finished). A combined event is
	// done when all its components are, doing when one has a result,
	// arranged when one has heats.
	State string `json:"state"`
}

// EventArranged is an event whose first round has heats and no result yet.
const EventArranged = "arranged"

// Step states. A step is off when the meet does not use it (团体总分 not
// enabled).
const (
	StepTodo  = "todo"
	StepDoing = "doing"
	StepDone  = "done"
	StepOff   = "off"
)

// OverviewStep is one step of the workflow: Done of Total, its state, and
// what is worth a look (Warnings). Extra carries the counts a step shows
// besides Done and Total.
type OverviewStep struct {
	Key      string `json:"key"` // events | units | entries | arrange | schedule | results | advance | team
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

// countState is done once there is any: a step that only needs something
// added.
func countState(n int) string {
	if n > 0 {
		return StepDone
	}
	return StepTodo
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

// Overview reads the meet once and sums up each step.
//
// Before the competition:
//   - events: the individual and team events, the combineds (Extra);
//   - units: the units, Warnings those of no group when the meet has groups;
//   - entries: units with an entry or a team of all units, Warnings the
//     athletes and events over a 报名限制;
//   - arrange: events with entries whose first round has heats;
//   - schedule: rounds of those events with a planned time, Warnings the
//     athlete time conflicts.
//
// During it:
//   - results: finished rounds of those events, Extra those with results
//     but not finished;
//   - advance, only with rounds that send on: such rounds whose next round
//     has heats, Extra the finished ones whose next round has none yet,
//     Warnings the ties among them that only a ruling can settle.
//
// After it:
//   - team: when 团体总分 is enabled, the events and combineds it scores
//     (with entries) that are finished.
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
	combinedList, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return nil, err
	}
	rules, err := s.store.TeamScoringRules(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := &Overview{Next: []OverviewRound{}, Units: len(summary.Units), Athletes: len(p.athletes), Groups: len(groups),
		Structure: overviewStructure(p, summary, combinedList)}

	events, combineds := 0, map[string]bool{}
	for _, e := range p.events {
		if e.CombinedID != "" {
			combineds[e.CombinedID] = true
		} else {
			events++
		}
	}
	entered, over, ungrouped := 0, 0, 0
	for _, u := range summary.Units {
		if u.Entries > 0 || u.Teams > 0 {
			entered++
		}
		if len(groups) > 0 && len(u.Unit.GroupIDs) == 0 {
			ungrouped++
		}
		over += u.Over
	}

	arranged, withEntries := 0, 0
	timed, rounds := 0, 0
	finished, started := 0, 0
	sending, advanced, waiting := 0, 0, 0
	// Finished rounds whose next round has no heats yet, by event.
	wait := map[string][]string{}
	// Events with entries whose final is finished, for 团体总分.
	live, final := map[string]bool{}, map[string]bool{}
	var next []OverviewRound
	var nextAt []string
	for _, e := range p.events {
		rs := p.rounds[e.ID]
		if p.entered[e.ID] == 0 && (len(rs) == 0 || len(rs[0].Heats) == 0) {
			continue
		}
		withEntries++
		live[e.ID] = true
		if len(rs) > 0 && len(rs[0].Heats) > 0 {
			arranged++
		}
		for i, r := range rs {
			rounds++
			at := domain.EarliestScheduled([]domain.Round{r})
			if at != "" {
				timed++
			}
			done, began := roundFinished(r)
			switch {
			case done:
				finished++
			case began:
				started++
			}
			if r.Stage == domain.StageFinal && done {
				final[e.ID] = true
			}
			if i < len(rs)-1 {
				sending++
				switch {
				case len(rs[i+1].Heats) > 0:
					advanced++
				case done:
					waiting++
					wait[e.ID] = append(wait[e.ID], r.ID)
				}
			}
			if !done && at != "" {
				next = append(next, OverviewRound{EventID: e.ID, EventName: e.Name, RoundLabel: r.Label(),
					StartText: domain.ScheduledText(at, p.meet.StartDate), Heats: len(r.Heats), Started: began})
				nextAt = append(nextAt, at)
			}
		}
	}
	ties, err := s.waitingTies(ctx, p, wait)
	if err != nil {
		return nil, err
	}
	results := stepState(finished, rounds)
	if results == StepTodo && started > 0 {
		results = StepDoing
	}
	during := []OverviewStep{{Key: "results", State: results, Done: finished, Total: rounds, Extra: started}}
	if sending > 0 {
		during = append(during, OverviewStep{Key: "advance", State: stepState(advanced, sending), Done: advanced, Total: sending, Warnings: ties, Extra: waiting})
	}
	team := OverviewStep{Key: "team", State: StepOff}
	if rules.Enabled {
		scored, done := teamProgress(p, combinedList, rules, live, final)
		team = OverviewStep{Key: "team", State: stepState(done, scored), Done: done, Total: scored}
	}
	out.Phases = []OverviewPhase{
		{Key: "before", Steps: []OverviewStep{
			{Key: "events", State: countState(events + len(combineds)), Done: events, Total: events, Extra: len(combineds)},
			{Key: "units", State: countState(len(summary.Units)), Done: len(summary.Units), Total: len(summary.Units), Warnings: ungrouped},
			{Key: "entries", State: stepState(entered, len(summary.Units)), Done: entered, Total: len(summary.Units), Warnings: over},
			{Key: "arrange", State: stepState(arranged, withEntries), Done: arranged, Total: withEntries},
			{Key: "schedule", State: stepState(timed, rounds), Done: timed, Total: rounds, Warnings: len(p.conflicts(nil))},
		}},
		{Key: "during", Steps: during},
		{Key: "after", Steps: []OverviewStep{team}},
	}
	for _, ph := range out.Phases {
		for _, st := range ph.Steps {
			if out.Current == "" && st.State != StepDone && st.State != StepOff {
				out.Current = st.Key
			}
		}
	}

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

// waitingTies counts the ties a ruling must settle in the rounds waiting to
// send on: the plan has no marks, so those events' rounds are read in full,
// in one go.
func (s *Service) waitingTies(ctx context.Context, p *meetPlan, wait map[string][]string) (int, error) {
	if len(wait) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(wait))
	for id := range wait {
		ids = append(ids, id)
	}
	full, err := s.store.LoadRounds(ctx, store.Scope{EventIDs: ids})
	if err != nil {
		return 0, err
	}
	n := 0
	for id, rounds := range wait {
		for _, r := range full[id] {
			if !slices.Contains(rounds, r.ID) {
				continue
			}
			for _, t := range roundTies(p.eventBy[id], r) {
				if !t.Ruled && t.Extra > 0 {
					n++
				}
			}
		}
	}
	return n, nil
}

// teamProgress counts what 团体总分 scores: the events (not components) and
// combineds with entries whose multiplier is above 0, and of them those
// finished — an event's final, every component's final.
func teamProgress(p *meetPlan, combineds []store.CombinedData, rules domain.TeamScoringRules, live, final map[string]bool) (scored, done int) {
	multiplier := func(eventID, combinedID string, base int64) int64 {
		for _, o := range rules.Overrides {
			if o.EventID == eventID && o.CombinedID == combinedID {
				return o.Multiplier
			}
		}
		return base
	}
	for _, e := range p.events {
		base := int64(1)
		if e.Kind == domain.KindRelay {
			base = rules.RelayMultiplier
		}
		if e.CombinedID != "" || !live[e.ID] || multiplier(e.ID, "", base) <= 0 {
			continue
		}
		scored++
		if final[e.ID] {
			done++
		}
	}
	for _, c := range combineds {
		entered, all := false, len(c.Components) > 0
		for _, comp := range c.Components {
			entered = entered || live[comp.EventID]
			all = all && final[comp.EventID]
		}
		if !entered || multiplier("", c.Combined.ID, 1) <= 0 {
			continue
		}
		scored++
		if all {
			done++
		}
	}
	return scored, done
}

// eventProgress is how far an event got, as OverviewEvent.State says.
func eventProgress(rs []domain.Round) string {
	if len(rs) == 0 || len(rs[0].Heats) == 0 {
		return StepTodo
	}
	started := false
	for _, r := range rs {
		done, began := roundFinished(r)
		if r.Stage == domain.StageFinal && done {
			return StepDone
		}
		started = started || began
	}
	if started {
		return StepDoing
	}
	return EventArranged
}

// combinedProgress sums up the components' states.
func combinedProgress(states []string) string {
	done, began, arranged := len(states) > 0, false, false
	for _, st := range states {
		done = done && st == StepDone
		began = began || st == StepDoing || st == StepDone
		arranged = arranged || st != StepTodo
	}
	switch {
	case done:
		return StepDone
	case began:
		return StepDoing
	case arranged:
		return EventArranged
	}
	return StepTodo
}

// shortName drops the group from an event's name when the name is the one
// composed from its fields; a name set by hand stays as it is.
func shortName(e domain.Event) string {
	if e.GroupLabel != "" && e.Name == domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel) {
		return domain.ComposeEventName(e.Gender, e.Discipline, "")
	}
	return e.Name
}

// overviewStructure lays the meet out by group from the 报名进度 and the
// events in the 项目 page's order; a combined event takes the place of its
// first component.
func overviewStructure(p *meetPlan, summary *EntrySummary, combineds []store.CombinedData) []OverviewBranch {
	units := map[string]UnitEntries{}
	for _, u := range summary.Units {
		units[u.Unit.ID] = u
	}
	unit := func(id string, athletes int) OverviewUnit {
		u := units[id]
		return OverviewUnit{ID: id, Name: u.Unit.Name, Athletes: athletes, Entered: u.Entries > 0 || u.Teams > 0, Over: u.Over}
	}
	var out []OverviewBranch
	index := map[string]int{}
	if len(summary.Groups) == 0 {
		b := OverviewBranch{Athletes: len(p.athletes), Units: []OverviewUnit{}, Events: []OverviewEvent{}}
		for _, u := range summary.Units {
			b.Units = append(b.Units, unit(u.Unit.ID, u.Athletes))
		}
		index[""] = 0
		out = append(out, b)
	}
	for _, g := range summary.Groups {
		b := OverviewBranch{GroupID: g.ID, Name: g.Name, Athletes: g.Athletes, Units: []OverviewUnit{}, Events: []OverviewEvent{}}
		for _, m := range g.Members {
			b.Units = append(b.Units, unit(m.UnitID, m.Athletes))
		}
		index[g.ID] = len(out)
		out = append(out, b)
	}
	add := func(groupID string, e OverviewEvent) {
		i, ok := index[groupID]
		if !ok {
			if groupID != "" {
				return
			}
			// Events of no group in a meet whose units all have one.
			i = len(out)
			index[""] = i
			out = append(out, OverviewBranch{Name: entryform.NoGroup, Units: []OverviewUnit{}, Events: []OverviewEvent{}})
		}
		out[i].Events = append(out[i].Events, e)
	}
	owner := map[string]store.CombinedData{}
	for _, c := range combineds {
		for _, comp := range c.Components {
			owner[comp.EventID] = c
		}
	}
	shown := map[string]bool{}
	for _, e := range p.events {
		if e.CombinedID == "" {
			add(e.GroupID, OverviewEvent{ID: e.ID, Name: shortName(e), State: eventProgress(p.rounds[e.ID])})
			continue
		}
		c, ok := owner[e.ID]
		if !ok || shown[c.Combined.ID] {
			continue
		}
		shown[c.Combined.ID] = true
		var states []string
		for _, comp := range c.Components {
			states = append(states, eventProgress(p.rounds[comp.EventID]))
		}
		add(c.Combined.GroupID, OverviewEvent{ID: c.Combined.ID, Combined: true, Name: c.Combined.Name, State: combinedProgress(states)})
	}
	return out
}
