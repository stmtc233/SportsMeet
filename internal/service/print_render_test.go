package service

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/store"
)

func dataURI(seed byte) string { return base64.StdEncoding.EncodeToString(pngBytes(seed)) }

// A style's CSS, cover slot and the meet's front pages reach every report.
func TestReportsUseTheMeetsStyleAndFrontPages(t *testing.T) {
	f := scheduleFixture(t)
	ctx := f.ctx
	// A meet with no style renders as before: no front section, no custom parts.
	plain, err := f.svc.RenderReport(ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if strings.Contains(plain, `class="front"`) || strings.Contains(plain, "covermark") {
		t.Fatal("the default style adds parts")
	}

	style := newStyle(t, f.svc, "校运会", ".stylemark{color:red}")
	_, err = f.svc.SavePrintStyle(ctx, ui, style.Style.ID, PrintStyleInput{Slots: map[string]string{"cover": `<p id="covermark">{{.Meet}}</p>`}})
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "全体同学：**欢迎**参加"})
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "只印成绩册", BodyMD: "仅成绩册可见", Reports: []string{"results"}})
	must(t, err)
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, style.Style.ID))
	// Saving the other settings without printStyleId keeps the choice.
	must(t, f.svc.SaveMeetSettings(ctx, ui, f.meet, DefaultMeetSettings()))
	if got, _ := f.svc.MeetSettings(ctx, f.meet); got.PrintStyleID != style.Style.ID {
		t.Fatalf("SaveMeetSettings cleared the style: %q", got.PrintStyleID)
	}

	startlist, err := f.svc.RenderReport(ctx, ReportStartList, f.meet, nil)
	must(t, err)
	order(t, startlist, ".stylemark", `<section class="cover">`, "covermark", `<section class="front">`, "<strong>欢迎</strong>")
	if strings.Contains(startlist, "仅成绩册可见") {
		t.Fatal("a front page for the results is in the start list")
	}
	results, err := f.svc.RenderReport(ctx, ReportResults, f.meet, nil)
	must(t, err)
	order(t, results, ".stylemark", "<strong>欢迎</strong>", "仅成绩册可见")
	for name, render := range map[string]func() (string, error){
		"entries":     func() (string, error) { return f.svc.RenderReport(ctx, ReportEntries, f.meet, nil) },
		"team_scores": func() (string, error) { return f.svc.RenderReport(ctx, ReportTeamScores, f.meet, nil) },
		"checkin":     func() (string, error) { return f.svc.RenderCheckIn(ctx, CheckInRequest{MeetID: f.meet}) },
		"recordsheet": func() (string, error) { return f.svc.RenderRecordSheet(ctx, CheckInRequest{MeetID: f.meet}) },
	} {
		html, err := render()
		must(t, err)
		if !strings.Contains(html, ".stylemark") || !strings.Contains(html, "covermark") || !strings.Contains(html, "<strong>欢迎</strong>") {
			t.Errorf("%s: style or front page missing", name)
		}
	}
	_, err = f.svc.RenderGroupedReport(ctx, GroupReportRequest{Kind: ReportResults, MeetID: f.meet})
	must(t, err)
	grouped, err := f.svc.RenderGroupedReport(ctx, GroupReportRequest{Kind: ReportResults, MeetID: f.meet})
	must(t, err)
	if !strings.Contains(grouped, ".stylemark") {
		t.Fatal("grouped report without the style")
	}

	// A style's parameters reach the document too.
	_, err = f.svc.SavePrintStyle(ctx, ui, style.Style.ID, PrintStyleInput{ParamsPatch: `{"paper":"a3","orientation":"landscape"}`})
	must(t, err)
	a3, err := f.svc.RenderReport(ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if !strings.Contains(a3, `data-paper="420x297"`) {
		t.Fatal("paper parameters not applied")
	}
}

