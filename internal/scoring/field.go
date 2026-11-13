package scoring

import (
	"errors"
	"fmt"
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
	for i, c := range s {
		if c == 'O' && i != len(s)-1 {
			return "", errors.New("成功后不能继续试跳")
		}
		if c == '-' && strings.ContainsAny(s[i+1:], "OX") {
			return "", errors.New("免跳后不能在同一高度试跳")
		}
	}
	return s, nil
}

// ValidateBars checks the sequence across heights, including failures carried
// over a passed height. Partial records are valid while the event is running.
func ValidateBars(bars []domain.Bar) error {
	failures := 0
	for i, b := range bars {
		if i > 0 && b.Height <= bars[i-1].Height {
			return errors.New("高度应从低到高")
		}
		trials, err := ParseTrials(b.Trials)
		if err != nil {
			return fmt.Errorf("%s：%w", FormatMark(domain.KindHeight, b.Height), err)
		}
		for _, c := range trials {
			if c == '-' {
				continue
			}
			if failures >= 3 {
				return errors.New("连续三次失败后不能继续试跳")
			}
			if c == 'O' {
				failures = 0
			} else {
				failures++
			}
		}
	}
	return nil
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
