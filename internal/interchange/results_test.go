package interchange

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecodeResultsSample(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/sample-results.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := DecodeResults(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.MeetID != "m1" || len(f.Heats) != 1 || len(f.Heats[0].Results) != 2 || *f.Heats[0].Results[0].Mark != 12340 {
		t.Fatalf("sample = %+v", f)
	}
}

func TestDecodeResultsProblems(t *testing.T) {
	if _, err := DecodeResults([]byte(`{"format":"sportsmeet-results","version":2}`)); !errors.Is(err, ErrNewerVersion) {
		t.Fatalf("newer: %v", err)
	}
	_, err := DecodeResults([]byte(`{"format":"sportsmeet-results","version":1,"exportedAt":"x","generator":{"name":"t"},"meetId":"m",
		"heats":[{"heatId":"h","startedAt":"2026-13-01T09:00","results":[
			{"position":1,"athleteId":"a","teamId":"t","status":"OK","mark":1},
			{"position":1,"athleteId":"b","status":"DNS","mark":5},
			{"position":2,"athleteId":"c","status":"OK"},
			{"position":3,"athleteId":"c","status":"XX"}]},{"heatId":"h","results":[]}]}`))
	var p Problems
	if !errors.As(err, &p) {
		t.Fatalf("err = %v", err)
	}
	text := strings.Join(p, "\n")
	for _, want := range []string{"heatId 重复", "startedAt", "其中一个", "position 重复", "应为 null", "应为正整数", "status 不正确", "运动员或队伍重复"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}
