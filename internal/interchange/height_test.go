package interchange

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestV6HeightProgress(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v6.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	pending := false
	for _, event := range f.Events {
		for _, round := range event.Rounds {
			for _, heat := range round.Heats {
				for _, slot := range heat.Slots {
					if slot.Result != nil {
						pending = pending || slot.Result.HeightPending
					}
				}
			}
		}
	}
	if !pending || f.Version != 6 {
		t.Fatal("missing height progress")
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatalf("round trip: %v", err)
	}
	for _, value := range []string{`true`, `"unknown"`, `{"extension":true}`} {
		legacy := strings.Replace(string(data), `"version": 6`, `"version": 5`, 1)
		legacy = strings.ReplaceAll(legacy, `"heightPending": true`, `"heightPending": `+value)
		old, err := Decode([]byte(legacy))
		if err != nil {
			t.Fatalf("v5 unknown field %s: %v", value, err)
		}
		for _, event := range old.Events {
			for _, round := range event.Rounds {
				for _, heat := range round.Heats {
					for _, slot := range heat.Slots {
						if slot.Result != nil && slot.Result.HeightPending {
							t.Fatal("v5 interpreted height progress")
						}
					}
				}
			}
		}
		if old.Events[0].Rounds[0].Heats[0].StartedAt == nil {
			t.Fatal("v5 start lost")
		}
	}
	if _, err := Decode([]byte(strings.ReplaceAll(string(data), `"heightPending": true`, `"heightPending": "bad"`))); err == nil {
		t.Fatal("accepted wrong type")
	}
}
