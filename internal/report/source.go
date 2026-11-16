package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

// What a template author needs to read: the built-in templates, the data
// they run over, and what the stylesheets offer. All of it comes from the
// code and files the reports are made of, so it cannot drift from them.

// kindTitles are the reports' names.
var kindTitles = map[string]string{
	KindEntries: "报名表", KindStartList: "秩序册", KindResults: "成绩册",
	KindTeamScores: "团体总分表", KindCheckIn: "检录单", KindRecordSheet: "成绩记录表",
}

// KindTitle is a report kind's printed name; "" for an unknown kind.
func KindTitle(kind string) string { return kindTitles[kind] }

// TemplateSource is a built-in report template.
type TemplateSource struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// DataType is the Go type the template runs over (Book, CheckInBook,
	// RecordBook); DataSchema describes it.
	DataType string `json:"dataType"`
	// Source is the template of the report's body. A custom template
	// replaces it whole, and takes over where the title block, the slots and
	// the printed-at footer go.
	Source string `json:"source"`
	// Parts are the named templates Source calls and a custom template may
	// call too ({{template "competitor" .}}), by the name they are defined with.
	Parts map[string]string `json:"parts"`
}

// BuiltinTemplate returns a kind's built-in template and the templates it
// calls. The 团体总分表 runs the 成绩册's template with TeamOnly set.
func BuiltinTemplate(kind string) (TemplateSource, error) {
	t := TemplateSource{Kind: kind, Title: kindTitles[kind], DataType: "Book", Parts: map[string]string{}}
	var shared []string
	switch kind {
	case KindEntries:
		t.Source, shared = entriesHTML, []string{partsHTML}
	case KindStartList:
		t.Source, shared = startListHTML, []string{partsHTML}
	case KindResults, KindTeamScores:
		t.Source, shared = resultsHTML, []string{teamScoresHTML, partsHTML}
	case KindCheckIn:
		t.Source, t.DataType = checkInHTML, "CheckInBook"
	case KindRecordSheet:
		t.Source, t.DataType = recordSheetHTML, "RecordBook"
	default:
		return TemplateSource{}, fmt.Errorf("没有名为「%s」的报表", kind)
	}
	for _, src := range shared {
		for name, body := range definitions(src) {
			t.Parts[name] = body
		}
	}
	return t, nil
}

// definitions are the {{define "name"}}…{{end}} blocks of a template file,
// name to the block as written.
func definitions(src string) map[string]string {
	out := map[string]string{}
	rest := src
	for {
		i := strings.Index(rest, `{{define "`)
		if i < 0 {
			return out
		}
		rest = rest[i:]
		nameEnd := strings.Index(rest[len(`{{define "`):], `"`)
		if nameEnd < 0 {
			return out
		}
		name := rest[len(`{{define "`) : len(`{{define "`)+nameEnd]
		// The block ends at its define's matching {{end}}: count the
		// actions that open one.
		depth, end := 0, -1
		for _, m := range actionRe.FindAllStringSubmatchIndex(rest, -1) {
			word := rest[m[2]:m[3]]
			switch word {
			case "define", "if", "range", "with", "block":
				depth++
			case "end":
				depth--
			}
			if depth == 0 {
				if close := strings.Index(rest[m[1]:], "}}"); close >= 0 {
					end = m[1] + close + 2
				}
				break
			}
		}
		if end < 0 {
			return out
		}
		out[name] = strings.TrimSpace(rest[:end])
		rest = rest[end:]
	}
}

// actionRe finds the first word of each action, to follow the blocks.
var actionRe = regexp.MustCompile(`\{\{-?\s*(define|if|range|with|block|end)\b`)

// Field is a field of a data type.
type Field struct {
	// Name is the Go name a template writes ({{.Meet}}); JSON is the key in
	// get_report_data's JSON ("" = not in the JSON).
	Name string `json:"name"`
	JSON string `json:"json,omitempty"`
	Type string `json:"type"`
}

// DataType is a struct a template reads.
type DataType struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
	// Methods are the methods a template may call on a value of the type,
	// with their results.
	Methods []string `json:"methods,omitempty"`
}

var (
	htmlType = reflect.TypeFor[template.HTML]()
	cssType  = reflect.TypeFor[template.CSS]()
)

func typeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Slice:
		return "[]" + typeName(t.Elem())
	case reflect.Map:
		return "map[" + typeName(t.Key()) + "]" + typeName(t.Elem())
	case reflect.Pointer:
		return "*" + typeName(t.Elem())
	}
	if t == htmlType || t == cssType {
		return "html"
	}
	return t.Name()
}

