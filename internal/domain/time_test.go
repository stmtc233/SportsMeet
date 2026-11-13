package domain

import (
	"testing"
	"time"
)

func TestDates(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"2026-11-12", true},
		{"2028-02-29", true},
		{"2026-02-29", false},
		{"2026-13-01", false},
		{"2026-1-12", false},
		{"0000-01-01", false},
		{"9999-12-31", true},
		{" 2026-11-12", false},
		{"", false},
	} {
		if got := ValidDate(c.in); got != c.ok {
			t.Errorf("ValidDate(%q) = %v", c.in, got)
		}
	}
}

func TestScheduled(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"2026-11-12T09:30", true},
		{"2026-11-12T23:59", true},
		{"2026-11-12T24:00", false},
		{"2026-11-12T9:30", false},
		{"2026-11-31T09:30", false},
		{"2026-11-12 09:30", false},
		{"2026-11-12T09:30:00", false},
		{"2026-11-12T09:30Z", false},
		{"0000-01-01T00:00", false},
		{"", false},
	} {
		if got := ValidScheduled(c.in); got != c.ok {
			t.Errorf("ValidScheduled(%q) = %v", c.in, got)
		}
	}
	at, _ := ParseScheduled("2026-03-08T01:59")
	next, ok := FormatScheduled(at.Add(2 * time.Minute))
	if !ok || next != "2026-03-08T02:01" { // no daylight-saving gap on wall-clock fields
		t.Errorf("FormatScheduled = %q, %v", next, ok)
	}
	end, _ := ParseScheduled("9999-12-31T23:59")
	if _, ok := FormatScheduled(end.Add(time.Minute)); ok {
		t.Error("FormatScheduled accepted year 10000")
	}
	if ScheduledDay("2026-11-12T09:30") != "2026-11-12" || ScheduledDay("bad") != "" {
		t.Error("ScheduledDay")
	}
}

func TestScheduledText(t *testing.T) {
	for _, c := range []struct{ at, day, want string }{
		{"2026-11-12T09:30", "2026-11-12", "09:30"},
		{"2026-11-13T00:01", "2026-11-12", "2026-11-13 00:01"},
		{"2026-11-12T09:30", "", "2026-11-12 09:30"},
		{"", "2026-11-12", ""},
		{"2026-11-12", "2026-11-12", ""},
	} {
		if got := ScheduledText(c.at, c.day); got != c.want {
			t.Errorf("ScheduledText(%q, %q) = %q, want %q", c.at, c.day, got, c.want)
		}
	}
}

func TestDayText(t *testing.T) {
	for _, c := range []struct{ day, want string }{
		{"2026-11-12", "2026-11-12 周四"},
		{"2026-11-15", "2026-11-15 周日"},
		{"", ""},
		{"2026-02-30", "2026-02-30"},
	} {
		if got := DayText(c.day); got != c.want {
			t.Errorf("DayText(%q) = %q", c.day, got)
		}
	}
}

func TestDateRangeText(t *testing.T) {
	for _, c := range []struct{ start, end, want string }{
		{"2026-11-12", "", "2026-11-12"},
		{"2026-11-12", "2026-11-12", "2026-11-12"},
		{"2026-11-12", "2026-11-13", "2026-11-12 – 2026-11-13"},
	} {
		if got := DateRangeText(c.start, c.end); got != c.want {
			t.Errorf("DateRangeText(%q, %q) = %q", c.start, c.end, got)
		}
	}
}

func TestRecordedText(t *testing.T) {
	shanghai := time.FixedZone("UTC+8", 8*3600)
	ms := time.Date(2026, 11, 12, 1, 31, 59, 0, time.UTC).UnixMilli()
	if got := RecordedText(ms, shanghai); got != "2026-11-12 09:31" {
		t.Errorf("RecordedText = %q", got)
	}
	if got := RecordedText(ms, time.UTC); got != "2026-11-12 01:31" {
		t.Errorf("RecordedText UTC = %q", got)
	}
	if RecordedText(0, time.UTC) != "" {
		t.Error("RecordedText(0) should be empty")
	}
}

func TestScheduleOrder(t *testing.T) {
	heat := func(at string) Heat { return Heat{ScheduledAt: at} }
	events := []Event{
		{ID: "undated-late", Date: ""},   // meet's start, 15:00
		{ID: "day2", Date: "2026-11-13"}, // second day, no time
		{ID: "c1", CombinedID: "c", Date: "2026-11-12"},
		{ID: "dated-early", Date: "2026-11-12"}, // 09:00
		{ID: "untimed", Date: "2026-11-12"},     // no heat time
		{ID: "c2", CombinedID: "c"},
		{ID: "dated-early-2", Date: "2026-11-12"}, // 09:00 too, created later
		{ID: "bad-date", Date: "2026-02-30"},
		{ID: "moved", Date: "2026-11-13"}, // planned on the first day by its round's start
	}
	rounds := map[string][]Round{
		"undated-late":  {{Heats: []Heat{heat("2026-11-12T15:00")}}},
		"dated-early":   {{Heats: []Heat{heat("2026-11-12T10:00")}}, {Heats: []Heat{heat(""), heat("2026-11-12T09:00")}}},
		"dated-early-2": {{Heats: []Heat{heat("2026-11-12T09:00")}}},
		"untimed":       {{Heats: []Heat{heat("")}}},
		"c1":            {{Heats: []Heat{heat("2026-11-12T08:00")}}},
		"moved":         {{ScheduledAt: "2026-11-12T08:30", Heats: []Heat{heat("")}}},
	}
	var got []string
	for _, e := range ScheduleOrder(events, rounds, "2026-11-12") {
		got = append(got, e.ID)
	}
	want := []string{"moved", "dated-early", "dated-early-2", "undated-late", "untimed", "day2", "bad-date", "c1", "c2"}
	if len(got) != len(want) {
		t.Fatalf("ScheduleOrder = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ScheduleOrder = %v, want %v", got, want)
		}
	}
}

