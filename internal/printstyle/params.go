// Package printstyle is the compute-only half of print styles: the layout
// parameters and the cleaning, rendering and file format of what a style
// holds (custom CSS, templates, Markdown front pages, images). It knows
// nothing of the reports; internal/report applies a style to its books.
//
// Custom content is written by AI through MCP, so everything that reaches a
// report is cleaned (SanitizeCSS, SanitizeDocument, SanitizeFragment) and
// the document carries a Content-Security-Policy as a second line.
package printstyle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Paper sizes. Custom takes PaperWidth and PaperHeight.
const (
	PaperA4     = "a4"
	PaperA3     = "a3"
	PaperB5     = "b5"
	Paper16K    = "16k"
	PaperLetter = "letter"
	PaperCustom = "custom"
)

// Orientations.
const (
	Portrait  = "portrait"
	Landscape = "landscape"
)

// Body fonts. Each is a font stack with fallbacks for macOS, Windows and Linux.
const (
	FontSong = "song" // 宋体
	FontHei  = "hei"  // 黑体
	FontKai  = "kai"  // 楷体
	FontFang = "fang" // 仿宋
	FontSans = "sans" // 无衬线
)

// Table lines: horizontal rules only, or a frame around every cell.
const (
	LinesRules = "rules"
	LinesGrid  = "grid"
)

// Page number formats.
const (
	PageNOfM = "n_of_m" // 第 N 页 / 共 M 页
	PageDash = "dash"   // - N -
	PageNone = "none"
)

// Params are the layout knobs of a style. DefaultParams is the look the
// reports have without any style.
type Params struct {
	Paper       string  `json:"paper"`       // a4 / a3 / b5 / 16k / letter / custom
	PaperWidth  float64 `json:"paperWidth"`  // mm, custom only
	PaperHeight float64 `json:"paperHeight"` // mm, custom only
	Orientation string  `json:"orientation"` // portrait / landscape

	MarginTop    float64 `json:"marginTop"` // mm
	MarginRight  float64 `json:"marginRight"`
	MarginBottom float64 `json:"marginBottom"`
	MarginLeft   float64 `json:"marginLeft"`

	Font           string  `json:"font"`           // song / hei / kai / fang / sans
	FontSize       float64 `json:"fontSize"`       // body, pt
	LineHeight     float64 `json:"lineHeight"`     // times the font size
	TitleSize      float64 `json:"titleSize"`      // the meet's name at the top, pt
	EventTitleSize float64 `json:"eventTitleSize"` // an event's heading, pt

	TableLines string  `json:"tableLines"` // rules / grid
	LineWidth  float64 `json:"lineWidth"`  // the heavy rule, pt; lighter rules follow

	Header        bool   `json:"header"`        // running head from the second page
	PageNumber    string `json:"pageNumber"`    // n_of_m / dash / none
	RosterColumns int    `json:"rosterColumns"` // athletes a row of the 代表队名单

	ShowUnit bool `json:"showUnit"` // the 单位 column
	ShowSeed bool `json:"showSeed"` // the 报名成绩 column
	ShowBib  bool `json:"showBib"`  // the 号码 column
}

// DefaultParams matches what the reports looked like before styles existed.
func DefaultParams() Params {
	return Params{
		Paper: PaperA4, Orientation: Portrait,
		MarginTop: 15, MarginRight: 15, MarginBottom: 15, MarginLeft: 15,
		Font: FontSans, FontSize: 9.5, LineHeight: 1.4, TitleSize: 18, EventTitleSize: 12.5,
		TableLines: LinesRules, LineWidth: 1,
		Header: true, PageNumber: PageNOfM, RosterColumns: 4,
		ShowUnit: true, ShowSeed: true, ShowBib: true,
	}
}

// ParamsFromJSON reads Params over the defaults, so a document may name
// only the fields it changes. An unknown field is an error: a misspelt
// name would otherwise be silently ignored.
func ParamsFromJSON(data []byte) (Params, error) {
	p := DefaultParams()
	if len(bytes.TrimSpace(data)) == 0 {
		return p, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return DefaultParams(), fmt.Errorf("版面参数格式不对：%v", err)
	}
	return p, p.Validate()
}

var paperSizes = map[string][2]float64{
	PaperA4:     {210, 297},
	PaperA3:     {297, 420},
	PaperB5:     {176, 250},
	Paper16K:    {195, 270},
	PaperLetter: {215.9, 279.4},
}

