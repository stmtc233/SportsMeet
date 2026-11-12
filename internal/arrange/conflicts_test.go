package arrange

import (
	"fmt"
	"testing"
)

func mins(n int) *int { return &n }

func describe(list []Conflict) []string {
	out := []string{}
	for _, c := range list {
		kind := "rest"
		if c.Overlap {
			kind = "overlap"
		}
		out = append(out, fmt.Sprintf("%s:%s-%s:%s:%d", c.AthleteID, c.First, c.Second, kind, c.GapMinutes))
	}
	return out
}

func TestConflicts(t *testing.T) {
	heats := []TimedHeat{
		{HeatID: "a", Start: "2026-11-12T09:00", Minutes: mins(10), Athletes: []string{"x", "y", "x"}}, // x listed twice: no self conflict
		{HeatID: "b", Start: "2026-11-12T09:05", Minutes: mins(10), Athletes: []string{"x"}},           // overlaps a
		{HeatID: "c", Start: "2026-11-12T09:10", Athletes: []string{"y"}},                              // right after a: fine with no rest
		{HeatID: "d", Start: "2026-11-12T09:30", Athletes: []string{"z"}},                              // point in time
		{HeatID: "e", Start: "2026-11-12T09:30", Athletes: []string{"z"}},                              // same minute as d
		{HeatID: "f", Start: "", Minutes: mins(10), Athletes: []string{"x"}},                           // unscheduled: not checked
		{HeatID: "g", Start: "2026-11-12T09:20", Athletes: []string{"x"}},                              // 5 minutes after b
		{HeatID: "h", Start: "2026-11-13T09:00", Athletes: []string{"x", "team-member"}},
		{HeatID: "i", Start: "2026-11-12T23:58", Minutes: mins(5), Athletes: []string{"team-member"}}, // ends next day 00:03
		{HeatID: "j", Start: "2026-11-13T00:02", Athletes: []string{"team-member"}},
	}
	got := describe(Conflicts(heats, 0))
	want := []string{"team-member:i-j:overlap:-1", "x:a-b:overlap:-5", "z:d-e:overlap:0"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("no rest: %v, want %v", got, want)
	}
	got = describe(Conflicts(heats, 10))
	want = []string{"team-member:i-j:overlap:-1", "x:a-b:overlap:-5", "x:b-g:rest:5", "y:a-c:rest:0", "z:d-e:overlap:0"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("10 minutes rest: %v, want %v", got, want)
	}
	if len(Conflicts(heats, -3)) != 3 {
		t.Fatal("a negative rest counts as none")
	}
}

// Many athletes in many heats: the scan stays close to linear.
func TestConflictsScale(t *testing.T) {
	var heats []TimedHeat
	for i := 0; i < 2000; i++ {
		at := fmt.Sprintf("2026-11-12T%02d:%02d", 8+i/60%12, i%60)
		heats = append(heats, TimedHeat{HeatID: fmt.Sprint(i), Start: at, Minutes: mins(1), Athletes: []string{fmt.Sprint("a", i%50)}})
	}
	if got := Conflicts(heats, 0); len(got) > 2000 {
		t.Fatalf("%d conflicts", len(got))
	}
}
