package scoring

import (
	"sort"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Ranked is one competitor to rank: its key and result. Attempts and Bars
// break ties between equal field marks; leave them nil when unknown.
type Ranked struct {
	Key      string
	Status   domain.Status
	Mark     int64 // only read when Status is OK
	Attempts []domain.Attempt
	Bars     []domain.Bar
	// TieBreak is the referee's ruling among competitors equal on every
	// rule below: lower is ahead, equal stays tied, 0 = no ruling.
	TieBreak int
}

// Rank returns key → place for valid marks. Equal marks share a place and
// the next place skips by the tie (two tied 2nd → next is 4th). Non-OK
// results get no place.
//
// Equal field marks are separated first:
//   - distance: by the second best valid attempt, then the third, and so on
//     (a missing attempt loses to any attempt);
//   - height: by fewer failures at the height last cleared, then by fewer
//     failures in the whole competition up to and including that height.
//
// Competitors still equal after that share the place, unless every one of
// them carries a ruling (TieBreak): then they are placed in its order and
// only equal rulings share a place. A tie with a competitor who has no
// ruling stays whole, so a ruling never decides who it did not cover.
func Rank(kind domain.EventKind, list []Ranked) map[string]int {
	places := make(map[string]int, len(list))
	start := 0
	for _, g := range tied(kind, list) {
		if ruled(g) {
			sort.SliceStable(g, func(i, j int) bool { return g[i].TieBreak < g[j].TieBreak })
		}
		for i, r := range g {
			switch {
			case i == 0 || (ruled(g) && r.TieBreak != g[i-1].TieBreak):
				places[r.Key] = start + i + 1
			default:
				places[r.Key] = places[g[i-1].Key]
			}
		}
		start += len(g)
	}
	return places
}

// Tie is two or more valid results equal on every rule of Rank, so only a
// ruling separates them.
type Tie struct {
	Place int      // the place they share without a ruling
	Keys  []string // in ruled order when Ruled, else in input order
	Ruled bool     // every one carries a ruling
}

// Ties lists the ties of Rank, best place first.
func Ties(kind domain.EventKind, list []Ranked) []Tie {
	var out []Tie
	start := 0
	for _, g := range tied(kind, list) {
		if len(g) > 1 {
			t := Tie{Place: start + 1, Ruled: ruled(g)}
			if t.Ruled {
				sort.SliceStable(g, func(i, j int) bool { return g[i].TieBreak < g[j].TieBreak })
			}
			for _, r := range g {
				t.Keys = append(t.Keys, r.Key)
			}
			out = append(out, t)
		}
		start += len(g)
	}
	return out
}

func ruled(g []Ranked) bool {
	for _, r := range g {
		if r.TieBreak <= 0 {
			return false
		}
	}
	return true
}

// tied orders the valid results best first, in runs equal on every rule
// but the ruling; each run keeps the input order.
func tied(kind domain.EventKind, list []Ranked) [][]Ranked {
	type row struct {
		r   Ranked
		cmp []int64 // higher is better, compared left to right
	}
	rows := make([]row, 0, len(list))
	for _, r := range list {
		if r.Status == domain.StatusOK {
			rows = append(rows, row{r, rankKey(kind, r)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return compareKeys(rows[i].cmp, rows[j].cmp) > 0 })
	var out [][]Ranked
	for i, r := range rows {
		if i > 0 && compareKeys(rows[i-1].cmp, r.cmp) == 0 {
			out[len(out)-1] = append(out[len(out)-1], r.r)
			continue
		}
		out = append(out, []Ranked{r.r})
	}
	return out
}

func compareKeys(a, b []int64) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		switch {
		case i >= len(a):
			return -1
		case i >= len(b):
			return 1
		case a[i] != b[i]:
			if a[i] > b[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func rankKey(kind domain.EventKind, r Ranked) []int64 {
	switch kind {
	case domain.KindCount, domain.KindScore:
		return []int64{r.Mark}
	case domain.KindDistance:
		key := []int64{r.Mark}
		var rest []int64
		for _, a := range r.Attempts {
			if a.Outcome == "valid" && a.Mark != nil {
				rest = append(rest, *a.Mark)
			}
		}
		sort.Slice(rest, func(i, j int) bool { return rest[i] > rest[j] })
		if len(rest) > 0 {
			key = append(key, rest[1:]...) // rest[0] is the mark itself
		}
		return key
	case domain.KindHeight:
		var atLast, total int64
		for _, b := range r.Bars {
			if b.Height > r.Mark {
				break
			}
			f := int64(strings.Count(b.Trials, "X"))
			total += f
			if b.Height == r.Mark {
				atLast = f
			}
		}
		return []int64{r.Mark, -atLast, -total}
	}
	return []int64{-r.Mark}
}
