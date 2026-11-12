package service

import (
	"errors"
	"strings"
	"testing"
)

// roundStart sets the start of an event's first round.
func roundStart(t *testing.T, f *fixture, at string) {
	t.Helper()
	_, err := f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: f.detail().Rounds[0].ID, ScheduledAt: at})
	must(t, err)
}

// timetableFixture is a meet with a 100m (two heats), a relay whose team
// holds 选手01 and a two-event all-around entered by 选手01 and 选手02.
type timetableFixture struct {
	*fixture
	sprint, relay string
	combined      []string // component event ids
}

func newTimetableFixture(t *testing.T) *timetableFixture {
	t.Helper()
	f := newFixture(t, 4, track())
	tf := &timetableFixture{fixture: f, sprint: f.event}
	f.addHeat(0, f.athletes[0], f.athletes[1])
	f.addHeat(0, f.athletes[2], f.athletes[3])

	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", relay())
	must(t, err)
	tf.relay, f.event = e.ID, e.ID
	tm := team(t, f, "甲队", f.athletes[0])
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{"new"},
		Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 1, TeamID: tm.ID}}}}}}}))

	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes[:2]))
	d, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	for _, part := range d.Components {
		tf.combined = append(tf.combined, part.Event.ID)
		f.event = part.Event.ID
		f.addHeat(0, f.athletes[:2]...)
	}
	f.event = tf.sprint
	return tf
}

func (tf *timetableFixture) on(eventID string) *fixture {
	tf.event = eventID
	return tf.fixture
}

func TestTimetableAndConflicts(t *testing.T) {
	tf := newTimetableFixture(t)
	t.Run("nothing planned", func(t *testing.T) {
		tt, err := tf.svc.Schedule(tf.ctx, tf.meet)
		must(t, err)
		if len(tt.Days) != 0 || len(tt.Unscheduled) != 4 || len(tt.Conflicts) != 0 {
			t.Fatalf("unscheduled timetable: %+v", tt)
		}
		if r := tt.Unscheduled[0]; r.EventID != tf.sprint || r.HeatCount != 2 || r.People != 4 {
			t.Fatalf("sprint row: %+v", r)
		}
	})

	// 100m: round at 09:00, heat 2 its own 09:05 for 5 minutes, heat 1
	// follows the round (a point). Relay at 09:03 for 4 minutes: 选手01
	// through the team. Long jump of the all-around at 09:00 next day.
	roundStart(t, tf.on(tf.sprint), "2026-11-12T09:00")
	five := 5
	sprint := tf.detail()
	h2 := sprint.Rounds[0].Heats[1]
	must(t, tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.sprint, HeatID: h2.ID, Base: h2.Base, Name: "", ScheduledAt: "2026-11-12T09:05", DurationMinutes: &five}))
	err := tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.sprint, HeatID: h2.ID, Base: h2.Base, ScheduledAt: "2026-11-12T09:06"})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("edit with an old fingerprint: %v", err)
	}
	relayHeat := tf.on(tf.relay).detail().Rounds[0].Heats[0]
	four := 4
	must(t, tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, ScheduledAt: "2026-11-12T09:00", DurationMinutes: &four}))
	roundStart(t, tf.on(tf.combined[1]), "2026-11-13T09:00")
	if got := tf.on(tf.relay).detail().Rounds[0].Heats[0]; len(got.Slots) != 1 || got.Slots[0].TeamID == "" {
		t.Fatal("editing the time changed the slots")
	}

	tt, err := tf.svc.Schedule(tf.ctx, tf.meet)
	must(t, err)
	if len(tt.Days) != 2 || tt.Days[0].Day != "2026-11-12" || len(tt.Days[0].Rounds) != 2 || len(tt.Unscheduled) != 1 || tt.Unscheduled[0].EventID != tf.combined[0] {
		t.Fatalf("days: %+v", tt)
	}
	first := tt.Days[0].Rounds[0]
	if first.EventID != tf.sprint || first.StartText != "09:00" || !first.Heats[0].ScheduleInherited || first.Heats[1].EndText != "09:10" || first.Heats[1].DurationText != "5 分钟" {
		t.Fatalf("sprint round: %+v", first)
	}
	if tt.Days[1].Rounds[0].ScheduledText != "09:00" {
		t.Fatalf("next day text: %+v", tt.Days[1].Rounds[0])
	}
	// 选手01: 100m heat 1 at 09:00 (point) and relay 09:00–09:04 start at
	// the same minute: an overlap. Nobody else is in two timed heats.
	if len(tt.Conflicts) != 1 || !tt.Conflicts[0].Overlap || tt.Conflicts[0].AthleteName != "选手01" || tt.Conflicts[0].Text != "时间重叠" {
		t.Fatalf("conflicts: %+v", tt.Conflicts)
	}
	if c := first.Heats[0].Conflicts; len(c) != 1 || c[0].Other.EventID != tf.relay {
		t.Fatalf("heat conflicts: %+v", c)
	}
	hc, err := tf.svc.EventConflicts(tf.ctx, tf.relay)
	must(t, err)
	if len(hc) != 1 || hc[0].HeatID != relayHeat.ID || hc[0].Conflicts[0].Other.HeatLabel != "第1组" {
		t.Fatalf("event conflicts: %+v", hc)
	}

	// Move the relay to 08:50–08:54: no overlap; a 10-minute rest flags
	// 08:54 → 09:00, which leaves 6 minutes.
	relayHeat = tf.on(tf.relay).detail().Rounds[0].Heats[0]
	must(t, tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, ScheduledAt: "2026-11-12T08:50", DurationMinutes: &four}))
	cl, err := tf.svc.MeetConflicts(tf.ctx, tf.meet)
	must(t, err)
	if len(cl.Conflicts) != 0 {
		t.Fatalf("no rest asked: %+v", cl)
	}
	prefs, err := tf.svc.Preferences(tf.ctx)
	must(t, err)
	prefs.MinRestMinutes = 10
	must(t, tf.svc.SavePreferences(tf.ctx, prefs))
	cl, err = tf.svc.MeetConflicts(tf.ctx, tf.meet)
	must(t, err)
	if cl.MinRestMinutes != 10 || len(cl.Conflicts) != 1 || cl.Conflicts[0].Overlap || cl.Conflicts[0].Text != "间隔 6 分钟" || cl.Conflicts[0].First.EventID != tf.relay {
		t.Fatalf("rest conflicts: %+v", cl)
	}
	prefs.MinRestMinutes = -1
	wantInvalid(t, tf.svc.SavePreferences(tf.ctx, prefs), "最短休息时间")

	// All-around components take part: the all-around's 100m at 09:02 is
	// 2 minutes after 100m heat 1 (选手01, 选手02) and 8 after the relay
	// (选手01).
	roundStart(t, tf.on(tf.combined[0]), "2026-11-12T09:02")
	cl, err = tf.svc.MeetConflicts(tf.ctx, tf.meet)
	must(t, err)
	names := map[string]int{}
	for _, c := range cl.Conflicts {
		if c.Second.EventID == tf.combined[0] || c.First.EventID == tf.combined[0] {
			names[c.AthleteName]++
		}
	}
	if names["选手01"] != 2 || names["选手02"] != 1 {
		t.Fatalf("all-around conflicts: %+v", cl.Conflicts)
	}
}

