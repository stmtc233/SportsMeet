package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// checkInFixture has four heats: 第1组 09:30 and 第3组 08:10 on the 12th,
// 第2组 08:00 on the 13th, 第4组 without a time.
func checkInFixture(t *testing.T) *fixture {
	t.Helper()
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T09:30")
	setHeatTime(t, f, 1, "2026-11-13T08:00")
	setHeatTime(t, f, 2, "2026-11-12T08:10")
	return f
}

func renderCheckIn(t *testing.T, f *fixture, req CheckInRequest) string {
	t.Helper()
	req.MeetID = f.meet
	html, err := f.svc.RenderCheckIn(f.ctx, req)
	must(t, err)
	return html
}

// order checks that parts appear in html in this order.
func order(t *testing.T, html string, parts ...string) {
	t.Helper()
	last := -1
	for _, p := range parts {
		i := strings.Index(html, p)
		if i < 0 || i < last {
			t.Fatalf("%q missing or out of order in:\n%s", p, html)
		}
		last = i
	}
}

func TestCheckInTargets(t *testing.T) {
	f := checkInFixture(t)
	empty, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "200m", Kind: "track", LaneCount: 8})
	must(t, err)
	got, err := f.svc.CheckInTargets(f.ctx, f.meet)
	must(t, err)
	if strings.Join(got.Days, ",") != "2026-11-12,2026-11-13" || len(got.Events) != 1 || got.Events[0].ID == empty.ID {
		t.Fatalf("targets = %+v", got)
	}
	r := got.Events[0].Rounds
	if len(r) != 1 || r[0].HeatCount != 4 || r[0].Label != "决赛" || strings.Join(r[0].Days, ",") != "2026-11-12,2026-11-13" {
		t.Fatalf("rounds = %+v", r)
	}
}

func TestCheckInSheets(t *testing.T) {
	f := checkInFixture(t)
	html := renderCheckIn(t, f, CheckInRequest{})
	if n := strings.Count(html, `<section class="sheet`); n != 4 {
		t.Fatalf("%d sheets", n)
	}
	// List order, the default lead of 20 minutes, a blank time for 第4组.
	order(t, html, "秋季运动会 · 检录单", "第1组", "2026-11-12 09:30", "09:10", "到", "缺", "001", "选手01", "一班", "应到 1 人", "检录员",
		"第2组", "2026-11-13 08:00", "07:40", "第3组", "07:50", "第4组", "选手04")
	// One round: its heats share pages.
	if strings.Count(html, "检录时间") != 4 || strings.Contains(html, `class="sheet break"`) {
		t.Fatalf("check-in times or page breaks:\n%s", html)
	}

	// A day: only its heats, by time.
	html = renderCheckIn(t, f, CheckInRequest{Day: "2026-11-12"})
	if strings.Count(html, `<section class="sheet`) != 2 || strings.Contains(html, "第2组") || strings.Contains(html, "第4组") {
		t.Fatalf("day filter:\n%s", html)
	}
	order(t, html, "第3组", "第1组")
	html = renderCheckIn(t, f, CheckInRequest{Day: "2026-11-14"})
	if !strings.Contains(html, "没有可打印的组") {
		t.Fatalf("empty day:\n%s", html)
	}

	// Rounds: only those asked for.
	html = renderCheckIn(t, f, CheckInRequest{RoundIDs: []string{"other"}})
	if strings.Contains(html, `<section class="sheet`) {
		t.Fatal("printed a round that was not asked for")
	}
	html = renderCheckIn(t, f, CheckInRequest{RoundIDs: []string{f.detail().Rounds[0].ID}})
	if strings.Count(html, `<section class="sheet`) != 4 {
		t.Fatal("the asked round is missing")
	}

	// Lead 0 prints no check-in time; a new page per sheet.
	prefs, err := f.svc.MeetSettings(f.ctx, f.meet)
	must(t, err)
	if prefs.CheckInLeadMinutes != domain.DefaultCheckInLeadMinutes || prefs.PrintCheckInBreak != SheetBreakRound || prefs.PrintResultsTime {
		t.Fatalf("defaults = %+v", prefs)
	}
	prefs.CheckInLeadMinutes, prefs.PrintCheckInBreak = 0, SheetBreakHeat
	must(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs))
	html = renderCheckIn(t, f, CheckInRequest{})
	if strings.Contains(html, "检录时间") || strings.Count(html, `class="sheet break"`) != 3 {
		t.Fatalf("lead 0 with new pages:\n%s", html)
	}
	prefs.CheckInLeadMinutes = 45
	must(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs))
	order(t, renderCheckIn(t, f, CheckInRequest{}), "第1组", "08:45")

	for _, lead := range []int{-1, domain.MaxCheckInLeadMinutes + 1} {
		prefs.CheckInLeadMinutes = lead
		wantInvalid(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs), "检录提前时间")
	}
	_, err = f.svc.RenderCheckIn(f.ctx, CheckInRequest{MeetID: f.meet, Day: "2026-13-01"})
	wantInvalid(t, err, "日期格式")
}

func TestCheckInTeams(t *testing.T) {
	f := newFixture(t, 4, relay())
	tm := team(t, f, "一班1队", f.athletes...)
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{
		RoundID: r.ID, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 4, TeamID: tm.ID}}}},
	}}}))
	html := renderCheckIn(t, f, CheckInRequest{})
	order(t, html, "道次", "队伍", "棒次", "一班1队", "选手01 001", "选手04 004")
	if !strings.Contains(html, "<th>计划时间</th><td></td><th>检录时间</th><td></td>") {
		t.Fatalf("a heat without a time does not leave its times blank:\n%s", html)
	}
}
