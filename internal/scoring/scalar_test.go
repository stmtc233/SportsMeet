package scoring

import (
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestScalarUnitsPrecisionAndOverflow(t *testing.T) {
	count := domain.Event{Kind: domain.KindCount}
	score := domain.Event{Kind: domain.KindScore, CompetitionConfig: domain.CompetitionConfig{ScoreDecimals: 2}}
	for _, tt := range []struct {
		e    domain.Event
		text string
		want int64
	}{{count, "0", 0}, {count, "125", 125}, {score, "12.50", 1250}, {score, "0", 0}} {
		m, err := ParseEventMark(tt.e, tt.text)
		if err != nil || m != tt.want {
			t.Fatalf("%s => %d,%v", tt.text, m, err)
		}
	}
	for _, text := range []string{"-1", "1.0", "1:20", "1次", "9223372036854775808"} {
		if _, err := ParseEventMark(count, text); err == nil {
			t.Fatalf("accepted count %q", text)
		}
	}
	for _, text := range []string{"12.501", "-0.5", "92233720368547758.08"} {
		if _, err := ParseEventMark(score, text); err == nil {
			t.Fatalf("accepted score %q", text)
		}
	}
	if FormatEventMark(score, 1250) != "12.50" || FormatEventMark(count, 0) != "0" {
		t.Fatal("scalar format")
	}
}
func TestScalarAggregationAndRanking(t *testing.T) {
	e := domain.Event{Kind: domain.KindCount, CompetitionConfig: domain.CompetitionConfig{AttemptMode: "best"}}
	attempts := []domain.Attempt{}
	for _, text := range []string{"120", "124", "122", "X", "-"} {
		a, err := ParseEventAttempt(e, text)
		if err != nil {
			t.Fatal(err)
		}
		attempts = append(attempts, a)
	}
	m, ok, err := Aggregate(e, attempts)
	if err != nil || !ok || m != 124 {
		t.Fatal(m, ok, err)
	}
	e.AttemptMode = "sum"
	m, ok, err = Aggregate(e, attempts)
	if err != nil || !ok || m != 366 {
		t.Fatal(m, ok, err)
	}
	e.RankDirection = "asc"
	e.AttemptMode = "best"
	m, _, _ = Aggregate(e, attempts)
	if m != 120 {
		t.Fatal(m)
	}
	zero := int64(0)
	m, ok, err = Aggregate(e, []domain.Attempt{{Outcome: "valid", Mark: &zero}})
	if err != nil || !ok || m != 0 {
		t.Fatal("zero is valid")
	}
	_, ok, _ = Aggregate(e, []domain.Attempt{{Outcome: "foul"}})
	if ok {
		t.Fatal("foul is not zero")
	}
	ranks := RankEvent(e, []Ranked{{Key: "a", Status: domain.StatusOK, Mark: 0}, {Key: "b", Status: domain.StatusOK, Mark: 2}, {Key: "c", Status: domain.StatusOK, Mark: 2}, {Key: "d", Status: domain.StatusOK, Mark: 3}})
	if ranks["a"] != 1 || ranks["b"] != 2 || ranks["c"] != 2 || ranks["d"] != 4 {
		t.Fatal(ranks)
	}
}
