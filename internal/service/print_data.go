package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
)

// What a print-style author reads (through MCP): the report data of a
// meet, a report kind's built-in template and the interface description
// (PrintGuide). None of it writes.

// ReportRequest names a report and what it covers; fields a kind does not
// use are ignored.
type ReportRequest struct {
	// Kind is one of printstyle.ReportKinds.
	Kind   string `json:"kind"`
	MeetID string `json:"meetId"`
	// EventIDs limit the 报名表, 秩序册, 成绩册 and 团体总分表 (ordinary
	// events and 全能); empty = all.
	EventIDs []string `json:"eventIds"`
	// GroupIDs limit the 成绩册 and 团体总分表 by 组别; empty = all, ""
	// selects the ungrouped.
	GroupIDs []string `json:"groupIds"`
	// RoundIDs and Day select the heats of the 检录单 and 成绩记录表:
	// those of the rounds (empty = every round), and with Day only those
	// planned on that day (YYYY-MM-DD).
	RoundIDs []string `json:"roundIds"`
	Day      string   `json:"day"`
}

// assembleReport gathers a report of any kind for a request.
func (s *Service) assembleReport(ctx context.Context, req ReportRequest) (*assembledReport, error) {
	switch req.Kind {
	case ReportEntries:
		return s.entriesBook(ctx, req.MeetID, req.EventIDs)
	case ReportStartList:
		return s.startListBook(ctx, req.MeetID, req.EventIDs)
	case ReportResults, ReportTeamScores:
		return s.groupedBook(ctx, GroupReportRequest{Kind: req.Kind, MeetID: req.MeetID, GroupIDs: req.GroupIDs, EventIDs: req.EventIDs})
	case report.KindCheckIn:
		return s.checkInBook(ctx, CheckInRequest{MeetID: req.MeetID, RoundIDs: req.RoundIDs, Day: req.Day})
	case report.KindRecordSheet:
		return s.recordSheetBook(ctx, CheckInRequest{MeetID: req.MeetID, RoundIDs: req.RoundIDs, Day: req.Day})
	}
	return nil, invalid("报表类型不正确，可用 %s", strings.Join(printstyle.ReportKinds, "、"))
}

// RenderReportRequest renders any report kind in the style opt chooses
// (the meet's own without options), with the style's warnings.
func (s *Service) RenderReportRequest(ctx context.Context, req ReportRequest, opt RenderOptions) (*RenderedReport, error) {
	a, err := s.assembleReport(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.renderAssembled(ctx, req.MeetID, a, opt)
}

// ReportData is the data a report's template runs over.
type ReportData struct {
	Kind        string `json:"kind"`
	DataVersion int    `json:"dataVersion"`
	// Data is Book (entries, startlist, results, team_scores),
	// CheckInBook or RecordBook as JSON; report.DataSchema says what each
	// field is, with the Go name a template writes.
	Data json.RawMessage `json:"data"`
}

// GetReportData returns a meet's report data for a kind and range: the
// book the report's template receives, as the meet's settings and its
// style's parameters shape it. Read only. It is assembled and prepared
// by the same code that renders the report, so it cannot differ from what a
// template gets.
func (s *Service) GetReportData(ctx context.Context, req ReportRequest) (*ReportData, error) {
	a, err := s.assembleReport(ctx, req)
	if err != nil {
		return nil, err
	}
	st, _, err := s.reportStyle(ctx, req.MeetID, a.styleID, RenderOptions{})
	if err != nil {
		return nil, err
	}
	book, err := report.Prepare(a.kind, a.book, st)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(book)
	if err != nil {
		return nil, err
	}
	return &ReportData{Kind: req.Kind, DataVersion: report.DataVersion, Data: raw}, nil
}

// ReportTemplateInfo is a report kind's built-in template with the data it
// runs over and the functions it may use.
type ReportTemplateInfo struct {
	report.TemplateSource
	DataVersion int                  `json:"dataVersion"`
	DataRoot    string               `json:"dataRoot"`
	DataTypes   []report.DataType    `json:"dataTypes"`
	Funcs       []printstyle.FuncDoc `json:"funcs"`
	Sample      json.RawMessage      `json:"sample,omitempty"`
}

// GetReportTemplate returns a kind's built-in template source, the parts
// it calls, the data types and the template functions; withSample adds the
// kind's sample data as JSON.
func (s *Service) GetReportTemplate(kind string, withSample bool) (*ReportTemplateInfo, error) {
	src, err := report.BuiltinTemplate(kind)
	if err != nil {
		return nil, invalid("%s，可用 %s", err, strings.Join(printstyle.ReportKinds, "、"))
	}
	root, types, err := report.DataSchema(kind)
	if err != nil {
		return nil, invalid("%s", err)
	}
	info := &ReportTemplateInfo{TemplateSource: src, DataVersion: report.DataVersion, DataRoot: root, DataTypes: types, Funcs: printstyle.TemplateFuncs()}
	if withSample {
		if info.Sample, err = report.SampleData(kind); err != nil {
			return nil, invalid("%s", err)
		}
	}
	return info, nil
}

// ---- the interface description ----

// ParamDoc describes one layout parameter of a style.
type ParamDoc struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default any    `json:"default"`
	// Min and Max bound a number; Values list the allowed words.
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
	Values []string `json:"values,omitempty"`
	Doc    string   `json:"doc"`
}

