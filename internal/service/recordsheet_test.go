package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/store"
)

func renderRecord(t *testing.T, f *fixture, req CheckInRequest) string {
	t.Helper()
	req.MeetID = f.meet
	html, err := f.svc.RenderRecordSheet(f.ctx, req)
	must(t, err)
	return html
}

// recordSheetOf adds an event with one heat of two athletes and returns its sheet.
func recordSheetOf(t *testing.T, f *fixture, in EventInput) string {
	t.Helper()
	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", in)
	must(t, err)
	f.event = e.ID
	f.addHeat(0, f.athletes[0], f.athletes[1])
	html := renderRecord(t, f, CheckInRequest{RoundIDs: []string{f.detail().Rounds[0].ID}})
	if strings.Count(html, `<section class="sheet record`) != 1 || strings.Count(html, `class="line"`) != 2 {
		t.Fatalf("%s: sheets or lines:\n%s", in.Discipline, html)
	}
	return html
}

func TestRecordSheetColumns(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	const wind, attemptWind, group = "<th>风速</th>", `<td class="tag">风速</td>`, `colspan="`
	has := func(html string, want map[string]bool) {
		t.Helper()
		for part, ok := range want {
			if strings.Contains(html, part) != ok {
				t.Fatalf("%q present = %v, want %v in:\n%s", part, !ok, ok, html)
			}
		}
	}
	tries := func(html string) int { return strings.Count(html, `<th class="try`) }

	// Track in lanes: the mark, the rank and the heat's wind.
	html := recordSheetOf(t, f, track())
	order(t, html, "秋季运动会 · 成绩记录表", "计划时间", "实际开赛", "风速", "道次", "号码", "姓名", "单位", ">成绩<", "名次", "001", "选手01", "一班", "应到 2 人", "记录员", "裁判长")
	has(html, map[string]bool{wind: true, attemptWind: false, group: false})
	if tries(html) != 0 {
		t.Fatalf("track has attempt columns:\n%s", html)
	}
	in := track()
	in.Discipline, in.StartMode = "1500m", "group"
	has(recordSheetOf(t, f, in), map[string]bool{wind: false, "顺序": true})

	// Distance: a column per attempt and the best mark; jumps with wind rows.
	html = recordSheetOf(t, f, EventInput{Gender: "男", Discipline: "跳远", Kind: "distance", AttemptCount: 4})
	has(html, map[string]bool{wind: false, attemptWind: true, "最好成绩": true, `rowspan="2"`: true})
	order(t, html, `<th class="try">1</th>`, `<th class="try">4</th>`)
	if tries(html) != 4 || strings.Count(html, `<td class="try"></td>`) != 16 {
		t.Fatalf("jump cells:\n%s", html)
	}
	html = recordSheetOf(t, f, EventInput{Gender: "男", Discipline: "铅球", Kind: "distance", DistanceMode: "throw", AttemptCount: 3})
	has(html, map[string]bool{attemptWind: false, "最好成绩": true, `rowspan="2"`: false})
	if tries(html) != 3 || strings.Count(html, `<td class="try"></td>`) != 6 {
		t.Fatalf("throw cells:\n%s", html)
	}

	// Height: blank headings for the heights under one group heading.
	html = recordSheetOf(t, f, EventInput{Gender: "男", Discipline: "跳高", Kind: "height"})
	has(html, map[string]bool{`colspan="10">高度</th>`: true, "最好成绩": true, attemptWind: false})
	if strings.Count(html, `<th class="try write"></th>`) != 10 || strings.Count(html, `<td class="try"></td>`) != 20 {
		t.Fatalf("height cells:\n%s", html)
	}

	// Count and score: the mark, after the attempts when there are several.
	html = recordSheetOf(t, f, EventInput{Gender: "男", Discipline: "跳绳", Kind: "count"})
	has(html, map[string]bool{">次数<": true, wind: false})
	if tries(html) != 0 {
		t.Fatalf("count has attempt columns:\n%s", html)
	}
	html = recordSheetOf(t, f, EventInput{Gender: "男", Discipline: "投篮", Kind: "score", AttemptMode: "best", AttemptCount: 2})
	has(html, map[string]bool{">分数<": true})
	if tries(html) != 2 {
		t.Fatalf("score attempts:\n%s", html)
	}
}

