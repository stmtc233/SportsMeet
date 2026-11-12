package arrange

import (
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// TimedHeat is one heat as the conflict check sees it: its effective
// planned start, its expected duration and who competes in it (a team
// heat lists the team's members).
type TimedHeat struct {
	HeatID   string
	Start    string // effective YYYY-MM-DDTHH:mm; "" or invalid = not checked
	Minutes  *int   // nil = a point in time (ends when it starts)
	Athletes []string
}

// Conflict is one athlete in two heats too close together. First starts
// no later than Second.
type Conflict struct {
	AthleteID string
	First     string // heat id
	Second    string // heat id
	// Overlap is true when the two heats' times overlap, or both start at
	// the same minute; otherwise the rest between them is shorter than
	// the minimum.
	Overlap bool
	// GapMinutes is Second's start minus First's end; negative when they overlap.
	GapMinutes int
}

type span struct {
	heat       string
	start, end int64 // minutes
}

// Conflicts lists, for every athlete, the pairs of heats whose times
// overlap or leave less than minRest minutes between the end of one and
// the start of the next (docs/agents/arrange.md「冲突检查」). Heats without
// a valid start are left out; a heat without a duration is a point in
// time. Per athlete the heats are sorted and scanned once, keeping only
// those that can still reach the next start, so the work grows with the
// heats each athlete is in, not with all pairs of heats.
func Conflicts(heats []TimedHeat, minRest int) []Conflict {
	if minRest < 0 {
		minRest = 0
	}
	byAthlete := map[string][]span{}
	for _, h := range heats {
		t, ok := domain.ParseScheduled(h.Start)
		if !ok {
			continue
		}
		start := t.Unix() / 60
		end := start
		if h.Minutes != nil && *h.Minutes > 0 {
			end += int64(*h.Minutes)
		}
		seen := map[string]bool{}
		for _, a := range h.Athletes {
			if a == "" || seen[a] {
				continue // one athlete is in a heat once
			}
			seen[a] = true
			byAthlete[a] = append(byAthlete[a], span{h.HeatID, start, end})
		}
	}
	athletes := make([]string, 0, len(byAthlete))
	for a := range byAthlete {
		athletes = append(athletes, a)
	}
	sort.Strings(athletes)

	rest := int64(minRest)
	var out []Conflict
	for _, a := range athletes {
		list := byAthlete[a]
		sort.Slice(list, func(i, j int) bool {
			if list[i].start != list[j].start {
				return list[i].start < list[j].start
			}
			if list[i].end != list[j].end {
				return list[i].end < list[j].end
			}
			return list[i].heat < list[j].heat
		})
		var open []span
		for _, cur := range list {
			kept := open[:0]
			for _, prev := range open {
				// Starts only grow: a heat that ended long enough before this
				// start ends long enough before every later one too.
				if prev.end+rest <= cur.start && prev.start < cur.start {
					continue
				}
				kept = append(kept, prev)
				if prev.heat == cur.heat {
					continue
				}
				overlap := cur.start < prev.end || cur.start == prev.start
				out = append(out, Conflict{AthleteID: a, First: prev.heat, Second: cur.heat,
					Overlap: overlap, GapMinutes: int(cur.start - prev.end)})
			}
			open = append(kept, cur)
		}
	}
	return out
}