// SlotDoc says where a slot's output goes.
type SlotDoc struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
}

// ReportDoc names a report kind and its data.
type ReportDoc struct {
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	DataType string `json:"dataType"`
	Doc      string `json:"doc"`
}

// PrintStyleGuide describes the interface a print style is written against.
// Params, variables, classes, slots, kinds and limits are read from the
// code and the stylesheets; the rest is the rules in words.
type PrintStyleGuide struct {
	Workflow      []string             `json:"workflow"`
	DataVersion   int                  `json:"dataVersion"`
	Params        []ParamDoc           `json:"params"`
	CSSVariables  []report.CSSVariable `json:"cssVariables"`
	Classes       []string             `json:"classes"`
	Reports       []ReportDoc          `json:"reports"`
	Slots         []SlotDoc            `json:"slots"`
	Pagination    []string             `json:"pagination"`
	Templates     []string             `json:"templates"`
	TemplateFuncs []printstyle.FuncDoc `json:"templateFuncs"`
	FrontPages    []string             `json:"frontPages"`
	Assets        []string             `json:"assets"`
	Sanitizing    []string             `json:"sanitizing"`
	Limits        []string             `json:"limits"`
}

// paramDocs are what the parameters mean, by json name. The ranges of the
// numbers come from printstyle.NumberLimits, which Validate enforces.
var paramDocs = map[string]struct {
	doc    string
	values []string
}{
	"paper":          {doc: "纸张；custom 用 paperWidth / paperHeight", values: printstyle.Papers},
	"paperWidth":     {doc: "自定义纸张宽，mm，仅 paper=custom"},
	"paperHeight":    {doc: "自定义纸张高，mm，仅 paper=custom"},
	"orientation":    {doc: "方向：portrait 纵向、landscape 横向", values: []string{printstyle.Portrait, printstyle.Landscape}},
	"marginTop":      {doc: "上边距，mm"},
	"marginRight":    {doc: "右边距，mm"},
	"marginBottom":   {doc: "下边距，mm"},
	"marginLeft":     {doc: "左边距，mm"},
	"font":           {doc: "正文字体：song 宋体、hei 黑体、kai 楷体、fang 仿宋、sans 无衬线（各带 macOS / Windows / Linux 的备选字体）", values: printstyle.Fonts},
	"fontSize":       {doc: "正文字号，pt"},
	"lineHeight":     {doc: "行距，字号的倍数"},
	"titleSize":      {doc: "页首赛事名称的字号，pt"},
	"eventTitleSize": {doc: "项目标题的字号，pt"},
	"tableLines":     {doc: "表格线：rules 只有横线、grid 全框线", values: []string{printstyle.LinesRules, printstyle.LinesGrid}},
	"lineWidth":      {doc: "粗线宽度，pt，细线按比例跟随"},
	"header":         {doc: "从第二页起印页眉（赛事名与报表名）"},
	"pageNumber":     {doc: "页码：n_of_m 第 N 页 / 共 M 页、dash - N -、none 不印", values: printstyle.PageNumbers},
	"rosterColumns":  {doc: "代表队名单每行的人数"},
	"showUnit":       {doc: "表格里印「单位」列"},
	"showSeed":       {doc: "表格里印「报名成绩」列"},
	"showBib":        {doc: "表格里印「号码」列"},
}