func TestEffectiveAndEnd(t *testing.T) {
	five := 5
	round := Round{ScheduledAt: "2026-11-12T23:58"}
	if at, inherited := EffectiveScheduled(round, Heat{}); at != round.ScheduledAt || !inherited {
		t.Fatalf("inherited = %q %v", at, inherited)
	}
	if at, inherited := EffectiveScheduled(round, Heat{ScheduledAt: "2026-11-12T09:00"}); at != "2026-11-12T09:00" || inherited {
		t.Fatalf("own = %q %v", at, inherited)
	}
	if at, inherited := EffectiveScheduled(Round{}, Heat{}); at != "" || inherited {
		t.Fatal("unscheduled heat")
	}
	if end, ok := HeatEnd(round, Heat{DurationMinutes: &five}); !ok || end != "2026-11-13T00:03" {
		t.Fatalf("end across midnight = %q", end)
	}
	if _, ok := HeatEnd(round, Heat{}); ok {
		t.Fatal("end without a duration")
	}
	if _, ok := HeatEnd(Round{}, Heat{DurationMinutes: &five}); ok {
		t.Fatal("end without a time")
	}
	if _, ok := ScheduledEnd("9999-12-31T23:58", &five); ok {
		t.Fatal("end past year 9999")
	}
	for m, ok := range map[int]bool{0: false, 1: true, 1440: true, 1441: false, -5: false} {
		if ValidDuration(&m) != ok {
			t.Errorf("ValidDuration(%d) = %v", m, !ok)
		}
	}
	if !ValidDuration(nil) || DurationText(nil) != "" || DurationText(&five) != "5 分钟" {
		t.Fatal("duration text")
	}
}

func TestPlannedDayAndOutsideMeet(t *testing.T) {
	e := Event{Date: "2026-11-13"}
	if PlannedDay(e, nil, "2026-11-12") != "2026-11-13" || PlannedDay(Event{}, nil, "2026-11-12") != "2026-11-12" {
		t.Fatal("day without times")
	}
	rounds := []Round{{ScheduledAt: "2026-11-14T08:00", Heats: []Heat{{ScheduledAt: "2026-11-12T16:00"}}}}
	if PlannedDay(e, rounds, "2026-11-12") != "2026-11-12" || EarliestScheduled(rounds) != "2026-11-12T16:00" {
		t.Fatal("day from the earliest planned time")
	}
	meet := Meet{StartDate: "2026-11-12", EndDate: "2026-11-13"}
	for at, outside := range map[string]bool{
		"2026-11-11T23:59": true, "2026-11-12T00:00": false, "2026-11-13T23:59": false, "2026-11-14T00:00": true, "": false, "bad": false,
	} {
		if OutsideMeet(at, meet) != outside {
			t.Errorf("OutsideMeet(%q) = %v", at, !outside)
		}
	}
	if !OutsideMeet("2026-11-13T09:00", Meet{StartDate: "2026-11-12"}) || OutsideMeet("2026-11-13T09:00", Meet{}) {
		t.Fatal("single-day meet / meet without days")
	}
}

func TestWallClock(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	ms := time.Date(2026, 11, 12, 16, 5, 59, 0, time.UTC).UnixMilli()
	if got := WallClock(ms, zone); got != "2026-11-13T00:05" {
		t.Errorf("WallClock = %q", got)
	}
	if WallClock(0, zone) != "" {
		t.Error("WallClock(0) should be empty")
	}
}

func TestCheckInAt(t *testing.T) {
	for _, c := range []struct {
		at   string
		lead int
		want string
	}{
		{"2026-11-12T09:30", 20, "2026-11-12T09:10"},
		{"2026-11-13T00:10", 20, "2026-11-12T23:50"},
		{"2026-03-01T00:00", 240, "2026-02-28T20:00"},
		{"2026-11-12T09:30", 0, ""},
		{"", 20, ""},
		{"0001-01-01T00:10", 20, ""},
	} {
		if got := CheckInAt(c.at, c.lead); got != c.want {
			t.Errorf("CheckInAt(%q, %d) = %q, want %q", c.at, c.lead, got, c.want)
		}
	}
	for m, ok := range map[int]bool{-1: false, 0: true, 20: true, 240: true, 241: false} {
		if ValidCheckInLead(m) != ok {
			t.Errorf("ValidCheckInLead(%d) != %v", m, ok)
		}
	}
}

func TestStartDeltaText(t *testing.T) {
	for _, c := range []struct{ planned, started, want string }{
		{"2026-11-12T09:30", "2026-11-12T09:30", "准时"},
		{"2026-11-12T09:30", "2026-11-12T09:42", "晚 12 分钟"},
		{"2026-11-12T09:30", "2026-11-12T09:25", "早 5 分钟"},
		{"2026-11-12T23:50", "2026-11-13T00:05", "晚 15 分钟"},
		{"2026-11-12T09:30", "2026-11-12T10:35", "晚 1 小时 5 分钟"},
		{"2026-11-12T09:30", "2026-11-12T11:30", "晚 2 小时"},
		{"2026-11-12T09:30", "2026-11-13T11:30", "晚 1 天 2 小时"},
		{"2026-12-31T23:00", "2026-11-30T23:00", "早 31 天"},
		{"", "2026-11-12T09:30", ""},
		{"2026-11-12T09:30", "", ""},
	} {
		if got := StartDeltaText(c.planned, c.started); got != c.want {
			t.Errorf("StartDeltaText(%q, %q) = %q, want %q", c.planned, c.started, got, c.want)
		}
	}
}
