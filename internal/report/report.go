// Package report renders printable HTML: 秩序册 (start lists) and 成绩册
// (results). The page shows it in a frame and prints it from the WebView
// (docs/agents/frontend.md「打印」). Texts follow docs/agents/copywriting.md.
package report

import (
	"bytes"
	_ "embed"
	"html/template"
)

// Book is one meet's printable content.
type Book struct {
	Title    string // 秩序册 / 成绩册
	Meet     string
	Dates    string // 2026-11-12 or 2026-11-12 – 2026-11-13
	Location string
	Events   []Event
}

// Event is one item.
type Event struct {
	Name   string
	Date   string
	Timed  bool // lanes (道次) rather than an order (顺序)
	Rounds []Round
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
	Time  string // 09:30, with the date when it differs from the event's
	Lines []Line
}

// Line is one athlete.
type Line struct {
	Position int
	Rank     string
	Bib      string
	Name     string
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

//go:embed results.html
var resultsHTML string

var (
	startListTmpl = template.Must(template.New("startlist").Parse(startListHTML))
	resultsTmpl   = template.Must(template.New("results").Parse(resultsHTML))
)

type page struct {
	Book
	Style template.CSS
}

func render(t *template.Template, b Book) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, page{Book: b, Style: template.CSS(style)}); err != nil {
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
