// Package entryform writes and reads the 报名登记表: one xlsx per unit, in
// a fixed layout a class fills in by hand and the organizer imports as is
// (docs/agents/domain.md「报名登记表」). It computes only; the service
// decides what goes in and what a filled form means for the meet.
package entryform

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Sheet names and the labels the layout is read back by.
const (
	SheetAthletes = "个人项目"
	SheetTeams    = "接力与集体"
	LabelMeet     = "赛事"
	LabelUnit     = "参赛单位"
	LabelGroup    = "组别"
	// NoGroup is the 组别 of the form for the units that have no group.
	NoGroup    = "未分组"
	LabelEvent = "项目"
	Tick       = "√"
	// BlankRows is how many athlete rows get drop-downs; rows below work
	// the same without them.
	BlankRows = 60
)

// athleteColumns come before the event columns of the athlete sheet.
var athleteColumns = []string{"姓名", "性别", "学号"}

// ticks are what an event cell may hold to enter the athlete.
var ticks = map[string]bool{Tick: true, "✓": true, "✔": true}

// ErrNotForm says the file is not a 报名登记表.
var ErrNotForm = errors.New("不是报名登记表")

// Spec is what one form lists: one unit's, or with Units set, one group's,
// each row then naming its unit.
type Spec struct {
	Meet   string
	Unit   string
	Group  string     // the group of a group's form
	Units  []string   // the units of a group's form
	Note   string     // the meet's entry limits, as one line; "" = none
	Events []string   // individual events, by full name
	Teams  []TeamSpec // team events
}

// TeamSpec is one team event of the team sheet.
type TeamSpec struct {
	Event string
	Size  int // member cells of a row
	Count int // rows, one per team
}

// Form is a filled form as read.
type Form struct {
	Meet string `json:"meet"`
	Unit string `json:"unit"` // "" on a group's form
	// Group is the group of a group's form; "" = a unit's form.
	Group    string        `json:"group,omitempty"`
	Athletes []FormAthlete `json:"athletes"`
	Teams    []FormTeam    `json:"teams"`
}

// FormAthlete is one filled row of the athlete sheet.
type FormAthlete struct {
	Line          int      `json:"line"` // the sheet's row number
	Name          string   `json:"name"`
	Gender        string   `json:"gender"`
	StudentNumber string   `json:"studentNumber"`
	Unit          string   `json:"unit,omitempty"` // on a group's form
	Events        []string `json:"events"`         // ticked events, by full name
	// Problem is a cell the row cannot be read with; "" = none.
	Problem string `json:"problem,omitempty"`
}

// FormTeam is one filled row of the team sheet: members by name or student
// number, in roster (leg) order.
type FormTeam struct {
	Line    int      `json:"line"`
	Event   string   `json:"event"`
	Unit    string   `json:"unit,omitempty"` // on a group's form
	Members []string `json:"members"`
}

