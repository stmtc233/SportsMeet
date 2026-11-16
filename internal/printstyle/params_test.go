package printstyle

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestDefaultParams(t *testing.T) {
	p := DefaultParams()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	css := p.CSS()
	for _, want := range []string{
		"--paper-w:210mm", "--paper-h:297mm", "--m-t:15mm", "--m-l:15mm", "--fs:9.5pt", "--lh:1.4", "--fs-title:18pt", "--fs-event:12.5pt",
		"--rule:1pt", "--rule-event:1.2pt", "--rule-mid:0.6pt", "--rule-thin:0.3pt", "--cell-line:0;", "@page{size:210mm 297mm;margin:15mm 15mm 15mm 15mm}",
		`"PingFang SC"`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS lacks %q:\n%s", want, css)
		}
	}
	paper, margin := p.DataAttrs()
	if paper != "210x297" || margin != "15,15,15,15" {
		t.Errorf("DataAttrs = %q %q", paper, margin)
	}
}

func TestPaperMM(t *testing.T) {
	for _, c := range []struct {
		name  string
		edit  func(*Params)
		w, h  float64
		paper string
	}{
		{"a3", func(p *Params) { p.Paper = PaperA3 }, 297, 420, "297x420"},
		{"a4 landscape", func(p *Params) { p.Orientation = Landscape }, 297, 210, "297x210"},
		{"16k", func(p *Params) { p.Paper = Paper16K }, 195, 270, "195x270"},
		{"b5", func(p *Params) { p.Paper = PaperB5 }, 176, 250, "176x250"},
		{"letter", func(p *Params) { p.Paper = PaperLetter }, 215.9, 279.4, "215.9x279.4"},
		{"custom", func(p *Params) { p.Paper, p.PaperWidth, p.PaperHeight = PaperCustom, 200, 300 }, 200, 300, "200x300"},
		{"custom landscape", func(p *Params) {
			p.Paper, p.PaperWidth, p.PaperHeight, p.Orientation = PaperCustom, 200, 300, Landscape
		}, 300, 200, "300x200"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := DefaultParams()
			c.edit(&p)
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			a := p.PaperMM()
			if a.Width != c.w || a.Height != c.h {
				t.Errorf("PaperMM = %v x %v", a.Width, a.Height)
			}
			if got, _ := p.DataAttrs(); got != c.paper {
				t.Errorf("data-paper = %q", got)
			}
		})
	}
}

func TestParamsGrid(t *testing.T) {
	p := DefaultParams()
	p.TableLines, p.LineWidth = LinesGrid, 0.5
	css := p.CSS()
	if !strings.Contains(css, "--cell-line:var(--rule-mid) solid #555") || !strings.Contains(css, "--rule:0.5pt") || !strings.Contains(css, "--rule-mid:0.3pt") {
		t.Errorf("CSS = %s", css)
	}
}

func TestParamsValidate(t *testing.T) {
	for name, edit := range map[string]func(*Params){
		"paper":           func(p *Params) { p.Paper = "a0" },
		"custom size":     func(p *Params) { p.Paper, p.PaperWidth = PaperCustom, 5 },
		"orientation":     func(p *Params) { p.Orientation = "diagonal" },
		"negative margin": func(p *Params) { p.MarginLeft = -1 },
		"huge margin":     func(p *Params) { p.MarginTop = 100 },
		"margins eat page": func(p *Params) {
			p.MarginLeft, p.MarginRight = 60, 60
			p.Paper, p.PaperWidth, p.PaperHeight = PaperCustom, 100, 200
		},
		"font":             func(p *Params) { p.Font = "comic" },
		"font size":        func(p *Params) { p.FontSize = 2 },
		"line height":      func(p *Params) { p.LineHeight = 5 },
		"title size":       func(p *Params) { p.TitleSize = 100 },
		"event title size": func(p *Params) { p.EventTitleSize = 1 },
		"table lines":      func(p *Params) { p.TableLines = "dots" },
		"line width":       func(p *Params) { p.LineWidth = 9 },
		"page number":      func(p *Params) { p.PageNumber = "roman" },
		"roster columns":   func(p *Params) { p.RosterColumns = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := DefaultParams()
			edit(&p)
			err := p.Validate()
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.ContainsAny(err.Error(), "纸方边字体行距号线页宽高") {
				t.Errorf("error is not in Chinese: %v", err)
			}
		})
	}
}

func TestParamsFromJSON(t *testing.T) {
	p, err := ParamsFromJSON([]byte(`{"paper":"a3","orientation":"landscape","showSeed":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Paper != PaperA3 || p.Orientation != Landscape || p.ShowSeed || !p.ShowUnit || p.FontSize != 9.5 {
		t.Errorf("params = %+v", p)
	}
	if _, err := ParamsFromJSON([]byte(`{"papr":"a3"}`)); err == nil {
		t.Error("an unknown field was accepted")
	}
	if _, err := ParamsFromJSON([]byte(`{"fontSize":99}`)); err == nil {
		t.Error("an invalid value was accepted")
	}
	if p, err := ParamsFromJSON(nil); err != nil || p != DefaultParams() {
		t.Errorf("empty = %+v %v", p, err)
	}
}

var update = flag.Bool("update", false, "rewrite the golden file")

// The UI tests read the default parameters' CSS from this file to build
// report documents that look as the app's do (tests/ui/reportDocument.ts).
func TestDefaultParamsCSSGolden(t *testing.T) {
	const path = "../../testdata/report/params-default.css"
	got := DefaultParams().CSS()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("DefaultParams().CSS() changed: run go test ./internal/printstyle -run TestDefaultParamsCSSGolden -update\n%s\n%s", got, want)
	}
}
