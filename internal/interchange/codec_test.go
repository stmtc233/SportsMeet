package interchange

import (
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
		{"newer version", `"version": 1`, `"version": 2`, ""},
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
