package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stmtc233/sportsmeet/internal/store"
)

// clock makes the fixture's recorded times predictable: each reading is
// one minute after the previous one, shown in UTC+8. Start after the
// fixture's own rows, which carry the real time.
func (f *fixture) clock(start time.Time) {
	now := start.UnixMilli()
	f.svc.store.Now = func() int64 { now += 60_000; return now }
	f.svc.zone = time.FixedZone("UTC+8", 8*3600)
}

// at fixes the clock at one moment, shown in UTC+8.
func (f *fixture) at(t time.Time) {
	f.svc.store.Now = func() int64 { return t.UnixMilli() }
	f.svc.zone = time.FixedZone("UTC+8", 8*3600)
}

func TestAuditLog(t *testing.T) {
	f := newFixture(t, 2, track())
	f.clock(time.Date(2030, 1, 1, 1, 0, 0, 0, time.UTC))
	other, err := f.svc.CreateMeet(f.ctx, store.SourceMCP, MeetInput{Name: "冬季运动会", StartDate: "2026-12-01"})
	must(t, err)
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes...)
	heat := f.detail().Rounds[0].Heats[0]
	must(t, f.svc.SaveResults(f.ctx, store.SourceMCP, heat.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))

	all, err := f.svc.ListAudit(f.ctx, AuditQuery{})
	must(t, err)
	if all.Next != nil || len(all.Rows) < 8 {
		t.Fatalf("rows = %d, next = %+v", len(all.Rows), all.Next)
	}
	first := all.Rows[0]
	if first.Action != "results.saved" || first.ActionText != "录入成绩" || first.SourceText != "MCP" ||
		first.MeetName != "秋季运动会" || first.HeatID != heat.ID || !strings.HasPrefix(first.AtText, "2030-01-01 09:") {
		t.Fatalf("newest row = %+v", first)
	}
	for i := 1; i < len(all.Rows); i++ {
		a, b := all.Rows[i-1], all.Rows[i]
		if a.At < b.At || a.At == b.At && a.ID < b.ID {
			t.Fatalf("rows out of order at %d: %+v then %+v", i, a, b)
		}
	}

	mcp, err := f.svc.ListAudit(f.ctx, AuditQuery{Source: "mcp"})
	must(t, err)
	if len(mcp.Rows) != 2 || mcp.Rows[1].MeetID != other.ID || mcp.Rows[1].ActionText != "新建赛事" {
		t.Fatalf("mcp rows = %+v", mcp.Rows)
	}
	meet, err := f.svc.ListAudit(f.ctx, AuditQuery{MeetID: other.ID})
	must(t, err)
	if len(meet.Rows) != 1 || meet.Rows[0].MeetName != "冬季运动会" {
		t.Fatalf("meet rows = %+v", meet.Rows)
	}

	// Pages continue where the previous one ended, also across equal times.
	f.svc.store.Now = func() int64 { return all.Rows[0].At }
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, "", UnitInput{Name: "三班"})
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, "", UnitInput{Name: "四班"})
	must(t, err)
	var paged []AuditRow
	var after *AuditCursor
	for range 20 {
		page, err := f.svc.ListAudit(f.ctx, AuditQuery{After: after, Limit: 3})
		must(t, err)
		paged = append(paged, page.Rows...)
		if after = page.Next; after == nil {
			break
		}
	}
	whole, err := f.svc.ListAudit(f.ctx, AuditQuery{Limit: 200})
	must(t, err)
	if len(paged) != len(whole.Rows) {
		t.Fatalf("paged %d rows, whole %d", len(paged), len(whole.Rows))
	}
	for i := range paged {
		if paged[i].ID != whole.Rows[i].ID {
			t.Fatalf("page row %d = %d, want %d", i, paged[i].ID, whole.Rows[i].ID)
		}
	}
	if whole.Rows[0].Message != "四班" || whole.Rows[1].Message != "三班" {
		t.Fatalf("equal times not in insertion order: %+v", whole.Rows[:2])
	}

	// The meet goes, its rows stay with their message but no meet name.
	must(t, f.svc.DeleteMeet(f.ctx, ui, other.ID))
	meet, err = f.svc.ListAudit(f.ctx, AuditQuery{MeetID: other.ID})
	must(t, err)
	if len(meet.Rows) != 2 || meet.Rows[1].MeetName != "" || meet.Rows[1].Message != "冬季运动会" {
		t.Fatalf("deleted meet rows = %+v", meet.Rows)
	}

	_, err = f.svc.ListAudit(f.ctx, AuditQuery{Source: "web"})
	wantInvalid(t, err, "来源")
	_, err = f.svc.ListAudit(f.ctx, AuditQuery{Limit: 201})
	wantInvalid(t, err, "每页")
}

