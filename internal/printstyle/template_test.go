package printstyle

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type tdata struct {
	Meet  string
	Names []string
}

func TestTemplateExecute(t *testing.T) {
	tpl, err := CompileTemplate(`<h1>{{.Meet}}</h1>{{range $i, $n := .Names}}<p>{{add $i 1}} {{$n}}</p>{{end}}{{template "part" .}}`, `{{define "part"}}<i>{{upper "x"}}</i>{{end}}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.Execute(tdata{Meet: "<运动会>", Names: []string{"张三", "李四"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != `<h1>&lt;运动会&gt;</h1><p>1 张三</p><p>2 李四</p><i>X</i>` {
		t.Errorf("got %s", got)
	}
}

func TestTemplateAutoEscapes(t *testing.T) {
	tpl, _ := CompileTemplate(`<p title="{{.Meet}}">{{.Meet}}</p><a href="{{.Meet}}">x</a>`)
	got, err := tpl.Execute(tdata{Meet: `"><script>alert(1)</script>`})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script") {
		t.Errorf("got %s", got)
	}
}

func TestTemplateSyntaxError(t *testing.T) {
	_, err := CompileTemplate(`{{if}}`)
	if err == nil || !strings.Contains(err.Error(), "语法") {
		t.Errorf("err = %v", err)
	}
	if _, err := CompileTemplate(strings.Repeat("a", MaxTemplateBytes+1)); err == nil {
		t.Error("an over-long template was accepted")
	}
}

func TestTemplateRuntimeError(t *testing.T) {
	tpl, _ := CompileTemplate(`{{.Missing}}`)
	if _, err := tpl.Execute(tdata{}); err == nil || !strings.Contains(err.Error(), "出错") {
		t.Errorf("err = %v", err)
	}
	tpl, _ = CompileTemplate(`{{mod 1 0}}`)
	if _, err := tpl.Execute(tdata{}); err == nil {
		t.Error("mod by zero did not fail")
	}
	tpl, _ = CompileTemplate(`{{range seq 1 100000}}x{{end}}`)
	if _, err := tpl.Execute(tdata{}); err == nil {
		t.Error("seq over its limit did not fail")
	}
}

func TestTemplateNoUnsafeFuncs(t *testing.T) {
	for _, src := range []string{`{{exec "ls"}}`, `{{readFile "/etc/passwd"}}`, `{{env "HOME"}}`, `{{getenv "HOME"}}`} {
		if _, err := CompileTemplate(src); err == nil {
			t.Errorf("%s compiled", src)
		}
	}
}

