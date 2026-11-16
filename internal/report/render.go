package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"slices"
	"sort"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

// Report kinds: the keys of Style.Templates and of the custom templates a
// style file holds (printstyle.ReportKinds).
const (
	KindEntries     = "entries"
	KindStartList   = "startlist"
	KindResults     = "results"
	KindTeamScores  = "team_scores"
	KindCheckIn     = "checkin"
	KindRecordSheet = "recordsheet"
)

// Style is what a print style adds to the default look of a report. The
// zero value is the default style: nothing is changed, and the output is the
// built-in one.
type Style struct {
	// Params are the layout knobs; the zero value reads as DefaultParams.
	Params printstyle.Params
	// CSS follows the report's own CSS; it is cleaned when rendering.
	CSS string
	// Slots are small templates by slot name (printstyle.SlotNames), run
	// over the report's data. cover is a page of its own before everything;
	// before_rosters and after_events are placed by the report's template;
	// header_left, header_right and footer are text for the running head
	// and foot, where footer may use {page} and {pages}.
	Slots map[string]string
	// Templates replace a whole report's body, by kind. They may use
	// {{template "competitor" .}}, {{template "bars" .Bars}} and, in
	// results, {{template "teamScores" .TeamScores}}.
	Templates map[string]string
	// DataVersion is the report data version Slots and Templates were
	// written for; they are not used when it is not DataVersion.
	DataVersion int
	// FrontPages go at the start of the reports they name, each on a page
	// of its own, after the cover.
	FrontPages []FrontPage
	// Assets are the images asset:名称 refers to, a meet's over a style's
	// (the caller merges them).
	Assets map[string]printstyle.Asset
}

// FrontPage is a meet's text page, already rendered from Markdown
// (printstyle.RenderMarkdown).
type FrontPage struct {
	Title string `json:"title"`
	HTML  string `json:"html"`
	// Reports are the report kinds it goes in; empty = all of them.
	Reports []string `json:"reports"`
}

// StyleFromFile is the style a style file holds, with its images; the
// caller adds front pages and the meet's images.
func StyleFromFile(f printstyle.File) (Style, error) {
	assets, err := f.AssetMap()
	if err != nil {
		return Style{}, err
	}
	return Style{Params: f.Params, CSS: f.CSS, Slots: f.Slots, Templates: f.Templates, DataVersion: f.DataVersion, Assets: assets}, nil
}

// custom reports whether the style holds anything that needs the cleaning
// of the whole document.
func (s Style) custom() bool {
	return s.CSS != "" || len(s.Slots) > 0 || len(s.Templates) > 0
}

// params is the style's Params, the defaults for the zero value and for
// parameters that do not validate (with a warning).
func (s Style) params() (printstyle.Params, []string) {
	if s.Params == (printstyle.Params{}) {
		return printstyle.DefaultParams(), nil
	}
	if err := s.Params.Validate(); err != nil {
		return printstyle.DefaultParams(), []string{"版面参数无效，已用默认值：" + err.Error()}
	}
	return s.Params, nil
}

//go:embed base.css
var baseCSS string

//go:embed default.css
var defaultCSS string

//go:embed parts.html
var partsHTML string

//go:embed startlist.html
var startListHTML string

//go:embed entries.html
var entriesHTML string

//go:embed team_scores.html
var teamScoresHTML string

//go:embed results.html
var resultsHTML string

//go:embed checkin.html
var checkInHTML string

//go:embed recordsheet.html
var recordSheetHTML string

var (
	recordSheetTmpl = template.Must(template.New("recordsheet").Parse(recordSheetHTML))
	startListTmpl   = template.Must(template.New("startlist").Parse(startListHTML + partsHTML))
	entriesTmpl     = template.Must(template.New("entries").Parse(entriesHTML + partsHTML))
	resultsTmpl     = template.Must(template.New("results").Parse(resultsHTML + teamScoresHTML + partsHTML))
	checkInTmpl     = template.Must(template.New("checkin").Parse(checkInHTML))
)

