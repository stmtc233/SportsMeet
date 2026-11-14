package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestV9EntryLimits(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v9.json")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile("../../docs/examples/sample-meet.json")
	if err != nil || string(example) != string(data) {
		t.Fatal("documented example differs from v9 golden")
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.EntryLimits{AthleteEvents: 2, UnitAthletes: 2, UnitTeams: 1}
	if f.Version != 9 || f.Meet.EntryLimits == nil || *f.Meet.EntryLimits != want {
		t.Fatalf("entry limits = %+v", f.Meet.EntryLimits)
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatal("v9 roundtrip", err)
	}
	f.Meet.EntryLimits.UnitTeams = domain.MaxEntryLimit + 1
	if len(Validate(f)) == 0 {
		t.Fatal("out-of-range limit accepted")
	}

	// A v8 file never had limits: whatever it carries there is ignored.
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["version"] = 8
	doc["meet"].(map[string]any)["entryLimits"] = "future"
	legacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Decode(legacy)
	if err != nil || old.Meet.EntryLimits != nil {
		t.Fatal("v8 entryLimits interpreted", err)
	}
}
