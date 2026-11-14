package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
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

func editHeat(t *testing.T, f *fixture, in HeatEdit) *HeatEditResult {
	t.Helper()
	out, err := f.svc.EditHeat(f.ctx, ui, in)
	must(t, err)
	return out
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
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.sprint, HeatID: h2.ID, Base: h2.Base, Name: "", ScheduledAt: "2026-11-12T09:05", DurationMinutes: &five})
	_, err := tf.svc.EditHeat(tf.ctx, ui, HeatEdit{EventID: tf.sprint, HeatID: h2.ID, Base: h2.Base, ScheduledAt: "2026-11-12T09:06"})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("edit with an old fingerprint: %v", err)
	}
	relayHeat := tf.on(tf.relay).detail().Rounds[0].Heats[0]
	four := 4
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, ScheduledAt: "2026-11-12T09:00", DurationMinutes: &four})
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
	relayDetail := tf.on(tf.relay).detail()
	hc, err := tf.svc.DraftConflicts(tf.ctx, DraftConflictRequest{EventID: tf.relay, Heats: []DraftTime{{Key: relayHeat.ID, RoundID: relayDetail.Rounds[0].ID,
		ScheduledAt: "2026-11-12T09:00", DurationMinutes: &four, Competitors: []string{relayDetail.Rounds[0].Heats[0].Slots[0].TeamID}}}})
	must(t, err)
	if len(hc) != 1 || hc[0].HeatID != relayHeat.ID || hc[0].Conflicts[0].Other.HeatLabel != "第1组" || hc[0].Conflicts[0].Other.Text != "2026-11-12 09:00 男子100m 决赛 第1组" {
		t.Fatalf("event conflicts: %+v", hc)
	}

	// Move the relay to 08:50–08:54: no overlap; a 10-minute rest flags
	// 08:54 → 09:00, which leaves 6 minutes.
	relayHeat = tf.on(tf.relay).detail().Rounds[0].Heats[0]
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, ScheduledAt: "2026-11-12T08:50", DurationMinutes: &four})
	cl, err := tf.svc.MeetConflicts(tf.ctx, tf.meet)
	must(t, err)
	if len(cl.Conflicts) != 0 {
		t.Fatalf("no rest asked: %+v", cl)
	}
	prefs, err := tf.svc.MeetSettings(tf.ctx, tf.meet)
	must(t, err)
	prefs.MinRestMinutes = 10
	must(t, tf.svc.SaveMeetSettings(tf.ctx, ui, tf.meet, prefs))
	cl, err = tf.svc.MeetConflicts(tf.ctx, tf.meet)
	must(t, err)
	if cl.MinRestMinutes != 10 || len(cl.Conflicts) != 1 || cl.Conflicts[0].Overlap || cl.Conflicts[0].Text != "间隔 6 分钟" || cl.Conflicts[0].First.EventID != tf.relay {
		t.Fatalf("rest conflicts: %+v", cl)
	}
	prefs.MinRestMinutes = -1
	wantInvalid(t, tf.svc.SaveMeetSettings(tf.ctx, ui, tf.meet, prefs), "最短休息时间")

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
	if !strings.Contains(html, "竞赛日程表") || !strings.Contains(html, `<td class="time">09:00</td>`) {
		t.Fatal("start list lacks the schedule")
	}
	// Only scheduled rounds are printed: the relay and the all-around have no time.
	if strings.Contains(html, "未安排时间") || strings.Contains(html, "<td>男子4×100m</td>") {
		t.Fatal("unscheduled rounds printed")
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
	prefs, err := tf.svc.MeetSettings(tf.ctx, tf.meet)
	must(t, err)
	prefs.PrintSchedule = false
	must(t, tf.svc.SaveMeetSettings(tf.ctx, ui, tf.meet, prefs))
	html, err = tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, nil)
	must(t, err)
	if strings.Contains(html, "竞赛日程表") {
		t.Fatal("schedule printed while switched off")
	}
}

