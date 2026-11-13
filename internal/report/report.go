// Package report renders printable HTML: 秩序册 (start lists), 成绩册
// (results), 检录单 (check-in sheets) and 成绩记录表 (blank record sheets). The page shows it in a frame and
// prints it from the WebView (docs/agents/frontend.md「打印」). Texts follow
// docs/agents/copywriting.md.
package report

import (
	"bytes"
	_ "embed"
	"html/template"
)

// Book is one meet's printable content.
type Book struct {
	GroupName  string
	GroupBooks []Book
	NewPage    bool   // Start each event on a new printed page.
	Title      string // 秩序册 / 成绩册
	Meet       string
	Dates      string // 2026-11-12 or 2026-11-12 – 2026-11-13
	Location   string
	PrintedAt  string        // 2026-11-12 09:31, local time
	Rosters    []UnitRoster  // 代表队名单, start lists only; empty = not printed
	Schedule   []ScheduleDay // 竞赛日程表, start lists only; empty = not printed
	Events     []Event
	Combineds  []Combined
	TeamScores *TeamScoreTable
	TeamOnly   bool
}

// UnitRoster is one unit of the 代表队名单.
type UnitRoster struct {
	Name   string
	Count  int // athletes in Groups
	Groups []RosterGroup
}

// RosterGroup is a unit's athletes of one gender, in the printed order.
type RosterGroup struct {
	Label    string // 男子 / 女子 / 其他; "" = no heading
	Athletes []Line // Bib and Name
}

// rosterColumns is how many athletes a row of the 代表队名单 holds.
const rosterColumns = 4

// Rows lays the athletes out in full rows; the last one is padded with
// empty lines so every row has the same cells.
func (g RosterGroup) Rows() [][]Line {
	var out [][]Line
	for start := 0; start < len(g.Athletes); start += rosterColumns {
		row := make([]Line, rosterColumns)
		copy(row, g.Athletes[start:min(start+rosterColumns, len(g.Athletes))])
		out = append(out, row)
	}
	return out
}

// ScheduleDay is one day of the 竞赛日程表.
type ScheduleDay struct {
	Day  string
	Rows []ScheduleRow
}

// ScheduleRow is one round in the 竞赛日程表.
type ScheduleRow struct {
	Time   string // 09:30
	Event  string
	Round  string
	Heats  int
	People int
}

// Event is one item.
type Event struct {
	Name          string
	Date          string
	Relay         bool
	RosterLabel   string
	PositionLabel string
	AttemptLabel  string
	Configuration string
	Rounds        []Round
}

// Round is one round. Ranked rounds print Standings across the heats;
// rounds that advance print each heat with its Q / q.
type Round struct {
	Label     string
	Advance   string // 各组前 2 名与其余成绩最好的 2 名晋级, "" = none
	Heats     []Heat
	Standings []Line
	ByHeat    bool
}

// Heat is one group.
type Heat struct {
	Label string
	Time  string // 09:30, with the date when it differs from the event's day
	// Started is when it actually started, shown as Time is; "" = not recorded.
	Started string
	// ResultsTime is when its results were last written, results books only
	// and when the settings ask for it; "" = not printed.
	ResultsTime string
	Lines       []Line
}

// Line is one athlete.
type Line struct {
	Position int
	Attempts []string
	Rank     string
	Bib      string
	Name     string
	Members  []string
	Unit     string
	Seed     string // entry mark, start lists
	Mark     string // mark, or the status label
	Qualify  string
	Heat     string // heat label, in standings
}

//go:embed style.css
var style string

//go:embed startlist.html
var startListHTML string

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
	startListTmpl   = template.Must(template.New("startlist").Parse(startListHTML))
	resultsTmpl     = template.Must(template.New("results").Parse(resultsHTML + teamScoresHTML))
	checkInTmpl     = template.Must(template.New("checkin").Parse(checkInHTML))
)

type page struct {
	Book
	Style template.CSS
}