// Build writes a blank form.
func Build(spec Spec) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", SheetAthletes); err != nil {
		return nil, err
	}
	if err := buildAthletes(f, spec); err != nil {
		return nil, err
	}
	if len(spec.Teams) > 0 {
		if err := buildTeams(f, spec); err != nil {
			return nil, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type styles struct{ label, head, cell, tick, off int }

func newStyles(f *excelize.File) (styles, error) {
	var st styles
	border := []excelize.Border{{Type: "left", Color: "BFBFBF", Style: 1}, {Type: "right", Color: "BFBFBF", Style: 1}, {Type: "top", Color: "BFBFBF", Style: 1}, {Type: "bottom", Color: "BFBFBF", Style: 1}}
	center := &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}
	for _, s := range []struct {
		dst   *int
		style excelize.Style
	}{
		{&st.label, excelize.Style{Font: &excelize.Font{Bold: true}}},
		{&st.head, excelize.Style{Font: &excelize.Font{Bold: true}, Border: border, Alignment: center, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EDEDED"}}}},
		{&st.cell, excelize.Style{Border: border}},
		{&st.tick, excelize.Style{Border: border, Alignment: center}},
		{&st.off, excelize.Style{Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9D9D9"}}}},
	} {
		id, err := f.NewStyle(&s.style)
		if err != nil {
			return st, err
		}
		*s.dst = id
	}
	return st, nil
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// widthOf sizes a column to its header, CJK characters counting double.
func widthOf(text string, least float64) float64 {
	w := 0.0
	for _, r := range text {
		if utf8.RuneLen(r) > 1 {
			w += 2
		} else {
			w++
		}
	}
	return max(least, w+2)
}

// buildAthletes lays out the athlete sheet: the meet and unit, the header
// row, then blank rows whose gender and event cells drop down.
func buildAthletes(f *excelize.File, spec Spec) error {
	sh := SheetAthletes
	st, err := newStyles(f)
	if err != nil {
		return err
	}
	columns := athleteColumns
	meta := []any{LabelUnit, spec.Unit}
	if len(spec.Units) > 0 {
		columns = append(append([]string{}, athleteColumns...), LabelUnit)
		meta = []any{LabelGroup, spec.Group}
	}
	head := append(append([]string{}, columns...), spec.Events...)
	last := len(head)
	const headRow = 3
	steps := []func() error{
		func() error { return f.SetSheetRow(sh, "A1", &[]any{LabelMeet, spec.Meet}) },
		func() error { return f.SetSheetRow(sh, "A2", &meta) },
		func() error { return f.SetCellStyle(sh, "A1", "A2", st.label) },
		func() error { return f.MergeCell(sh, "B1", "C1") },
		func() error { return f.MergeCell(sh, "B2", "C2") },
		func() error { return f.SetSheetRow(sh, cellName(1, headRow), &head) },
		func() error { return f.SetCellStyle(sh, cellName(1, headRow), cellName(last, headRow), st.head) },
		func() error { return f.SetRowHeight(sh, headRow, 36) },
		func() error {
			return f.SetCellStyle(sh, cellName(1, headRow+1), cellName(len(columns), headRow+BlankRows), st.cell)
		},
		func() error { return f.SetColWidth(sh, "A", "A", 12) },
		func() error { return f.SetColWidth(sh, "B", "B", 7) },
		func() error { return f.SetColWidth(sh, "C", "C", 14) },
		func() error {
			return f.SetPanes(sh, &excelize.Panes{Freeze: true, XSplit: len(columns), YSplit: headRow,
				TopLeftCell: cellName(len(columns)+1, headRow+1), ActivePane: "bottomRight"})
		},
		func() error {
			return addList(f, sh, cellName(2, headRow+1)+":"+cellName(2, headRow+BlankRows), []string{"男", "女"})
		},
	}
	if len(spec.Units) > 0 {
		at := len(columns)
		col, _ := excelize.ColumnNumberToName(at)
		width := 12.0
		for _, u := range spec.Units {
			width = max(width, widthOf(u, 12))
		}
		steps = append(steps,
			func() error { return f.SetColWidth(sh, col, col, min(width, 24)) },
			func() error {
				return addList(f, sh, cellName(at, headRow+1)+":"+cellName(at, headRow+BlankRows), spec.Units)
			})
	}
	if spec.Note != "" {
		steps = append(steps, func() error { return f.SetCellValue(sh, "E1", spec.Note) })
	}
	if len(spec.Events) > 0 {
		first := len(columns) + 1
		steps = append(steps,
			func() error {
				return f.SetCellStyle(sh, cellName(first, headRow+1), cellName(last, headRow+BlankRows), st.tick)
			},
			func() error {
				return addList(f, sh, cellName(first, headRow+1)+":"+cellName(last, headRow+BlankRows), []string{Tick})
			})
		for i, name := range spec.Events {
			col, _ := excelize.ColumnNumberToName(first + i)
			steps = append(steps, func() error { return f.SetColWidth(sh, col, col, min(widthOf(name, 8), 16)) })
		}
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// buildTeams lays out the team sheet: a row per team, numbered member
// cells, and the cells past an event's size greyed out.
func buildTeams(f *excelize.File, spec Spec) error {
	sh, teams := SheetTeams, spec.Teams
	if _, err := f.NewSheet(sh); err != nil {
		return err
	}
	st, err := newStyles(f)
	if err != nil {
		return err
	}
	size := 1
	for _, t := range teams {
		size = max(size, t.Size)
	}
	// A group's form has a unit column, and a row per unit of each team.
	lead := 1
	head := []string{LabelEvent}
	units := []string{""}
	if len(spec.Units) > 0 {
		lead, head, units = 2, append(head, LabelUnit), spec.Units
	}
	for i := 1; i <= size; i++ {
		head = append(head, strconv.Itoa(i))
	}
	last := lead + size
	if err := f.SetSheetRow(sh, "A1", &head); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "A1", cellName(last, 1), st.head); err != nil {
		return err
	}
	width, unitWidth := 12.0, 12.0
	row := 2
	for _, t := range teams {
		width = max(width, widthOf(t.Event, 12))
		for _, u := range units {
			unitWidth = max(unitWidth, widthOf(u, 12))
			for range max(1, t.Count) {
				values := []any{t.Event}
				if lead == 2 {
					values = append(values, u)
				}
				if err := f.SetSheetRow(sh, cellName(1, row), &values); err != nil {
					return err
				}
				if err := f.SetCellStyle(sh, cellName(1, row), cellName(last, row), st.cell); err != nil {
					return err
				}
				if t.Size < size {
					if err := f.SetCellStyle(sh, cellName(lead+t.Size+1, row), cellName(last, row), st.off); err != nil {
						return err
					}
				}
				row++
			}
		}
	}
	if err := f.SetColWidth(sh, "A", "A", width); err != nil {
		return err
	}
	if lead == 2 {
		if err := f.SetColWidth(sh, "B", "B", min(unitWidth, 24)); err != nil {
			return err
		}
	}
	first, _ := excelize.ColumnNumberToName(lead + 1)
	end, _ := excelize.ColumnNumberToName(last)
	if err := f.SetColWidth(sh, first, end, 10); err != nil {
		return err
	}
	return f.SetPanes(sh, &excelize.Panes{Freeze: true, XSplit: lead, YSplit: 1, TopLeftCell: cellName(lead+1, 2), ActivePane: "bottomRight"})
}

// addList gives the cells a drop-down of keys. A list too long for the
// validation itself lives in a hidden sheet the drop-down refers to.
func addList(f *excelize.File, sheet, sqref string, keys []string) error {
	dv := excelize.NewDataValidation(true)
	dv.Sqref = sqref
	if err := dv.SetDropList(keys); errors.Is(err, excelize.ErrDataValidationFormulaLength) {
		if err := addListSheet(f, keys); err != nil {
			return err
		}
		dv.SetSqrefDropList(fmt.Sprintf("'%s'!$A$1:$A$%d", sheetLists, len(keys)))
	} else if err != nil {
		return err
	}
	dv.SetError(excelize.DataValidationErrorStyleStop, "", "请从下拉列表中选择")
	return f.AddDataValidation(sheet, dv)
}

// sheetLists holds the long drop-down lists; Parse never reads it.
const sheetLists = "列表"

func addListSheet(f *excelize.File, keys []string) error {
	if _, err := f.NewSheet(sheetLists); err != nil {
		return err
	}
	for i, k := range keys {
		if err := f.SetCellValue(sheetLists, cellName(1, i+1), k); err != nil {
			return err
		}
	}
	return f.SetSheetVisible(sheetLists, false)
}

// Parse reads a filled form. Rows left blank are skipped; a row with a
// cell it cannot read keeps the cell in Problem.
func Parse(data []byte) (*Form, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotForm
	}
	defer f.Close()
	rows, err := f.GetRows(SheetAthletes)
	if err != nil {
		return nil, ErrNotForm
	}
	form := &Form{Athletes: []FormAthlete{}, Teams: []FormTeam{}}
	head := -1
	for i, row := range rows {
		if len(row) > 0 && clean(row[0]) == athleteColumns[0] {
			head = i
			break
		}
		for j, c := range row {
			next := ""
			for _, v := range row[j+1:] {
				if v = clean(v); v != "" {
					next = v
					break
				}
			}
			switch clean(c) {
			case LabelMeet:
				form.Meet = next
			case LabelUnit:
				form.Unit = next
			case LabelGroup:
				form.Group = next
			}
		}
	}
	if head < 0 {
		return nil, ErrNotForm
	}
	columns := map[string]int{}
	type eventCol struct {
		index int
		name  string
	}
	var events []eventCol
	for j, c := range rows[head] {
		c = clean(c)
		switch {
		case c == "":
		case slices.Contains(athleteColumns, c) || c == LabelUnit:
			columns[c] = j
		default:
			events = append(events, eventCol{j, c})
		}
	}
	for i, row := range rows[head+1:] {
		at := func(j int) string {
			if j < len(row) {
				return clean(row[j])
			}
			return ""
		}
		if allBlank(row) {
			continue
		}
		a := FormAthlete{Line: head + i + 2, Events: []string{}}
		if j, ok := columns["姓名"]; ok {
			a.Name = at(j)
		}
		if j, ok := columns["性别"]; ok {
			a.Gender = at(j)
		}
		if j, ok := columns["学号"]; ok {
			a.StudentNumber = at(j)
		}
		if j, ok := columns[LabelUnit]; ok {
			a.Unit = at(j)
		}
		for _, e := range events {
			switch v := at(e.index); {
			case v == "":
			case ticks[v]:
				a.Events = append(a.Events, e.name)
			case a.Problem == "":
				a.Problem = fmt.Sprintf("「%s」只能填 %s", e.name, Tick)
			}
		}
		form.Athletes = append(form.Athletes, a)
	}
	if teams, err := f.GetRows(SheetTeams); err == nil {
		form.Teams = parseTeams(teams)
	}
	return form, nil
}

// parseTeams reads the team sheet; rows with no member are skipped.
func parseTeams(rows [][]string) []FormTeam {
	out := []FormTeam{}
	lead := 1 // cells before the members
	if len(rows) > 0 && len(rows[0]) > 1 && clean(rows[0][1]) == LabelUnit {
		lead = 2
	}
	for i, row := range rows {
		if len(row) == 0 || (i == 0 && clean(row[0]) == LabelEvent) {
			continue
		}
		t := FormTeam{Line: i + 1, Event: clean(row[0]), Members: []string{}}
		if lead == 2 && len(row) > 1 {
			t.Unit = clean(row[1])
		}
		if len(row) <= lead {
			continue
		}
		for _, c := range row[lead:] {
			if c = clean(c); c != "" {
				t.Members = append(t.Members, c)
			}
		}
		if len(t.Members) > 0 {
			out = append(out, t)
		}
	}
	return out
}

func clean(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, " ", " ")) }

func allBlank(row []string) bool {
	for _, c := range row {
		if clean(c) != "" {
			return false
		}
	}
	return true
}
