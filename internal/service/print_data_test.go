package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
)

func TestGetReportDataIsWhatTheTemplateGets(t *testing.T) {
	f := newFixture(t, 3, EventInput{Gender: "男", Discipline: "100m", Kind: "track"})
	f.addHeat(0, f.athletes...)
	for _, kind := range printstyle.ReportKinds {
		d, err := f.svc.GetReportData(f.ctx, ReportRequest{Kind: kind, MeetID: f.meet})
		must(t, err)
		if d.Kind != kind || d.DataVersion != report.DataVersion {
			t.Fatalf("%s: %+v", kind, d)
		}
		var m map[string]any
		must(t, json.Unmarshal(d.Data, &m))
		if m["meet"] != "秋季运动会" || m["title"] != report.KindTitle(kind) {
			t.Fatalf("%s: meet=%v title=%v", kind, m["meet"], m["title"])
		}
	}
	// The start list holds the arranged competitors.
	d, err := f.svc.GetReportData(f.ctx, ReportRequest{Kind: "startlist", MeetID: f.meet})
	must(t, err)
	var book report.Book
	must(t, json.Unmarshal(d.Data, &book))
	if len(book.Events) != 1 || len(book.Events[0].Rounds) == 0 || len(book.Events[0].Rounds[0].Heats[0].Lines) != 3 || book.Events[0].Rounds[0].Heats[0].Lines[0].Name != "选手01" {
		t.Fatalf("book = %+v", book.Events)
	}
	// A meet's own style choice shapes the data as it shapes the report: the
	// columns the style hides are the ones its template sees hidden.
	st := newStyle(t, f.svc, "无号码", "")
	p := printstyle.DefaultParams()
	p.ShowBib = false
	_, err = f.svc.SavePrintStyle(f.ctx, ui, st.Style.ID, PrintStyleInput{Params: &p})
	must(t, err)
	must(t, f.svc.SetMeetPrintStyle(f.ctx, ui, f.meet, st.Style.ID))
	d, err = f.svc.GetReportData(f.ctx, ReportRequest{Kind: "startlist", MeetID: f.meet})
	must(t, err)
	var hidden report.Book
	must(t, json.Unmarshal(d.Data, &hidden))
	if !hidden.HideBib || hidden.HideUnit || hidden.RosterColumns != p.RosterColumns {
		t.Fatalf("columns = %+v", hidden)
	}
	// The same book is what a custom template runs over.
	probe := `{{if .HideBib}}NOBIB{{end}}{{.Title}}`
	r, err := f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "startlist", MeetID: f.meet}, RenderOptions{Draft: &PrintStyleInput{Templates: map[string]string{"startlist": probe}}})
	must(t, err)
	if !strings.Contains(r.HTML, "NOBIB秩序册") {
		t.Fatal("template did not see the hidden column")
	}

	wantInvalid(t, func() error {
		_, err := f.svc.GetReportData(f.ctx, ReportRequest{Kind: "nope", MeetID: f.meet})
		return err
	}(), "报表类型")
	if _, err := f.svc.GetReportData(f.ctx, ReportRequest{Kind: "startlist", MeetID: "missing"}); err == nil {
		t.Fatal("unknown meet accepted")
	}
	// Reading data writes nothing.
	n := len(*f.changes)
	_, err = f.svc.GetReportData(f.ctx, ReportRequest{Kind: "results", MeetID: f.meet})
	must(t, err)
	if len(*f.changes) != n {
		t.Fatal("reading report data changed something")
	}
}