func render(t *template.Template, b Book) (string, error) {
	css := style
	if b.NewPage {
		css += "\n.event ~ .event, .rosters + .event, .schedule + .event { break-before: page; page-break-before: always; }"
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, page{Book: b, Style: template.CSS(css)}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// StartList renders the 秩序册: every heat with its positions.
func StartList(b Book) (string, error) {
	b.Title = "秩序册"
	return render(startListTmpl, b)
}

// Results renders the 成绩册: every round's results and ranking.
func Results(b Book) (string, error) {
	b.Title = "成绩册"
	return render(resultsTmpl, b)
}

// CheckInBook is the 检录单: one sheet per heat.
type CheckInBook struct {
	Title     string
	Meet      string
	PrintedAt string
	// ShowCheckIn prints the check-in time; false when the settings' lead is 0.
	ShowCheckIn bool
	Sheets      []CheckInSheet
}

// CheckInSheet is one heat to check in.
type CheckInSheet struct {
	Break         bool // Start on a new page (.sheet.break in style.css).
	Event         string
	Round         string
	Heat          string
	Time          string // planned, with its date; "" = not planned
	CheckIn       string // 09:10, with the date when it differs from Time's; "" = none
	PositionLabel string // 道次 / 顺序
	Team          bool
	RosterLabel   string // 棒次 / 成员
	Lines         []Line
}

// CheckIn renders the 检录单.
func CheckIn(b CheckInBook) (string, error) {
	b.Title = "检录单"
	var buf bytes.Buffer
	if err := checkInTmpl.Execute(&buf, struct {
		CheckInBook
		Style template.CSS
	}{b, template.CSS(style)}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RecordBook is the 成绩记录表: one blank sheet per heat, filled in by hand.
type RecordBook struct {
	Title     string
	Meet      string
	PrintedAt string
	Sheets    []RecordSheet
}

// RecordSheet is one heat to judge. After the competitor it has Columns to
// fill in, then the mark and the rank.
type RecordSheet struct {
	Break         bool // Start on a new page (.sheet.break in style.css).
	Event         string
	Round         string
	Heat          string
	Time          string // planned, with its date; "" = not planned
	HeatWind      bool   // Leave a place for the heat's wind.
	PositionLabel string // 道次 / 顺序
	Team          bool
	RosterLabel   string // 棒次 / 成员
	// Columns are the headings of the cells before the mark: attempt
	// numbers, or "" for a heading written by hand.
	Columns []string
	// ColumnsTitle heads Columns as a group (高度); "" = no group heading.
	ColumnsTitle string
	// AttemptWind gives each competitor a second row for the wind of each attempt.
	AttemptWind bool
	MarkLabel   string // 成绩 / 最好成绩 / 次数 / 分数
	Lines       []Line
}

// HeadRows and LineRows are the row spans of the cells beside Columns.
func (s RecordSheet) HeadRows() int {
	if s.ColumnsTitle != "" {
		return 2
	}
	return 1
}

func (s RecordSheet) LineRows() int {
	if s.AttemptWind {
		return 2
	}
	return 1
}

// RecordSheets renders the 成绩记录表.
func RecordSheets(b RecordBook) (string, error) {
	b.Title = "成绩记录表"
	var buf bytes.Buffer
	if err := recordSheetTmpl.Execute(&buf, struct {
		RecordBook
		Style template.CSS
	}{b, template.CSS(style)}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type Combined struct {
	Name   string
	State  string
	Roster []Line
	Tables []CombinedTable
}
type CombinedTable struct {
	Label   string
	Headers []string
	Rows    []CombinedRow
}
type CombinedRow struct {
	Name  string
	Bib   string
	Unit  string
	Rank  string
	Total string
	Cells []string
}

type TeamScoreTable struct {
	State   string
	Scope   string
	Counted int
	Total   int
	Targets []string
	Rows    []TeamScoreRow
}
type TeamScoreRow struct{ Rank, Unit, Male, Female, Mixed, Total string }

func TeamScores(b Book) (string, error) {
	b.Title = "团体总分表"
	b.TeamOnly = true
	return render(resultsTmpl, b)
}
