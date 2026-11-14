package scoring

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

var unsignedDecimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func ParseEventMark(e domain.Event, text string) (int64, error) {
	if !e.Kind.Scalar() {
		return ParseMark(e.Kind, text)
	}
	text = strings.TrimSpace(text)
	if !unsignedDecimal.MatchString(text) {
		return 0, ErrBadMark
	}
	decimals := e.ScoreDecimals
	if e.Kind == domain.KindCount {
		decimals = 0
	}
	return parseDecimal(text, decimals)
}
func FormatEventMark(e domain.Event, mark int64) string {
	if !e.Kind.Scalar() {
		return FormatMark(e.Kind, mark)
	}
	decimals := e.ScoreDecimals
	if e.Kind == domain.KindCount {
		decimals = 0
	}
	unit := int64(1)
	for range decimals {
		unit *= 10
	}
	if decimals == 0 {
		return fmt.Sprint(mark)
	}
	return fmt.Sprintf("%d.%0*d", mark/unit, decimals, mark%unit)
}
func ParseEventAttempt(e domain.Event, text string) (domain.Attempt, error) {
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "X", "×", "犯规":
		return domain.Attempt{Outcome: "foul"}, nil
	case "-", "—", "免跳", "免投", "免":
		return domain.Attempt{Outcome: "pass"}, nil
	}
	mark, err := ParseEventMark(e, text)
	if err != nil {
		return domain.Attempt{}, err
	}
	return domain.Attempt{Outcome: "valid", Mark: &mark}, nil
}
func FormatEventAttempt(e domain.Event, a domain.Attempt) string {
	if a.Outcome == "foul" {
		return "X"
	}
	if a.Outcome == "pass" {
		return "-"
	}
	if a.Mark == nil {
		return ""
	}
	return FormatEventMark(e, *a.Mark)
}
func Aggregate(e domain.Event, attempts []domain.Attempt) (int64, bool, error) {
	var mark int64
	ok := false
	for _, a := range attempts {
		if a.Outcome != "valid" || a.Mark == nil {
			continue
		}
		v := *a.Mark
		if v < 0 {
			return 0, false, ErrBadMark
		}
		if e.AttemptMode == "sum" {
			if v > math.MaxInt64-mark {
				return 0, false, ErrBadMark
			}
			mark += v
		} else if !ok || (e.Ascending() && v < mark) || (!e.Ascending() && v > mark) {
			mark = v
		}
		ok = true
	}
	return mark, ok, nil
}
func RankEvent(e domain.Event, list []Ranked) map[string]int {
	kind, list := rankInput(e, list)
	return Rank(kind, list)
}

// TiesEvent is Ties under the event's own ordering.
func TiesEvent(e domain.Event, list []Ranked) []Tie {
	kind, list := rankInput(e, list)
	return Ties(kind, list)
}

// rankInput maps a count or score event onto the ordering of Rank: equal
// final values tie, without the field events' countback.
func rankInput(e domain.Event, list []Ranked) (domain.EventKind, []Ranked) {
	if !e.Kind.Scalar() {
		return e.Kind, list
	}
	copy := append([]Ranked(nil), list...)
	kind := domain.KindDistance
	if e.Ascending() {
		kind = domain.KindTrack
	}
	for i := range copy {
		copy[i].Attempts = nil
		copy[i].Bars = nil
	}
	return kind, copy
}

// FormatRuleMark retains every stored unit when editing scoring thresholds.
func FormatRuleMark(e domain.Event, mark int64) string {
	if e.Kind.Scalar() {
		return FormatEventMark(e, mark)
	}
	return fmt.Sprintf("%d.%03d", mark/1000, mark%1000)
}