func TestRenderReportRequestEveryKindAndStyleOptions(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "100m", Kind: "track"})
	f.addHeat(0, f.athletes...)
	for _, kind := range printstyle.ReportKinds {
		r, err := f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: kind, MeetID: f.meet}, RenderOptions{})
		must(t, err)
		if !strings.Contains(r.HTML, "秋季运动会") || r.Warnings == nil {
			t.Fatalf("%s: %q %v", kind, r.HTML[:80], r.Warnings)
		}
	}
	// A draft is tried, not saved.
	before, err := f.svc.ListPrintStyles(f.ctx)
	must(t, err)
	r, err := f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "entries", MeetID: f.meet}, RenderOptions{Draft: &PrintStyleInput{CSS: strp(".x{color:red} @import 'http://x';")}})
	must(t, err)
	if !strings.Contains(r.HTML, cleaned(t, ".x{color:red}")) || len(r.Warnings) == 0 {
		t.Fatalf("draft not applied: %v", r.Warnings)
	}
	after, err := f.svc.ListPrintStyles(f.ctx)
	must(t, err)
	if len(before) != len(after) {
		t.Fatal("draft saved a style")
	}
	wantInvalid(t, func() error {
		_, err := f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "entries", MeetID: f.meet}, RenderOptions{StyleID: "missing"})
		return err
	}(), "不存在")
	wantInvalid(t, func() error {
		_, err := f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "nope", MeetID: f.meet}, RenderOptions{})
		return err
	}(), "报表类型")
	// Check-in and record sheets take rounds and a day.
	rounds := f.detail().Rounds
	r, err = f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "checkin", MeetID: f.meet, RoundIDs: []string{rounds[0].ID}}, RenderOptions{})
	must(t, err)
	if !strings.Contains(r.HTML, "检录单") {
		t.Fatal("no check-in sheet")
	}
	_, err = f.svc.RenderReportRequest(f.ctx, ReportRequest{Kind: "recordsheet", MeetID: f.meet, Day: "bad"}, RenderOptions{})
	wantInvalid(t, err, "日期")
}

func TestGetReportTemplate(t *testing.T) {
	svc, _ := newTestService(t)
	for _, kind := range printstyle.ReportKinds {
		info, err := svc.GetReportTemplate(kind, kind == "results")
		must(t, err)
		if info.Source == "" || info.DataRoot == "" || len(info.DataTypes) == 0 || len(info.Funcs) == 0 || info.DataVersion != report.DataVersion {
			t.Fatalf("%s: %+v", kind, info)
		}
		if (kind == "results") != (len(info.Sample) > 0) {
			t.Fatalf("%s sample = %d", kind, len(info.Sample))
		}
		// The template as offered is accepted as a custom one.
		_, err = svc.SavePrintStyle(t.Context(), ui, "", PrintStyleInput{Name: strp("改自内置 " + kind), Templates: map[string]string{kind: info.Source}})
		must(t, err)
	}
	_, err := svc.GetReportTemplate("nope", false)
	wantInvalid(t, err, "没有名为")
}