func TestTimetableReportSection(t *testing.T) {
	tf := newTimetableFixture(t)
	roundStart(t, tf.on(tf.sprint), "2026-11-12T09:00")
	html, err := tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, nil)
	must(t, err)
	if !strings.Contains(html, "竞赛日程表") || !strings.Contains(html, "未安排时间") || !strings.Contains(html, `<td class="time">09:00</td>`) {
		t.Fatal("start list lacks the schedule")
	}
	results, err := tf.svc.RenderReport(tf.ctx, ReportResults, tf.meet, nil)
	must(t, err)
	if strings.Contains(results, "竞赛日程表") {
		t.Fatal("results print the schedule")
	}
	only, err := tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, []string{tf.relay})
	must(t, err)
	if strings.Contains(only, "<td>男子100m</td>") {
		t.Fatal("schedule ignores the selection")
	}
	prefs, err := tf.svc.Preferences(tf.ctx)
	must(t, err)
	prefs.PrintSchedule = false
	must(t, tf.svc.SavePreferences(tf.ctx, prefs))
	html, err = tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, nil)
	must(t, err)
	if strings.Contains(html, "竞赛日程表") {
		t.Fatal("schedule printed while switched off")
	}
}

func TestMultiRoundSchedule(t *testing.T) {
	tf := newTimetableFixture(t)
	sprintRound := tf.on(tf.sprint).detail().Rounds[0]
	relayRound := tf.on(tf.relay).detail().Rounds[0]
	in := ScheduleRequest{Rounds: []RoundRef{{tf.sprint, sprintRound.ID}, {tf.relay, relayRound.ID}},
		StartAt: "2026-11-12T09:00", IntervalMinutes: 5, RoundGapMinutes: 10}
	p, err := tf.svc.PreviewSchedule(tf.ctx, in)
	must(t, err)
	want := []string{"2026-11-12T09:00", "2026-11-12T09:05", "2026-11-12T09:20"}
	if len(p.Changes) != 3 || len(p.Errors) != 0 {
		t.Fatalf("multi-round preview: %+v", p)
	}
	for i, c := range p.Changes {
		if c.After != want[i] {
			t.Fatalf("time %d: %s", i, c.After)
		}
	}
	if p.Changes[2].RoundID != relayRound.ID || !strings.HasPrefix(p.Changes[2].Round, "男子4×100m") || len(p.Conflicts) != 0 {
		t.Fatalf("relay change: %+v", p)
	}
	// 选手01 runs 100m heat 1 09:00–09:05 and the relay at 09:20: a
	// 16-minute rest flags the 15 minutes between them in the preview.
	prefs, err := tf.svc.Preferences(tf.ctx)
	must(t, err)
	prefs.MinRestMinutes = 16
	must(t, tf.svc.SavePreferences(tf.ctx, prefs))
	p, err = tf.svc.PreviewSchedule(tf.ctx, in)
	must(t, err)
	if len(p.Conflicts) != 1 || p.Conflicts[0].GapMinutes != 15 || p.Warnings[len(p.Warnings)-1] != "排完后有 1 处时间冲突" {
		t.Fatalf("preview conflicts: %+v", p)
	}
	hash := p.PreviewHash

	// Stale: a heat of the second round changes after the preview.
	relayHeat := relayRound.Heats[0]
	one := 1
	must(t, tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, DurationMinutes: &one}))
	_, err = tf.svc.ApplySchedule(tf.ctx, ui, in, hash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("stale multi-round preview: %v", err)
	}
	p, err = tf.svc.PreviewSchedule(tf.ctx, in)
	must(t, err)
	before := len(*tf.changes)
	_, err = tf.svc.ApplySchedule(tf.ctx, ui, in, p.PreviewHash, true)
	must(t, err)
	if len(*tf.changes) != before+1 || (*tf.changes)[before].EventID != "" || (*tf.changes)[before].Kind != "heats" {
		t.Fatalf("one change for several events: %+v", (*tf.changes)[before:])
	}
	if h := tf.on(tf.relay).detail().Rounds[0].Heats[0]; h.ScheduledAt != "2026-11-12T09:20" || *h.DurationMinutes != 5 {
		t.Fatalf("relay heat: %+v", h)
	}
	rows, err := tf.svc.store.Query(tf.ctx, `SELECT message FROM audit_log WHERE action = 'heats.scheduled'`)
	must(t, err)
	if len(rows.Values) != 1 || rows.Values[0][0] != "2 个轮次" {
		t.Fatalf("audit: %+v", rows.Values)
	}

	// Rounds must be distinct and of one meet; the gap is bounded.
	_, err = tf.svc.PreviewSchedule(tf.ctx, ScheduleRequest{Rounds: []RoundRef{{tf.sprint, sprintRound.ID}, {tf.sprint, sprintRound.ID}}, StartAt: "2026-11-12T09:00", IntervalMinutes: 5})
	wantInvalid(t, err, "轮次重复")
	m, err := tf.svc.CreateMeet(tf.ctx, ui, MeetInput{Name: "另一赛事", StartDate: "2026-11-12"})
	must(t, err)
	e, err := tf.svc.SaveEvent(tf.ctx, ui, m.ID, "", track())
	must(t, err)
	otherRound := tf.on(e.ID).detail().Rounds[0].ID
	_, err = tf.svc.PreviewSchedule(tf.ctx, ScheduleRequest{Rounds: []RoundRef{{tf.sprint, sprintRound.ID}, {e.ID, otherRound}}, StartAt: "2026-11-12T09:00", IntervalMinutes: 5})
	wantInvalid(t, err, "同一赛事")
	p, err = tf.svc.PreviewSchedule(tf.ctx, ScheduleRequest{Rounds: in.Rounds, StartAt: "2026-11-12T09:00", IntervalMinutes: 5, RoundGapMinutes: 2000})
	must(t, err)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0], "轮次间隔") {
		t.Fatalf("gap bound: %+v", p.Errors)
	}
}
