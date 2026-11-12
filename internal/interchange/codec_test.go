package interchange

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func readSample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/interchange/sample-meet.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeSampleAndRoundTrip(t *testing.T) {
	f, err := Decode(readSample(t))
	if err != nil {
		t.Fatalf("Decode(sample) = %v", err)
	}
	if len(f.Events) != 3 || f.Events[1].Rounds[0].Heats[0].Slots[0].Result.Attempts[2].Outcome != "valid" {
		t.Fatalf("sample decoded unexpectedly: %+v", f.Events)
	}
	out, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode(Encode(sample)) = %v", err)
	}
	if !reflect.DeepEqual(f, again) {
		t.Fatal("round trip changed the file")
	}
}

func TestDecodeRejects(t *testing.T) {
	sample := string(readSample(t))
	cases := []struct {
		name, from, to, want string
	}{
		{"newer version", `"version": 1`, `"version": 4`, ""},
		{"wrong format", `"sportsmeet-interchange"`, `"other"`, "format"},
		{"OK without mark", `"status": "OK", "mark": 12650`, `"status": "OK", "mark": null`, "mark 必填"},
		{"unknown athlete", `"athleteId": "a3", "seedMark": 13100`, `"athleteId": "zz", "seedMark": 13100`, "找不到运动员 zz"},
		{"duplicate bib", `"bib": "1002"`, `"bib": "1001"`, "bib 1001 重复"},
		{"slot not entered", `{ "position": 5, "athleteId": "a3"`, `{ "position": 5, "athleteId": "a4"`, "未在本项目报名"},
		{"bad trials", `"trials": "XO"`, `"trials": "OO?"`, "trials 不正确"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := strings.Replace(sample, c.from, c.to, 1)
			if data == sample {
				t.Fatalf("pattern %q not found in sample", c.from)
			}
			_, err := Decode([]byte(data))
			if c.want == "" {
				if !errors.Is(err, ErrNewerVersion) {
					t.Fatalf("err = %v, want ErrNewerVersion", err)
				}
				return
			}
			var p Problems
			if !errors.As(err, &p) || !strings.Contains(p.Error(), c.want) {
				t.Fatalf("err = %v, want problem containing %q", err, c.want)
			}
		})
	}
}

func TestV2ConfigurationAndV1Defaults(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 2 || f.Events[3].StartMode != "group" || f.Events[4].DistanceMode != "throw" || f.Events[5].RelayLegCount != 4 {
		t.Fatal("v2 config lost")
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatalf("v2 roundtrip: %v", err)
	}
	for _, replacement := range []struct{ from, to string }{
		{`"startMode": "group"`, `"startMode": "unknown"`},
		{`"distanceMode": "throw"`, `"distanceMode": "unknown"`},
		{`"kind": "height"`, `"kind": "height", "startMode": "lanes"`},
		{`"startMode": "group"`, `"startMode": "group", "relayLegCount": 4`},
	} {
		if _, err := Decode([]byte(strings.Replace(string(data), replacement.from, replacement.to, 1))); err == nil {
			t.Fatalf("accepted %s", replacement.to)
		}
	}
	old := strings.Replace(string(readSample(t)), `"kind": "track"`, `"kind": "track", "startMode": {"custom": true}, "distanceMode": 17, "relayLegCount": "four"`, 1)
	legacy, err := Decode([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Events[0].StartMode != "" || legacy.Events[0].RelayLegCount != 0 {
		t.Fatal("v1 unknown fields interpreted")
	}
}

func TestV3ScalarTeamsAndCombined(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != Version || len(f.Combineds) != 1 {
		t.Fatal("v3 metadata")
	}
	out, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(out)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatalf("v3 roundtrip %v", err)
	}
	for _, mutate := range []func(*File){
		func(f *File) { f.Combineds[0].Components[0].EventID = "missing" },
		func(f *File) { f.Combineds[0].AthleteIDs = nil },
		func(f *File) { f.Combineds[0].Components[0].Rule.Bands[0].Points = -1 },
		func(f *File) { f.Events[len(f.Events)-2].Rounds[0].Heats[0].Slots[0].Result.Mark = ptrMark(1) },
	} {
		var invalid File
		if err := json.Unmarshal(data, &invalid); err != nil {
			t.Fatal(err)
		}
		mutate(&invalid)
		if len(Validate(&invalid)) == 0 {
			t.Fatal("invalid v3 accepted")
		}
	}
	legacy, err := os.ReadFile("../../testdata/interchange/sample-meet-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(legacy), `"version": 2`, `"version": 2, "combineds": {"extension":true}`, 1)
	text = strings.Replace(text, `"kind": "track"`, `"kind": "track", "participantMode": 12, "scoreDecimals": "extension"`, 1)
	old, err := Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Combineds) != 0 || old.Events[0].ParticipantMode != "" {
		t.Fatal("v2 extension interpreted")
	}
}
func ptrMark(n int64) *int64 { return &n }