func TestRenderOptionsOverrideAndDraft(t *testing.T) {
	f := scheduleFixture(t)
	ctx := f.ctx
	mine := newStyle(t, f.svc, "甲", ".minemark{color:red}").Style.ID
	other := newStyle(t, f.svc, "乙", ".othermark{color:red}").Style.ID
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, mine))

	r, err := f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{})
	must(t, err)
	if !strings.Contains(r.HTML, ".minemark") || r.Warnings == nil {
		t.Fatalf("meet's style: %v", r.Warnings)
	}
	r, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{StyleID: other})
	must(t, err)
	if !strings.Contains(r.HTML, ".othermark") || strings.Contains(r.HTML, ".minemark") {
		t.Fatal("styleId did not override the meet's style")
	}
	r, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{StyleID: DefaultPrintStyleID})
	must(t, err)
	if strings.Contains(r.HTML, "mark{") {
		t.Fatal("the default style kept custom CSS")
	}
	_, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{StyleID: "nope"})
	wantInvalid(t, err, "不存在")

	// A draft shows without being saved, and is checked as a save is.
	notified := len(*f.changes)
	r, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{Draft: &PrintStyleInput{
		CSS: strp(".draftmark{color:red} @import url(http://x/y.css);"), Slots: map[string]string{"cover": "<p id=\"draftcover\">草稿</p>"}, Name: strp("ignored")}})
	must(t, err)
	if !strings.Contains(r.HTML, ".draftmark") || !strings.Contains(r.HTML, "draftcover") || strings.Contains(r.HTML, ".minemark") || len(r.Warnings) == 0 {
		t.Fatalf("draft not applied: %v", r.Warnings)
	}
	d, _ := f.svc.GetPrintStyle(ctx, mine)
	if d.Version != 1 || strings.Contains(d.CSS, "draft") || len(*f.changes) != notified {
		t.Fatal("rendering a draft saved it")
	}
	_, err = f.svc.RenderCheckInWith(ctx, CheckInRequest{MeetID: f.meet}, RenderOptions{Draft: &PrintStyleInput{Slots: map[string]string{"cover": "{{"}}})
	wantInvalid(t, err, "插槽")
	c, err := f.svc.RenderCheckInWith(ctx, CheckInRequest{MeetID: f.meet}, RenderOptions{StyleID: other})
	must(t, err)
	rs, err := f.svc.RenderRecordSheetWith(ctx, CheckInRequest{MeetID: f.meet}, RenderOptions{StyleID: other})
	must(t, err)
	g, err := f.svc.RenderGroupedReportWith(ctx, GroupReportRequest{Kind: ReportTeamScores, MeetID: f.meet}, RenderOptions{StyleID: other})
	must(t, err)
	for _, h := range []string{c.HTML, rs.HTML, g.HTML} {
		if !strings.Contains(h, ".othermark") {
			t.Fatal("a render path ignores the options")
		}
	}

	// The sample data of a style, saved or drafted, without a meet.
	s, err := f.svc.RenderStyleSample(ctx, "results", other, &PrintStyleInput{CSS: strp(".sampledraft{color:red}")})
	must(t, err)
	if !strings.Contains(s.HTML, ".sampledraft") || strings.Contains(s.HTML, ".othermark") {
		t.Fatal("sample render")
	}
	s, err = f.svc.RenderStyleSample(ctx, "checkin", other, nil)
	must(t, err)
	if !strings.Contains(s.HTML, ".othermark") {
		t.Fatal("sample render of a saved style")
	}
	_, err = f.svc.RenderStyleSample(ctx, "nope", other, nil)
	wantInvalid(t, err, "没有名为")
}

func TestRenderWarnings(t *testing.T) {
	f := scheduleFixture(t)
	ctx := f.ctx
	id := newStyle(t, f.svc, "甲", ".logo{background:url(asset:missing)}").Style.ID
	_, err := f.svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Slots: map[string]string{"cover": `<p id="covermark">{{.Meet}}</p>`}})
	must(t, err)
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, id))
	r, err := f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{})
	must(t, err)
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "missing") {
		t.Fatalf("warnings = %v", r.Warnings)
	}
	// Templates written for another report data version are not used, and the report says so.
	must(t, f.svc.store.Write(ctx, store.Stmt(`UPDATE print_styles SET data_version = 99 WHERE id = ?`, id)))
	r, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{})
	must(t, err)
	if strings.Contains(r.HTML, "covermark") || !strings.Contains(strings.Join(r.Warnings, "\n"), "已失效") {
		t.Fatalf("stale templates used: %v", r.Warnings)
	}
	// Saving CSS alone leaves the stale stamp (the templates were not checked).
	saved, err := f.svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{CSS: strp(".b{color:red}")})
	must(t, err)
	if saved.Style.DataVersion != 99 || len(saved.Warnings) == 0 {
		t.Fatalf("saved = %+v", saved)
	}
	// Touching a slot validates it against the current data and stamps the version.
	saved, err = f.svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Slots: map[string]string{"cover": "<p id=\"covermark\">新</p>"}})
	must(t, err)
	if saved.Style.DataVersion != 1 {
		t.Fatalf("data version = %d", saved.Style.DataVersion)
	}
}

// A meet's image wins over a style's of the same name.
func TestMeetImagesWinOverStyleImages(t *testing.T) {
	f := scheduleFixture(t)
	ctx := f.ctx
	id := newStyle(t, f.svc, "甲", ".logo{background:url(asset:logo)} .only{background:url(asset:styleonly)}").Style.ID
	_, err := f.svc.SaveAsset(ctx, ui, "style", id, "logo", "image/png", pngBytes(1))
	must(t, err)
	_, err = f.svc.SaveAsset(ctx, ui, "style", id, "styleonly", "image/png", pngBytes(3))
	must(t, err)
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, id))
	r, err := f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{})
	must(t, err)
	if !strings.Contains(r.HTML, dataURI(1)) || len(r.Warnings) != 0 {
		t.Fatalf("the style's image missing: %v", r.Warnings)
	}
	_, err = f.svc.SaveAsset(ctx, ui, "meet", f.meet, "logo", "image/png", pngBytes(2))
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "图", BodyMD: "![x](asset:logo)"})
	must(t, err)
	r, err = f.svc.RenderReportWith(ctx, ReportStartList, f.meet, nil, RenderOptions{})
	must(t, err)
	if strings.Contains(r.HTML, dataURI(1)) || !strings.Contains(r.HTML, dataURI(2)) || !strings.Contains(r.HTML, dataURI(3)) || len(r.Warnings) != 0 {
		t.Fatalf("meet image priority: %v", r.Warnings)
	}
	if !strings.Contains(r.HTML, `<img src="data:image/png;base64,`+dataURI(2)) {
		t.Fatal("the front page's image is not the meet's")
	}
}
