package interchange

import (
	"encoding/json"
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// The results format (docs/results-format.md) carries finished marks of
// some heats back into a meet, located by the ids of an exported file.
const (
	ResultsFormat  = "sportsmeet-results"
	ResultsVersion = 1
)

// ResultsFile is a decoded results file.
type ResultsFile struct {
	Format     string        `json:"format"`
	Version    int           `json:"version"`
	ExportedAt string        `json:"exportedAt"`
	Generator  *Generator    `json:"generator"`
	MeetID     string        `json:"meetId"`
	Heats      []ResultsHeat `json:"heats"`
}

// ResultsHeat is the marks of one heat.
type ResultsHeat struct {
	HeatID    string       `json:"heatId"`
	Label     string       `json:"label,omitempty"`
	StartedAt *string      `json:"startedAt,omitempty"`
	Results   []HeatResult `json:"results"`
}

// HeatResult is one competitor's mark.
type HeatResult struct {
	Position  int          `json:"position"`
	AthleteID string       `json:"athleteId,omitempty"`
	TeamID    string       `json:"teamId,omitempty"`
	Check     *ResultCheck `json:"check,omitempty"`
	Status    string       `json:"status"`
	Mark      *int64       `json:"mark"`
	Note      string       `json:"note,omitempty"`
}

// ResultCheck identifies the competitor when the id no longer does.
type ResultCheck struct {
	Name          string `json:"name,omitempty"`
	StudentNumber string `json:"studentNumber,omitempty"`
	Bib           string `json:"bib,omitempty"`
}

// CompetitorID is the athlete or team the result is for.
func (r HeatResult) CompetitorID() string {
	if r.TeamID != "" {
		return r.TeamID
	}
	return r.AthleteID
}

// DecodeResults parses and validates a results file (spec §5 items 1–6;
// the target meet is the caller's). A non-nil error is a JSON error,
// ErrNewerVersion, or Problems.
func DecodeResults(data []byte) (*ResultsFile, error) {
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("不是有效的 JSON:%w", err)
	}
	if head.Format != ResultsFormat {
		return nil, Problems{"format 必须为 " + ResultsFormat}
	}
	if head.Version > ResultsVersion {
		return nil, ErrNewerVersion
	}
	if head.Version < 1 {
		return nil, Problems{"version 必须为正整数"}
	}
	var f ResultsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("字段类型不正确:%w", err)
	}
	if p := validateResults(&f); len(p) > 0 {
		return nil, p
	}
	return &f, nil
}

func validateResults(f *ResultsFile) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if f.ExportedAt == "" {
		add("缺少 exportedAt")
	}
	if f.Generator == nil || f.Generator.Name == "" {
		add("缺少 generator.name")
	}
	if f.MeetID == "" {
		add("缺少 meetId")
	}
	if f.Heats == nil {
		add("缺少 heats")
	}
	heats := map[string]bool{}
	for i, h := range f.Heats {
		at := fmt.Sprintf("heats[%d]", i)
		switch {
		case h.HeatID == "":
			add("%s 缺少 heatId", at)
		case heats[h.HeatID]:
			add("%s.heatId 重复", at)
		}
		heats[h.HeatID] = true
		if h.StartedAt != nil && !domain.ValidScheduled(*h.StartedAt) {
			add("%s.startedAt 应为 YYYY-MM-DDTHH:mm", at)
		}
		if h.Results == nil {
			add("%s 缺少 results", at)
		}
		positions, competitors := map[int]bool{}, map[string]bool{}
		for j, r := range h.Results {
			rat := fmt.Sprintf("%s.results[%d]", at, j)
			if r.Position < 1 {
				add("%s.position 应从 1 开始", rat)
			} else if positions[r.Position] {
				add("%s.position 重复", rat)
			}
			positions[r.Position] = true
			switch {
			case (r.AthleteID == "") == (r.TeamID == ""):
				add("%s 应填 athleteId 或 teamId 其中一个", rat)
			case competitors[r.CompetitorID()]:
				add("%s 的运动员或队伍重复", rat)
			}
			competitors[r.CompetitorID()] = true
			status := domain.Status(r.Status)
			switch {
			case !status.Valid():
				add("%s.status 不正确", rat)
			case status == domain.StatusOK && (r.Mark == nil || *r.Mark <= 0):
				add("%s.mark 应为正整数", rat)
			case status != domain.StatusOK && r.Mark != nil:
				add("%s.mark 应为 null", rat)
			}
		}
	}
	return p
}
