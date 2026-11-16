package report

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const goldenDir = "../../testdata/report"

// defaultBooks renders every report kind over the sample data with the
// default style, as the name of its golden file and its HTML.
func defaultBooks(t *testing.T) map[string]string {
	t.Helper()
	b := SampleBook()
	compact := b
	compact.Compact, compact.NewPage = true, true
	groups := Book{Meet: b.Meet, Dates: b.Dates, PrintedAt: b.PrintedAt, GroupBooks: []Book{
		{GroupName: "初一组", Events: b.Events[:1], TeamScores: b.TeamScores},
		{GroupName: "初二组", Events: b.Events[1:2]},
	}}
	out := map[string]string{}
	run := func(name string, html string, warns []string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(warns) > 0 {
			t.Errorf("%s: warnings %v", name, warns)
		}
		out[name] = html
	}
	h, w, err := Entries(b, Style{})
	run("entries", h, w, err)
	h, w, err = StartList(b, Style{})
	run("startlist", h, w, err)
	h, w, err = StartList(compact, Style{})
	run("startlist-compact", h, w, err)
	h, w, err = Results(b, Style{})
	run("results", h, w, err)
	h, w, err = Results(groups, Style{})
	run("results-groups", h, w, err)
	h, w, err = TeamScores(b, Style{})
	run("team_scores", h, w, err)
	h, w, err = CheckIn(SampleCheckInBook(), Style{})
	run("checkin", h, w, err)
	h, w, err = RecordSheets(SampleRecordBook(), Style{})
	run("recordsheet", h, w, err)
	return out
}

// bodyOf is a document's content after its <body> tag, without trailing
// white space: what the default style leaves as it was before print styles.
func bodyOf(t *testing.T, doc string) string {
	t.Helper()
	i := strings.Index(doc, "<body")
	if i < 0 {
		t.Fatalf("no body in %s", doc)
	}
	j := strings.Index(doc[i:], ">")
	body := strings.TrimRight(doc[i+j+1:], " \n")
	body = strings.TrimSuffix(body, "</html>")
	body = strings.TrimRight(body, " \n")
	return strings.TrimSuffix(body, "</body>")
}

// TestDefaultStyleKeepsTheOutput: with no style, the body of every report
// is what it was before print styles (testdata/report/default-*.body.html,
// written from the code of that time), and the head adds a CSP meta.
func TestDefaultStyleKeepsTheOutput(t *testing.T) {
	for name, doc := range defaultBooks(t) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(goldenDir, "default-"+name+".body.html")
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// The body of the books: the start of the first element on is
			// the same, the line after <body> aside.
			if got, want := strings.TrimRight(bodyOf(t, doc), " \n"), strings.TrimRight(bodyOf(t, "<body>"+string(want)), " \n"); got != want {
				t.Errorf("the body of %s changed\n%s", name, firstDiff(got, want))
			}
			if !strings.Contains(doc, printstyle.CSPMeta) {
				t.Error("no CSP meta")
			}
			if strings.Contains(doc, "<script") {
				t.Error("a script")
			}
		})
	}
}

func firstDiff(a, b string) string {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return "got  …" + a[max(0, i-60):min(len(a), i+60)] + "\nwant …" + b[max(0, i-60):min(len(b), i+60)]
		}
	}
	return ""
}

