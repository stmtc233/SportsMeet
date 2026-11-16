package printstyle

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"sync"
	"text/template/parse"
	"time"
)

// Limits of a custom template.
const (
	MaxTemplateBytes = 256 << 10
	MaxOutputBytes   = 20 << 20
	// ExecTimeout and MaxSteps bound a run: a template can loop or recurse
	// without writing, and the output limit does not see that. A step is a
	// pass through a range body or a call of a named template (see tick).
	ExecTimeout = 10 * time.Second
	MaxSteps    = 2_000_000
	// maxSeq is the longest list seq makes.
	maxSeq = 10000
)

// ErrOutputTooLarge is returned when a template writes more than MaxOutputBytes.
var ErrOutputTooLarge = errors.New("模板输出超过 20 MB")

// templateFuncs is everything a template may call besides the builtins of
// text/template (and, or, not, eq, ne, lt, le, gt, ge, len, index, slice,
// printf, print, println, html, js, urlquery, call): small arithmetic and
// text helpers, nothing that touches files, the network or reflection.
// The function map and the description a template author reads (TemplateFuncs)
// both come from this list.
var templateFuncs = []struct {
	name string
	doc  string
	fn   any
}{
	{"add", "整数相加", func(a, b int) int { return a + b }},
	{"sub", "整数相减", func(a, b int) int { return a - b }},
	{"mul", "整数相乘", func(a, b int) int { return a * b }},
	{"mod", "取余，b 不能为 0", func(a, b int) (int, error) {
		if b == 0 {
			return 0, errors.New("mod：除数不能为 0")
		}
		return a % b, nil
	}},
	// seq 1 3 is [1 2 3]; at most maxSeq items.
	{"seq", fmt.Sprintf("从 from 到 to 的整数列表（含两端，至多 %d 项），常用于 range", maxSeq), seq},
	{"join", "把字符串列表连成一个字符串，第二个参数是分隔符", strings.Join},
	{"lower", "转小写", strings.ToLower},
	{"upper", "转大写", strings.ToUpper},
	{"contains", "是否包含子串", strings.Contains},
	{"hasPrefix", "是否以前缀开头", strings.HasPrefix},
	{"hasSuffix", "是否以后缀结尾", strings.HasSuffix},
	{"printf", "同 Go 的 fmt.Sprintf；宽度与精度至多 3 位数", safePrintf},
	{"repeat", "把字符串重复 n 次，结果至多 64 KB", repeat},
}

// seq counts its items instead of stepping to "to": from+1 can overflow,
// and "i <= to" holds forever when to is the largest int. The count is
// taken unsigned, so from and to far apart do not wrap round either.
func seq(from, to int) ([]int, error) {
	if to < from {
		return nil, nil
	}
	n := uint64(to) - uint64(from) + 1 // to >= from: the difference fits uint64
	if n == 0 || n > maxSeq {
		return nil, fmt.Errorf("seq：最多 %d 项", maxSeq)
	}
	out := make([]int, n)
	for k := range out {
		out[k] = from + k
	}
	return out, nil
}

// repeat checks the length before multiplying: n*len(s) can overflow.
func repeat(s string, n int) (string, error) {
	const most = 1 << 16
	if n < 0 || (len(s) > 0 && n > most/len(s)) {
		return "", errors.New("repeat：重复次数太多")
	}
	return strings.Repeat(s, n), nil
}

var funcs = func() template.FuncMap {
	m := template.FuncMap{tickName: tick}
	for _, f := range templateFuncs {
		m[f.name] = f.fn
	}
	return m
}()

// FuncDoc describes one function a template may call.
type FuncDoc struct {
	Name string `json:"name"`
	// Signature is the call as the template writes it, with the types the
	// function takes and returns (an error result is not shown: it stops
	// the template).
	Signature string `json:"signature"`
	Doc       string `json:"doc"`
}

// TemplateFuncs describes the functions a template may call besides the
// builtins, read from the functions themselves.
func TemplateFuncs() []FuncDoc {
	out := make([]FuncDoc, 0, len(templateFuncs))
	errType := reflect.TypeFor[error]()
	for _, f := range templateFuncs {
		t := reflect.TypeOf(f.fn)
		args := make([]string, t.NumIn())
		for i := range args {
			args[i] = t.In(i).String()
		}
		sig := f.name + "(" + strings.Join(args, ", ") + ")"
		if t.NumOut() > 0 && t.Out(0) != errType {
			sig += " " + t.Out(0).String()
		}
		out = append(out, FuncDoc{Name: f.name, Signature: sig, Doc: f.doc})
	}
	return out
}

// Template is a compiled custom template.
type Template struct{ t *template.Template }

var (
	cacheMu sync.Mutex
	cache   = map[[32]byte]*Template{}
)

const cacheLimit = 128

