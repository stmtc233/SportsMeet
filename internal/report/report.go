// Package report renders printable HTML: 秩序册 (start lists), 成绩册
// (results), 报名表 (entries), 检录单 (check-in sheets) and 成绩记录表 (blank record sheets). The page shows it in a frame and
// prints it from the WebView (docs/agents/frontend.md「打印」). Texts follow
// docs/agents/copywriting.md.
//
// The books below are the data every report is made of. They are also the
// data custom templates are written against (internal/printstyle), so their
// fields and json names are an interface: DataVersion changes when one
// does, and testdata/report/data-v1.json pins them.
package report

import (
	"fmt"
	"html/template"
	"strconv"
)

// DataVersion is the version of the report data (the books below). A
// custom template is written for one version and is not used with another.
const DataVersion = 1

// Book is one meet's printable content.
type Book struct {
	GroupName  string `json:"groupName"`
	GroupBooks []Book `json:"groupBooks"`
	NewPage    bool   `json:"newPage"` // Start each event on a new printed page.
	// Compact lays a 秩序册's lists out side by side: the athletes of an
	// individual event's heat, of a 全能's roster and of the 代表队名单 go as
	// many a row as fit instead of one a table row. Teams keep their table.
	Compact    bool            `json:"compact"`
	Title      string          `json:"title"` // 报名表 / 秩序册 / 成绩册 / 团体总分表
	Meet       string          `json:"meet"`
	Dates      string          `json:"dates"` // 2026-11-12 or 2026-11-12 – 2026-11-13
	Location   string          `json:"location"`
	PrintedAt  string          `json:"printedAt"` // 2026-11-12 09:31, local time
	Rosters    []UnitRoster    `json:"rosters"`   // 代表队名单, start lists only; empty = not printed
	Schedule   []ScheduleDay   `json:"schedule"`  // 竞赛日程表, start lists only; empty = not printed
	Events     []Event         `json:"events"`
	Combineds  []Combined      `json:"combineds"`
	TeamScores *TeamScoreTable `json:"teamScores"`
	TeamOnly   bool            `json:"teamOnly"`

	// The columns a style may leave out of the tables that read: 号码, 单位
	// and 报名成绩 (render sets them from the style's Params; the zero
	// value prints them all), and how many athletes a row of the 代表队名单
	// holds (0 = 4).
	HideBib       bool `json:"hideBib"`
	HideUnit      bool `json:"hideUnit"`
	HideSeed      bool `json:"hideSeed"`
	RosterColumns int  `json:"rosterColumns"`

	// Slots are the custom pieces a style puts in the report, by slot name
	// (printstyle.SlotNames), already made from this data. Templates print
	// them where they belong: {{.Slots.before_rosters}}.
	Slots map[string]template.HTML `json:"-"`
}

// UnitRoster is one unit of the 代表队名单.
type UnitRoster struct {
	Name   string        `json:"name"`
	Count  int           `json:"count"` // athletes in Groups
	Bibs   string        `json:"bibs"`  // 11101–11208, the lowest and highest bib; "" = none
	Groups []RosterGroup `json:"groups"`
}

// RosterGroup is a unit's athletes of one gender, in the printed order.
type RosterGroup struct {
	Label    string `json:"label"`    // 男子 / 女子 / 其他; "" = no heading
	Athletes []Line `json:"athletes"` // Bib and Name
}

// rosterColumns is how many athletes a row of the 代表队名单 holds unless the style says otherwise.
const rosterColumns = 4

// maxRosterColumns bounds RowsOf, far above any sheet's width.
const maxRosterColumns = 100

// Rows lays the athletes out in full rows of the default width.
func (g RosterGroup) Rows() [][]Line { return g.RowsOf(rosterColumns) }

