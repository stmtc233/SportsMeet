package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestCombinedTitle(t *testing.T) {
	for _, c := range []struct {
		in   domain.Combined
		want string
	}{
		{domain.Combined{Name: "三项全能", Gender: domain.GenderMale, GroupLabel: "初一组"}, "男子三项全能 初一组"},
		{domain.Combined{Name: "女子五项全能", Gender: domain.GenderFemale}, "女子五项全能"},
		{domain.Combined{Name: "初一组 全能", Gender: domain.GenderMale, GroupLabel: "初一组"}, "男子初一组 全能"},
	} {
		if got := combinedTitle(c.in); got != c.want {
			t.Errorf("combinedTitle(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBestAttemptIndex(t *testing.T) {
	mark := func(v int64) *int64 { return &v }
	tries := []domain.Attempt{{Outcome: "valid", Mark: mark(500)}, {Outcome: "foul"}, {Outcome: "valid", Mark: mark(520)}}
	if got := bestAttemptIndex(domain.Event{Kind: domain.KindDistance}, tries, 520); got != 2 {
		t.Fatalf("distance best = %d", got)
	}
	if got := bestAttemptIndex(domain.Event{Kind: domain.KindCount, AttemptMode: "sum"}, tries, 1020); got != -1 {
		t.Fatalf("sum best = %d", got)
	}
}

func TestPrintedTeamNames(t *testing.T) {
	st := &eventState{units: map[string]domain.Unit{"u1": {Name: "初一（1）班", ShortName: "一班"}, "u2": {Name: "初一（2）班"}}}
	e := domain.Event{ID: "e", Kind: domain.KindRelay, ParticipantMode: "team"}
	set := func(teams ...domain.Team) map[string]bool {
		st.teams = map[string]domain.Team{}
		for _, tm := range teams {
			tm.EventID = "e"
			st.teams[tm.ID] = tm
		}
		return printedTeamNames(e, st)
	}
	// One name for every team says nothing.
	if got := set(domain.Team{ID: "a", Name: "接力队", UnitID: "u1"}, domain.Team{ID: "b", Name: "接力队", UnitID: "u2"}); len(got) != 0 {
		t.Fatalf("shared name printed: %v", got)
	}
	// The unit's own name or short name repeats the unit column.
	got := set(domain.Team{ID: "a", Name: "一班", UnitID: "u1"}, domain.Team{ID: "b", Name: "飞鹰队", UnitID: "u2"})
	if got["a"] || !got["b"] {
		t.Fatalf("names = %v", got)
	}
	if competitorHeading(e, true) != "队伍 / 棒次" || competitorHeading(e, false) != "棒次" || competitorHeading(domain.Event{Kind: domain.KindTrack}, false) != "姓名" {
		t.Fatal("headings")
	}
}

func TestResultsPrintHeightTrials(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "跳高", Kind: "height"})
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, h, []ResultInput{
		{Position: 1, HeightFinished: true, Bars: []BarInput{{"1.50", "O"}, {"1.55", "XO"}, {"1.60", "XXX"}}},
		{Position: 2, HeightFinished: true, Bars: []BarInput{{"1.50", "XO"}, {"1.55", "XXX"}}},
	}))
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	order(t, html, "试跳记录", "选手01", "<span>1.50 O</span><span>1.55 XO</span><span>1.60 XXX</span>", "1.55", "选手02", "<span>1.50 XO</span>")
	// One heat: no heat column in the standings.
	if strings.Contains(html, `<th class="heat-col">`) {
		t.Fatal("heat column printed for one heat")
	}
}

func TestStartListScheduleAndHeadings(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "100m", Kind: "track", LaneCount: 8})
	f.addHeat(0, f.athletes...)
	rid := f.detail().Rounds[0].ID
	_, err := f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: rid, ScheduledAt: "2026-11-12T09:00"})
	must(t, err)
	var html string
	html, err = f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	// Numbered rows by day with the weekday and the kind; heats with their count.
	order(t, html, "竞赛日程表", "2026-11-12 周四", `<td class="num">1</td><td class="time">09:00</td><td class="kind">径赛</td>`,
		"男子100m", "2026-11-12 周四 · 分道", "<small>09:00 · 2 人</small>")
	// No entry marks: no column for them.
	if strings.Contains(html, "报名成绩") {
		t.Fatal("empty entry marks printed")
	}
	// Books name the meet and the report for the preview's running head;
	// sheets name them on each sheet instead.
	if !strings.Contains(html, `<body data-meet="秋季运动会" data-title="秩序册">`) {
		t.Fatal("running head data missing")
	}
	sheets, err := f.svc.RenderCheckIn(f.ctx, CheckInRequest{MeetID: f.meet})
	must(t, err)
	if strings.Contains(sheets, "data-meet") {
		t.Fatal("sheets repeat the running head")
	}
}
