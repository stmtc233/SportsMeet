package domain

import (
	"regexp"
	"time"
)

// The meet has three kinds of time (docs/agents/domain.md「时间」):
//
//   - a date, YYYY-MM-DD: meets, events, birth dates;
//   - a scheduled time, YYYY-MM-DDTHH:mm: a local wall-clock minute with
//     no time zone, for when rounds and heats are planned and when a heat
//     actually started;
//   - a recorded time, milliseconds since the epoch: when a row was
//     written, shown in the computer's local time.
//
// Dates and scheduled times are calendar fields, never converted between
// zones: they are parsed and computed in UTC so no daylight-saving shift
// can move them.

const (
	dateLayout      = "2006-01-02"
	scheduledLayout = "2006-01-02T15:04"
	minuteLayout    = "2006-01-02 15:04"
)

var (
	datePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	scheduledPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$`)
)

// ParseDate reads YYYY-MM-DD; ok is false unless it is a real calendar day
// in years 1–9999.
func ParseDate(s string) (t time.Time, ok bool) {
	if !datePattern.MatchString(s) {
		return time.Time{}, false
	}
	t, err := time.Parse(dateLayout, s)
	return t, err == nil && t.Year() >= 1
}

// ValidDate reports whether s is a date ParseDate accepts.
func ValidDate(s string) bool {
	_, ok := ParseDate(s)
	return ok
}

// ParseScheduled reads YYYY-MM-DDTHH:mm as wall-clock fields; ok is false
// unless the day and the minute both exist.
func ParseScheduled(s string) (t time.Time, ok bool) {
	if !scheduledPattern.MatchString(s) {
		return time.Time{}, false
	}
	t, err := time.Parse(scheduledLayout, s)
	return t, err == nil && t.Year() >= 1
}

// ValidScheduled reports whether s is a scheduled time ParseScheduled accepts.
func ValidScheduled(s string) bool {
	_, ok := ParseScheduled(s)
	return ok
}

// FormatScheduled writes wall-clock fields in the stored form; ok is false
// past year 9999, which the stored form cannot hold.
func FormatScheduled(t time.Time) (s string, ok bool) {
	if t.Year() < 1 || t.Year() > 9999 {
		return "", false
	}
	return t.Format(scheduledLayout), true
}

// ScheduledDay is the date of a scheduled time, "" when it is not one.
func ScheduledDay(s string) string {
	t, ok := ParseScheduled(s)
	if !ok {
		return ""
	}
	return t.Format(dateLayout)
}

// ScheduledText shows a scheduled time: only HH:mm on day, otherwise with
// its date (day "" always shows the date). Unscheduled or unreadable
// values show as "".
func ScheduledText(s, day string) string {
	t, ok := ParseScheduled(s)
	if !ok {
		return ""
	}
	if day != "" && t.Format(dateLayout) == day {
		return t.Format("15:04")
	}
	return t.Format(minuteLayout)
}

// DayText shows a day with its weekday: 2026-11-12 周四. Unreadable
// values show as they are.
func DayText(day string) string {
	t, ok := ParseDate(day)
	if !ok {
		return day
	}
	return day + " " + [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[t.Weekday()]
}

// DateRangeText shows a meet's days: 2026-11-12, or 2026-11-12 – 2026-11-13.
func DateRangeText(start, end string) string {
	if end == "" || end == start {
		return start
	}
	return start + " – " + end
}

// RecordedText shows a recorded time to the minute in loc; 0 shows as "".
func RecordedText(ms int64, loc *time.Location) string {
	if ms == 0 {
		return ""
	}
	if loc == nil {
		loc = time.Local
	}
	return time.UnixMilli(ms).In(loc).Format(minuteLayout)
}

// WallClock is a recorded time as a scheduled time in loc: the local
// minute it happened, in the stored YYYY-MM-DDTHH:mm form. 0 gives "".
func WallClock(ms int64, loc *time.Location) string {
	if ms == 0 {
		return ""
	}
	if loc == nil {
		loc = time.Local
	}
	at, _ := FormatScheduled(time.UnixMilli(ms).In(loc))
	return at
}

// EventDay is the day an event is held: its own date, or the meet's start.
func EventDay(e Event, meetStart string) string {
	if e.Date != "" {
		return e.Date
	}
	return meetStart
}