func TestPrintGuideIsGeneratedFromTheCode(t *testing.T) {
	svc, _ := newTestService(t)
	g := svc.PrintGuide()
	if g.DataVersion != report.DataVersion || len(g.Workflow) == 0 || len(g.Pagination) == 0 || len(g.Templates) == 0 || len(g.FrontPages) == 0 || len(g.Assets) == 0 || len(g.Sanitizing) == 0 || len(g.Limits) == 0 {
		t.Fatalf("guide incomplete: %+v", g)
	}
	if len(g.Reports) != len(printstyle.ReportKinds) || len(g.Slots) != len(printstyle.SlotNames) || len(g.CSSVariables) == 0 || len(g.Classes) == 0 || len(g.TemplateFuncs) == 0 {
		t.Fatalf("guide lists: %d %d %d %d", len(g.Reports), len(g.Slots), len(g.CSSVariables), len(g.Classes))
	}
	for _, s := range g.Slots {
		if s.Doc == "" {
			t.Errorf("slot %s has no description", s.Name)
		}
	}
	for _, r := range g.Reports {
		if r.Doc == "" || r.Title == "" {
			t.Errorf("report %s has no description", r.Kind)
		}
	}

	// Every parameter is described, with the bounds Validate enforces.
	fields := map[string]bool{}
	rt := reflect.TypeFor[printstyle.Params]()
	for i := range rt.NumField() {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		fields[name] = true
	}
	if len(g.Params) != len(fields) {
		t.Fatalf("params = %d, fields = %d", len(g.Params), len(fields))
	}
	set := func(p *printstyle.Params, name string, v float64) {
		rv := reflect.ValueOf(p).Elem()
		for i := range rt.NumField() {
			if n, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ","); n == name {
				if rv.Field(i).Kind() == reflect.Int {
					rv.Field(i).SetInt(int64(v))
				} else {
					rv.Field(i).SetFloat(v)
				}
			}
		}
	}
	for _, d := range g.Params {
		if !fields[d.Name] || d.Doc == "" || d.Type == "" {
			t.Errorf("param %+v", d)
		}
		if d.Min != nil && d.Max != nil {
			base := printstyle.DefaultParams()
			if strings.HasPrefix(d.Name, "paper") {
				base.Paper, base.PaperWidth, base.PaperHeight = printstyle.PaperCustom, 210, 297
			}
			step := 0.001
			if d.Type == "integer" {
				step = 1
			}
			for _, tc := range []struct {
				v  float64
				ok bool
			}{{*d.Min, true}, {*d.Max, true}, {*d.Min - step, false}, {*d.Max + step, false}} {
				p := base
				set(&p, d.Name, tc.v)
				if err := p.Validate(); (err == nil) != tc.ok {
					t.Errorf("%s = %v: documented range %v–%v, Validate says %v", d.Name, tc.v, *d.Min, *d.Max, err)
				}
			}
		}
		for _, v := range d.Values {
			p := printstyle.DefaultParams()
			rv := reflect.ValueOf(&p).Elem()
			for i := range rt.NumField() {
				if n, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ","); n == d.Name {
					rv.Field(i).SetString(v)
				}
			}
			if d.Name == "paper" && v == printstyle.PaperCustom {
				p.PaperWidth, p.PaperHeight = 200, 200
			}
			if err := p.Validate(); err != nil {
				t.Errorf("%s = %q is documented but invalid: %v", d.Name, v, err)
			}
		}
	}

	// The elements the guide lists survive the cleaning (names from the guide text).
	listed := "header footer main section article div span p br hr h1 h2 h3 h4 h5 h6 small strong b em i u s del sub sup mark pre code blockquote ul ol li dl dt dd table thead tbody tfoot tr th td caption colgroup col figure figcaption img"
	var doc strings.Builder
	doc.WriteString("<body>")
	for _, el := range strings.Fields(listed) {
		switch el {
		case "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col":
			continue
		case "li":
			doc.WriteString("<ul><li>x</li></ul>")
		case "dt", "dd":
			doc.WriteString("<dl><" + el + ">x</" + el + "></dl>")
		case "img":
			doc.WriteString(`<img src="asset:x">`)
		case "br", "hr":
			doc.WriteString("<" + el + ">")
		default:
			doc.WriteString("<" + el + ">x</" + el + ">")
		}
	}
	doc.WriteString("<table><caption>c</caption><colgroup><col></colgroup><thead><tr><th>h</th></tr></thead><tbody><tr><td>d</td></tr></tbody><tfoot><tr><td>f</td></tr></tfoot></table></body>")
	out, _ := printstyle.SanitizeDocument(doc.String())
	for _, el := range strings.Fields(listed) {
		if !strings.Contains(out, "<"+el) {
			t.Errorf("element %s is listed as allowed but is removed", el)
		}
	}
	// The listed at-rules and the removed ones.
	clean, _, err := printstyle.SanitizeCSS("@import 'x'; @media print{p{color:red}} @page{margin:1mm}")
	must(t, err)
	if strings.Contains(clean, "@import") || !strings.Contains(clean, "@media") || !strings.Contains(clean, "@page") {
		t.Fatalf("css = %q", clean)
	}
}
