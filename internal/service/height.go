package service

import (
	"strconv"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
)

// HeightCellPreview describes one editable height, including permitted next trials.
type HeightCellPreview struct {
	Height     string `json:"height"`
	Trials     string `json:"trials"`
	Error      string `json:"error"`
	CanSuccess bool   `json:"canSuccess"`
	CanFailure bool   `json:"canFailure"`
	CanPass    bool   `json:"canPass"`
	// State is the cell's place in the rotation: open (still to jump),
	// done (cleared, passed or ended by a third failure), skipped (empty
	// below a height already jumped), out (eliminated earlier) or off
	// (a status is set or the row does not parse).
	State string `json:"state"`
}

type HeightRowPreview struct {
	Position int                 `json:"position"`
	MarkText string              `json:"markText"`
	Rank     *int                `json:"rank"`
	Error    string              `json:"error"`
	Cells    []HeightCellPreview `json:"cells"`
}

// HeightNext is the trial that comes next in the jumping order.
type HeightNext struct {
	Position int    `json:"position"`
	Height   string `json:"height"`
	Attempt  int    `json:"attempt"`
}

// HeightPreview is a read-only view of a heat's draft; it never writes results.
type HeightPreview struct {
	Rows      []HeightRowPreview `json:"rows"`
	CanFinish bool               `json:"canFinish"`
	Valid     bool               `json:"valid"`
	// Next is nil when every athlete is done with every height.
	Next *HeightNext `json:"next"`
	// NextHeight suggests the bar after the highest one: the last raise
	// repeated, or 5 cm after a single height.
	NextHeight string `json:"nextHeight"`
}

func PreviewHeight(inputs []ResultInput) HeightPreview {
	event := domain.Event{Kind: domain.KindHeight}
	out := HeightPreview{Rows: []HeightRowPreview{}, Valid: true, CanFinish: len(inputs) > 0}
	rankedRows := []scoring.Ranked{}
	for _, input := range inputs {
		row := HeightRowPreview{Position: input.Position, Cells: []HeightCellPreview{}}
		for i, b := range input.Bars {
			cell := HeightCellPreview{Height: b.Height}
			if strings.TrimSpace(b.Trials) != "" {
				trials, err := scoring.ParseTrials(b.Trials)
				if err != nil {
					cell.Error = err.Error()
				} else {
					cell.Trials = trials
				}
			}
			allowed := func(symbol string) bool {
				if input.Status != "" || cell.Error != "" {
					return false
				}
				candidate := input
				candidate.Bars = append([]BarInput(nil), input.Bars...)
				candidate.Bars[i].Trials = cell.Trials + symbol
				_, err := buildResultEvent(event, candidate)
				return err == nil
			}
			cell.CanSuccess, cell.CanFailure, cell.CanPass = allowed("O"), allowed("X"), allowed("-")
			row.Cells = append(row.Cells, cell)
		}
		r, err := buildResultEvent(event, input)
		if err != nil {
			row.Error = err.Error()
			out.Valid, out.CanFinish = false, false
		} else if r == nil {
			out.CanFinish = false
		} else {
			if r.Mark != nil {
				row.MarkText = scoring.FormatMark(domain.KindHeight, *r.Mark)
			}
			if r.Status == domain.StatusNM {
				row.MarkText = "未过杆"
				if input.Status != "" {
					row.MarkText = r.Status.Label()
				}
			}
			rankedRows = append(rankedRows, ranked(strconv.Itoa(input.Position), r))
		}
		out.Rows = append(out.Rows, row)
	}
	markStates(inputs, out.Rows)
	out.Next = nextTrial(inputs, out.Rows)
	out.NextHeight = nextHeight(inputs)
	places := scoring.Rank(domain.KindHeight, rankedRows)
	for i := range out.Rows {
		if rank, ok := places[strconv.Itoa(out.Rows[i].Position)]; ok {
			out.Rows[i].Rank = &rank
		}
	}
	return out
}

// markStates places every cell in the rotation. Bars arrive in ascending
// height order, as the editor sends them.
func markStates(inputs []ResultInput, rows []HeightRowPreview) {
	for r, input := range inputs {
		row := &rows[r]
		off := input.Status != "" || row.Error != ""
		last := -1
		for i, cell := range row.Cells {
			if cell.Trials != "" {
				last = i
			}
		}
		failures := 0
		for i := range row.Cells {
			cell := &row.Cells[i]
			trials := cell.Trials
			switch {
			case off:
				cell.State = "off"
			case trials == "" && failures >= 3:
				cell.State = "out"
			case trials == "" && i < last:
				cell.State = "skipped"
			}
			for _, c := range trials {
				switch c {
				case 'O':
					failures = 0
				case 'X':
					failures++
				}
			}
			if cell.State != "" {
				continue
			}
			if strings.ContainsAny(trials, "O-") || len(trials) >= 3 || failures >= 3 {
				cell.State = "done"
			} else {
				cell.State = "open"
			}
		}
	}
}

// nextTrial follows the jumping order: at the lowest height anyone still
// has to jump, everyone takes a first trial in position order, then the
// second trials, then the third.
func nextTrial(inputs []ResultInput, rows []HeightRowPreview) *HeightNext {
	if len(rows) == 0 {
		return nil
	}
	for i := range rows[0].Cells {
		var next *HeightNext
		for r, row := range rows {
			if i >= len(row.Cells) || row.Cells[i].State != "open" {
				continue
			}
			attempt := len(row.Cells[i].Trials) + 1
			if next == nil || attempt < next.Attempt {
				next = &HeightNext{Position: inputs[r].Position, Height: row.Cells[i].Height, Attempt: attempt}
			}
		}
		if next != nil {
			return next
		}
	}
	return nil
}

func nextHeight(inputs []ResultInput) string {
	if len(inputs) == 0 {
		return ""
	}
	heights := []int64{}
	for _, b := range inputs[0].Bars {
		h, err := scoring.ParseMark(domain.KindHeight, b.Height)
		if err != nil {
			return ""
		}
		heights = append(heights, h)
	}
	n := len(heights)
	switch {
	case n == 0:
		return ""
	case n == 1:
		return scoring.FormatMark(domain.KindHeight, heights[0]+50)
	case heights[n-1] > heights[n-2]:
		return scoring.FormatMark(domain.KindHeight, 2*heights[n-1]-heights[n-2])
	}
	return ""
}