var slotDocs = map[string]string{
	"cover":          "封面：独占第一页，输出放在 <section class=\"cover\"> 里，所有报表都有；之后的前置页与正文各另起一页",
	"before_rosters": "秩序册「代表队名单」之前；内置秩序册模板输出它，自定义模板要自己写 {{with .Slots.before_rosters}}{{.}}{{end}}",
	"after_events":   "全部内容之后、「打印时间」之前，所有报表；自定义模板要自己写 {{with .Slots.after_events}}{{.}}{{end}}",
	"header_left":    "页眉左侧文字（默认赛事名称），第二页起；只取纯文本，标签会被去掉",
	"header_right":   "页眉右侧文字（默认报表名称），第二页起；只取纯文本",
	"footer":         "页脚文字，代替页码格式；可用 {page} 与 {pages}（总页数）；只取纯文本",
}

var reportDocs = map[string]string{
	report.KindEntries:     "按项目列出全部报名者，未编排也可印",
	report.KindStartList:   "每个组的位置；含代表队名单、竞赛日程表",
	report.KindResults:     "各轮次成绩与名次；多组别时 GroupBooks 每组别一本、各另起页",
	report.KindTeamScores:  "团体总分表，与成绩册同一个模板与数据类型，TeamOnly=true",
	report.KindCheckIn:     "检录单，每组一张 .sheet",
	report.KindRecordSheet: "成绩记录表，每组一张空白 .sheet，供裁判手写",
}

func paramDocList() []ParamDoc {
	d := printstyle.DefaultParams()
	rv := reflect.ValueOf(d)
	var out []ParamDoc
	for i := range rv.NumField() {
		f := rv.Type().Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		typ := f.Type.Name()
		if f.Type.Kind() == reflect.Float64 {
			typ = "number"
		} else if f.Type.Kind() == reflect.Int {
			typ = "integer"
		} else if f.Type.Kind() == reflect.Bool {
			typ = "boolean"
		} else if f.Type.Kind() == reflect.String {
			typ = "string"
		}
		doc := paramDocs[name]
		pd := ParamDoc{Name: name, Type: typ, Default: rv.Field(i).Interface(), Values: doc.values, Doc: doc.doc}
		if l, ok := printstyle.Limit(name); ok {
			pd.Min, pd.Max = &l.Min, &l.Max
		}
		out = append(out, pd)
	}
	return out
}

// PrintParamRules are the limits of a style's layout parameters, for the
// form to show a wrong value at its field before it is saved.
type PrintParamRules struct {
	Numbers []printstyle.NumberLimit `json:"numbers"`
	// Papers are the named sheets, width and height in mm, portrait.
	Papers map[string][2]float64 `json:"papers"`
	// MinContent is the least the margins leave of the sheet each way, mm;
	// MarginsTooBig is the message when they leave less.
	MinContent    float64 `json:"minContent"`
	MarginsTooBig string  `json:"marginsTooBig"`
}

// PrintParamRules returns the limits Params.Validate checks.
func (s *Service) PrintParamRules() PrintParamRules {
	return PrintParamRules{Numbers: printstyle.NumberLimits, Papers: printstyle.PaperSizes(), MinContent: printstyle.MinContentMM, MarginsTooBig: printstyle.MarginsTooBig}
}