// Every action a write logs has a text for the settings page.
func TestAuditActionTexts(t *testing.T) {
	files, err := filepath.Glob("*.go")
	must(t, err)
	action := regexp.MustCompile(`"((?:meet|unit|athlete|event|team|entry|board|heats|heat|round|results|settings|combined)\.[a-z]+)"`)
	seen := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || path == "audit.go" {
			continue
		}
		src, err := os.ReadFile(path)
		must(t, err)
		for _, m := range action.FindAllStringSubmatch(string(src), -1) {
			seen++
			if auditActions[m[1]] == "" {
				t.Errorf("%s: 动作 %s 没有显示文本,在 audit.go 的 auditActions 登记", path, m[1])
			}
		}
	}
	if seen < len(auditActions) {
		t.Errorf("found %d actions in the source, fewer than the %d texts", seen, len(auditActions))
	}
}

func TestEventTimes(t *testing.T) {
	f := newFixture(t, 4, track())
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes[:2]...)
	f.addHeat(0, f.athletes[2:]...)
	d := f.detail()
	if d.Day != "2026-11-12" {
		t.Fatalf("day = %q", d.Day)
	}
	r := d.Rounds[0]
	save := BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{r.Heats[0].ID, r.Heats[1].ID}, Changed: []HeatSave{
		{ID: r.Heats[0].ID, Base: r.Heats[0].Base, ScheduledAt: "2026-11-12T09:30", Slots: slotSaves(r.Heats[0])},
		{ID: r.Heats[1].ID, Base: r.Heats[1].Base, ScheduledAt: "2026-11-13T08:05", Slots: slotSaves(r.Heats[1])},
	}}}}
	must(t, f.svc.SaveBoard(f.ctx, ui, save))
	bad := save
	bad.Rounds = []RoundSave{{RoundID: r.ID, Order: save.Rounds[0].Order, Changed: []HeatSave{
		{ID: r.Heats[0].ID, Base: f.detail().Rounds[0].Heats[0].Base, ScheduledAt: "2026-02-30T09:30", Slots: slotSaves(r.Heats[0])}}}}
	wantInvalid(t, f.svc.SaveBoard(f.ctx, ui, bad), "计划时间")

	hs := f.detail().Rounds[0].Heats
	if hs[0].ScheduledText != "09:30" || hs[1].ScheduledText != "2026-11-13 08:05" || hs[0].ResultsUpdatedText != "" {
		t.Fatalf("heats = %+v / %+v", hs[0], hs[1])
	}
	f.at(time.Date(2026, 11, 12, 1, 40, 0, 0, time.UTC))
	must(t, f.svc.SaveResults(f.ctx, ui, hs[0].ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	f.at(time.Date(2026, 11, 12, 1, 42, 30, 0, time.UTC))
	must(t, f.svc.SaveResults(f.ctx, ui, hs[0].ID, []ResultInput{{Position: 2, Mark: "12.50"}}))
	f.at(time.Date(2026, 11, 12, 2, 8, 0, 0, time.UTC))
	hs = f.detail().Rounds[0].Heats
	// The latest of the heat's results, in UTC+8 to the minute.
	if hs[0].ResultsUpdatedText != "2026-11-12 09:42" || hs[1].ResultsUpdatedText != "" {
		t.Fatalf("results updated = %q / %q", hs[0].ResultsUpdatedText, hs[1].ResultsUpdatedText)
	}

	html, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	for _, want := range []string{"打印时间 2026-11-12 10:08", "<small>09:30</small>", "<small>2026-11-13 08:05</small>"} {
		if !strings.Contains(html, want) {
			t.Errorf("start list lacks %q", want)
		}
	}
	meets, err := f.svc.ListMeets(f.ctx)
	must(t, err)
	if meets[0].DatesText != "2026-11-12" {
		t.Errorf("dates text = %q", meets[0].DatesText)
	}
}

func slotSaves(h HeatView) []SlotSave {
	out := make([]SlotSave, len(h.Slots))
	for i, s := range h.Slots {
		out[i] = SlotSave{Position: s.Position, AthleteID: s.AthleteID}
	}
	return out
}

func TestEventListOrder(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	add := func(discipline, date string) string {
		e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: discipline, Kind: "track", LaneCount: 8, Date: date})
		must(t, err)
		return e.ID
	}
	untimed := add("400m", "")
	late := add("200m", "")
	day2 := add("800m", "2026-11-13")
	early := add("100m", "2026-11-12")
	schedule := func(eventID, at string) {
		f.event = eventID
		f.enter(f.athletes[0])
		f.addHeat(len(f.detail().Rounds)-1, f.athletes[0])
		r := f.detail().Rounds[0]
		h := r.Heats[0]
		must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: eventID, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{h.ID},
			Changed: []HeatSave{{ID: h.ID, Base: h.Base, ScheduledAt: at, Slots: slotSaves(h)}}}}}))
	}
	schedule(late, "2026-11-12T15:00")
	schedule(early, "2026-11-12T09:00")
	schedule(day2, "2026-11-13T09:00")

	rows, err := f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	var got []string
	for _, r := range rows {
		got = append(got, r.Event.ID+"="+r.TimeText)
	}
	// One event is on the second day, so every row carries its date.
	want := []string{early + "=2026-11-12 09:00", late + "=2026-11-12 15:00", untimed + "=2026-11-12", day2 + "=2026-11-13 09:00"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ListEvents = %v, want %v", got, want)
	}
	// With every event on the first day only the times show.
	must(t, f.svc.DeleteEvent(f.ctx, ui, day2, true))
	rows, err = f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	got = nil
	for _, r := range rows {
		got = append(got, r.Event.ID+"="+r.TimeText)
	}
	want = []string{early + "=09:00", late + "=15:00", untimed + "="}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("one day = %v, want %v", got, want)
	}
	targets, err := f.svc.ReportTargets(f.ctx, f.meet)
	must(t, err)
	if len(targets) != 3 || targets[0].ID != early || targets[2].ID != untimed {
		t.Fatalf("targets = %+v", targets)
	}
}