var fontStacks = map[string]string{
	FontSans: `-apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif`,
	FontHei:  `"PingFang SC", "Heiti SC", "Microsoft YaHei", "SimHei", "Noto Sans CJK SC", "Noto Sans SC", "WenQuanYi Micro Hei", sans-serif`,
	FontSong: `"Songti SC", "STSong", "SimSun", "NSimSun", "Noto Serif CJK SC", "Noto Serif SC", "Source Han Serif SC", serif`,
	FontKai:  `"Kaiti SC", "STKaiti", "KaiTi", "KaiTi_GB2312", "AR PL UKai CN", "Noto Serif CJK SC", serif`,
	FontFang: `"STFangsong", "FangSong", "FangSong_GB2312", "Noto Serif CJK SC", serif`,
}

// Papers, Fonts and so on list the accepted values, for callers that
// describe them to a user.
var (
	Papers      = []string{PaperA4, PaperA3, PaperB5, Paper16K, PaperLetter, PaperCustom}
	Fonts       = []string{FontSong, FontHei, FontKai, FontFang, FontSans}
	PageNumbers = []string{PageNOfM, PageDash, PageNone}
)

func inRange(v, lo, hi float64) bool { return !math.IsNaN(v) && v >= lo && v <= hi }

// NumberLimit is the range of a numeric parameter, by its json name. The
// limits are Validate's, and the layout form and the MCP guide show the
// same ones: a range written down a second time would drift from this one.
type NumberLimit struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Unit    string  `json:"unit"`
	Integer bool    `json:"integer"`
}

// NumberLimits are the numeric parameters in the order Validate checks them.
var NumberLimits = []NumberLimit{
	{Key: "paperWidth", Label: "纸张宽", Min: 100, Max: 1000, Unit: "mm"},
	{Key: "paperHeight", Label: "纸张高", Min: 100, Max: 1000, Unit: "mm"},
	{Key: "marginTop", Label: "上边距", Min: 0, Max: 60, Unit: "mm"},
	{Key: "marginRight", Label: "右边距", Min: 0, Max: 60, Unit: "mm"},
	{Key: "marginBottom", Label: "下边距", Min: 0, Max: 60, Unit: "mm"},
	{Key: "marginLeft", Label: "左边距", Min: 0, Max: 60, Unit: "mm"},
	{Key: "fontSize", Label: "正文字号", Min: 6, Max: 24, Unit: "pt"},
	{Key: "lineHeight", Label: "行距", Min: 1, Max: 2.5, Unit: "倍"},
	{Key: "titleSize", Label: "一级标题字号", Min: 8, Max: 48, Unit: "pt"},
	{Key: "eventTitleSize", Label: "项目标题字号", Min: 8, Max: 36, Unit: "pt"},
	{Key: "lineWidth", Label: "线宽", Min: 0.1, Max: 3, Unit: "pt"},
	{Key: "rosterColumns", Label: "名单每行人数", Min: 1, Max: 8, Unit: "人", Integer: true},
}

// MinContentMM is the least the margins leave of the sheet, each way, in mm.
const MinContentMM = 50

// MarginsTooBig is Validate's message for margins that leave too little.
const MarginsTooBig = "边距太大，纸上放不下内容"

// PaperSizes are the named sheets, width and height in mm, portrait.
func PaperSizes() map[string][2]float64 {
	out := make(map[string][2]float64, len(paperSizes))
	for k, v := range paperSizes {
		out[k] = v
	}
	return out
}

// Limit is the range of the numeric parameter named key.
func Limit(key string) (NumberLimit, bool) {
	for _, l := range NumberLimits {
		if l.Key == key {
			return l, true
		}
	}
	return NumberLimit{}, false
}