// CompileTemplate parses source as an html/template. Each of defs, parsed
// first, may define named templates the source uses (and overrides). The
// result is cached by content, so a report that is rendered again does not
// parse its templates again; a Template is safe for concurrent use.
func CompileTemplate(source string, defs ...string) (*Template, error) {
	if len(source) > MaxTemplateBytes {
		return nil, fmt.Errorf("模板超过 %d KB", MaxTemplateBytes>>10)
	}
	h := sha256.New()
	for _, d := range defs {
		fmt.Fprintf(h, "%d:", len(d))
		h.Write([]byte(d))
	}
	h.Write([]byte(source))
	var key [32]byte
	copy(key[:], h.Sum(nil))
	cacheMu.Lock()
	t, ok := cache[key]
	cacheMu.Unlock()
	if ok {
		return t, nil
	}
	root := template.New("report").Funcs(funcs)
	for _, d := range defs {
		if _, err := root.Parse(d); err != nil {
			return nil, fmt.Errorf("模板有语法错误：%v", err)
		}
	}
	if _, err := root.Parse(source); err != nil {
		return nil, fmt.Errorf("模板有语法错误：%v", err)
	}
	addTicks(root)
	t = &Template{t: root}
	cacheMu.Lock()
	if len(cache) >= cacheLimit {
		clear(cache)
	}
	cache[key] = t
	cacheMu.Unlock()
	return t, nil
}

// safePrintf is text/template's printf without the widths that make one
// call allocate a gigabyte: "%0999999999d" is refused before it is formatted.
func safePrintf(format string, args ...any) (string, error) {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		digits := 0
		for i++; i < len(format); i++ {
			c := format[i]
			switch {
			case c >= '0' && c <= '9':
				if digits++; digits > 3 {
					return "", errors.New("printf：宽度或精度太大")
				}
			case c == '*':
				return "", errors.New("printf：不支持 * 宽度")
			case c == '.' || c == '+' || c == '-' || c == '#' || c == ' ' || c == '[' || c == ']':
				digits = 0
			default:
				goto verb
			}
		}
	verb:
	}
	return fmt.Sprintf(format, args...), nil
}

// A run counts its steps through tick, which every range body and every
// named template starts with (addTicks). text/template cannot be stopped
// from outside, and a loop that writes nothing never reaches the output
// limit; tick returns an error instead, which ends the run where it is.
// Runs take execMu one at a time, so tick reads the state of the one that
// is running.
var (
	execMu sync.Mutex
	run    struct {
		steps    int
		deadline time.Time
	}
)

// tickName is the function and tickVar the variable of the inserted
// action, "{{$__tick := __tick}}": a declaration prints nothing, and
// html/template leaves it alone in any context.
const (
	tickName = "__tick"
	tickVar  = "$__tick"
)

var errTooLong = fmt.Errorf("模板运行超过 %d 秒或 %d 步，可能有死循环", int(ExecTimeout.Seconds()), MaxSteps)

func tick() (int, error) {
	run.steps++
	if run.steps > MaxSteps || (run.steps%1024 == 0 && time.Now().After(run.deadline)) {
		return 0, errTooLong
	}
	return 0, nil
}

// tickNode is the action addTicks inserts; each place gets its own copy,
// because html/template rewrites the actions it escapes in place.
var tickNode = func() *parse.ActionNode {
	trees, err := parse.Parse("tick", "{{"+tickVar+" := "+tickName+"}}", "{{", "}}", map[string]any{tickName: tick})
	if err != nil {
		panic(err)
	}
	return trees["tick"].Root.Nodes[0].(*parse.ActionNode)
}()

// addTicks makes every named template and every range body start with a tick.
func addTicks(root *template.Template) {
	var walk func(parse.Node)
	prepend := func(l *parse.ListNode) {
		if l != nil {
			l.Nodes = append([]parse.Node{tickNode.Copy()}, l.Nodes...)
		}
	}
	walk = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				walk(c)
			}
		case *parse.IfNode:
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			walk(n.List)
			walk(n.ElseList)
			prepend(n.List)
		}
	}
	for _, t := range root.Templates() {
		if t.Tree == nil || t.Tree.Root == nil {
			continue
		}
		walk(t.Tree.Root)
		prepend(t.Tree.Root)
	}
}

type limitedWriter struct {
	b    strings.Builder
	over bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.b.Len()+len(p) > MaxOutputBytes {
		w.over = true
		return 0, ErrOutputTooLarge
	}
	return w.b.Write(p)
}

// Execute runs the template over data, in this goroutine: the output is
// capped at MaxOutputBytes, and the run stops with an error at ExecTimeout
// or MaxSteps (tick). Runs take turns.
func (t *Template) Execute(data any) (out string, err error) {
	execMu.Lock()
	defer execMu.Unlock()
	run.steps, run.deadline = 0, time.Now().Add(ExecTimeout)
	w := &limitedWriter{}
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("模板运行出错：%v", r)
		}
	}()
	// html/template escapes on the first run, under its own lock.
	if err := t.t.Execute(w, data); err != nil {
		switch {
		case w.over || errors.Is(err, ErrOutputTooLarge):
			return "", ErrOutputTooLarge
		case errors.Is(err, errTooLong):
			return "", errTooLong
		}
		return "", fmt.Errorf("模板运行出错：%v", err)
	}
	return w.b.String(), nil
}
