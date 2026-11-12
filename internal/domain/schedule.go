package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// MaxDurationMinutes bounds a heat's expected duration: one day.
const MaxDurationMinutes = 1440

// ValidDuration reports whether a heat's expected duration is absent or
// 1–MaxDurationMinutes minutes.
func ValidDuration(m *int) bool {
	return m == nil || (*m >= 1 && *m <= MaxDurationMinutes)
}

// DurationText shows an expected duration: "5 分钟"; "" when absent.
func DurationText(m *int) string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%d 分钟", *m)
}

// EffectiveScheduled is when a heat is planned to start: its own time, or
// else its round's start (inherited true). "" = unscheduled.
func EffectiveScheduled(r Round, h Heat) (at string, inherited bool) {
	if h.ScheduledAt != "" {
		return h.ScheduledAt, false
	}
	return r.ScheduledAt, r.ScheduledAt != ""
}

// ScheduledEnd is a planned time plus minutes; ok is false when either is
// missing or the end falls outside the planned-time range.
func ScheduledEnd(at string, minutes *int) (end string, ok bool) {
	t, ok := ParseScheduled(at)
	if !ok || minutes == nil {
		return "", false
	}
	return FormatScheduled(t.Add(time.Duration(*minutes) * time.Minute))
}

// HeatEnd is when a heat is planned to end: its effective start plus its
// expected duration; ok is false when either is missing.
func HeatEnd(r Round, h Heat) (end string, ok bool) {
	at, _ := EffectiveScheduled(r, h)
	return ScheduledEnd(at, h.DurationMinutes)
}

// Check-in lead: how long before a heat's planned start its athletes check
// in, in minutes; 0 prints no check-in time.
const (
	DefaultCheckInLeadMinutes = 20
	MaxCheckInLeadMinutes     = 240
)

// ValidCheckInLead reports whether a check-in lead is 0–MaxCheckInLeadMinutes.
func ValidCheckInLead(m int) bool { return m >= 0 && m <= MaxCheckInLeadMinutes }

// CheckInAt is when a heat's athletes check in: its effective planned time
// minus lead minutes. "" when the heat has no planned time, the lead is 0,
// or the result falls before year 1.
func CheckInAt(at string, lead int) string {
	t, ok := ParseScheduled(at)
	if !ok || lead <= 0 {
		return ""
	}
	out, _ := FormatScheduled(t.Add(-time.Duration(lead) * time.Minute))
	return out
}

// StartDeltaText says how a heat's actual start compares with its planned
// one: 准时, 晚 12 分钟, 早 1 小时 5 分钟, 晚 1 天 2 小时. The difference counts
// across midnight and days. "" when either time is missing.
func StartDeltaText(planned, started string) string {
	p, ok1 := ParseScheduled(planned)
	s, ok2 := ParseScheduled(started)
	if !ok1 || !ok2 {
		return ""
	}
	minutes := int(s.Sub(p) / time.Minute)
	word := "晚"
	if minutes < 0 {
		word, minutes = "早", -minutes
	}
	if minutes == 0 {
		return "准时"
	}
	parts := []string{}
	for _, unit := range []struct {
		size int
		name string
	}{{1440, "天"}, {60, "小时"}, {1, "分钟"}} {
		if n := minutes / unit.size; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, unit.name))
			minutes %= unit.size
		}
	}
	return word + " " + strings.Join(parts, " ")
}

// EarliestScheduled is the first planned time among the rounds' starts and
// their heats' own times, which is the earliest effective heat time when
// every round has heats; "" when nothing is scheduled.
func EarliestScheduled(rounds []Round) string {
	first := ""
	consider := func(at string) {
		if ValidScheduled(at) && (first == "" || at < first) {
			first = at
		}
	}
	for _, r := range rounds {
		consider(r.ScheduledAt)
		for _, h := range r.Heats {
			consider(h.ScheduledAt)
		}
	}
	return first
}

// PlannedDay is the day an event is shown on: the day of its earliest
// planned time, or else EventDay (its date, else the meet's start).
func PlannedDay(e Event, rounds []Round, meetStart string) string {
	if day := ScheduledDay(EarliestScheduled(rounds)); day != "" {
		return day
	}
	return EventDay(e, meetStart)
}

// OutsideMeet reports whether a planned time's day falls outside the
// meet's days. Unscheduled times and meets without a start are never
// outside.
func OutsideMeet(at string, m Meet) bool {
	day := ScheduledDay(at)
	if day == "" || m.StartDate == "" {
		return false
	}
	end := m.EndDate
	if end == "" {
		end = m.StartDate
	}
	return day < m.StartDate || day > end
}

// ScheduleOrder returns a meet's events as its lists show them: ordinary
// events by day (PlannedDay), then by their earliest planned time, then in
// the given order (creation order); an event without a day or a time goes
// after those that have one. All-around components follow in the given
// order, which is their all-around's (docs/agents/domain.md「时间」).
func ScheduleOrder(events []Event, rounds map[string][]Round, meetStart string) []Event {
	type keyed struct {
		event Event
		day   string
		at    string
	}
	var ordinary []keyed
	var components []Event
	for _, e := range events {
		if e.CombinedID != "" {
			components = append(components, e)
			continue
		}
		day := PlannedDay(e, rounds[e.ID], meetStart)
		if !ValidDate(day) {
			day = ""
		}
		ordinary = append(ordinary, keyed{e, day, EarliestScheduled(rounds[e.ID])})
	}
	// "" sorts last; otherwise the stored forms compare as text.
	before := func(a, b string) (less, decided bool) {
		switch {
		case a == b:
			return false, false
		case a == "":
			return false, true
		case b == "":
			return true, true
		}
		return a < b, true
	}
	sort.SliceStable(ordinary, func(i, j int) bool {
		if less, ok := before(ordinary[i].day, ordinary[j].day); ok {
			return less
		}
		less, _ := before(ordinary[i].at, ordinary[j].at)
		return less
	})
	out := make([]Event, 0, len(events))
	for _, k := range ordinary {
		out = append(out, k.event)
	}
	return append(out, components...)
}
