package service

import (
	"os"
	"strings"
	"testing"
)

func TestEntriesReportUnarrangedAndSelection(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 3, in)
	f.enter(f.athletes[:2]...)
	must(t, f.svc.SetSeed(f.ctx, ui, f.event, f.athletes[0], "12.34"))
	other, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"})
	must(t, err)
	html, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, []string{f.event})
	must(t, err)
	order(t, html, `data-title="报名表"`, "男子100m", "2 人", "报名成绩", "001", "选手01", "一班", "12.34", "002", "选手02")
	for _, absent := range []string{"选手03", "男子跳远", "尚未编排", "代表队名单", "竞赛日程表", "道次"} {
		if strings.Contains(html, absent) {
			t.Fatalf("unexpected %q in entries", absent)
		}
	}
	// Two rounds of positions must neither remove unarranged entries nor duplicate an athlete.
	f.addHeat(0, f.athletes[0])
	f.addHeat(1, f.athletes[0])
	after, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, []string{f.event})
	must(t, err)
	if strings.Count(after, "选手01") != 1 || strings.Count(after, "选手02") != 1 {
		t.Fatal("entries changed with positions")
	}
	empty, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, []string{other.ID})
	must(t, err)
	if !strings.Contains(empty, "还没有报名") || strings.Contains(empty, "报名成绩") || strings.Contains(empty, "选手01") {
		t.Fatal("empty event or selection")
	}
	unknown, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, []string{"missing"})
	must(t, err)
	if strings.Contains(unknown, "选手01") || !strings.Contains(unknown, "没有项目") {
		t.Fatal("unknown selection prints all")
	}
	_, err = f.svc.RenderReport(f.ctx, ReportEntries, "missing", nil)
	if err == nil {
		t.Fatal("missing meet accepted")
	}
}

func TestEntriesReportTeamsAndCombined(t *testing.T) {
	f := newFixture(t, 5, relay())
	a := team(t, f, "飞鹰队", f.athletes[1], f.athletes[0])
	team(t, f, "未报名队", f.athletes[2])
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{TeamID: a.ID, Seed: "50.12"}}))
	html, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, nil)
	must(t, err)
	order(t, html, "1 队", "队伍 / 棒次", "飞鹰队", "1棒 选手02 002", "2棒 选手01 001", "50.12")
	if strings.Contains(html, "未报名队") || strings.Contains(html, `scope="col" class="bib"`) {
		t.Fatal("unentered team or team bib column printed")
	}
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes[3:]))
	html, err = f.svc.RenderReport(f.ctx, ReportEntries, f.meet, []string{c.ID})
	must(t, err)
	order(t, html, "男子两项全能", "2 人", "选手04", "选手05")
	if strings.Count(html, `class="event"`) != 1 || strings.Contains(html, "男子100m") || strings.Contains(html, "选手01") {
		t.Fatal("combined components or unselected event printed")
	}
	all, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, nil)
	must(t, err)
	if strings.Count(all, `class="event"`) != 2 || strings.Count(all, "选手04") != 1 {
		t.Fatal("combined roster repeated")
	}
}

func TestEntriesReportLongTable(t *testing.T) {
	f := newFixture(t, 140, track())
	f.enter(f.athletes...)
	prefs, err := f.svc.Preferences(f.ctx)
	must(t, err)
	prefs.PrintNewPage = true
	must(t, f.svc.SavePreferences(f.ctx, prefs))
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "跳远", Kind: "distance"})
	must(t, err)
	html, err := f.svc.RenderReport(f.ctx, ReportEntries, f.meet, nil)
	must(t, err)
	if strings.Count(html, `<td class="num">`) != 140 || !strings.Contains(html, "break-before: page") {
		t.Fatal("long roster or page setting")
	}
	if path := os.Getenv("SPORTSMEET_ENTRIES_REPORT_OUTPUT"); path != "" {
		must(t, os.WriteFile(path, []byte(html), 0o644))
	}
}
