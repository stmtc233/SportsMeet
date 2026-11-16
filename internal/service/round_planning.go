package service

import "github.com/stmtc233/sportsmeet/internal/domain"

// roundTimeState counts effective times, not just the earliest time. An empty
// round can be scheduled before its qualifiers or even its entries are known.
func roundTimeState(r domain.Round) string {
	if len(r.Heats) == 0 {
		if r.ScheduledAt != "" {
			return StepDone
		}
		return StepTodo
	}
	timed := 0
	for _, h := range r.Heats {
		if at, _ := domain.EffectiveScheduled(r, h); at != "" {
			timed++
		}
	}
	return stepState(timed, len(r.Heats))
}

// qualifierProgress compares actual qualified competitors with the next round's
// slots. Empty groups and partially placed qualifiers do not finish advancement.
// Call only once the previous round is finished; Q/q come from saved scoring.
func qualifierProgress(prev, next domain.Round) (total, unplaced int) {
	placed := map[string]bool{}
	for _, h := range next.Heats {
		for _, sl := range h.Slots {
			placed[sl.CompetitorID()] = true
		}
	}
	for _, h := range prev.Heats {
		for _, sl := range h.Slots {
			if sl.Result != nil && sl.Result.Qualify != "" {
				total++
				if !placed[sl.CompetitorID()] {
					unplaced++
				}
			}
		}
	}
	return
}
