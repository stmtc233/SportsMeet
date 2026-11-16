package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func genderFile(t *testing.T, version int, fields map[string]any) []byte {
	t.Helper()
	athlete := map[string]any{"id": "a", "name": "甲"}
	for key, value := range fields {
		athlete[key] = value
	}
	data, err := json.Marshal(map[string]any{"format": Format, "version": version, "exportedAt": "2026-11-16T10:00:00+08:00", "generator": map[string]string{"name": "SportsMeet", "version": "test"}, "meet": map[string]string{"id": "m", "name": "校运会", "startDate": "2026-11-16"}, "units": []any{}, "athletes": []any{athlete}, "events": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLegacyAthleteGenderHasNoIdentity(t *testing.T) {
	for version := 1; version < 12; version++ {
		// Unknown future fields are ignored even when their JSON types are invalid.
		data := genderFile(t, version, map[string]any{"gender": "女", "competitionGender": []any{"男"}, "genderIdentity": map[string]any{"value": "非二元"}})
		file, err := Decode(data)
		if err != nil {
			t.Fatalf("v%d: %v", version, err)
		}
		if file.Athletes[0].CompetitionGender != "女" || file.Athletes[0].GenderIdentity != "" {
			t.Fatalf("v%d inferred identity: %+v", version, file.Athletes[0])
		}
		encoded, err := Encode(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "competitionGender") || strings.Contains(string(encoded), "genderIdentity") {
			t.Fatal("legacy re-encoding emitted future fields")
		}
		if _, err := Decode(encoded); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV12AthleteGenderFields(t *testing.T) {
	for _, identity := range []string{"男", "女", "非二元", "其他", "不愿透露", ""} {
		file, err := Decode(genderFile(t, 12, map[string]any{"competitionGender": "女", "genderIdentity": identity, "gender": []any{"unknown legacy field"}}))
		if err != nil {
			t.Fatal(err)
		}
		if file.Athletes[0].CompetitionGender != "女" || file.Athletes[0].GenderIdentity != identity || file.Athletes[0].Gender != "" {
			t.Fatal(file.Athletes)
		}
	}
	for _, fields := range []map[string]any{{"competitionGender": "混合"}, {"competitionGender": "非二元"}, {"genderIdentity": "混合"}, {"genderIdentity": "NonBinary"}, {"genderIdentity": 42}, {"competitionGender": false}} {
		if _, err := Decode(genderFile(t, 12, fields)); err == nil {
			t.Fatalf("invalid fields accepted: %v", fields)
		}
	}
	file, err := Decode(genderFile(t, 12, map[string]any{"genderIdentity": "非二元"}))
	if err != nil || file.Athletes[0].CompetitionGender != "" {
		t.Fatal("identity inferred category", file, err)
	}
}

func TestV12DocumentedGoldenRoundTrip(t *testing.T) {
	golden, err := os.ReadFile("../../testdata/interchange/sample-meet-v12.json")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile("../../docs/examples/sample-meet.json")
	if err != nil || string(example) != string(golden) {
		t.Fatal("documented example differs from v12 golden")
	}
	file, err := Decode(golden)
	if err != nil {
		t.Fatal(err)
	}
	if file.Version != Version || file.Athletes[0].GenderIdentity != "非二元" || len(file.FrontPages) != 2 || len(file.Assets) != 1 {
		t.Fatal("v12 lost snapshot fields", file)
	}
	data, err := Encode(file)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := Decode(data)
	if err != nil || !reflect.DeepEqual(file, copy) {
		t.Fatal("v12 roundtrip changed snapshot", err)
	}
}