func TestRoundStartAndDuration(t *testing.T) {
	f := newFixture(t, 2, track())
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes[0])
	f.addHeat(0, f.athletes[1])
	d := f.detail()
	r := d.Rounds[0]
	changes := len(*f.changes)

	// A heat without its own time follows the round's start.
	res, err := f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: r.ID, ScheduledAt: "2026-11-13T14:00"})
	must(t, err)
	if len(res.Warnings) != 1 || res.Warnings[0] != "时间不在赛事日期内" {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	if len(*f.changes) != changes+1 || (*f.changes)[changes].Kind != "heats" {
		t.Fatal("round start not notified")
	}
	d = f.detail()
	h0, h1 := d.Rounds[0].Heats[0], d.Rounds[0].Heats[1]
	if d.Day != "2026-11-13" || d.Rounds[0].ScheduledText != "14:00" || !h0.ScheduleInherited || h0.EffectiveAt != "2026-11-13T14:00" ||
		h0.ScheduledAt != "" || h0.ScheduledText != "14:00" || h0.Base != r.Heats[0].Base {
		t.Fatalf("inherited: day %s, %+v", d.Day, h0)
	}

	// Own time and duration: shown as the heat's own, ending duration later.
	five := 5
	save := BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{h0.ID, h1.ID}, Changed: []HeatSave{
		{ID: h1.ID, Base: h1.Base, ScheduledAt: "2026-11-13T14:20", DurationMinutes: &five, Slots: slotSaves(h1)}}}}}
	must(t, f.svc.SaveBoard(f.ctx, ui, save))
	h1 = f.detail().Rounds[0].Heats[1]
	if h1.ScheduleInherited || h1.ScheduledText != "14:20" || h1.DurationText != "5 分钟" || h1.EndAt != "2026-11-13T14:25" || h1.Base == save.Rounds[0].Changed[0].Base {
		t.Fatalf("own time: %+v", h1)
	}
	for _, bad := range []int{0, 1441} {
		again := f.detail().Rounds[0].Heats[1]
		wantInvalid(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{h0.ID, h1.ID},
			Changed: []HeatSave{{ID: again.ID, Base: again.Base, DurationMinutes: &bad, Slots: slotSaves(again)}}}}}), "预计用时")
	}

	// The list, the start list and the export use the effective time.
	rows, err := f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	if rows[0].TimeText != "2026-11-13 14:00" {
		t.Fatalf("list time = %q", rows[0].TimeText)
	}
	html, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "<small>14:00</small>") || !strings.Contains(html, "<small>14:20</small>") {
		t.Fatal("start list lacks the effective times")
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	er := file.Events[0].Rounds[0]
	if er.ScheduledAt == nil || *er.ScheduledAt != "2026-11-13T14:00" || er.Heats[0].ScheduledAt != nil ||
		er.Heats[1].DurationMinutes == nil || *er.Heats[1].DurationMinutes != 5 {
		t.Fatalf("export: %+v", er)
	}

	// Inside the meet's days, no warning; the same time again writes nothing.
	warning, err := f.svc.ScheduleWarning(f.ctx, f.event, "2026-11-12T08:00")
	must(t, err)
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if warning, _ = f.svc.ScheduleWarning(f.ctx, f.event, "2026-11-11T08:00"); warning == "" {
		t.Fatal("no warning before the meet")
	}
	changes = len(*f.changes)
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: r.ID, ScheduledAt: "2026-11-13T14:00"})
	must(t, err)
	if len(*f.changes) != changes {
		t.Fatal("unchanged round start notified")
	}
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: r.ID, ScheduledAt: "2026-11-13 14:00"})
	wantInvalid(t, err, "计划时间")
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: "missing", ScheduledAt: ""})
	wantInvalid(t, err, "轮次")
	res, err = f.svc.SetRoundStart(f.ctx, store.SourceMCP, RoundStart{EventID: f.event, RoundID: r.ID, ScheduledAt: ""})
	must(t, err)
	if h := f.detail().Rounds[0].Heats[0]; len(res.Warnings) != 0 || h.EffectiveAt != "" || h.ScheduleInherited || h.ScheduledText != "" {
		t.Fatal("cleared round start")
	}
	audit, err := f.svc.ListAudit(f.ctx, AuditQuery{MeetID: f.meet})
	must(t, err)
	if audit.Rows[0].ActionText != "修改轮次开始时间" || audit.Rows[0].Source != "mcp" {
		t.Fatalf("audit = %+v", audit.Rows[0])
	}

	// Batch scheduling shows an inherited time as the round's and warns outside the meet.
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: r.ID, ScheduledAt: "2026-11-12T09:00"})
	must(t, err)
	p, err := f.svc.PreviewSchedule(f.ctx, ScheduleRequest{EventID: f.event, RoundID: r.ID, StartAt: "2026-11-12T23:58", IntervalMinutes: 5})
	must(t, err)
	if len(p.Changes) != 2 || p.Changes[0].BeforeText != "2026-11-12 09:00（沿用轮次）" || p.Changes[1].BeforeText != "2026-11-13 14:20 · 5 分钟" ||
		len(p.Warnings) != 1 || p.Warnings[0] != "1 组不在赛事日期内" {
		t.Fatalf("preview: %+v", p)
	}
}

func TestCheckHeatPlan(t *testing.T) {
	p, err := CheckHeatPlan("2026-11-12T09:00", "", " 12 ", "2026-11-12")
	must(t, err)
	if p.ScheduledAt != "" || *p.DurationMinutes != 12 || !p.ScheduleInherited || p.ScheduledText != "09:00" || p.DurationText != "12 分钟" || p.EndAt != "2026-11-12T09:12" {
		t.Fatalf("inherited plan = %+v", p)
	}
	p, err = CheckHeatPlan("", "2026-11-13T08:00", "", "2026-11-12")
	must(t, err)
	if p.DurationMinutes != nil || p.ScheduleInherited || p.ScheduledText != "2026-11-13 08:00" || p.EndAt != "" {
		t.Fatalf("own plan = %+v", p)
	}
	for _, minutes := range []string{"0", "1441", "5.5", "五"} {
		_, err = CheckHeatPlan("", "", minutes, "")
		wantInvalid(t, err, "预计用时")
	}
	_, err = CheckHeatPlan("", "2026-11-12 08:00", "", "")
	wantInvalid(t, err, "计划时间")
}
