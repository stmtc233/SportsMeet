// Package scoring parses, formats and ranks marks. It is the only place
// these rules live; the frontend calls it (docs/agents/domain.md「成绩」).
package scoring

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ErrBadMark is returned for input that is not a mark of the event kind.
var ErrBadMark = errors.New("成绩格式不正确")

// ParseMark reads a mark typed by a person:
//   - timed: 12.34 / 12.3 / 1:05.67 / 1:02:03.4 → milliseconds;
//   - field: 6.52 / 6.5 / 6 → millimetres (metres in, at most 3 decimals).
func ParseMark(kind domain.EventKind, s string) (int64, error) {
	if kind.Scalar() {
		return ParseEventMark(domain.Event{Kind: kind}, s)
	}
	s = strings.TrimSpace(strings.NewReplacer("：", ":", "。", ".", "m", "", "M", "", "s", "", "秒", "", "米", "").Replace(s))
	if s == "" {
		return 0, ErrBadMark
	}
	if kind.Timed() {
		return parseTime(s)
	}
	return parseDecimal(s, 3) // metres → mm
}

func parseTime(s string) (int64, error) {
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, ErrBadMark
	}
	secMs, err := parseDecimal(parts[len(parts)-1], 3)
	if err != nil {
		return 0, err
	}
	if len(parts) > 1 && secMs >= 60_000 {
		return 0, ErrBadMark
	}
	total := secMs
	mul := int64(60_000)
	for i := len(parts) - 2; i >= 0; i-- {
		n, err := strconv.ParseInt(parts[i], 10, 64)
		if err != nil || n < 0 || (i > 0 && n >= 60) {
			return 0, ErrBadMark
		}
		total += n * mul
		mul *= 60
	}
	return total, nil
}

// parseDecimal reads a non-negative decimal into an integer of 10^-scale
// units, rejecting more decimals than scale (no silent rounding).
func parseDecimal(s string, scale int) (int64, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > scale {
		return 0, ErrBadMark
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w < 0 {
		return 0, ErrBadMark
	}
	f := int64(0)
	if frac != "" {
		if f, err = strconv.ParseInt(frac, 10, 64); err != nil || f < 0 {
			return 0, ErrBadMark
		}
	}
	for range scale - len(frac) {
		f *= 10
	}
	unit := int64(1)
	for range scale {
		unit *= 10
	}
	if w > (math.MaxInt64-f)/unit {
		return 0, ErrBadMark
	}
	return w*unit + f, nil
}

// FormatMark shows a mark: times as 12.34 / 1:05.67 (hundredths, rounded
// up as athletics rules require), field marks as 6.52 (centimetres,
// rounded down).
func FormatMark(kind domain.EventKind, mark int64) string {
	if kind.Scalar() {
		return FormatEventMark(domain.Event{Kind: kind}, mark)
	}
	if kind.Timed() {
		cs := (mark + 9) / 10
		sec, hund := cs/100, cs%100
		if sec < 60 {
			return fmt.Sprintf("%d.%02d", sec, hund)
		}
		m, s := sec/60, sec%60
		if m < 60 {
			return fmt.Sprintf("%d:%02d.%02d", m, s, hund)
		}
		return fmt.Sprintf("%d:%02d:%02d.%02d", m/60, m%60, s, hund)
	}
	cm := mark / 10
	return fmt.Sprintf("%d.%02d", cm/100, cm%100)
}
