package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func resultsFile(t *testing.T, meetID, heatID string, results ...interchange.HeatResult) []byte {
	t.Helper()
	started := "2026-11-12T09:30"
	if results == nil {
		results = []interchange.HeatResult{}
	}
	b, err := json.Marshal(interchange.ResultsFile{Format: interchange.ResultsFormat, Version: 1, ExportedAt: "2026-11-12T10:00:00+08:00",
		Generator: &interchange.Generator{Name: "test", Version: "1"}, MeetID: meetID,
		Heats: []interchange.ResultsHeat{{HeatID: heatID, StartedAt: &started, Results: results}}})
	must(t, err)
	return b
}

func ms(v int64) *int64 { return &v }

func TestImportResults(t *testing.T) {
	f := newFixture(t, 4, track())
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes...)
	heat := f.detail().Rounds[0].Heats[0].ID

	data := resultsFile(t, f.meet, heat,
		interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "OK", Mark: ms(12340)},
		interchange.HeatResult{Position: 2, AthleteID: f.athletes[1], Status: "DNS"},
		interchange.HeatResult{Position: 3, AthleteID: "gone", Check: &interchange.ResultCheck{Bib: " 003 "}, Status: "OK", Mark: ms(12500)},
		interchange.HeatResult{Position: 1 + 4, AthleteID: f.athletes[3], Status: "OK", Mark: ms(11000)},
	)
	p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) != 0 || p.Heats != 1 || p.ResultsNew != 4 || p.ResultsOverwritten != 0 || len(p.Warnings) != 2 {
		t.Fatalf("preview = %+v", p)
	}
	if _, err := f.svc.ImportResults(f.ctx, ui, data, f.meet, "stale"); err != ErrStale {
		t.Fatalf("stale hash: err = %v", err)
	}
	_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
	must(t, err)

	h := f.detail().Rounds[0].Heats[0]
	if h.StartedAt != "2026-11-12T09:30" {
		t.Fatalf("startedAt = %q", h.StartedAt)
	}
	wantRanks := []int{2, 0, 3, 1} // places come from scoring; DNS has none
	for i, sl := range h.Slots {
		r := sl.Result
		if r == nil {
			t.Fatalf("slot %d has no result", i+1)
		}
		got := 0
		if r.Rank != nil {
			got = *r.Rank
		}
		if got != wantRanks[i] {
			t.Fatalf("slot %d rank = %d, want %d", i+1, got, wantRanks[i])
		}
	}

	// Again with one mark changed: one overwrite, the rest unchanged.
	data = resultsFile(t, f.meet, heat,
		interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "OK", Mark: ms(10990)},
		interchange.HeatResult{Position: 2, AthleteID: f.athletes[1], Status: "DNS"},
	)
	p, err = f.svc.PreviewResultsImport(f.ctx, data, f.meet)
	must(t, err)
	if p.ResultsOverwritten != 1 || p.ResultsUnchanged != 1 || len(p.Overwrites) != 1 || p.Overwrites[0].After == "" {
		t.Fatalf("second preview = %+v", p)
	}
	_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
	must(t, err)
	if r := f.detail().Rounds[0].Heats[0].Slots[0].Result; r.Rank == nil || *r.Rank != 1 {
		t.Fatalf("rescored rank = %+v", r)
	}
}

func TestImportResultsRefusals(t *testing.T) {
	f := newFixture(t, 2, track())
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes...)
	heat := f.detail().Rounds[0].Heats[0].ID

	cases := map[string][]byte{
		"不属于本赛事": resultsFile(t, "other", heat),
		"mark":   resultsFile(t, f.meet, heat, interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "OK"}),
		"重复":     resultsFile(t, f.meet, heat, interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "DNS"}, interchange.HeatResult{Position: 1, AthleteID: f.athletes[1], Status: "DNS"}),
		"升级":     []byte(`{"format":"sportsmeet-results","version":3}`),
		"format": []byte(`{"format":"sportsmeet-interchange","version":1}`),
	}
	for want, data := range cases {
		p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
		must(t, err)
		if !strings.Contains(strings.Join(p.Errors, "\n"), want) {
			t.Fatalf("%s: errors = %v", want, p.Errors)
		}
		_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
		wantInvalid(t, err, "文件有误")
	}

	// A heat elsewhere is skipped with a warning, not an error.
	p, err := f.svc.PreviewResultsImport(f.ctx, resultsFile(t, f.meet, "missing", interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "DNS"}), f.meet)
	must(t, err)
	if len(p.Errors) != 0 || len(p.Warnings) != 1 || p.ResultsNew != 0 {
		t.Fatalf("missing heat = %+v", p)
	}
}