func TestStartListRoster(t *testing.T) {
	tf := newTimetableFixture(t)
	// A woman with no unit and no bib, entered nowhere.
	_, err := tf.svc.SaveAthlete(tf.ctx, ui, tf.meet, "", AthleteInput{Name: "散客", Gender: "女"})
	must(t, err)
	html, err := tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, nil)
	must(t, err)
	for _, want := range []string{"代表队名单", `<small>2 人</small>`, `<span class="bib">001</span>选手01`, "无参赛单位", `<span class="bib"></span>散客`} {
		if !strings.Contains(html, want) {
			t.Fatalf("roster lacks %q", want)
		}
	}
	if strings.Index(html, "代表队名单") > strings.Index(html, "竞赛日程表") && strings.Contains(html, "竞赛日程表") {
		t.Fatal("roster printed after the schedule")
	}
	// The relay's team has one member: only that athlete's unit is listed.
	only, err := tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, []string{tf.relay})
	must(t, err)
	if !strings.Contains(only, `<small>1 人</small>`) || strings.Contains(only, `<span class="bib">002</span>`) || strings.Contains(only, "无参赛单位") {
		t.Fatal("roster ignores the selection")
	}
	results, err := tf.svc.RenderReport(tf.ctx, ReportResults, tf.meet, nil)
	must(t, err)
	if strings.Contains(results, "代表队名单") {
		t.Fatal("results print the roster")
	}
	prefs, err := tf.svc.MeetSettings(tf.ctx, tf.meet)
	must(t, err)
	if !prefs.PrintRoster {
		t.Fatal("roster is off by default")
	}
	prefs.PrintRoster = false
	must(t, tf.svc.SaveMeetSettings(tf.ctx, ui, tf.meet, prefs))
	html, err = tf.svc.RenderReport(tf.ctx, ReportStartList, tf.meet, nil)
	must(t, err)
	if strings.Contains(html, "代表队名单") {
		t.Fatal("roster printed while switched off")
	}
}

func TestRosterBook(t *testing.T) {
	units := []domain.Unit{{ID: "u1", Name: "一班"}, {ID: "u2", Name: "二班"}, {ID: "u3", Name: "三班"}}
	athletes := []domain.Athlete{
		{ID: "a", Name: "甲", Gender: domain.GenderFemale, Bib: "12", UnitID: "u1"},
		{ID: "b", Name: "乙", Gender: domain.GenderMale, Bib: "101", UnitID: "u1"},
		{ID: "c", Name: "丙", Gender: domain.GenderMale, Bib: "9", UnitID: "u1"},
		{ID: "d", Name: "丁", Gender: domain.GenderMale, Bib: "", UnitID: "u1"},
		{ID: "e", Name: "戊", Gender: domain.GenderMale, Bib: "A1", UnitID: "u1"},
		{ID: "f", Name: "己", UnitID: "u2"},
		{ID: "g", Name: "庚", Gender: domain.GenderMale, UnitID: "gone"},
	}
	got := rosterBook(units, athletes, nil)
	if len(got) != 3 || got[0].Name != "一班" || got[0].Count != 5 || got[2].Name != "无参赛单位" {
		t.Fatalf("units: %+v", got)
	}
	men := got[0].Groups[0]
	names := ""
	for _, a := range men.Athletes {
		names += a.Name
	}
	if men.Label != "男子" || names != "丙乙戊丁" || got[0].Groups[1].Label != "女子" {
		t.Fatalf("groups: %+v", got[0].Groups)
	}
	if rows := men.Rows(); len(rows) != 1 || len(rows[0]) != 4 {
		t.Fatalf("rows: %+v", rows)
	}
	if g := got[1].Groups; len(g) != 1 || g[0].Label != "" {
		t.Fatalf("unknown gender: %+v", g)
	}
	if got := rosterBook(units, athletes, map[string]bool{"a": true}); len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("listed: %+v", got)
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
	prefs, err := tf.svc.MeetSettings(tf.ctx, tf.meet)
	must(t, err)
	prefs.MinRestMinutes = 16
	must(t, tf.svc.SaveMeetSettings(tf.ctx, ui, tf.meet, prefs))
	p, err = tf.svc.PreviewSchedule(tf.ctx, in)
	must(t, err)
	if len(p.Conflicts) != 1 || p.Conflicts[0].GapMinutes != 15 || p.Warnings[len(p.Warnings)-1] != "排完后有 1 处时间冲突" {
		t.Fatalf("preview conflicts: %+v", p)
	}
	hash := p.PreviewHash

	// Stale: a heat of the second round changes after the preview.
	relayHeat := relayRound.Heats[0]
	one := 1
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, DurationMinutes: &one})
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