// docSpec is what differs between the report kinds.
type docSpec struct {
	kind    string
	title   string
	meet    string
	tmpl    *template.Template
	defs    string // shared definitions a custom template is parsed with
	header  bool   // a running head from the second page
	newPage bool
}

// sharedDefs are the named templates of a kind's built-in template that a
// custom template may call.
func sharedDefs(kind string) string {
	switch kind {
	case KindEntries, KindStartList:
		return partsHTML
	case KindResults, KindTeamScores:
		return teamScoresHTML + partsHTML
	}
	return ""
}

// slotTexts are the slots drawn by the preview itself.
var slotTexts = []string{"header_left", "header_right", "footer"}

// renderDoc builds the document: the head (CSP, the base rules, the
// parameters, the default look, the style's CSS), the body (cover, front
// pages, then the report from its custom or built-in template) and, for a
// style with custom parts, cleans the whole document. warnings say what
// the style could not do.
func renderDoc[T any](d docSpec, st Style, p printstyle.Params, warns []string, data *T, setSlots func(map[string]template.HTML)) (string, []string, error) {
	// Slots and templates are written for one version of the data.
	useCustom := len(st.Slots) > 0 || len(st.Templates) > 0
	if useCustom && st.DataVersion != DataVersion {
		warns = append(warns, "自定义模板已失效，已用默认样式")
		useCustom = false
	}

	var cover string
	slots := map[string]template.HTML{}
	texts := map[string]string{}
	if useCustom {
		for _, name := range sortedNames(st.Slots) {
			src := st.Slots[name]
			if strings.TrimSpace(src) == "" {
				continue
			}
			out, err := runTemplate(src, d.defs, *data)
			if err != nil {
				warns = append(warns, fmt.Sprintf("插槽「%s」出错，已略过：%v", name, err))
				continue
			}
			switch {
			case name == "cover":
				cover = out
			case slices.Contains(slotTexts, name):
				texts[name] = printstyle.PlainText(out)
			default:
				slots[name] = template.HTML(out)
			}
		}
	}
	setSlots(slots)

	var body string
	if src := st.Templates[d.kind]; useCustom && strings.TrimSpace(src) != "" {
		out, err := runTemplate(src, d.defs, *data)
		if err != nil {
			warns = append(warns, fmt.Sprintf("自定义模板出错，已用默认样式：%v", err))
		} else {
			body = out
		}
	}
	if body == "" {
		var buf bytes.Buffer
		if err := d.tmpl.Execute(&buf, *data); err != nil {
			return "", warns, err
		}
		body = buf.String()
	}

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"UTF-8\">\n")
	b.WriteString(printstyle.CSPMeta + "\n")
	fmt.Fprintf(&b, "<title>%s %s</title>\n", html.EscapeString(d.meet), html.EscapeString(d.title))
	b.WriteString("<style>" + baseCSS)
	if d.newPage {
		b.WriteString("\n.event ~ .event, .rosters + .event, .schedule + .event { break-before: page; page-break-before: always; }")
	}
	b.WriteString("</style>\n<style>" + p.CSS() + "</style>\n<style>" + defaultCSS + "</style>\n")
	if st.CSS != "" {
		b.WriteString("<style>" + st.CSS + "</style>\n")
	}
	b.WriteString("</head>\n<body")
	paper, margin := p.DataAttrs()
	attr := func(name, value string) { fmt.Fprintf(&b, " %s=\"%s\"", name, html.EscapeString(value)) }
	// The running head and foot are drawn by the preview (paginate.ts) in
	// the margins: body's data-meet / data-title are its left and right
	// text, data-head-left / data-head-right replace them; data-foot
	// replaces the page number (with {page} and {pages}), data-pagenum is
	// that number's format. Without data-meet there is no running head.
	if d.header && p.Header {
		attr("data-meet", d.meet)
		attr("data-title", d.title)
		if v, ok := texts["header_left"]; ok {
			attr("data-head-left", v)
		}
		if v, ok := texts["header_right"]; ok {
			attr("data-head-right", v)
		}
	}
	attr("data-paper", paper)
	attr("data-margin", margin)
	if v := texts["footer"]; v != "" {
		attr("data-foot", v)
	}
	attr("data-pagenum", p.PageNumber)
	b.WriteString(">\n")
	if strings.TrimSpace(cover) != "" {
		b.WriteString("<section class=\"cover\">" + cover + "</section>\n")
	}
	for _, fp := range st.FrontPages {
		if len(fp.Reports) > 0 && !slices.Contains(fp.Reports, d.kind) {
			continue
		}
		clean, _ := printstyle.SanitizeFragment(fp.HTML)
		b.WriteString("<section class=\"front\">" + clean + "</section>\n")
	}
	// The templates end in white space (their definitions follow the content).
	b.WriteString(strings.TrimRight(body, " \t\r\n") + "\n</body>\n</html>\n")

	out := b.String()
	if st.custom() {
		var w []string
		out, w = printstyle.SanitizeDocument(out)
		warns = append(warns, w...)
	}
	if len(st.Assets) > 0 || strings.Contains(out, "asset:") {
		var w []string
		out, w = printstyle.ResolveAssets(out, st.Assets)
		warns = append(warns, w...)
	}
	return out, warns, nil
}

func sortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func runTemplate(src, defs string, data any) (string, error) {
	t, err := printstyle.CompileTemplate(src, defs)
	if err != nil {
		return "", err
	}
	return t.Execute(data)
}

// applyColumns sets the style's column switches and roster width on a book
// and on the books of its groups.
func applyColumns(b *Book, p printstyle.Params) {
	b.HideBib, b.HideUnit, b.HideSeed = !p.ShowBib, !p.ShowUnit, !p.ShowSeed
	b.RosterColumns = p.RosterColumns
	for i := range b.GroupBooks {
		applyColumns(&b.GroupBooks[i], p)
	}
}

// prepareBook is the book a kind's template receives: titled, with the
// style's column switches, and for the 团体总分表 flagged and with its
// tables standing alone (the title block names them; they do not head
// themselves again).
func prepareBook(kind string, b Book, st Style) (Book, printstyle.Params, []string) {
	b.Title = KindTitle(kind)
	p, warns := st.params()
	applyColumns(&b, p)
	if kind == KindTeamScores {
		b.TeamOnly = true
		if b.TeamScores != nil {
			ts := *b.TeamScores
			ts.Standalone = true
			b.TeamScores = &ts
		}
		groups := make([]Book, len(b.GroupBooks))
		for i, g := range b.GroupBooks {
			if g.TeamScores != nil {
				ts := *g.TeamScores
				ts.Standalone = true
				g.TeamScores = &ts
			}
			groups[i] = g
		}
		b.GroupBooks = groups
	}
	return b, p, warns
}

func renderBook(kind string, tmpl *template.Template, b Book, st Style) (string, []string, error) {
	b, p, warns := prepareBook(kind, b, st)
	d := docSpec{kind: kind, title: b.Title, meet: b.Meet, tmpl: tmpl, defs: sharedDefs(kind), header: true, newPage: b.NewPage}
	return renderDoc(d, st, p, warns, &b, func(s map[string]template.HTML) { b.Slots = s })
}

// Prepare returns the data a report kind's template receives for a
// book (Book, CheckInBook or RecordBook, as the service assembled it) in
// a style: what it is rendered over, so that data and report cannot differ.
func Prepare(kind string, book any, st Style) (any, error) {
	switch b := book.(type) {
	case Book:
		if kind != KindEntries && kind != KindStartList && kind != KindResults && kind != KindTeamScores {
			break
		}
		out, _, _ := prepareBook(kind, b, st)
		return out, nil
	case CheckInBook:
		if kind == KindCheckIn {
			b.Title = KindTitle(kind)
			return b, nil
		}
	case RecordBook:
		if kind == KindRecordSheet {
			b.Title = KindTitle(kind)
			return b, nil
		}
	}
	return nil, fmt.Errorf("报表「%s」的数据类型不符", kind)
}

