package scoring

import (
	"errors"
	"math"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

type CombinedSource struct {
	Event   domain.Event
	Rule    domain.PointRule
	Results map[string]*domain.Result
}
type CombinedCell struct {
	EventID  string        `json:"eventId"`
	MarkText string        `json:"markText"`
	Status   domain.Status `json:"status"`
	Points   *int64        `json:"points"`
	Pending  bool          `json:"pending"`
	Excluded bool          `json:"excluded"`
}
type CombinedStanding struct {
	AthleteID string         `json:"athleteId"`
	Cells     []CombinedCell `json:"cells"`
	Total     int64          `json:"total"`
	Rank      *int           `json:"rank"`
	Pending   bool           `json:"pending"`
	Excluded  bool           `json:"excluded"`
}

func Points(e domain.Event, r domain.PointRule, mark int64) int64 {
	for _, band := range r.Bands {
		if (e.Ascending() && mark <= band.Mark) || (!e.Ascending() && mark >= band.Mark) {
			return band.Points
		}
	}
	return 0
}
func CombinedRank(athletes []string, sources []CombinedSource) ([]CombinedStanding, bool, error) {
	rows := make([]CombinedStanding, 0, len(athletes))
	complete := len(athletes) > 0
	for _, id := range athletes {
		row := CombinedStanding{AthleteID: id, Cells: []CombinedCell{}}
		for _, src := range sources {
			c := CombinedCell{EventID: src.Event.ID}
			result := src.Results[id]
			if result == nil {
				c.Pending = true
				row.Pending = true
				complete = false
			} else {
				if result.HeightPending {
					c.Pending, row.Pending, complete = true, true, false
				}
				c.Status = result.Status
				var pts int64
				if result.Status == domain.StatusOK && result.Mark != nil {
					c.MarkText = FormatEventMark(src.Event, *result.Mark)
					pts = Points(src.Event, src.Rule, *result.Mark)
				} else {
					c.MarkText = result.Status.Label()
					c.Excluded = src.Rule.InvalidPolicies[string(result.Status)] != "zero"
				}
				if !c.Excluded {
					c.Points = &pts
					if pts > math.MaxInt64-row.Total {
						return nil, false, errors.New("总分超出范围")
					}
					row.Total += pts
				} else {
					row.Excluded = true
				}
			}
			row.Cells = append(row.Cells, c)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Excluded != b.Excluded {
			return !a.Excluded
		}
		return a.Total > b.Total
	})
	place := 0
	lastTotal := int64(-1)
	lastRank := 0
	for i := range rows {
		if rows[i].Excluded {
			continue
		}
		place++
		if rows[i].Total != lastTotal {
			lastRank = place
			lastTotal = rows[i].Total
		}
		rank := lastRank
		rows[i].Rank = &rank
	}
	return rows, complete, nil
}