func TestMultiRoundScheduleFollowsRequestOrder(t *testing.T) {
	tf := newTimetableFixture(t)
	sprintRound := tf.on(tf.sprint).detail().Rounds[0]
	relayRound := tf.on(tf.relay).detail().Rounds[0]
	// The relay first, then the 100m: rounds[] is the order, not the event list.
	in := ScheduleRequest{Rounds: []RoundRef{{tf.relay, relayRound.ID}, {tf.sprint, sprintRound.ID}}, StartAt: "2026-11-12T09:00", IntervalMinutes: 5}
	p, err := tf.svc.PreviewSchedule(tf.ctx, in)
	must(t, err)
	got := []string{}
	for _, c := range p.Changes {
		got = append(got, c.RoundID+"@"+c.After)
	}
	want := []string{relayRound.ID + "@2026-11-12T09:00", sprintRound.ID + "@2026-11-12T09:05", sprintRound.ID + "@2026-11-12T09:10"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order: %v", got)
	}
	reversed := in
	reversed.Rounds = []RoundRef{in.Rounds[1], in.Rounds[0]}
	q, err := tf.svc.PreviewSchedule(tf.ctx, reversed)
	must(t, err)
	if q.PreviewHash == p.PreviewHash || q.Changes[0].RoundID != sprintRound.ID {
		t.Fatal("another order must be another plan")
	}
	_, err = tf.svc.ApplySchedule(tf.ctx, ui, reversed, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("a preview of another order confirmed: %v", err)
	}
}

func TestSetHeatTime(t *testing.T) {
	tf := newTimetableFixture(t)
	sprint := tf.on(tf.sprint).detail()
	h := sprint.Rounds[0].Heats[0]
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.sprint, HeatID: h.ID, Base: h.Base, Name: "甲组"})
	must(t, tf.svc.SaveResults(tf.ctx, ui, h.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	h = tf.detail().Rounds[0].Heats[0]
	relayHeat := tf.on(tf.relay).detail().Rounds[0].Heats[0]
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.relay, HeatID: relayHeat.ID, Base: relayHeat.Base, ScheduledAt: "2026-11-12T09:02"})

	five := 5
	out, err := tf.svc.SetHeatTime(tf.ctx, store.SourceMCP, HeatTimeEdit{EventID: tf.sprint, HeatID: h.ID, Base: h.Base, ScheduledAt: "2026-11-13T09:00", DurationMinutes: &five})
	must(t, err)
	after := tf.on(tf.sprint).detail().Rounds[0].Heats[0]
	if after.Name != "甲组" || after.ScheduledAt != "2026-11-13T09:00" || *after.DurationMinutes != 5 || after.Base != out.Base {
		t.Fatalf("heat after: %+v / %+v", after, out)
	}
	if len(after.Slots) != len(h.Slots) || after.Slots[0].Result == nil || after.Slots[0].Result.MarkText == "" {
		t.Fatal("slots or results changed")
	}
	if len(out.Warnings) != 1 || out.Warnings[0] != "时间不在赛事日期内" || len(out.Conflicts) != 0 {
		t.Fatalf("result: %+v", out)
	}
	_, err = tf.svc.SetHeatTime(tf.ctx, store.SourceMCP, HeatTimeEdit{EventID: tf.sprint, HeatID: h.ID, Base: h.Base, ScheduledAt: "2026-11-12T09:00"})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("old fingerprint: %v", err)
	}
	// Back next to the relay (选手01 at 09:02): the conflict comes back with it.
	out, err = tf.svc.SetHeatTime(tf.ctx, store.SourceMCP, HeatTimeEdit{EventID: tf.sprint, HeatID: h.ID, Base: out.Base, ScheduledAt: "2026-11-12T09:00", DurationMinutes: &five})
	must(t, err)
	if len(out.Warnings) != 0 || len(out.Conflicts) != 1 || out.Conflicts[0].Other.EventID != tf.relay || out.Conflicts[0].AthleteName != "选手01" {
		t.Fatalf("conflicts: %+v", out)
	}
	_, err = tf.svc.SetHeatTime(tf.ctx, store.SourceMCP, HeatTimeEdit{EventID: tf.sprint, HeatID: h.ID, Base: out.Base, ScheduledAt: "9:00"})
	wantInvalid(t, err, "计划时间格式")
	zero := 0
	_, err = tf.svc.SetHeatTime(tf.ctx, store.SourceMCP, HeatTimeEdit{EventID: tf.sprint, HeatID: h.ID, Base: out.Base, DurationMinutes: &zero})
	wantInvalid(t, err, "预计用时")
	rows, err := tf.svc.store.Query(tf.ctx, `SELECT source FROM audit_log WHERE action = 'board.saved' AND source = 'mcp'`)
	must(t, err)
	if len(rows.Values) != 2 {
		t.Fatalf("audit: %+v", rows.Values)
	}
}