// StartList renders the 秩序册: every heat with its positions.
func StartList(b Book, st Style) (string, []string, error) {
	return renderBook(KindStartList, startListTmpl, b, st)
}

// Entries renders the 报名表, including entries without arranged positions.
func Entries(b Book, st Style) (string, []string, error) {
	return renderBook(KindEntries, entriesTmpl, b, st)
}

// Results renders the 成绩册: every round's results and ranking.
func Results(b Book, st Style) (string, []string, error) {
	return renderBook(KindResults, resultsTmpl, b, st)
}

// TeamScores renders the 团体总分表 on its own.
func TeamScores(b Book, st Style) (string, []string, error) {
	return renderBook(KindTeamScores, resultsTmpl, b, st)
}

// CheckIn renders the 检录单.
func CheckIn(b CheckInBook, st Style) (string, []string, error) {
	b.Title = KindTitle(KindCheckIn)
	p, warns := st.params()
	d := docSpec{kind: KindCheckIn, title: b.Title, meet: b.Meet, tmpl: checkInTmpl, defs: sharedDefs(KindCheckIn)}
	return renderDoc(d, st, p, warns, &b, func(s map[string]template.HTML) { b.Slots = s })
}

// RecordSheets renders the 成绩记录表.
func RecordSheets(b RecordBook, st Style) (string, []string, error) {
	b.Title = KindTitle(KindRecordSheet)
	p, warns := st.params()
	d := docSpec{kind: KindRecordSheet, title: b.Title, meet: b.Meet, tmpl: recordSheetTmpl, defs: sharedDefs(KindRecordSheet)}
	return renderDoc(d, st, p, warns, &b, func(s map[string]template.HTML) { b.Slots = s })
}

// RenderSample renders a kind's built-in sample data (SampleBook and the
// like) with a style: the way to try a style, and the templates in it,
// before a meet's data meets them.
func RenderSample(kind string, st Style) (string, []string, error) {
	switch kind {
	case KindEntries:
		return Entries(SampleBook(), st)
	case KindStartList:
		return StartList(SampleBook(), st)
	case KindResults:
		return Results(SampleBook(), st)
	case KindTeamScores:
		return TeamScores(SampleBook(), st)
	case KindCheckIn:
		return CheckIn(SampleCheckInBook(), st)
	case KindRecordSheet:
		return RecordSheets(SampleRecordBook(), st)
	}
	return "", nil, fmt.Errorf("没有名为「%s」的报表", kind)
}

// sampleFor is the data of a kind's sample, as a template sees it.
func sampleFor(kind string) (any, error) {
	switch kind {
	case KindEntries, KindStartList, KindResults:
		b := SampleBook()
		applyColumns(&b, printstyle.DefaultParams())
		return b, nil
	case KindTeamScores:
		b := SampleBook()
		b.TeamOnly = true
		applyColumns(&b, printstyle.DefaultParams())
		return b, nil
	case KindCheckIn:
		return SampleCheckInBook(), nil
	case KindRecordSheet:
		return SampleRecordBook(), nil
	}
	return nil, fmt.Errorf("没有名为「%s」的报表", kind)
}

// ValidateTemplate checks that a custom template for a report kind parses
// and runs over the kind's sample data.
func ValidateTemplate(kind, source string) error {
	data, err := sampleFor(kind)
	if err != nil {
		return err
	}
	_, err = runTemplate(source, sharedDefs(kind), data)
	return err
}

// ValidateSlot checks that a slot template parses and runs over the sample
// data of every report kind it may be used in.
func ValidateSlot(slot, source string) error {
	if !slices.Contains(printstyle.SlotNames, slot) {
		return fmt.Errorf("没有名为「%s」的插槽", slot)
	}
	for _, kind := range printstyle.ReportKinds {
		data, _ := sampleFor(kind)
		if _, err := runTemplate(source, sharedDefs(kind), data); err != nil {
			return fmt.Errorf("在「%s」的样例数据上出错：%v", kind, err)
		}
	}
	return nil
}