// DataSchema describes the data a report kind's template runs over: the
// root type's name and every struct reachable from it, in the order met.
func DataSchema(kind string) (root string, types []DataType, err error) {
	var rt reflect.Type
	switch kind {
	case KindEntries, KindStartList, KindResults, KindTeamScores:
		rt = reflect.TypeFor[Book]()
	case KindCheckIn:
		rt = reflect.TypeFor[CheckInBook]()
	case KindRecordSheet:
		rt = reflect.TypeFor[RecordBook]()
	default:
		return "", nil, fmt.Errorf("没有名为「%s」的报表", kind)
	}
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Slice || t.Kind() == reflect.Pointer || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] || t == htmlType {
			return
		}
		seen[t] = true
		dt := DataType{Name: t.Name()}
		for f := range t.Fields() {
			if !f.IsExported() {
				continue
			}
			tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if tag == "-" {
				tag = ""
			}
			dt.Fields = append(dt.Fields, Field{Name: f.Name, JSON: tag, Type: typeName(f.Type)})
		}
		for i := range t.NumMethod() {
			m := t.Method(i)
			sig := m.Name + "("
			var args []string
			for j := 1; j < m.Type.NumIn(); j++ {
				args = append(args, typeName(m.Type.In(j)))
			}
			sig += strings.Join(args, ", ") + ")"
			if m.Type.NumOut() > 0 {
				sig += " " + typeName(m.Type.Out(0))
			}
			dt.Methods = append(dt.Methods, sig)
		}
		types = append(types, dt)
		for f := range t.Fields() {
			if f.IsExported() {
				walk(f.Type)
			}
		}
	}
	walk(rt)
	return rt.Name(), types, nil
}

// SampleData is a kind's built-in sample data as the template sees it, as
// JSON: the way to learn the data of a meet that has none yet.
func SampleData(kind string) (json.RawMessage, error) {
	v, err := sampleFor(kind)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// ---- what the stylesheets offer ----

var (
	varDeclRe = regexp.MustCompile(`(--[a-z][a-z0-9-]*)\s*:\s*([^;}]+)`)
	varUseRe  = regexp.MustCompile(`var\((--[a-z][a-z0-9-]*)`)
	classRe   = regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_-]*)`)
	attrRe    = regexp.MustCompile(`class="([^"]*)"`)
	actionsRe = regexp.MustCompile(`\{\{.*?\}\}`)
	wordRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

// CSSVariable is a custom property the reports' CSS reads.
type CSSVariable struct {
	Name string `json:"name"`
	// Default is its value with the default style's parameters; "" for a
	// variable the report sets itself (inline, on a part of the page).
	Default string `json:"default"`
	// FromParams: the style's parameters set it; custom CSS may override it
	// in :root.
	FromParams bool `json:"fromParams"`
}

// CSSVariables lists the variables the parameters set (with their values
// for the default parameters) and those the report sets inline.
func CSSVariables() []CSSVariable {
	var out []CSSVariable
	seen := map[string]bool{}
	for _, m := range varDeclRe.FindAllStringSubmatch(printstyle.DefaultParams().CSS(), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, CSSVariable{Name: m[1], Default: strings.TrimSpace(m[2]), FromParams: true})
		}
	}
	var inline []string
	for _, css := range []string{baseCSS, defaultCSS} {
		for _, m := range varUseRe.FindAllStringSubmatch(css, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				inline = append(inline, m[1])
			}
		}
	}
	sort.Strings(inline)
	for _, n := range inline {
		out = append(out, CSSVariable{Name: n})
	}
	return out
}

// Classes are the class names of the reports: those the stylesheets style
// and those the templates write, without the preview's own (pv-).
func Classes() []string {
	set := map[string]bool{}
	for _, css := range []string{baseCSS, defaultCSS} {
		for _, m := range classRe.FindAllStringSubmatch(stripCSSNoise(css), -1) {
			set[m[1]] = true
		}
	}
	for _, src := range []string{partsHTML, startListHTML, entriesHTML, teamScoresHTML, resultsHTML, checkInHTML, recordSheetHTML} {
		// class="sheet{{if .Break}} break{{end}}": the words outside the actions.
		for _, m := range attrRe.FindAllStringSubmatch(src, -1) {
			for _, c := range strings.Fields(actionsRe.ReplaceAllString(m[1], " ")) {
				if wordRe.MatchString(c) {
					set[c] = true
				}
			}
		}
	}
	var out []string
	for c := range set {
		if !strings.HasPrefix(c, "pv-") {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

// stripCSSNoise removes comments and numbers like 1.4 or .5em that look
// like class selectors.
func stripCSSNoise(css string) string {
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	return regexp.MustCompile(`[0-9]\.[0-9]+|\.[0-9]+`).ReplaceAllString(css, "0")
}