func TestDataGolden(t *testing.T) {
	got, err := json.MarshalIndent(map[string]any{
		"dataVersion": DataVersion,
		"book":        SampleBook(),
		"checkInBook": SampleCheckInBook(),
		"recordBook":  SampleRecordBook(),
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(goldenDir, "data-v1.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the report data changed: bump DataVersion and add a new golden file (or run with -update if the change is meant to stay compatible)\n%s", firstDiff(string(got), string(want)))
	}
}

func TestHeadAndBodyAttributes(t *testing.T) {
	doc, _, err := Results(SampleBook(), Style{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<meta charset="UTF-8">`, printstyle.CSPMeta, "<title>第一届校园运动会 成绩册</title>",
		`data-paper="210x297"`, `data-margin="15,15,15,15"`, `data-meet="第一届校园运动会"`, `data-title="成绩册"`, `data-pagenum="n_of_m"`,
		"--paper-w:210mm", "@page{size:210mm 297mm", ".pv-page", "break-before: page", "font-variant-numeric: tabular-nums",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %q", want)
		}
	}
	// check-in and record sheets have no running head
	doc, _, _ = CheckIn(SampleCheckInBook(), Style{})
	if strings.Contains(doc, "data-meet") {
		t.Error("check-in sheets have a running head")
	}
}

func TestOwnCSSIsClean(t *testing.T) {
	for name, css := range map[string]string{"base": baseCSS, "default": defaultCSS, "params": printstyle.DefaultParams().CSS()} {
		got, warns, err := printstyle.SanitizeCSS(css)
		if err != nil || len(warns) > 0 {
			t.Errorf("%s: %v %v", name, err, warns)
		}
		if len(strings.Fields(got)) < len(strings.Fields(css))/4 {
			t.Errorf("%s: the cleaned CSS is much shorter", name)
		}
	}
}

func TestStyledOutputIsSanitizedAndKeepsLooks(t *testing.T) {
	st := Style{CSS: "h1{color:#036} p{background:url(https://evil.example/x)}"}
	doc, warns, err := Entries(SampleBook(), st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc, "evil") || !strings.Contains(doc, "color:#036") || len(warns) == 0 {
		t.Errorf("warnings %v; doc has evil: %v", warns, strings.Contains(doc, "evil"))
	}
	// Our own head survives the cleaning.
	if !strings.Contains(doc, "Content-Security-Policy") || !strings.Contains(doc, `data-paper="210x297"`) || !strings.Contains(doc, "font-variant-numeric") {
		t.Error("the report's own parts were lost in cleaning")
	}
	if !strings.Contains(doc, "<h2>男子100m") {
		t.Error("content lost")
	}
}

func TestParams(t *testing.T) {
	p := printstyle.DefaultParams()
	p.Paper, p.Orientation = printstyle.PaperA3, printstyle.Landscape
	p.ShowBib, p.ShowUnit, p.ShowSeed = false, false, false
	p.RosterColumns = 2
	p.PageNumber, p.Header = printstyle.PageDash, false
	doc, warns, err := StartList(SampleBook(), Style{Params: p})
	if err != nil || len(warns) > 0 {
		t.Fatalf("%v %v", err, warns)
	}
	for _, want := range []string{`data-paper="420x297"`, `data-pagenum="dash"`, "--paper-w:420mm"} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %q", want)
		}
	}
	if strings.Contains(doc, "data-meet") {
		t.Error("a running head with Header off")
	}
	body := bodyOf(t, doc)
	for _, bad := range []string{`class="bib"`, `class="org"`, `class="mark"`, "报名成绩", "<th class=\"bib\">", "<th class=\"org\">"} {
		if strings.Contains(strings.ReplaceAll(body, `class="num"`, ""), bad) && bad != `class="mark"` {
			t.Errorf("body still has %s", bad)
		}
	}
	if strings.Contains(body, "报名成绩") {
		t.Error("the seed column is printed")
	}
	// 3 athletes in 2 columns: one full row and one padded row.
	if n := strings.Count(body, `<table class="names">`); n != 2 {
		t.Errorf("roster tables = %d", n)
	}
	if !strings.Contains(body, "<td></td></tr>") {
		t.Error("the roster rows are not padded to 2")
	}

	// a bad Params is a warning and the default
	p.FontSize = 1
	doc, warns, err = StartList(SampleBook(), Style{Params: p})
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "默认值") || !strings.Contains(doc, `data-paper="210x297"`) {
		t.Errorf("bad params: %v %v", err, warns)
	}
}

func TestGridTables(t *testing.T) {
	p := printstyle.DefaultParams()
	p.TableLines = printstyle.LinesGrid
	doc, _, _ := Entries(SampleBook(), Style{Params: p})
	if !strings.Contains(doc, "--cell-line:var(--rule-mid) solid #555") {
		t.Error("no grid")
	}
}

func TestSlots(t *testing.T) {
	st := Style{
		DataVersion: DataVersion,
		Slots: map[string]string{
			"cover":          `<h1>{{.Meet}}</h1><p>封面</p>`,
			"before_rosters": `<p class="note">名单前 {{.Meet}}</p>`,
			"after_events":   `<p class="note">项目后 {{len .Events}} 个</p>`,
			"header_left":    `<b>{{.Meet}}</b>`,
			"header_right":   `{{.Title}} · 内部`,
			"footer":         `{{.Meet}} 第 {page} 页 / 共 {pages} 页`,
		},
	}
	doc, warns, err := StartList(SampleBook(), st)
	if err != nil || len(warns) > 0 {
		t.Fatalf("%v %v", err, warns)
	}
	for _, want := range []string{
		`<section class="cover"><h1>第一届校园运动会</h1><p>封面</p></section>`,
		"名单前 第一届校园运动会", "项目后 4 个",
		`data-head-left="第一届校园运动会"`, `data-head-right="秩序册 · 内部"`, `data-foot="第一届校园运动会 第 {page} 页 / 共 {pages} 页"`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %q", want)
		}
	}
	if i, j := strings.Index(doc, "名单前"), strings.Index(doc, `class="rosters"`); i < 0 || j < 0 || i > j {
		t.Error("before_rosters is not before the rosters")
	}
	if i, j := strings.Index(doc, "项目后"), strings.Index(doc, "打印时间"); i < 0 || i > j {
		t.Error("after_events is not before the print time")
	}
	// every kind takes the cover and after_events
	plain := Style{DataVersion: DataVersion, Slots: map[string]string{"cover": "<p>封面</p>", "after_events": "<p>项目后</p>"}}
	for _, kind := range printstyle.ReportKinds {
		doc, warns, err := RenderSample(kind, plain)
		if err != nil || len(warns) > 0 {
			t.Fatalf("%s: %v %v", kind, err, warns)
		}
		if !strings.Contains(doc, "封面") || !strings.Contains(doc, "项目后") {
			t.Errorf("%s: slots missing", kind)
		}
	}
}

func TestSlotErrorIsSkipped(t *testing.T) {
	doc, warns, err := StartList(SampleBook(), Style{DataVersion: DataVersion, Slots: map[string]string{"cover": "{{.Nope}}", "after_events": "<p>好</p>"}})
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "cover") || !strings.Contains(doc, "<p>好</p>") || strings.Contains(doc, `class="cover"`) {
		t.Errorf("%v %v", err, warns)
	}
}

func TestCustomTemplate(t *testing.T) {
	st := Style{DataVersion: DataVersion, Templates: map[string]string{
		KindEntries: `<h1>{{.Meet}}</h1>{{range .Events}}<section class="event"><h2>{{.Name}}</h2>{{range .Entries}}<p>{{template "competitor" .}} {{.Bib}}</p>{{end}}</section>{{end}}<script>alert(1)</script>`,
	}}
	doc, warns, err := Entries(SampleBook(), st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, "<h1>第一届校园运动会</h1>") || !strings.Contains(doc, "<p>张三 101</p>") {
		t.Errorf("custom body missing:\n%s", bodyOf(t, doc))
	}
	if strings.Contains(doc, "<script") || len(warns) == 0 {
		t.Errorf("script kept or no warning: %v", warns)
	}
	// the same style leaves the other reports alone
	doc, _, _ = StartList(SampleBook(), st)
	if !strings.Contains(doc, `class="rosters"`) {
		t.Error("other reports changed")
	}
}

func TestStaleAndBrokenTemplatesFallBack(t *testing.T) {
	tmpl := map[string]string{KindEntries: "<p>自定义</p>"}
	doc, warns, err := Entries(SampleBook(), Style{DataVersion: DataVersion + 1, Templates: tmpl, Slots: map[string]string{"cover": "封"}})
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "已失效") || strings.Contains(doc, "自定义") || strings.Contains(doc, "封") {
		t.Errorf("stale: %v %v", err, warns)
	}
	doc, warns, err = Entries(SampleBook(), Style{DataVersion: DataVersion, Templates: map[string]string{KindEntries: "{{.Missing}}"}})
	if err != nil || len(warns) != 1 || !strings.Contains(doc, "<h2>男子100m") {
		t.Errorf("broken: %v %v", err, warns)
	}
	doc, warns, err = Entries(SampleBook(), Style{DataVersion: DataVersion, Templates: map[string]string{KindEntries: "{{if}}"}})
	if err != nil || len(warns) != 1 || !strings.Contains(doc, "<h2>男子100m") {
		t.Errorf("syntax: %v %v", err, warns)
	}
}

func TestFrontPages(t *testing.T) {
	md1, _ := printstyle.RenderMarkdown("# 致辞\n\n欢迎\n\n---pagebreak---\n\n第二页")
	md2, _ := printstyle.RenderMarkdown("# 只在成绩册")
	st := Style{FrontPages: []FrontPage{
		{Title: "致辞", HTML: md1},
		{Title: "成绩册前言", HTML: md2, Reports: []string{KindResults}},
		{Title: "危险", HTML: `<p onclick="x">页</p><script>alert(1)</script>`},
	}}
	doc, _, err := Entries(SampleBook(), st)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyOf(t, doc)
	if n := strings.Count(body, `<section class="front">`); n != 2 {
		t.Errorf("front sections in entries = %d", n)
	}
	if strings.Contains(body, "只在成绩册") || strings.Contains(body, "onclick") || strings.Contains(body, "<script") {
		t.Error("filter or cleaning failed")
	}
	if i, j := strings.Index(body, `class="front"`), strings.Index(body, "<header>"); i < 0 || i > j {
		t.Error("front pages are not first")
	}
	if !strings.Contains(body, `class="page-break"`) {
		t.Error("page break lost")
	}
	doc, _, _ = Results(SampleBook(), st)
	if n := strings.Count(bodyOf(t, doc), `<section class="front">`); n != 3 {
		t.Errorf("front sections in results = %d", n)
	}
	// front pages alone do not make the document custom: no cleaning pass, output as before
	if !strings.Contains(doc, "font-variant-numeric") {
		t.Error("head lost")
	}
}

func TestAssets(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	st := Style{
		CSS:        "h1{background:url(asset:底纹) no-repeat}",
		Assets:     map[string]printstyle.Asset{"校徽": {Name: "校徽", MIME: "image/png", Data: png}, "底纹": {Name: "底纹", MIME: "image/png", Data: png}},
		FrontPages: []FrontPage{{HTML: `<p><img src="asset:校徽" alt="徽"></p><p><img src="asset:没有"></p>`}},
	}
	doc, warns, err := Entries(SampleBook(), st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(doc, "data:image/png;base64,") != 2 || strings.Contains(doc, "asset:校徽") || strings.Contains(doc, "asset:底纹") {
		t.Error("assets are not embedded")
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w, "没有") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning for the missing image: %v", warns)
	}
}

func TestValidateTemplate(t *testing.T) {
	if err := ValidateTemplate(KindEntries, `{{range .Events}}{{.Name}}{{end}}`); err != nil {
		t.Error(err)
	}
	if err := ValidateTemplate(KindEntries, `{{.Nope}}`); err == nil {
		t.Error("an unknown field passed")
	}
	if err := ValidateTemplate(KindCheckIn, `{{range .Sheets}}{{.Event}}{{end}}`); err != nil {
		t.Error(err)
	}
	if err := ValidateTemplate("cards", `x`); err == nil {
		t.Error("unknown kind passed")
	}
	if err := ValidateSlot("cover", `<h1>{{.Meet}}</h1><p>{{.Dates}} {{.Location}}</p>`); err != nil {
		t.Error(err)
	}
	if err := ValidateSlot("cover", `{{.Events}}`); err == nil {
		t.Error("a slot that fails on check-in data passed")
	}
	if err := ValidateSlot("sidebar", `x`); err == nil {
		t.Error("unknown slot passed")
	}
}

func TestStyleFromFile(t *testing.T) {
	f := printstyle.File{Name: "x", Params: printstyle.DefaultParams(), CSS: "p{a:b}", DataVersion: 1,
		Slots: map[string]string{"cover": "x"}, Assets: []printstyle.FileAsset{{Name: "a", MIME: "image/png", Data: "iVBORw0KGgo="}}}
	st, err := StyleFromFile(f)
	if err != nil || st.CSS != "p{a:b}" || len(st.Assets) != 1 || st.Slots["cover"] != "x" {
		t.Errorf("%+v %v", st, err)
	}
}

func TestSampleRendersEveryKind(t *testing.T) {
	for _, kind := range printstyle.ReportKinds {
		doc, warns, err := RenderSample(kind, Style{})
		if err != nil || len(warns) > 0 || !strings.Contains(doc, "第一届校园运动会") {
			t.Errorf("%s: %v %v", kind, err, warns)
		}
	}
	if _, _, err := RenderSample("cards", Style{}); err == nil {
		t.Error("unknown kind")
	}
}

// A template chooses RowsOf's width; it does not choose how much is allocated.
func TestRowsOfIsBounded(t *testing.T) {
	g := RosterGroup{Athletes: []Line{{Name: "甲"}, {Name: "乙"}}}
	for _, n := range []int{1 << 40, int(^uint(0) >> 1)} {
		rows := g.RowsOf(n)
		if len(rows) != 1 || len(rows[0]) != maxRosterColumns {
			t.Errorf("RowsOf(%d): %d rows of %d", n, len(rows), len(rows[0]))
		}
	}
	if rows := g.RowsOf(0); len(rows) != 1 || len(rows[0]) != rosterColumns {
		t.Errorf("RowsOf(0) = %v", rows)
	}
}

func TestTeamScoresMedalsTable(t *testing.T) {
	b := SampleBook()
	ts := *b.TeamScores
	ts.Medals = true
	b.TeamScores = &ts
	html, warns, err := TeamScores(b, Style{})
	if err != nil || len(warns) > 0 {
		t.Fatal(err, warns)
	}
	for _, want := range []string{"金牌", "银牌", "铜牌", "奖牌总数"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(html, "男子得分") || strings.Contains(html, "总分</th>") {
		t.Fatal("points columns in the medal table")
	}
}