// RowsOf lays the athletes out in full rows of n; the last one is padded
// with empty lines so every row has the same cells. n < 1 reads as the
// default and n is at most maxRosterColumns: a custom template passes n,
// and a row is allocated n wide before anything is written.
func (g RosterGroup) RowsOf(n int) [][]Line {
	if n < 1 {
		n = rosterColumns
	}
	n = min(n, maxRosterColumns)
	var out [][]Line
	for start := 0; start < len(g.Athletes); start += n {
		row := make([]Line, n)
		copy(row, g.Athletes[start:min(start+n, len(g.Athletes))])
		out = append(out, row)
	}
	return out
}

// RosterStyle lines up the compact 代表队名单 across all its units.
func (b Book) RosterStyle() template.CSS {
	var lines []Line
	for _, u := range b.Rosters {
		for _, g := range u.Groups {
			lines = append(lines, g.Athletes...)
		}
	}
	return flowStyle(lines)
}

// flowStyle sets the least widths of a compact list's parts (default.css
// .entry): each the widest of its texts in the list, in its own em, so the
// parts line up across rows and heats. A text wider than its estimate
// widens its own entry instead of running into the next one.
func flowStyle(lines []Line) template.CSS {
	var pos, bib, name, unit, seed float64
	for _, l := range lines {
		pos = max(pos, textWidth(strconv.Itoa(l.Position), 0.67))
		bib = max(bib, textWidth(l.Bib, 0.62))
		name = max(name, textWidth(l.Name, 0.62))
		unit = max(unit, textWidth(l.Unit, 0.62))
		seed = max(seed, textWidth(l.Seed, 0.62))
	}
	return template.CSS(fmt.Sprintf("--pos:%.2fem;--bib:%.2fem;--name:%.2fem;--unit:%.2fem;--seed:%.2fem", pos, bib, name, unit, seed))
}

// textWidth estimates how wide a text prints, in em of its own font size,
// from the report's fonts (default.css body): Han and full-width forms are
// square, figures (figure em each) and Latin letters narrower. A little
// over the measured widths, for fonts with wider figures and letters.
func textWidth(s string, figure float64) float64 {
	w := 0.0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			w += figure
		case r >= 'A' && r <= 'Z':
			w += 0.7
		case r >= 'a' && r <= 'z':
			w += 0.55
		case r < 0x80:
			w += 0.45
		case r < 0x2E80: // Latin with accents, ·, ×
			w += 0.7
		default: // CJK, kana, full-width forms
			w++
		}
	}
	return w
}

// ScheduleDay is one day of the 竞赛日程表.
type ScheduleDay struct {
	Day  string        `json:"day"`
	Rows []ScheduleRow `json:"rows"`
}

// ScheduleRow is one round in the 竞赛日程表.
type ScheduleRow struct {
	No     int    `json:"no"`   // across the whole schedule, from 1
	Time   string `json:"time"` // 09:30
	Kind   string `json:"kind"` // 径赛 / 田赛 / 计次 / 计分
	Event  string `json:"event"`
	Round  string `json:"round"`
	Heats  int    `json:"heats"`
	People int    `json:"people"`
}

// Event is one item.
type Event struct {
	Entries       []Line   `json:"entries"` // 报名表, independent of arranged positions
	Count         string   `json:"count"`   // entry count, in 人 or 队
	Name          string   `json:"name"`
	Meta          string   `json:"meta"`          // 2026-11-12 周四 · 分道: its day and configuration
	Team          bool     `json:"team"`          // Lines are teams: no bib column, members listed
	NameLabel     string   `json:"nameLabel"`     // 姓名 / 队伍 / 棒次 / 棒次 …
	PositionLabel string   `json:"positionLabel"` // 道次 / 顺序
	Seeded        bool     `json:"seeded"`        // some line has an entry mark (start lists)
	TryHeads      []string `json:"tryHeads"`      // 第1次 …, one column per attempt; empty = none
	Bars          bool     `json:"bars"`          // height trials (Line.Bars) are printed
	// Wide lays the heats one per row: their tables have too many columns,
	// or team members, for half the page.
	Wide   bool    `json:"wide"`
	Rounds []Round `json:"rounds"`
}

