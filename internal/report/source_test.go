package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

func TestBuiltinTemplateEveryKind(t *testing.T) {
	for _, kind := range printstyle.ReportKinds {
		src, err := BuiltinTemplate(kind)
		if err != nil || src.Source == "" || src.Title == "" || src.Title != KindTitle(kind) {
			t.Fatalf("%s: %+v, %v", kind, src, err)
		}
		if _, _, err := DataSchema(kind); err != nil {
			t.Fatal(err)
		}
		// What is offered runs: the built-in source is a valid custom template.
		if err := ValidateTemplate(kind, src.Source); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	if _, err := BuiltinTemplate("nope"); err == nil {
		t.Fatal("unknown kind accepted")
	}
	res, _ := BuiltinTemplate(KindResults)
	for _, name := range []string{"competitor", "bars", "teamScores"} {
		if !strings.Contains(res.Parts[name], "<") && name != "bars" {
			t.Fatalf("part %s = %q", name, res.Parts[name])
		}
		if res.Parts[name] == "" {
			t.Fatalf("part %s missing in %v", name, res.Parts)
		}
	}
	if ts := res.Parts["teamScores"]; !strings.HasPrefix(ts, `{{define "teamScores"}}`) || !strings.HasSuffix(ts, "{{end}}{{end}}") {
		t.Fatalf("teamScores block = %q", res.Parts["teamScores"])
	}
	check, _ := BuiltinTemplate(KindCheckIn)
	if len(check.Parts) != 0 || check.DataType != "CheckInBook" {
		t.Fatalf("checkin = %+v", check)
	}
}

func TestDataSchemaNamesFieldsAndMethods(t *testing.T) {
	root, types, err := DataSchema(KindStartList)
	if err != nil || root != "Book" {
		t.Fatalf("%s %v", root, err)
	}
	by := map[string]DataType{}
	for _, d := range types {
		by[d.Name] = d
	}
	for _, name := range []string{"Book", "Event", "Round", "Heat", "Line", "UnitRoster", "RosterGroup", "ScheduleDay", "ScheduleRow", "Combined", "TeamScoreTable"} {
		if _, ok := by[name]; !ok {
			t.Fatalf("type %s missing: %v", name, types)
		}
	}
	var slots, bib Field
	for _, f := range by["Book"].Fields {
		if f.Name == "Slots" {
			slots = f
		}
	}
	for _, f := range by["Line"].Fields {
		if f.Name == "Bib" {
			bib = f
		}
	}
	if slots.JSON != "" || slots.Type != "map[string]html" || bib.JSON != "bib" || bib.Type != "string" {
		t.Fatalf("fields: %+v %+v", slots, bib)
	}
	if !strings.Contains(strings.Join(by["Book"].Methods, " "), "RosterStyle() html") {
		t.Fatalf("methods: %v", by["Book"].Methods)
	}
	if !strings.Contains(strings.Join(by["RosterGroup"].Methods, " "), "RowsOf(int) [][]Line") {
		t.Fatalf("methods: %v", by["RosterGroup"].Methods)
	}
	if root, _, _ := DataSchema(KindRecordSheet); root != "RecordBook" {
		t.Fatal(root)
	}
	if _, _, err := DataSchema("nope"); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestPrepareIsWhatTheTemplateGets(t *testing.T) {
	p := printstyle.DefaultParams()
	p.ShowBib, p.RosterColumns = false, 3
	st := Style{Params: p}
	for _, tc := range []struct {
		kind  string
		book  any
		check func(map[string]any) bool
	}{
		{KindStartList, SampleBook(), func(m map[string]any) bool {
			return m["title"] == "秩序册" && m["events"] != nil && m["hideBib"] == true && m["rosterColumns"] == float64(3)
		}},
		{KindEntries, SampleBook(), func(m map[string]any) bool { return m["title"] == "报名表" && m["hideBib"] == true }},
		{KindResults, SampleBook(), func(m map[string]any) bool { return m["title"] == "成绩册" && m["teamOnly"] != true }},
		{KindTeamScores, SampleBook(), func(m map[string]any) bool { return m["teamOnly"] == true }},
		{KindCheckIn, SampleCheckInBook(), func(m map[string]any) bool { return m["title"] == "检录单" && m["sheets"] != nil }},
		{KindRecordSheet, SampleRecordBook(), func(m map[string]any) bool { return m["title"] == "成绩记录表" && m["sheets"] != nil }},
	} {
		v, err := Prepare(tc.kind, tc.book, st)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		raw, err := json.Marshal(v)
		var m map[string]any
		if err != nil || json.Unmarshal(raw, &m) != nil || !tc.check(m) {
			t.Fatalf("%s: %v %s", tc.kind, err, raw[:min(200, len(raw))])
		}
	}
	if _, err := Prepare(KindCheckIn, SampleBook(), st); err == nil {
		t.Fatal("book of the wrong type accepted")
	}
	if _, err := Prepare("nope", SampleBook(), st); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestSampleDataAndGeneratedLists(t *testing.T) {
	for _, kind := range printstyle.ReportKinds {
		raw, err := SampleData(kind)
		if err != nil || !json.Valid(raw) {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	vars := map[string]CSSVariable{}
	for _, v := range CSSVariables() {
		vars[v.Name] = v
	}
	for _, name := range []string{"--paper-w", "--m-t", "--font", "--fs", "--rule", "--cell-line"} {
		if v, ok := vars[name]; !ok || !v.FromParams || v.Default == "" {
			t.Fatalf("%s: %+v", name, v)
		}
	}
	for _, name := range []string{"--pos", "--bib", "--name", "--unit", "--seed"} {
		if v, ok := vars[name]; !ok || v.FromParams {
			t.Fatalf("%s: %+v", name, v)
		}
	}
	classes := map[string]bool{}
	for _, c := range Classes() {
		classes[c] = true
	}
	for _, c := range []string{"heat", "sheet", "break", "line", "unit", "event", "round", "cover", "front", "page-break", "center", "num", "bib", "mark", "entries", "entry", "report-group", "members"} {
		if !classes[c] {
			t.Errorf("class %s missing", c)
		}
	}
	for c := range classes {
		if strings.HasPrefix(c, "pv-") || strings.ContainsAny(c, "0123456789") && c[0] >= '0' && c[0] <= '9' {
			t.Errorf("class %s should not be listed", c)
		}
	}
}