// PrintGuide describes the interface of print styles for whoever writes
// one: parameters, CSS variables and classes, slots, pagination, template
// and Markdown rules, cleaning and limits.
func (s *Service) PrintGuide() PrintStyleGuide {
	g := PrintStyleGuide{
		DataVersion:   report.DataVersion,
		Params:        paramDocList(),
		CSSVariables:  report.CSSVariables(),
		Classes:       report.Classes(),
		TemplateFuncs: printstyle.TemplateFuncs(),
	}
	for _, kind := range printstyle.ReportKinds {
		src, _ := report.BuiltinTemplate(kind)
		g.Reports = append(g.Reports, ReportDoc{Kind: kind, Title: src.Title, DataType: src.DataType, Doc: reportDocs[kind]})
	}
	for _, name := range printstyle.SlotNames {
		g.Slots = append(g.Slots, SlotDoc{Name: name, Doc: slotDocs[name]})
	}
	g.Workflow = []string{
		"样式全局保存，跨赛事；用 set_meet_print_style 让某个赛事使用它。内置「默认」样式不能改，要改先 copy_print_style。",
		"save_print_style 直接生效，不需要预览确认；每次保存留一个版本，写坏了用 list_print_style_versions 与 restore_print_style_version 回退。",
		"改完用 render_style_sample（内置样例数据）或 render_report（某赛事的真实数据，可用 draft 试写不保存）检查，留意返回的 warnings；用户在应用「打印」页立即看到效果。",
		"优先顺序：先调 params（纸张、边距、字号、表格线、列）；不够再写 css（改变量、改外观）；再不够才写 slots（封面、页眉页脚、插槽）；最后才是整个报表的 templates。",
		"templates 与 slots 针对 dataVersion 写成，报表数据版本变化后它们不再使用（报表退回默认外观并给出警告），需要重写。",
		"前置页（致辞、规程、须知）是赛事的内容，用 save_front_page 写 Markdown，不属于样式，换样式不丢。",
		"图片用 save_asset 保存到样式（scope=style）或赛事（scope=meet），在 CSS、模板、Markdown 里写 asset:名称 引用；赛事的同名图片优先于样式的。",
	}
	g.Pagination = []string{
		"打印的是预览排好的纸面：预览按 params 的纸张与边距把内容排到一页页的纸上，页眉、页脚、页码画在边距里，表格断页处画「续下页」。",
		"报表的 <body> 带 data-paper（宽x高 mm）、data-margin（上,右,下,左 mm）、data-pagenum，页眉页脚文字用 data-meet / data-title / data-head-left / data-head-right / data-foot；这些由 params 与插槽生成，不要在模板里写 body，也不要改 .pv-* 类（预览内部使用）。",
		"不会被拆开跨页：.heat、.unit、.sheet、表格的行 tr；表格可以跨页，thead 每页重复；h2 h3 h4 不会孤立在页底。",
		"强制另起一页：.sheet.break、.report-group 的第二个起（按组别分榜）、.page-break（Markdown 的 ---pagebreak---）、.front（前置页）、.cover 之后的内容；赛事设置「每个项目另起一页」时 .event 也另起页。",
		"模板里要另起页：给元素加 class=\"page-break\"（独占一行的分页线），或在 CSS 里写 break-before: page。",
	}
	g.Templates = []string{
		"模板是 Go html/template：用 {{.字段}} 取数据，字段用 Go 名（如 {{.Meet}}、{{range .Events}}）；get_report_data 返回的 JSON 键名是各字段的 json 名，get_report_template 的 dataTypes 同时给出两者以及可调用的方法（如 {{.RosterStyle}}、{{range .RowsOf 4}}）。",
		"自定义报表模板只写 <body> 里的内容，替换整个正文：页首的赛事名称块（<header>）、各插槽的位置、「打印时间」页脚都要自己写，可以从 get_report_template 给出的内置模板改起。不要写 <html>、<head>、<style>、<script>：外观放在 css 里。",
		"可调用 {{template \"competitor\" .}}（姓名与队员）、{{template \"bars\" .Bars}}（试跳记录），成绩册与团体总分表还有 {{template \"teamScores\" .TeamScores}}；各类报表可用的部分以 get_report_template 的 parts 为准。",
		"cover、前置页、页眉页脚由系统放进文档，不在模板里写。插槽（slots）与报表模板同样运行在报表数据上。",
		"内容自动转义；插槽里的 Slots 值已是 HTML，直接输出即可。出错的模板运行时会退回内置版式并在 warnings 里说明；保存时会用样例数据试跑，不通过则拒绝保存。",
	}
	g.FrontPages = []string{
		"前置页是赛事的 Markdown 页面，印在所选报表（reports 为空即所有报表）的最前面，每页独占一页，在封面之后；save_front_page 直接生效。",
		fmt.Sprintf("语法：标题、列表、粗斜体、表格（GitHub 风格）、~~删除线~~，段内换行即换行；一行写 %s 另起一页；以 -> 开头、<- 结尾的段落居中（如 ->校运会<-）；图片写 ![说明](asset:名称)；原始 HTML 不会输出。", printstyle.PageBreakLine),
		fmt.Sprintf("标题 1–%d 个字，每个赛事至多 %d 页，正文至多 %d KB。", maxFrontPageTitle, maxFrontPages, printstyle.MaxMarkdownBytes>>10),
	}
	g.Assets = []string{
		"图片只能是 png、jpeg、webp，类型以内容为准；名称 1–" + fmt.Sprint(printstyle.MaxNameRunes) + " 个字，不含空格、引号、括号与 & % ; ,。",
		"引用写 asset:名称：模板或 Markdown 里 <img src=\"asset:校徽\"> / ![](asset:校徽)，CSS 里 url(asset:校徽)；系统把它换成内嵌图片。找不到的图片显示为空并在 warnings 里提示。",
		fmt.Sprintf("单张至多 %d MB，每个样式或赛事至多 %d 张、共 %d MB；同名保存即替换。", printstyle.MaxAssetBytes>>20, printstyle.MaxAssets, printstyle.MaxAssetsBytes>>20),
	}
	g.Sanitizing = []string{
		"写入的 CSS、模板输出、插槽、Markdown 都会被清洗，被删掉的内容在 warnings 里列出；文档另带 Content-Security-Policy，不会联网、不会执行脚本。",
		"CSS：删除 @import 与未知的 @ 规则（保留 media、supports、page、font-face、keyframes、layer、container、counter-style），url() 只允许内嵌的 png/jpeg/webp 图片、内嵌字体（data:）与 asset: 引用，删除 expression()、behavior、-moz-binding、javascript:。字体请用 data: 内嵌，不能引用网络字体。",
		"HTML：只保留版式元素（header footer main section article div span p br hr h1–h6 small strong b em i u s del sub sup mark pre code blockquote ul ol li dl dt dd table thead tbody tfoot tr th td caption colgroup col figure figcaption img）；删除 script、iframe、form、input、button、svg、video、audio、canvas、link、base 等；删除事件属性与所有外链，img 只能指向内嵌图片或 asset:；style 属性按 CSS 规则清洗。",
	}
	g.Limits = []string{
		fmt.Sprintf("CSS 至多 %d KB；单个模板 / 插槽至多 %d KB，运行至多 %d 秒或 %d 步（每进一次 range 循环体或调用一次子模板算一步）、输出至多 %d MB；printf 的宽度与精度至多 3 位数。", printstyle.MaxCSSBytes>>10, printstyle.MaxTemplateBytes>>10, int(printstyle.ExecTimeout.Seconds()), printstyle.MaxSteps, printstyle.MaxOutputBytes>>20),
		fmt.Sprintf("样式名称至多 %d 个字，至多 %d 个样式，每个样式保留最近 %d 个版本。", printstyle.MaxNameRunes, maxPrintStyles, maxPrintStyleVersions),
	}
	return g
}
