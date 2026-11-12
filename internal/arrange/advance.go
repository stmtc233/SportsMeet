package arrange

import (
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Finisher is one result of a round, for advancement.
type Finisher struct {
	ID     string
	Status domain.Status
	Mark   int64
	Rank   int // place inside its heat, 0 = none
}

// Qualify marks who advances from a finished round: Q for the first
// byPlace of each heat, q for the byMark best of the others. Competitors
// tied on the last place or on the last mark all advance, since nothing in
// the results separates them; the referee removes the extra one by hand.
func QualifyEvent(event domain.Event, heats [][]Finisher, byPlace, byMark int) map[string]string {
	out := map[string]string{}
	var rest []Finisher
	for _, heat := range heats {
		for _, f := range heat {
			switch {
			case f.Status != domain.StatusOK || f.Rank == 0:
			case f.Rank <= byPlace:
				out[f.ID] = "Q"
			default:
				rest = append(rest, f)
			}
		}
	}
	sort.SliceStable(rest, func(i, j int) bool {
		if event.Ascending() {
			return rest[i].Mark < rest[j].Mark
		}
		return rest[i].Mark > rest[j].Mark
	})
	for i, f := range rest {
		if byMark <= 0 || (i >= byMark && f.Mark != rest[byMark-1].Mark) {
			break
		}
		out[f.ID] = "q"
	}
	return out
}

func Qualify(kind domain.EventKind, h [][]Finisher, p, m int) map[string]string {
	return QualifyEvent(domain.Event{Kind: kind}, h, p, m)
}