// FlowStyle lines up a compact event's athletes across all its heats.
func (e Event) FlowStyle() template.CSS {
	var lines []Line
	for _, r := range e.Rounds {
		for _, h := range r.Heats {
			lines = append(lines, h.Lines...)
		}
	}
	return flowStyle(lines)
}

// Round is one round. Ranked rounds print Standings across the heats;
// rounds that advance print each heat with its Q / q.
type Round struct {
	Label     string `json:"label"`
	Advance   string `json:"advance"` // 各组前 2 名与其余成绩最好的 2 名晋级, "" = none
	Heats     []Heat `json:"heats"`
	Standings []Line `json:"standings"`
	ByHeat    bool   `json:"byHeat"`
}

// Heat is one group.
type Heat struct {
	Label string `json:"label"`
	Time  string `json:"time"`  // 09:30, with the date when it differs from the event's day
	Count string `json:"count"` // 6 人 / 3 队, start lists
	// Started is when it actually started, shown as Time is; "" = not recorded.
	Started string `json:"started"`
	// ResultsTime is when its results were last written, results books only
	// and when the settings ask for it; "" = not printed.
	ResultsTime string `json:"resultsTime"`
	Lines       []Line `json:"lines"`
}

// Line is one athlete.
type Line struct {
	Position int      `json:"position"`
	Attempts []string `json:"attempts"` // one per Event.TryHeads, "" when not taken
	Best     int      `json:"best"`     // index of the attempt that is the mark, -1 = none
	Bars     []string `json:"bars"`     // height trials, one per bar: 1.10 XO
	Rank     string   `json:"rank"`
	Bib      string   `json:"bib"`
	Name     string   `json:"name"` // a team's name, "" when it tells nothing (Unit says it)
	Members  []string `json:"members"`
	Unit     string   `json:"unit"`
	Seed     string   `json:"seed"` // entry mark, start lists
	Mark     string   `json:"mark"` // mark, or the status label
	Qualify  string   `json:"qualify"`
	Heat     string   `json:"heat"` // heat label, in standings
}

// CheckInBook is the 检录单: one sheet per heat.
type CheckInBook struct {
	Title     string `json:"title"`
	Meet      string `json:"meet"`
	Dates     string `json:"dates"` // as in Book
	Location  string `json:"location"`
	PrintedAt string `json:"printedAt"`
	// ShowCheckIn prints the check-in time; false when the settings' lead is 0.
	ShowCheckIn bool           `json:"showCheckIn"`
	Sheets      []CheckInSheet `json:"sheets"`
	// Slots as in Book.
	Slots map[string]template.HTML `json:"-"`
}

// CheckInSheet is one heat to check in.
type CheckInSheet struct {
	Break         bool   `json:"break"` // Start on a new page (.sheet.break in base.css).
	Event         string `json:"event"`
	Round         string `json:"round"`
	From          string `json:"from"`          // the round its athletes advanced from; "" = first round
	Heat          string `json:"heat"`          //
	Time          string `json:"time"`          // planned, with its date; "" = not planned
	CheckIn       string `json:"checkIn"`       // 09:10, with the date when it differs from Time's; "" = none
	PositionLabel string `json:"positionLabel"` // 道次 / 顺序
	Team          bool   `json:"team"`
	NameLabel     string `json:"nameLabel"` // 姓名 / 棒次 / 队伍 / 棒次 …
	Lines         []Line `json:"lines"`
}

// RecordBook is the 成绩记录表: one blank sheet per heat, filled in by hand.
type RecordBook struct {
	Title     string        `json:"title"`
	Meet      string        `json:"meet"`
	Dates     string        `json:"dates"` // as in Book
	Location  string        `json:"location"`
	PrintedAt string        `json:"printedAt"`
	Sheets    []RecordSheet `json:"sheets"`
	// Slots as in Book.
	Slots map[string]template.HTML `json:"-"`
}

