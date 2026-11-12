// Package layers holds the tests that keep the packages in their layers
// (AGENTS.md §3, §5.1). It has no code of its own.
package layers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	module = "github.com/stmtc233/sportsmeet/internal/"
	mygo   = "github.com/egoist/mygo"
)

// allowed lists, for every package under internal/, the internal packages
// it may import. A new package or a new dependency is added here on
// purpose, together with AGENTS.md §3.
var allowed = map[string][]string{
	"domain":      {},
	"arrange":     {"domain"},
	"scoring":     {"domain"},
	"interchange": {"domain"},
	"report":      {},
	"store":       {"domain"},
	"service":     {"arrange", "domain", "interchange", "report", "scoring", "store"},
	"mcp":         {"service", "store"},
	"bind":        {"arrange", "domain", "interchange", "mcp", "scoring", "service", "store"},
	"layers":      {},
}

// pure packages compute only: no mygo, so no IO, windows or database.
var pure = []string{"domain", "arrange", "scoring", "interchange", "report"}

// storeNames is what bind and mcp may use from store: the types that
// appear in service signatures. Anything else (Open, Stmt, a *Store) would
// let them reach the database past the service.
var storeNames = []string{"Source", "SourceUI", "SourceMCP", "SourceSystem", "MeetCounts"}

func TestLayers(t *testing.T) {
	dirs, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		pkg := d.Name()
		may, known := allowed[pkg]
		if !known {
			t.Errorf("internal/%s 不在分层表里:在 internal/layers 的 allowed 与 AGENTS.md §3 登记", pkg)
			continue
		}
		files, err := filepath.Glob(filepath.Join("..", pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			storeAlias := ""
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if (p == mygo || strings.HasPrefix(p, mygo+"/")) && slices.Contains(pure, pkg) {
					t.Errorf("%s 引用了 %s:internal/%s 只做计算,不碰 mygo", path, p, pkg)
				}
				dep, ok := strings.CutPrefix(p, module)
				if !ok {
					continue
				}
				if !slices.Contains(may, dep) {
					t.Errorf("%s 引用了 internal/%s:internal/%s 只能引用 %v", path, dep, pkg, may)
				}
				if dep == "store" {
					storeAlias = "store"
					if imp.Name != nil {
						storeAlias = imp.Name.Name
					}
				}
			}
			if storeAlias == "" || (pkg != "bind" && pkg != "mcp") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == storeAlias && x.Obj == nil && !slices.Contains(storeNames, sel.Sel.Name) {
					t.Errorf("%s 使用了 store.%s:internal/%s 只经 service 读写数据", path, sel.Sel.Name, pkg)
				}
				return true
			})
		}
	}
}
