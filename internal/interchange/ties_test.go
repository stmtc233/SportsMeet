package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestV10TieBreak(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v10.json")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile("../../docs/examples/sample-meet.json")
	if err != nil || string(example) != string(data) {
		t.Fatal("documented example differs from v10 golden")
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	var ruled *Result
	for _, e := range f.Events {
		for _, r := range e.Rounds {
			for _, h := range r.Heats {
				for _, s := range h.Slots {
					if ruled == nil && s.Result != nil && s.Result.Status == "OK" {
						ruled = s.Result
					}
				}
			}
		}
	}
	ruled.TieBreak = 2
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"tieBreak": 2`) {
		t.Fatal("tieBreak not written")
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatal("v10 roundtrip", err)
	}
	ruled.TieBreak = -1
	if len(Validate(f)) == 0 {
		t.Fatal("negative tieBreak accepted")
	}

	// A v9 file never had rulings: whatever it carries there is ignored.
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	doc["version"] = 9
	legacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Decode(legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range old.Events {
		for _, r := range e.Rounds {
			for _, h := range r.Heats {
				for _, s := range h.Slots {
					if s.Result != nil && s.Result.TieBreak != 0 {
						t.Fatal("v9 tieBreak interpreted")
					}
				}
			}
		}
	}
}