// RecordSheet is one heat to judge. After the competitor it has Columns to
// fill in, then the mark and the rank.
type RecordSheet struct {
	Break         bool   `json:"break"` // Start on a new page (.sheet.break in base.css).
	Event         string `json:"event"`
	Round         string `json:"round"`
	Heat          string `json:"heat"`
	Time          string `json:"time"`          // planned, with its date; "" = not planned
	HeatWind      bool   `json:"heatWind"`      // Leave a place for the heat's wind.
	PositionLabel string `json:"positionLabel"` // 道次 / 顺序
	Team          bool   `json:"team"`
	NameLabel     string `json:"nameLabel"` // 姓名 / 棒次 / 队伍 / 棒次 …
	// Columns are the headings of the cells before the mark: attempt
	// numbers, or "" for a heading written by hand.
	Columns []string `json:"columns"`
	// ColumnsTitle heads Columns as a group (高度); "" = no group heading.
	ColumnsTitle string `json:"columnsTitle"`
	// AttemptWind gives each competitor a second row for the wind of each attempt.
	AttemptWind bool   `json:"attemptWind"`
	MarkLabel   string `json:"markLabel"` // 成绩 / 最好成绩 / 次数 / 分数
	Lines       []Line `json:"lines"`
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

// Combined is a 全能: its components and roster (start lists), or its
// standings in tables of at most four components (results books).
type Combined struct {
	Name       string          `json:"name"` // with its gender and group: 男子三项全能 初一组
	State      string          `json:"state"`
	Components []CombinedPart  `json:"components"`
	Roster     []Line          `json:"roster"`
	Tables     []CombinedTable `json:"tables"`
}

// FlowStyle lines up a compact roster.
func (c Combined) FlowStyle() template.CSS { return flowStyle(c.Roster) }

// CombinedPart is a component in the start list: its order and first time.
type CombinedPart struct {
	No   int    `json:"no"`
	Name string `json:"name"`
	Time string `json:"time"` // 2026-11-12 08:30; "" = not planned
}

type CombinedTable struct {
	Label   string        `json:"label"`
	Headers []string      `json:"headers"`
	Rows    []CombinedRow `json:"rows"`
}
type CombinedRow struct {
	Name  string         `json:"name"`
	Bib   string         `json:"bib"`
	Unit  string         `json:"unit"`
	Rank  string         `json:"rank"`
	Total string         `json:"total"`
	Cells []CombinedCell `json:"cells"`
}

// CombinedCell is a component's mark and the points it earns.
type CombinedCell struct {
	Mark   string `json:"mark"`   // 待录 when pending
	Points string `json:"points"` // "" when pending; 不列名次 when excluded
}

type TeamScoreTable struct {
	State   string   `json:"state"`
	Scope   string   `json:"scope"`
	Counted int      `json:"counted"`
	Total   int      `json:"total"`
	Targets []string `json:"targets"`
	Mixed   bool     `json:"mixed"` // some unit has mixed points; otherwise the column is left out
	// Medals is the 奖牌榜: units are ranked by medals and the rows carry
	// Gold, Silver, Bronze and Medals instead of the points columns.
	Medals bool `json:"medals"`
	// Standalone is the 团体总分表 printed on its own: no heading of its own.
	Standalone bool           `json:"standalone"`
	Rows       []TeamScoreRow `json:"rows"`
}
type TeamScoreRow struct {
	Rank   string `json:"rank"`
	Unit   string `json:"unit"`
	Male   string `json:"male"`
	Female string `json:"female"`
	Mixed  string `json:"mixed"`
	Total  string `json:"total"`
	Gold   string `json:"gold"`
	Silver string `json:"silver"`
	Bronze string `json:"bronze"`
	Medals string `json:"medals"` // medal total
}