func TestDraftConflicts(t *testing.T) {
	tf := newTimetableFixture(t)
	roundStart(t, tf.on(tf.sprint), "2026-11-12T09:00")
	relay := tf.on(tf.relay).detail()
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.relay, HeatID: relay.Rounds[0].Heats[0].ID, Base: relay.Rounds[0].Heats[0].Base, ScheduledAt: "2026-11-12T09:00"})
	sprint := tf.on(tf.sprint).detail()
	r := sprint.Rounds[0]
	draft := func(heats ...DraftTime) DraftConflictRequest {
		return DraftConflictRequest{EventID: tf.sprint, Heats: heats}
	}
	saved := func(i int) DraftTime {
		h := r.Heats[i]
		d := DraftTime{Key: h.ID, RoundID: r.ID, ScheduledAt: h.ScheduledAt, DurationMinutes: h.DurationMinutes}
		for _, sl := range h.Slots {
			d.Competitors = append(d.Competitors, sl.AthleteID)
		}
		return d
	}
	// As saved: 选手01 in heat 1 at 09:00 and in the relay at 09:00.
	got, err := tf.svc.DraftConflicts(tf.ctx, draft(saved(0), saved(1)))
	must(t, err)
	if len(got) != 1 || got[0].HeatID != r.Heats[0].ID || got[0].Conflicts[0].Other.EventID != tf.relay {
		t.Fatalf("saved draft: %+v", got)
	}
	// Unsaved: heat 1 moved to 10:00, 选手01 moved into a new heat at 09:00.
	moved := saved(0)
	moved.ScheduledAt, moved.Competitors = "2026-11-12T10:00", moved.Competitors[1:]
	added := DraftTime{Key: "new-1", RoundID: r.ID, Name: "", Competitors: []string{tf.athletes[0]}}
	got, err = tf.svc.DraftConflicts(tf.ctx, draft(moved, saved(1), added))
	must(t, err)
	if len(got) != 1 || got[0].HeatID != "new-1" || got[0].Conflicts[0].AthleteName != "选手01" {
		t.Fatalf("unsaved draft: %+v", got)
	}
	// The other heat of the same draft is named by its place in the draft.
	clash := saved(1)
	clash.ScheduledAt, clash.Competitors = "2026-11-12T10:00", append(clash.Competitors, tf.athletes[1])
	got, err = tf.svc.DraftConflicts(tf.ctx, draft(moved, clash))
	must(t, err)
	if len(got) != 2 || got[0].Conflicts[0].Other.HeatLabel != "第2组" || got[1].Conflicts[0].Other.HeatLabel != "第1组" {
		t.Fatalf("draft labels: %+v", got)
	}
	// A heat left out of the draft (deleted there) no longer counts.
	got, err = tf.svc.DraftConflicts(tf.ctx, draft(saved(1)))
	must(t, err)
	if len(got) != 0 {
		t.Fatalf("deleted heat counted: %+v", got)
	}
	if d := tf.detail(); d.Rounds[0].Heats[0].Base != r.Heats[0].Base {
		t.Fatal("checking a draft wrote")
	}
}

