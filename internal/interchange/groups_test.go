package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestV8GroupsReferencesAndLegacyFields(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v8.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 8 || len(f.Groups) == 0 || len(f.Units[0].GroupIDs) == 0 || f.Athletes[0].GroupID == "" {
		t.Fatal("missing group references")
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatal("v8 roundtrip", err)
	}
	for _, mutate := range []func(*File){
		func(f *File) { f.Groups = append(f.Groups, f.Groups[0]) },
		func(f *File) { f.Groups[0].Ordinal = 0 },
		func(f *File) { f.Events[0].GroupID = "missing" },
		func(f *File) { f.Athletes[0].GroupID = "missing" },
		func(f *File) { f.Units[0].GroupIDs = []string{"g1", "g1"} },
		func(f *File) { f.Events[0].GroupLabel = "missing" },
	} {
		invalid, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		mutate(invalid)
		if len(Validate(invalid)) == 0 {
			t.Fatal("invalid references accepted")
		}
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["version"] = 7
	doc["groups"] = map[string]any{"future": true}
	for _, name := range []string{"events", "combineds", "athletes"} {
		for _, v := range doc[name].([]any) {
			v.(map[string]any)["groupId"] = map[string]any{"future": true}
		}
	}
	for _, v := range doc["units"].([]any) {
		v.(map[string]any)["groupIds"] = 17
	}
	legacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Decode(legacy)
	if err != nil || len(old.Groups) > 0 || old.Events[0].GroupID != "" || len(old.Units[0].GroupIDs) > 0 {
		t.Fatal("legacy fields interpreted", err)
	}
}
