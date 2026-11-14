package arrange

import (
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Finisher is one result of a round, for advancement.
type Finisher struct {
	ID     string
	Status domain.Status
	Order  int // place in the whole round's order, ties shared; 0 = none
	Rank   int // place inside its heat, 0 = none
}

// Qualify marks who advances from a finished round: Q for the first
// byPlace of each heat, q for the byMark best of the others by the round's
// order. Competitors sharing the last place or the last order all advance;
// a referee's tie ruling, already in Rank and Order, is what separates them
// (docs/agents/domain.md「并列裁定」).
func Qualify(heats [][]Finisher, byPlace, byMark int) map[string]string {
	out := map[string]string{}
	var rest []Finisher
	for _, heat := range heats {
		for _, f := range heat {
			switch {
			case f.Status != domain.StatusOK || f.Rank == 0 || f.Order == 0:
			case f.Rank <= byPlace:
				out[f.ID] = "Q"
			default:
				rest = append(rest, f)
			}
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Order < rest[j].Order })
	place := 0
	for i, f := range rest {
		if i == 0 || f.Order != rest[i-1].Order {
			place = i + 1
		}
		if place > byMark {
			break
		}
		out[f.ID] = "q"
	}
	return out
}