func TestTimetablePeopleOrderAndDays(t *testing.T) {
	tf := newTimetableFixture(t)
	in := track()
	in.Discipline, in.Rounds = "200m", []RoundInput{{Stage: "heat", AdvanceByPlace: 2}}
	e, err := tf.svc.SaveEvent(tf.ctx, ui, tf.meet, "", in)
	must(t, err)
	tf.on(e.ID).enter(tf.athletes[:3]...)
	empty, err := tf.svc.Schedule(tf.ctx, tf.meet)
	must(t, err)
	// Unscheduled: rounds with heats first, then those without.
	var heats []int
	for _, r := range empty.Unscheduled {
		heats = append(heats, r.HeatCount)
	}
	if fmt.Sprint(heats) != "[2 1 1 1 0 0]" {
		t.Fatalf("unscheduled order: %v", heats)
	}
	byRound := map[string]TimetableRound{}
	for _, r := range empty.Unscheduled {
		byRound[r.EventID+r.RoundLabel] = r
	}
	prelim, final := byRound[e.ID+"预赛"], byRound[e.ID+"决赛"]
	if prelim.People != 3 || !prelim.PeopleEntered || final.People != 0 || final.PeopleEntered {
		t.Fatalf("entered people: %+v %+v", prelim, final)
	}
	if relay := byRound[tf.relay+"决赛"]; relay.People != 1 || relay.PeopleEntered {
		t.Fatalf("placed people: %+v", relay)
	}

	// Across midnight: the round is listed on its first day, later times carry their date.
	roundStart(t, tf.on(tf.sprint), "2026-11-12T23:55")
	r := tf.detail().Rounds[0]
	ten := 10
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.sprint, HeatID: r.Heats[1].ID, Base: r.Heats[1].Base, ScheduledAt: "2026-11-13T00:05", DurationMinutes: &ten})
	editHeat(t, tf.fixture, HeatEdit{EventID: tf.sprint, HeatID: r.Heats[0].ID, Base: r.Heats[0].Base, DurationMinutes: &ten})
	tt, err := tf.svc.Schedule(tf.ctx, tf.meet)
	must(t, err)
	if len(tt.Days) != 1 || tt.Days[0].Day != "2026-11-12" {
		t.Fatalf("days: %+v", tt.Days)
	}
	heat1, heat2 := tt.Days[0].Rounds[0].Heats[0], tt.Days[0].Rounds[0].Heats[1]
	if heat1.ScheduledText != "23:55" || heat1.EndText != "2026-11-13 00:05" || heat2.ScheduledText != "2026-11-13 00:05" || heat2.EndText != "2026-11-13 00:15" {
		t.Fatalf("midnight texts: %+v / %+v", heat1, heat2)
	}

	// A heat that competed while following the round keeps saying so when the round moves.
	must(t, tf.svc.SaveResults(tf.ctx, ui, heat1.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	roundStart(t, tf.fixture, "2026-11-12T15:00")
	tt, err = tf.svc.Schedule(tf.ctx, tf.meet)
	must(t, err)
	got := tt.Days[0].Rounds[0].Heats[0]
	if got.ScheduledText != "15:00" || !got.ScheduleInherited || got.KeepReason != "已有成绩" {
		t.Fatalf("scored inherited heat: %+v", got)
	}

	// An empty meet has no days and nothing unscheduled.
	m, err := tf.svc.CreateMeet(tf.ctx, ui, MeetInput{Name: "空赛事", StartDate: "2026-11-12"})
	must(t, err)
	none, err := tf.svc.Schedule(tf.ctx, m.ID)
	must(t, err)
	if len(none.Days) != 0 || len(none.Unscheduled) != 0 || len(none.Conflicts) != 0 {
		t.Fatalf("empty meet: %+v", none)
	}
	html, err := tf.svc.RenderReport(tf.ctx, ReportStartList, m.ID, nil)
	must(t, err)
	if strings.Contains(html, "竞赛日程表") {
		t.Fatal("empty schedule printed")
	}
}