// Check returns the problem with v, in Chinese, or nil.
func (l NumberLimit) Check(v float64) error {
	if l.Integer && v != math.Trunc(v) {
		return fmt.Errorf("%s需为整数", l.Label)
	}
	if inRange(v, l.Min, l.Max) {
		return nil
	}
	unit := l.Unit
	if unit == "mm" || unit == "pt" {
		unit = " " + unit
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	return fmt.Errorf("%s需在 %s–%s%s 之间", l.Label, num(l.Min), num(l.Max), unit)
}

func (p Params) number(key string) float64 {
	switch key {
	case "paperWidth":
		return p.PaperWidth
	case "paperHeight":
		return p.PaperHeight
	case "marginTop":
		return p.MarginTop
	case "marginRight":
		return p.MarginRight
	case "marginBottom":
		return p.MarginBottom
	case "marginLeft":
		return p.MarginLeft
	case "fontSize":
		return p.FontSize
	case "lineHeight":
		return p.LineHeight
	case "titleSize":
		return p.TitleSize
	case "eventTitleSize":
		return p.EventTitleSize
	case "lineWidth":
		return p.LineWidth
	case "rosterColumns":
		return float64(p.RosterColumns)
	}
	panic("printstyle: no number parameter " + key)
}

// Validate returns the first problem, in Chinese.
func (p Params) Validate() error {
	if _, ok := paperSizes[p.Paper]; !ok && p.Paper != PaperCustom {
		return fmt.Errorf("纸张「%s」不支持，可选 %s", p.Paper, strings.Join(Papers, "、"))
	}
	if p.Orientation != Portrait && p.Orientation != Landscape {
		return errors.New("方向只能是 portrait（纵向）或 landscape（横向）")
	}
	if _, ok := fontStacks[p.Font]; !ok {
		return fmt.Errorf("字体「%s」不支持，可选 %s", p.Font, strings.Join(Fonts, "、"))
	}
	if p.TableLines != LinesRules && p.TableLines != LinesGrid {
		return errors.New("表格线只能是 rules（只有横线）或 grid（全框线）")
	}
	if p.PageNumber != PageNOfM && p.PageNumber != PageDash && p.PageNumber != PageNone {
		return errors.New("页码格式只能是 n_of_m（第 N 页 / 共 M 页）、dash（- N -）或 none（不印）")
	}
	for _, l := range NumberLimits {
		if (l.Key == "paperWidth" || l.Key == "paperHeight") && p.Paper != PaperCustom {
			continue
		}
		if err := l.Check(p.number(l.Key)); err != nil {
			return err
		}
	}
	paper := p.PaperMM()
	if paper.Width-paper.Left-paper.Right < MinContentMM || paper.Height-paper.Top-paper.Bottom < MinContentMM {
		return errors.New(MarginsTooBig)
	}
	return nil
}

// Paper is the sheet as printed: its size with the orientation applied, and
// its margins, all in mm.
type Paper struct {
	Width, Height            float64
	Top, Right, Bottom, Left float64
}

// PaperMM is the sheet's size, orientation applied, and its margins.
func (p Params) PaperMM() Paper {
	w, h := p.PaperWidth, p.PaperHeight
	if size, ok := paperSizes[p.Paper]; ok {
		w, h = size[0], size[1]
	}
	if p.Orientation == Landscape {
		w, h = h, w
	}
	return Paper{Width: w, Height: h, Top: p.MarginTop, Right: p.MarginRight, Bottom: p.MarginBottom, Left: p.MarginLeft}
}

// num prints a number the way CSS wants it: no trailing zeros.
func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
}

// DataAttrs are the body attributes the preview reads (paginate.ts):
// data-paper "宽x高" and data-margin "上,右,下,左", in mm.
func (p Params) DataAttrs() (paper, margin string) {
	a := p.PaperMM()
	return num(a.Width) + "x" + num(a.Height), strings.Join([]string{num(a.Top), num(a.Right), num(a.Bottom), num(a.Left)}, ",")
}

// CSS is the style block of the parameters: variables the report's CSS
// reads, and @page. The look (default.css) and the page rules (base.css) in
// internal/report use these; a custom style may override any variable.
func (p Params) CSS() string {
	a := p.PaperMM()
	var b strings.Builder
	b.WriteString(":root{")
	fmt.Fprintf(&b, "--paper-w:%smm;--paper-h:%smm;", num(a.Width), num(a.Height))
	fmt.Fprintf(&b, "--m-t:%smm;--m-r:%smm;--m-b:%smm;--m-l:%smm;", num(a.Top), num(a.Right), num(a.Bottom), num(a.Left))
	fmt.Fprintf(&b, "--font:%s;--fs:%spt;--lh:%s;--fs-title:%spt;--fs-event:%spt;", fontStacks[p.Font], num(p.FontSize), num(p.LineHeight), num(p.TitleSize), num(p.EventTitleSize))
	w := p.LineWidth
	fmt.Fprintf(&b, "--rule:%spt;--rule-event:%spt;--rule-mid:%spt;--rule-thin:%spt;", num(w), num(w*1.2), num(w*.6), num(w*.3))
	if p.TableLines == LinesGrid {
		b.WriteString("--cell-line:var(--rule-mid) solid #555;--row-line:var(--rule-mid) solid #555;")
	} else {
		b.WriteString("--cell-line:0;--row-line:var(--rule-thin) solid #c8c8c8;")
	}
	b.WriteString("}\n")
	fmt.Fprintf(&b, "@page{size:%smm %smm;margin:%smm %smm %smm %smm}\n", num(a.Width), num(a.Height), num(a.Top), num(a.Right), num(a.Bottom), num(a.Left))
	return b.String()
}
