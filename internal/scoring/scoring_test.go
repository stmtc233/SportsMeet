package scoring

import (
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestParseMark(t *testing.T) {
	cases := []struct {
		kind domain.EventKind
		in   string
		want int64
		bad  bool
	}{
		{domain.KindTrack, "12.34", 12340, false},
		{domain.KindTrack, "12.3", 12300, false},
		{domain.KindTrack, "12", 12000, false},
		{domain.KindTrack, "1:05.67", 65670, false},
		{domain.KindTrack, "1：58.2", 118200, false},
		{domain.KindTrack, "1:02:03.4", 3723400, false},
		{domain.KindTrack, "1:60.00", 0, true},
		{domain.KindTrack, "12.3456", 0, true},
		{domain.KindTrack, "", 0, true},
		{domain.KindTrack, "abc", 0, true},
		{domain.KindDistance, "6.52", 6520, false},
		{domain.KindDistance, "6.5m", 6500, false},
		{domain.KindHeight, "1.85", 1850, false},
		{domain.KindHeight, "-1", 0, true},
	}
	for _, c := range cases {
		got, err := ParseMark(c.kind, c.in)
		if c.bad {
			if err == nil {
				t.Errorf("ParseMark(%s, %q) = %d, want error", c.kind, c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseMark(%s, %q) = %d, %v; want %d", c.kind, c.in, got, err, c.want)
		}
	}
}

func TestFormatMark(t *testing.T) {
	cases := []struct {
		kind domain.EventKind
		in   int64
		want string
	}{
		{domain.KindTrack, 12340, "12.34"},
		{domain.KindTrack, 12341, "12.35"}, // times round up
		{domain.KindTrack, 65670, "1:05.67"},
		{domain.KindTrack, 3723400, "1:02:03.40"},
		{domain.KindDistance, 6529, "6.52"}, // distances round down
		{domain.KindHeight, 1850, "1.85"},
	}
	for _, c := range cases {
		if got := FormatMark(c.kind, c.in); got != c.want {
			t.Errorf("FormatMark(%s, %d) = %q, want %q", c.kind, c.in, got, c.want)
		}
	}
}

func TestRankTies(t *testing.T) {
	ok := domain.StatusOK
	got := Rank(domain.KindTrack, []Ranked{
		{Key: "a", Status: ok, Mark: 12500}, {Key: "b", Status: ok, Mark: 12300}, {Key: "c", Status: ok, Mark: 12500}, {Key: "d", Status: ok, Mark: 12900}, {Key: "e", Status: domain.StatusDNF, Mark: 0},
	})
	want := map[string]int{"b": 1, "a": 2, "c": 2, "d": 4}
	if len(got) != len(want) {
		t.Fatalf("Rank = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("place of %s = %d, want %d", k, got[k], v)
		}
	}
	field := Rank(domain.KindDistance, []Ranked{{Key: "x", Status: ok, Mark: 4000}, {Key: "y", Status: ok, Mark: 4500}})
	if field["y"] != 1 || field["x"] != 2 {
		t.Errorf("field ranking = %v, want higher first", field)
	}
}

func i64p(v int64) *int64 { return &v }

func TestRankFieldTieBreaks(t *testing.T) {
	valid := func(marks ...int64) []domain.Attempt {
		var out []domain.Attempt
		for _, m := range marks {
			if m == 0 {
				out = append(out, domain.Attempt{Outcome: "foul"})
				continue
			}
			out = append(out, domain.Attempt{Outcome: "valid", Mark: i64p(m)})
		}
		return out
	}
	cases := []struct {
		name string
		kind domain.EventKind
		list []Ranked
		want map[string]int
	}{
		{"distance: second best decides", domain.KindDistance, []Ranked{
			{Key: "a", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6520, 6100, 0)},
			{Key: "b", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6300, 6520, 6000)},
			{Key: "c", Status: domain.StatusOK, Mark: 6000, Attempts: valid(6000)},
		}, map[string]int{"b": 1, "a": 2, "c": 3}},
		{"distance: a missing attempt loses", domain.KindDistance, []Ranked{
			{Key: "a", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6520, 0, 0)},
			{Key: "b", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6520, 5000, 0)},
		}, map[string]int{"b": 1, "a": 2}},
		{"distance: identical series share the place", domain.KindDistance, []Ranked{
			{Key: "a", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6520, 6100)},
			{Key: "b", Status: domain.StatusOK, Mark: 6520, Attempts: valid(6100, 6520)},
			{Key: "c", Status: domain.StatusOK, Mark: 5000},
		}, map[string]int{"a": 1, "b": 1, "c": 3}},
		{"height: fewer failures at the last height", domain.KindHeight, []Ranked{
			{Key: "a", Status: domain.StatusOK, Mark: 1850, Bars: []domain.Bar{{Height: 1800, Trials: "O"}, {Height: 1850, Trials: "XO"}, {Height: 1900, Trials: "XXX"}}},
			{Key: "b", Status: domain.StatusOK, Mark: 1850, Bars: []domain.Bar{{Height: 1800, Trials: "XXO"}, {Height: 1850, Trials: "O"}, {Height: 1900, Trials: "XXX"}}},
		}, map[string]int{"b": 1, "a": 2}},
		{"height: then fewer failures in total", domain.KindHeight, []Ranked{
			{Key: "a", Status: domain.StatusOK, Mark: 1850, Bars: []domain.Bar{{Height: 1800, Trials: "XO"}, {Height: 1850, Trials: "O"}}},
			{Key: "b", Status: domain.StatusOK, Mark: 1850, Bars: []domain.Bar{{Height: 1800, Trials: "O"}, {Height: 1850, Trials: "O"}, {Height: 1900, Trials: "XXX"}}},
			{Key: "c", Status: domain.StatusOK, Mark: 1850, Bars: []domain.Bar{{Height: 1800, Trials: "O"}, {Height: 1850, Trials: "O"}, {Height: 1900, Trials: "XXX"}}},
			{Key: "d", Status: domain.StatusNM},
		}, map[string]int{"b": 1, "c": 1, "a": 3}},
	}
	for _, c := range cases {
		got := Rank(c.kind, c.list)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestFieldRecords(t *testing.T) {
	a, err := ParseAttempt(domain.KindDistance, " x ")
	if err != nil || a.Outcome != "foul" {
		t.Fatalf("foul = %+v, %v", a, err)
	}
	a, _ = ParseAttempt(domain.KindDistance, "6.52")
	if a.Outcome != "valid" || *a.Mark != 6520 || FormatAttempt(domain.KindDistance, a) != "6.52" {
		t.Fatalf("valid = %+v", a)
	}
	if _, err := ParseAttempt(domain.KindDistance, "abc"); err == nil {
		t.Fatal("abc parsed")
	}
	if s, err := ParseTrials("xo"); err != nil || s != "XO" {
		t.Fatalf("ParseTrials = %q, %v", s, err)
	}
	if _, err := ParseTrials("XXXX"); err == nil {
		t.Fatal("four trials accepted")
	}
	if m, ok := BestAttempt([]domain.Attempt{{Outcome: "foul"}, {Outcome: "valid", Mark: i64p(5)}, {Outcome: "valid", Mark: i64p(9)}}); !ok || m != 9 {
		t.Fatalf("BestAttempt = %d, %v", m, ok)
	}
	if m, ok := ClearedHeight([]domain.Bar{{Height: 1500, Trials: "O"}, {Height: 1550, Trials: "XO"}, {Height: 1600, Trials: "XXX"}}); !ok || m != 1550 {
		t.Fatalf("ClearedHeight = %d, %v", m, ok)
	}
	if _, ok := ClearedHeight([]domain.Bar{{Height: 1500, Trials: "XXX"}}); ok {
		t.Fatal("no height cleared but ok")
	}
}