func TestRecordSheetSelectionAndPages(t *testing.T) {
	f := checkInFixture(t)
	html := renderRecord(t, f, CheckInRequest{})
	if strings.Count(html, `<section class="sheet record`) != 4 || strings.Contains(html, `record break"`) {
		t.Fatalf("one round's heats share pages by default:\n%s", html)
	}
	order(t, html, "第1组", "2026-11-12 09:30", "第2组", "第3组", "第4组")
	html = renderRecord(t, f, CheckInRequest{Day: "2026-11-12"})
	if strings.Count(html, `<section class="sheet record`) != 2 {
		t.Fatalf("day filter:\n%s", html)
	}
	order(t, html, "第3组", "第1组")
	if !strings.Contains(renderRecord(t, f, CheckInRequest{RoundIDs: []string{"other"}}), "没有可打印的组") {
		t.Fatal("printed a round that was not asked for")
	}
	_, err := f.svc.RenderRecordSheet(f.ctx, CheckInRequest{MeetID: f.meet, Day: "2026-13-01"})
	wantInvalid(t, err, "日期格式")

	// Results already entered stay off the sheet.
	d := f.detail()
	must(t, f.svc.SaveResults(f.ctx, ui, d.Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	if strings.Contains(renderRecord(t, f, CheckInRequest{}), "12.34") {
		t.Fatal("an entered mark is printed")
	}

	// A page per round by default: a second event starts one, its heats do not.
	first := f.event
	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"})
	must(t, err)
	f.event = e.ID
	f.addHeat(0, f.athletes[0])
	f.addHeat(0, f.athletes[1])
	f.event = first
	breaks := func() int { return strings.Count(renderRecord(t, f, CheckInRequest{}), `record break"`) }
	if n := breaks(); n != 1 {
		t.Fatalf("%d page breaks by round", n)
	}
	prefs, err := f.svc.MeetSettings(f.ctx, f.meet)
	must(t, err)
	if prefs.PrintRecordBreak != SheetBreakRound {
		t.Fatalf("defaults = %+v", prefs)
	}
	for mode, want := range map[string]int{SheetBreakHeat: 5, SheetBreakNone: 0} {
		prefs.PrintRecordBreak = mode
		must(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs))
		if n := breaks(); n != want {
			t.Fatalf("%s: %d page breaks, want %d", mode, n, want)
		}
	}
	prefs.PrintRecordBreak = "page"
	wantInvalid(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs), "分页方式")
}

// Settings saved with the 检录单's switch keep what it said.
func TestSheetBreakFromSavedSwitch(t *testing.T) {
	f := newFixture(t, 1, track())
	for raw, want := range map[string]string{
		`{"theme":"system","printCheckInNewPage":true}`:                            SheetBreakHeat,
		`{"theme":"system","printCheckInNewPage":false}`:                           SheetBreakRound,
		`{"theme":"system","printCheckInNewPage":true,"printCheckInBreak":"none"}`: SheetBreakNone,
	} {
		must(t, f.svc.store.Write(f.ctx, store.Stmt(`UPDATE meets SET settings = ? WHERE id = ?`, raw, f.meet)))
		p, err := f.svc.MeetSettings(f.ctx, f.meet)
		must(t, err)
		if p.PrintCheckInBreak != want || p.PrintRecordBreak != SheetBreakRound {
			t.Fatalf("%s: %+v", raw, p)
		}
	}
}

func TestRecordSheetTeams(t *testing.T) {
	f := newFixture(t, 4, relay())
	tm := team(t, f, "一班1队", f.athletes...)
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{
		RoundID: r.ID, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 4, TeamID: tm.ID}}}},
	}}}))
	html := renderRecord(t, f, CheckInRequest{})
	order(t, html, "道次", "队伍", "棒次", "一班1队", "选手01 001", "选手04 004")
	if strings.Contains(html, "<th>风速</th>") {
		t.Fatalf("a relay leaves room for wind:\n%s", html)
	}
}