func TestTemplateOutputLimit(t *testing.T) {
	// 4 KB a round, 6000 rounds: about 24 MB.
	tpl, err := CompileTemplate(`{{range seq 1 6000}}{{repeat "x" 4000}}{{end}}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tpl.Execute(nil)
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Errorf("err = %v, want ErrOutputTooLarge", err)
	}
}

func TestTemplateRecursionIsAnError(t *testing.T) {
	tpl, err := CompileTemplate(`{{define "loop"}}x{{template "loop" .}}{{end}}{{template "loop" .}}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.Execute(nil); err == nil {
		t.Fatal("infinite recursion did not fail")
	}
}

func TestTemplateCacheAndConcurrency(t *testing.T) {
	a, _ := CompileTemplate(`<p>{{.Meet}}</p>`)
	b, _ := CompileTemplate(`<p>{{.Meet}}</p>`)
	if a != b {
		t.Error("the same source was parsed twice")
	}
	c, _ := CompileTemplate(`<p>{{.Meet}}</p>`, `{{define "x"}}{{end}}`)
	if a == c {
		t.Error("different definitions share a template")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if got, err := a.Execute(tdata{Meet: "赛"}); err != nil || got != "<p>赛</p>" {
					t.Errorf("got %q %v", got, err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestTemplateFuncsDescribeTheFunctionMap(t *testing.T) {
	docs := TemplateFuncs()
	// The map also holds tick, which the inserted actions call and no author writes.
	if len(docs) != len(funcs)-1 {
		t.Fatalf("%d described, %d in the map", len(docs), len(funcs))
	}
	by := map[string]FuncDoc{}
	for _, d := range docs {
		if _, ok := funcs[d.Name]; !ok || d.Doc == "" || !strings.HasPrefix(d.Signature, d.Name+"(") {
			t.Fatalf("%+v", d)
		}
		by[d.Name] = d
	}
	for name, want := range map[string]string{
		"add":  "add(int, int) int",
		"mod":  "mod(int, int) int",
		"seq":  "seq(int, int) []int",
		"join": "join([]string, string) string",
	} {
		if by[name].Signature != want {
			t.Errorf("%s: %q, want %q", name, by[name].Signature, want)
		}
	}
}

// A template that loops or recurses without writing stops at the step
// limit, in the caller's goroutine, and leaves nothing running.
func TestRunawayTemplatesStop(t *testing.T) {
	for name, src := range map[string]string{
		"nested ranges":          `{{range seq 1 10000}}{{range seq 1 10000}}{{range seq 1 10000}}{{end}}{{end}}{{end}}`,
		"range over integer":     `{{range 1000000000000}}{{end}}`,
		"branching recursion":    `{{define "a"}}{{template "a" .}}{{template "a" .}}{{end}}{{template "a" .}}`,
		"shallow wide recursion": `{{define "b"}}{{if lt . 30}}{{template "b" (add . 1)}}{{template "b" (add . 1)}}{{end}}{{end}}{{template "b" 0}}`,
		"loop in an attribute":   `<p class="{{range seq 1 10000}}{{range seq 1 10000}}{{end}}{{end}}">x</p>`,
	} {
		tpl, err := CompileTemplate(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		before := runtime.NumGoroutine()
		start := time.Now()
		_, err = tpl.Execute(nil)
		if err == nil || (name != "branching recursion" && !errors.Is(err, errTooLong)) {
			t.Errorf("%s: %v", name, err)
		}
		if d := time.Since(start); d > ExecTimeout {
			t.Errorf("%s ran %v", name, d)
		}
		if runtime.NumGoroutine() > before {
			t.Errorf("%s left a goroutine running", name)
		}
	}
	// The inserted ticks print nothing, wherever they are.
	tpl, err := CompileTemplate(`{{define "x"}}[{{.}}]{{end}}<p title="{{range .}}{{.}}{{end}}">{{range .}}{{template "x" .}}{{end}}</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := tpl.Execute([]string{"a", "b"}); err != nil || got != `<p title="ab">[a][b]</p>` {
		t.Errorf("got %q %v", got, err)
	}
}

func TestPrintfRefusesHugeWidths(t *testing.T) {
	for _, f := range []string{`{{printf "%0999999999d" 1}}`, `{{printf "%.99999f" 1.0}}`, `{{printf "%*d" 1000000000 1}}`} {
		tpl, err := CompileTemplate(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tpl.Execute(nil); err == nil {
			t.Errorf("%s ran", f)
		}
	}
	tpl, _ := CompileTemplate(`{{printf "%03d|%.2f|%s|%%" 7 1.5 "x"}}`)
	if got, err := tpl.Execute(nil); err != nil || got != "007|1.50|x|%" {
		t.Errorf("got %q %v", got, err)
	}
}

// The helpers bound what they make on their own: the step limit does not
// reach inside a function.
func TestHelpersDoNotOverflow(t *testing.T) {
	const most, least = int(^uint(0) >> 1), -int(^uint(0)>>1) - 1
	for _, c := range []struct{ from, to, n int }{
		{most, most, 1}, {most - 2, most, 3}, {least, least + 1, 2}, {1, 3, 3}, {3, 1, 0},
	} {
		got, err := seq(c.from, c.to)
		if err != nil || len(got) != c.n || (c.n > 0 && (got[0] != c.from || got[c.n-1] != c.to)) {
			t.Errorf("seq %d %d = %v %v", c.from, c.to, got, err)
		}
	}
	for _, c := range [][2]int{{least, most}, {0, maxSeq}, {-1, most}} {
		if _, err := seq(c[0], c[1]); err == nil {
			t.Errorf("seq %d %d accepted", c[0], c[1])
		}
	}
	for _, n := range []int{most, most / 2, 1 << 62, 1<<16 + 1} {
		if _, err := repeat("ab", n); err == nil {
			t.Errorf("repeat %d accepted", n)
		}
	}
	if got, err := repeat("ab", 3); err != nil || got != "ababab" {
		t.Errorf("repeat = %q %v", got, err)
	}
	tpl, err := CompileTemplate(`{{seq 9223372036854775807 9223372036854775807}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := tpl.Execute(nil); err != nil || got != "[9223372036854775807]" {
		t.Errorf("got %q %v", got, err)
	}
}
