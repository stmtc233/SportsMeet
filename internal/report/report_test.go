package report

import (
	"strings"
	"testing"
)

func TestTextWidth(t *testing.T) {
	for _, c := range []struct {
		text string
		want float64
	}{
		{"", 0},
		{"吴语嫣", 3},
		{"初一（1）班", 5.62},
		{"11203", 3.1},
		{"6.55", 2.31},
		{"Ab", 1.25},
		{"买买提·艾力", 5.7},
	} {
		if got := textWidth(c.text, 0.62); got < c.want-1e-9 || got > c.want+1e-9 {
			t.Errorf("textWidth(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestFlowStyle(t *testing.T) {
	// Each part is as wide as its widest text in the list.
	got := string(flowStyle([]Line{
		{Position: 9, Bib: "101", Name: "王五", Unit: "初一（1）班"},
		{Position: 10, Bib: "11203", Name: "欧阳娜娜", Unit: "一班", Seed: "6.55"},
	}))
	for _, want := range []string{"--pos:1.34em", "--bib:3.10em", "--name:4.00em", "--unit:5.62em", "--seed:2.31em"} {
		if !strings.Contains(got, want) {
			t.Errorf("flowStyle = %s, lacks %s", got, want)
		}
	}
}
