package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestV7TeamScoringAndLegacyUnknownFields(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v7.json")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile("../../docs/examples/sample-meet.json")
	if err != nil || string(example) != string(data) {
		t.Fatal("documented example differs from v7 golden")
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 7 || f.Meet.TeamScoring == nil || !f.Meet.TeamScoring.Enabled || len(f.Meet.TeamScoring.Overrides) == 0 {
		t.Fatal("missing v7 rules")
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatal("v7 roundtrip", err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"enabled", "places", "tiePolicy", "relayMultiplier", "overrides"} {
		var copyDoc map[string]any
		if err := json.Unmarshal(data, &copyDoc); err != nil {
			t.Fatal(err)
		}
		delete(copyDoc["meet"].(map[string]any)["teamScoring"].(map[string]any), key)
		bad, err := json.Marshal(copyDoc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Decode(bad); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
	for _, v := range []any{true, "extension", map[string]any{"future": true}} {
		doc["version"] = 6
		doc["meet"].(map[string]any)["teamScoring"] = v
		legacy, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		old, err := Decode(legacy)
		if err != nil || old.Meet.TeamScoring != nil {
			t.Fatal("v6 interpreted unknown teamScoring", err)
		}
	}
	for _, change := range []func(*domain.TeamScoringRules){
		func(r *domain.TeamScoringRules) { r.Places = []int64{9, -1} },
		func(r *domain.TeamScoringRules) { r.Places = []int64{7, 9} },
		func(r *domain.TeamScoringRules) { r.TiePolicy = "countback" },
		func(r *domain.TeamScoringRules) { r.RelayMultiplier = -1 },
		func(r *domain.TeamScoringRules) {
			r.Overrides = []domain.TeamScoreOverride{{EventID: "missing", Multiplier: 1}}
		},
		func(r *domain.TeamScoringRules) { r.Overrides = append(r.Overrides, r.Overrides[0]) },
		func(r *domain.TeamScoringRules) {
			r.Overrides = []domain.TeamScoreOverride{{EventID: "e", CombinedID: "c", Multiplier: 1}}
		},
	} {
		bad, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		change(bad.Meet.TeamScoring)
		if len(Validate(bad)) == 0 {
			t.Fatal("accepted invalid rules")
		}
	}
	if len(f.Combineds) > 0 && len(f.Combineds[0].Components) > 0 {
		f.Meet.TeamScoring.Overrides = []domain.TeamScoreOverride{{EventID: f.Combineds[0].Components[0].EventID, Multiplier: 1}}
		if len(Validate(f)) == 0 {
			t.Fatal("component independently scored")
		}
	}
}
