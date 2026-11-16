package service

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// The layout form's tests and the UI tests read the limits from this file,
// so they check the form against the limits Go enforces.
func TestPrintParamRulesGolden(t *testing.T) {
	const path = "../../testdata/print/param-rules.json"
	got, err := json.MarshalIndent((&Service{}).PrintParamRules(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the parameter limits changed: run go test ./internal/service -run TestPrintParamRulesGolden -update\n%s", got)
	}
}

// Every limit names a parameter Validate reads, and its bounds are the ones
// Validate enforces.
func TestNumberLimitsAreValidates(t *testing.T) {
	for _, l := range printstyle.NumberLimits {
		raw := func(v float64) []byte {
			m := map[string]any{l.Key: v}
			if l.Key == "paperWidth" || l.Key == "paperHeight" {
				m["paper"] = printstyle.PaperCustom
				m["paperWidth"], m["paperHeight"] = 300.0, 300.0
				m[l.Key] = v
			}
			b, _ := json.Marshal(m)
			return b
		}
		if _, err := printstyle.ParamsFromJSON(raw(l.Min)); err != nil {
			t.Errorf("%s at its min: %v", l.Key, err)
		}
		if _, err := printstyle.ParamsFromJSON(raw(l.Max)); err != nil && err.Error() != printstyle.MarginsTooBig {
			t.Errorf("%s at its max: %v", l.Key, err)
		}
		if _, err := printstyle.ParamsFromJSON(raw(l.Max + 1)); err == nil {
			t.Errorf("%s above its max was accepted", l.Key)
		}
	}
}
