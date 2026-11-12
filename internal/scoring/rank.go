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
// Competitors still equal after that share the place.
func Rank(kind domain.EventKind, list []Ranked) map[string]int {
	type row struct {
		key string
		cmp []int64 // higher is better, compared left to right
	}
	rows := make([]row, 0, len(list))
	for _, r := range list {
		if r.Status == domain.StatusOK {
			rows = append(rows, row{r.Key, rankKey(kind, r)})
		}
	}
	compare := func(a, b []int64) int {
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
	sort.SliceStable(rows, func(i, j int) bool { return compare(rows[i].cmp, rows[j].cmp) > 0 })
	places := make(map[string]int, len(rows))
	for i, r := range rows {
		if i > 0 && compare(rows[i-1].cmp, r.cmp) == 0 {
			places[r.key] = places[rows[i-1].key]
			continue
		}
		places[r.key] = i + 1
	}
	return places
}

func rankKey(kind domain.EventKind, r Ranked) []int64 {
	switch kind {
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
