package scoring

import (
	"errors"
	"regexp"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ErrBadTrials is returned for a height record that is not 1–3 of O, X, -.
var ErrBadTrials = errors.New("试跳记录只能由 O、X、- 组成，最多 3 次")

var trialsRe = regexp.MustCompile(`^[OX-]{1,3}$`)

// ParseAttempt reads one typed distance attempt: a mark (6.52), a foul
// (X / 犯规) or a pass (- / 免跳).
func ParseAttempt(kind domain.EventKind, s string) (domain.Attempt, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "X", "×", "犯规":
		return domain.Attempt{Outcome: "foul"}, nil
	case "-", "—", "免跳", "免投", "免":
		return domain.Attempt{Outcome: "pass"}, nil
	}
	m, err := ParseMark(kind, s)
	if err != nil {
		return domain.Attempt{}, err
	}
	return domain.Attempt{Outcome: "valid", Mark: &m}, nil
}

// FormatAttempt shows an attempt the way ParseAttempt reads it.
func FormatAttempt(kind domain.EventKind, a domain.Attempt) string {
	switch a.Outcome {
	case "foul":
		return "X"
	case "pass":
		return "-"
	}
	if a.Mark == nil {
		return ""
	}
	return FormatMark(kind, *a.Mark)
}

// ParseTrials normalizes a typed height record (xo → XO).
func ParseTrials(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.NewReplacer("0", "O", "×", "X", "—", "-", "－", "-").Replace(s)
	if !trialsRe.MatchString(s) {
		return "", ErrBadTrials
	}
	return s, nil
}

// BestAttempt is the final mark of a distance result: the best valid
// attempt; ok is false when there is none.
func BestAttempt(attempts []domain.Attempt) (mark int64, ok bool) {
	for _, a := range attempts {
		if a.Outcome == "valid" && a.Mark != nil && (!ok || *a.Mark > mark) {
			mark, ok = *a.Mark, true
		}
	}
	return mark, ok
}

// ClearedHeight is the final mark of a height result: the highest height
// cleared; ok is false when none was.
func ClearedHeight(bars []domain.Bar) (mark int64, ok bool) {
	for _, b := range bars {
		if strings.Contains(b.Trials, "O") && (!ok || b.Height > mark) {
			mark, ok = b.Height, true
		}
	}
	return mark, ok
}
