package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type printClient struct {
	t   *testing.T
	ctx context.Context
	s   *sdk.ClientSession
}

func newPrintClient(t *testing.T, m *Manager) *printClient {
	t.Helper()
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	client := sdk.NewClient(&sdk.Implementation{Name: "print-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return &printClient{t: t, ctx: ctx, s: session}
}

// call returns the structured result as a generic value, or the error text
// of a failed call.
func (c *printClient) call(name string, args map[string]any) (out map[string]any, fail string) {
	c.t.Helper()
	res, err := c.s.CallTool(c.ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.t.Fatal(err)
	}
	if res.IsError {
		var sb strings.Builder
		for _, part := range res.Content {
			if tc, ok := part.(*sdk.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return nil, sb.String()
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		c.t.Fatal(err)
	}
	if string(data) == "null" {
		return nil, ""
	}
	if err = json.Unmarshal(data, &out); err != nil {
		// A list result is wrapped by the caller through callList.
		return map[string]any{"_raw": json.RawMessage(data)}, ""
	}
	return out, ""
}

func (c *printClient) ok(name string, args map[string]any) map[string]any {
	c.t.Helper()
	out, fail := c.call(name, args)
	if fail != "" {
		c.t.Fatalf("%s: %s", name, fail)
	}
	return out
}

func (c *printClient) fails(name string, args map[string]any, want string) {
	c.t.Helper()
	_, fail := c.call(name, args)
	if fail == "" || !strings.Contains(fail, want) {
		c.t.Fatalf("%s: want error containing %q, got %q", name, want, fail)
	}
}

func (c *printClient) list(name string, args map[string]any) []any {
	c.t.Helper()
	out := c.ok(name, args)
	var list []any
	raw, ok := out["_raw"].(json.RawMessage)
	if !ok {
		c.t.Fatalf("%s did not return a list: %v", name, out)
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		c.t.Fatal(err)
	}
	return list
}

func pngBase64() string {
	return base64.StdEncoding.EncodeToString(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 40)...))
}

func printFixture(t *testing.T) (*printClient, *service.Service, string) {
	t.Helper()
	m, svc := setup(t)
	c := newPrintClient(t, m)
	meet, err := svc.CreateMeet(c.ctx, store.SourceUI, service.MeetInput{Name: "打印测试", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	unit, err := svc.SaveUnit(c.ctx, store.SourceUI, meet.ID, "", service.UnitInput{Name: "一班"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAthlete(c.ctx, store.SourceUI, meet.ID, "", service.AthleteInput{Name: "选手", Gender: "男", Bib: "1", UnitID: unit.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveEvent(c.ctx, store.SourceUI, meet.ID, "", service.EventInput{Gender: "男", Discipline: "100m", Kind: "track"}); err != nil {
		t.Fatal(err)
	}
	return c, svc, meet.ID
}

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestPrintStyleToolsWriteAtOnceWithVersions(t *testing.T) {
	c, svc, _ := printFixture(t)
	styles := c.list("list_print_styles", nil)
	if len(styles) != 1 || styles[0].(map[string]any)["id"] != "default" {
		t.Fatalf("styles = %v", styles)
	}

	// The built-in style reads with the interface description, and cannot change.
	got := c.ok("get_print_style", map[string]any{"id": "default"})
	guide := got["guide"].(map[string]any)
	for _, key := range []string{"workflow", "params", "cssVariables", "classes", "reports", "slots", "pagination", "templates", "frontPages", "assets", "sanitizing", "limits"} {
		if list, _ := guide[key].([]any); len(list) == 0 {
			t.Fatalf("guide.%s is empty", key)
		}
	}
	if _, has := c.ok("get_print_style", map[string]any{"id": "default", "noGuide": true})["guide"]; has {
		t.Fatal("noGuide still returned the guide")
	}
	c.fails("save_print_style", map[string]any{"id": "default", "input": map[string]any{"css": "p{color:red}"}}, "")
	c.fails("get_print_style", map[string]any{"id": "missing"}, "不存在")

	// Creating writes at once: no preview, no confirmation.
	saved := c.ok("save_print_style", map[string]any{"input": map[string]any{
		"name": "校运会", "css": "p{color:red} @import url(http://x/y.css);",
		"paramsPatch": map[string]any{"fontSize": 11, "paper": "a3"},
	}})
	style := saved["style"].(map[string]any)
	id := style["id"].(string)
	params := style["params"].(map[string]any)
	if saved["version"].(float64) != 1 || saved["changed"] != true || params["fontSize"] != 11.0 || params["paper"] != "a3" || params["marginTop"] != 15.0 {
		t.Fatalf("saved = %v", saved)
	}
	if w := strs(saved["warnings"]); len(w) == 0 || !strings.Contains(strings.Join(w, ""), "@import") {
		t.Fatalf("warnings = %v", w)
	}
	if list, _ := svc.ListPrintStyles(c.ctx); len(list) != 2 {
		t.Fatal("style was not saved")
	}

	// Full parameters replace; a missing or misspelt field is an error.
	full := c.ok("get_print_style", map[string]any{"id": "default", "noGuide": true})["style"].(map[string]any)["params"].(map[string]any)
	full["fontSize"] = 10.0
	got = c.ok("save_print_style", map[string]any{"id": id, "input": map[string]any{"params": full}})
	if got["style"].(map[string]any)["params"].(map[string]any)["fontSize"] != 10.0 {
		t.Fatalf("params = %v", got)
	}
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"params": map[string]any{"fontSize": 10}}}, "纸张")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"params": map[string]any{"fontSiz": 10}}}, "格式不对")
	// A patch in text form works; both params and a patch do not; errors change nothing.
	c.ok("save_print_style", map[string]any{"id": id, "input": map[string]any{"paramsPatch": `{"lineHeight":1.6}`}})
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"paramsPatch": map[string]any{"fontSize": 99}}}, "字号")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"paramsPatch": map[string]any{"noSuchField": 1}}}, "格式不对")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"params": map[string]any{"paper": "a4"}, "paramsPatch": map[string]any{"fontSize": 10}}}, "只能给")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"templates": map[string]any{"entries": "{{range .Events}}"}}}, "报表模板")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"templates": map[string]any{"nope": "x"}}}, "")
	c.fails("save_print_style", map[string]any{"id": id, "input": map[string]any{"slots": map[string]any{"nope": "x"}}}, "插槽")
	c.fails("save_print_style", map[string]any{"id": "missing", "input": map[string]any{"css": "p{}"}}, "不存在")
	c.fails("save_print_style", map[string]any{"input": map[string]any{"name": ""}}, "")
	d := c.ok("get_print_style", map[string]any{"id": id, "noGuide": true})["style"].(map[string]any)
	if d["css"] == "" || d["params"].(map[string]any)["lineHeight"] != 1.6 || d["version"].(float64) != 3 {
		t.Fatalf("a failed save changed the style: %v", d)
	}
	// No change, no version.
	same := c.ok("save_print_style", map[string]any{"id": id, "input": map[string]any{"paramsPatch": `{"lineHeight":1.6}`}})
	if same["changed"] != false || same["version"].(float64) != 3 {
		t.Fatalf("same = %v", same)
	}

	// Slots and templates by key; a blank value removes the key.
	c.ok("save_print_style", map[string]any{"id": id, "input": map[string]any{
		"slots":     map[string]any{"footer": "{{.Meet}} 第 {page} 页", "cover": "<h1>{{.Meet}}</h1>"},
		"templates": map[string]any{"checkin": "{{range .Sheets}}<section class=\"sheet\">{{.Event}}</section>{{end}}"},
	}})
	c.ok("save_print_style", map[string]any{"id": id, "input": map[string]any{"slots": map[string]any{"cover": " "}}})
	d = c.ok("get_print_style", map[string]any{"id": id, "noGuide": true})["style"].(map[string]any)
	if len(d["slots"].(map[string]any)) != 1 || len(d["templates"].(map[string]any)) != 1 || d["hasTemplates"] != true {
		t.Fatalf("style = %v", d)
	}

	// Versions come back, newest first, and a restore is a new save.
	versions := c.list("list_print_style_versions", map[string]any{"id": id})
	if len(versions) != 5 || versions[0].(map[string]any)["current"] != true || versions[0].(map[string]any)["source"] != "mcp" {
		t.Fatalf("versions = %v", versions)
	}
	restored := c.ok("restore_print_style_version", map[string]any{"id": id, "version": 3})
	rs := restored["style"].(map[string]any)
	if restored["version"].(float64) != 6 || len(rs["slots"].(map[string]any)) != 0 || len(rs["templates"].(map[string]any)) != 0 {
		t.Fatalf("restored = %v", restored)
	}
	c.fails("restore_print_style_version", map[string]any{"id": id, "version": 99}, "没有第 99 版")
	c.fails("restore_print_style_version", map[string]any{"id": "default", "version": 1}, "")
	if list := c.list("list_print_style_versions", map[string]any{"id": "default"}); len(list) != 0 {
		t.Fatal("the built-in style has versions")
	}

	// Copy, rename, delete.
	cp := c.ok("copy_print_style", map[string]any{"id": id, "name": "副本"})["style"].(map[string]any)
	c.fails("copy_print_style", map[string]any{"id": id, "name": "副本"}, "已有名为")
	c.fails("copy_print_style", map[string]any{"id": "missing"}, "不存在")
	c.ok("copy_print_style", map[string]any{"id": "default"})
	c.fails("rename_print_style", map[string]any{"id": "default", "name": "x"}, "")
	c.fails("rename_print_style", map[string]any{"id": cp["id"], "name": "校运会"}, "")
	if n := c.ok("rename_print_style", map[string]any{"id": cp["id"], "name": "新名"})["style"].(map[string]any)["name"]; n != "新名" {
		t.Fatalf("name = %v", n)
	}
	c.ok("delete_print_style", map[string]any{"id": cp["id"]})
	c.fails("delete_print_style", map[string]any{"id": cp["id"]}, "不存在")
	c.fails("delete_print_style", map[string]any{"id": "default"}, "")
}

func TestPrintStyleExportImportRoundTrip(t *testing.T) {
	c, _, _ := printFixture(t)
	saved := c.ok("save_print_style", map[string]any{"input": map[string]any{"name": "带图", "css": "p{color:blue}"}})
	id := saved["style"].(map[string]any)["id"].(string)
	c.ok("save_asset", map[string]any{"scope": "style", "ownerId": id, "name": "校徽", "dataBase64": pngBase64()})

	file := c.ok("export_print_style", map[string]any{"id": id})
	if file["format"] != "sportsmeet-print-style" || file["name"] != "带图" || len(file["assets"].([]any)) != 1 {
		t.Fatalf("file = %v", file)
	}
	// The exported object imports as it is, and as text; names stay unique.
	one := c.ok("import_print_style", map[string]any{"file": file})
	raw, _ := json.Marshal(file)
	two := c.ok("import_print_style", map[string]any{"file": string(raw)})
	n1, n2 := one["style"].(map[string]any)["name"], two["style"].(map[string]any)["name"]
	if n1 == "带图" || n1 == n2 || !strings.HasPrefix(n1.(string), "带图") {
		t.Fatalf("names: %v %v", n1, n2)
	}
	if a := c.list("list_assets", map[string]any{"scope": "style", "ownerId": one["style"].(map[string]any)["id"]}); len(a) != 1 {
		t.Fatalf("imported assets = %v", a)
	}
	// The built-in style exports too; garbage and missing files are refused.
	c.ok("export_print_style", map[string]any{"id": "default"})
	c.fails("export_print_style", map[string]any{"id": "missing"}, "不存在")
	c.fails("import_print_style", map[string]any{"file": "not json"}, "")
	c.fails("import_print_style", map[string]any{"file": map[string]any{"format": "other"}}, "")
	c.fails("import_print_style", map[string]any{}, "")
}

func TestRenderReportOptionsAndDraftsAreNotSaved(t *testing.T) {
	c, svc, meet := printFixture(t)

	// Existing callers still get the document under html.
	r := c.ok("render_report", map[string]any{"meetId": meet, "kind": "startlist"})
	if !strings.Contains(r["html"].(string), "打印测试") || r["warnings"] == nil {
		t.Fatalf("render_report = %v", r)
	}
	for _, kind := range []string{"entries", "results", "team_scores", "checkin", "recordsheet"} {
		if h := c.ok("render_report", map[string]any{"meetId": meet, "kind": kind})["html"].(string); !strings.Contains(h, "<html") {
			t.Fatalf("%s: %q", kind, h)
		}
	}
	c.fails("render_report", map[string]any{"meetId": meet, "kind": "nope"}, "报表类型")
	c.fails("render_report", map[string]any{"meetId": "missing", "kind": "entries"}, "")
	c.fails("render_report", map[string]any{"meetId": meet, "kind": "entries", "styleId": "missing"}, "不存在")

	// A draft is applied and cleaned but never saved.
	before, _ := svc.ListPrintStyles(c.ctx)
	r = c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries", "draft": map[string]any{
		"css": ".marker{color:red} @import url(http://x/y.css);", "paramsPatch": map[string]any{"fontSize": 12},
	}})
	if !strings.Contains(r["html"].(string), ".marker") || !strings.Contains(strings.Join(strs(r["warnings"]), ""), "@import") {
		t.Fatalf("draft: %v", r["warnings"])
	}
	c.fails("render_report", map[string]any{"meetId": meet, "kind": "entries", "draft": map[string]any{"paramsPatch": map[string]any{"fontSize": 99}}}, "字号")
	c.fails("render_report", map[string]any{"meetId": meet, "kind": "entries", "draft": map[string]any{"templates": map[string]any{"entries": "{{.Nope.X}}"}}}, "")
	after, _ := svc.ListPrintStyles(c.ctx)
	if len(before) != len(after) {
		t.Fatal("a draft created a style")
	}

	// A saved style is used by the meet it is set on, or by styleId.
	st := c.ok("save_print_style", map[string]any{"input": map[string]any{"name": "红", "css": ".saved-marker{color:red}"}})["style"].(map[string]any)["id"].(string)
	if strings.Contains(c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries"})["html"].(string), "saved-marker") {
		t.Fatal("the style was used before it was chosen")
	}
	if !strings.Contains(c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries", "styleId": st})["html"].(string), "saved-marker") {
		t.Fatal("styleId ignored")
	}
	c.ok("set_meet_print_style", map[string]any{"meetId": meet, "styleId": st})
	if !strings.Contains(c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries"})["html"].(string), "saved-marker") {
		t.Fatal("the meet's style ignored")
	}
	if strings.Contains(c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries", "styleId": "default"})["html"].(string), "saved-marker") {
		t.Fatal("styleId default ignored")
	}
	c.fails("set_meet_print_style", map[string]any{"meetId": meet, "styleId": "missing"}, "不存在")
	c.fails("set_meet_print_style", map[string]any{"meetId": "missing", "styleId": st}, "")
	c.ok("set_meet_print_style", map[string]any{"meetId": meet, "styleId": "default"})
	// Deleting the style a meet uses falls back to the default.
	c.ok("set_meet_print_style", map[string]any{"meetId": meet, "styleId": st})
	c.ok("delete_print_style", map[string]any{"id": st})
	if strings.Contains(c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries"})["html"].(string), "saved-marker") {
		t.Fatal("a deleted style is still used")
	}
}

func TestRenderStyleSampleNeedsNoMeet(t *testing.T) {
	m, _ := setup(t)
	c := newPrintClient(t, m)
	for _, kind := range []string{"entries", "startlist", "results", "team_scores", "checkin", "recordsheet"} {
		r := c.ok("render_style_sample", map[string]any{"kind": kind})
		if !strings.Contains(r["html"].(string), "<html") || r["warnings"] == nil {
			t.Fatalf("%s: %v", kind, r)
		}
	}
	r := c.ok("render_style_sample", map[string]any{"kind": "results", "draft": map[string]any{"css": ".sample-marker{color:red} @import 'x';"}})
	if !strings.Contains(r["html"].(string), ".sample-marker") || len(strs(r["warnings"])) == 0 {
		t.Fatalf("draft: %v", r)
	}
	// A template that does not run is refused with the reason.
	c.fails("render_style_sample", map[string]any{"kind": "results", "draft": map[string]any{"templates": map[string]any{"results": "{{.Missing.Field}}"}}}, "")
	c.fails("render_style_sample", map[string]any{"kind": "results", "draft": map[string]any{"templates": map[string]any{"results": "{{range"}}}, "")
	c.fails("render_style_sample", map[string]any{"kind": "nope"}, "没有名为")
	c.fails("render_style_sample", map[string]any{"kind": "results", "styleId": "missing"}, "不存在")
	// A good template renders.
	r = c.ok("render_style_sample", map[string]any{"kind": "checkin", "draft": map[string]any{"templates": map[string]any{"checkin": "<section class=\"sheet\">自定义 {{.Meet}}</section>"}}})
	if !strings.Contains(r["html"].(string), "自定义") {
		t.Fatalf("template not used: %v", r["warnings"])
	}
}

func TestReportTemplateAndDataTools(t *testing.T) {
	c, _, meet := printFixture(t)
	tpl := c.ok("get_report_template", map[string]any{"kind": "results", "withSample": true})
	if tpl["source"] == "" || tpl["dataRoot"] != "Book" || tpl["sample"] == nil || len(tpl["dataTypes"].([]any)) == 0 || len(tpl["funcs"].([]any)) == 0 {
		t.Fatalf("template = %v", tpl)
	}
	if _, has := tpl["parts"].(map[string]any)["teamScores"]; !has {
		t.Fatalf("parts = %v", tpl["parts"])
	}
	if c.ok("get_report_template", map[string]any{"kind": "checkin"})["sample"] != nil {
		t.Fatal("sample without withSample")
	}
	c.fails("get_report_template", map[string]any{"kind": "nope"}, "没有名为")

	// What the template offered is accepted back as a custom template.
	src := tpl["source"].(string)
	c.ok("render_style_sample", map[string]any{"kind": "results", "draft": map[string]any{"templates": map[string]any{"results": src}}})

	for _, kind := range []string{"entries", "startlist", "results", "team_scores", "checkin", "recordsheet"} {
		d := c.ok("get_report_data", map[string]any{"meetId": meet, "kind": kind})
		data, _ := d["data"].(map[string]any)
		if d["kind"] != kind || d["dataVersion"] != 1.0 || data["meet"] != "打印测试" {
			t.Fatalf("%s: %v", kind, d)
		}
	}
	books := c.ok("get_report_data", map[string]any{"meetId": meet, "kind": "entries"})["data"].(map[string]any)
	if ev := books["events"].([]any); len(ev) != 1 || ev[0].(map[string]any)["name"] == "" {
		t.Fatalf("events = %v", books["events"])
	}
	if ev := c.ok("get_report_data", map[string]any{"meetId": meet, "kind": "entries", "eventIds": []string{"missing"}})["data"].(map[string]any)["events"]; ev != nil && len(ev.([]any)) != 0 {
		t.Fatalf("scope ignored: %v", ev)
	}
	c.fails("get_report_data", map[string]any{"meetId": meet, "kind": "nope"}, "报表类型")
	c.fails("get_report_data", map[string]any{"meetId": "missing", "kind": "entries"}, "")
	c.fails("get_report_data", map[string]any{"meetId": meet, "kind": "checkin", "day": "bad"}, "日期")
}

func TestFrontPageTools(t *testing.T) {
	c, _, meet := printFixture(t)
	if l := c.list("list_front_pages", map[string]any{"meetId": meet}); len(l) != 0 {
		t.Fatal("pages before any was saved")
	}
	a := c.ok("save_front_page", map[string]any{"meetId": meet, "input": map[string]any{"title": "致辞", "bodyMd": "-># 欢迎<-\n\n---pagebreak---\n\n![校徽](asset:校徽)"}})
	if len(strs(a["warnings"])) != 1 || !strings.Contains(strs(a["warnings"])[0], "校徽") {
		t.Fatalf("warnings = %v", a["warnings"])
	}
	aid := a["page"].(map[string]any)["id"].(string)
	b := c.ok("save_front_page", map[string]any{"meetId": meet, "input": map[string]any{"title": "须知", "bodyMd": "- 一\n- 二", "reports": []string{"startlist"}}})
	bid := b["page"].(map[string]any)["id"].(string)
	// The image arrives, and the warning is gone.
	c.ok("save_asset", map[string]any{"scope": "meet", "ownerId": meet, "name": "校徽", "dataBase64": pngBase64()})
	a = c.ok("save_front_page", map[string]any{"meetId": meet, "id": aid, "input": map[string]any{"title": "致辞（改）", "bodyMd": "![校徽](asset:校徽)"}})
	if len(strs(a["warnings"])) != 0 || a["page"].(map[string]any)["title"] != "致辞（改）" {
		t.Fatalf("a = %v", a)
	}
	pages := c.list("list_front_pages", map[string]any{"meetId": meet})
	if len(pages) != 2 || pages[0].(map[string]any)["id"] != aid {
		t.Fatalf("pages = %v", pages)
	}
	if h := c.ok("render_report", map[string]any{"meetId": meet, "kind": "startlist"})["html"].(string); !strings.Contains(h, "data:image/png") || strings.Count(h, `class="front"`) != 2 {
		t.Fatal("front pages not in the start list")
	}
	if h := c.ok("render_report", map[string]any{"meetId": meet, "kind": "entries"})["html"].(string); strings.Count(h, `class="front"`) != 1 {
		t.Fatal("reports filter ignored")
	}
	c.ok("reorder_front_pages", map[string]any{"meetId": meet, "ids": []string{bid, aid}})
	if pages = c.list("list_front_pages", map[string]any{"meetId": meet}); pages[0].(map[string]any)["id"] != bid {
		t.Fatalf("order = %v", pages)
	}
	c.fails("reorder_front_pages", map[string]any{"meetId": meet, "ids": []string{aid}}, "")
	c.fails("save_front_page", map[string]any{"meetId": meet, "input": map[string]any{"title": "", "bodyMd": "x"}}, "标题")
	c.fails("save_front_page", map[string]any{"meetId": meet, "input": map[string]any{"title": "x", "bodyMd": "x", "reports": []string{"nope"}}}, "没有名为")
	c.fails("save_front_page", map[string]any{"meetId": meet, "id": "missing", "input": map[string]any{"title": "x", "bodyMd": "x"}}, "不存在")
	c.fails("save_front_page", map[string]any{"meetId": "missing", "input": map[string]any{"title": "x", "bodyMd": "x"}}, "")
	c.ok("delete_front_page", map[string]any{"meetId": meet, "id": aid})
	c.fails("delete_front_page", map[string]any{"meetId": meet, "id": aid}, "不存在")
	if pages = c.list("list_front_pages", map[string]any{"meetId": meet}); len(pages) != 1 || pages[0].(map[string]any)["position"] != 1.0 {
		t.Fatalf("pages = %v", pages)
	}
}

func TestAssetToolsValidateAndNeverReturnData(t *testing.T) {
	c, _, meet := printFixture(t)
	sid := c.ok("save_print_style", map[string]any{"input": map[string]any{"name": "图"}})["style"].(map[string]any)["id"].(string)
	for _, owner := range []struct{ scope, id string }{{"style", sid}, {"meet", meet}} {
		got := c.ok("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "logo", "dataBase64": pngBase64()})
		if got["mime"] != "image/png" || got["size"] != 48.0 || got["sha256"] == "" || got["data"] != nil || got["Data"] != nil {
			t.Fatalf("saved = %v", got)
		}
		// A data URI is accepted as well, and a repeat is not an error.
		c.ok("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "logo", "dataBase64": "data:image/png;base64," + pngBase64()})
		list := c.list("list_assets", map[string]any{"scope": owner.scope, "ownerId": owner.id})
		if len(list) != 1 {
			t.Fatalf("list = %v", list)
		}
		for key := range list[0].(map[string]any) {
			if key != "name" && key != "mime" && key != "sha256" && key != "size" {
				t.Fatalf("list returns %q", key)
			}
		}
		c.fails("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "坏", "dataBase64": "@@@not base64@@@"}, "base64")
		c.fails("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "文字", "dataBase64": base64.StdEncoding.EncodeToString([]byte("hello"))}, "png")
		c.fails("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "a b", "dataBase64": pngBase64()}, "名称")
		c.fails("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "类型", "mime": "image/jpeg", "dataBase64": pngBase64()}, "不一致")
		c.fails("save_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "空", "dataBase64": ""}, "")
		c.ok("delete_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "logo"})
		c.fails("delete_asset", map[string]any{"scope": owner.scope, "ownerId": owner.id, "name": "logo"}, "不存在")
	}
	c.fails("save_asset", map[string]any{"scope": "other", "ownerId": sid, "name": "x", "dataBase64": pngBase64()}, "style 或 meet")
	c.fails("list_assets", map[string]any{"scope": "style", "ownerId": "missing"}, "不存在")
	c.fails("save_asset", map[string]any{"scope": "style", "ownerId": "default", "name": "x", "dataBase64": pngBase64()}, "")
	c.fails("save_asset", map[string]any{"scope": "meet", "ownerId": "missing", "name": "x", "dataBase64": pngBase64()}, "")
	// A style's image change is a version of the style.
	c.ok("save_asset", map[string]any{"scope": "style", "ownerId": sid, "name": "新", "dataBase64": pngBase64()})
	if v := c.list("list_print_style_versions", map[string]any{"id": sid}); v[0].(map[string]any)["reason"] == "" {
		t.Fatalf("versions = %v", v)
	}
}

func TestPrintToolsAreRegisteredWithHints(t *testing.T) {
	m, _ := setup(t)
	c := newPrintClient(t, m)
	tools, err := c.s.ListTools(c.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := map[string]bool{}
	for _, tool := range tools.Tools {
		readOnly[tool.Name] = tool.Annotations != nil && tool.Annotations.ReadOnlyHint
	}
	for name, want := range map[string]bool{
		"list_print_styles": true, "get_print_style": true, "save_print_style": false, "copy_print_style": false, "rename_print_style": false,
		"delete_print_style": false, "list_print_style_versions": true, "restore_print_style_version": false, "export_print_style": true,
		"import_print_style": false, "get_report_template": true, "get_report_data": true, "render_report": true, "render_style_sample": true,
		"set_meet_print_style": false, "list_front_pages": true, "save_front_page": false, "delete_front_page": false, "reorder_front_pages": false,
		"list_assets": true, "save_asset": false, "delete_asset": false,
	} {
		got, ok := readOnly[name]
		if !ok || got != want {
			t.Errorf("%s: registered=%v readOnly=%v, want %v", name, ok, got, want)
		}
	}
	// Writing them needs no confirmation arguments.
	for _, tool := range tools.Tools {
		if strings.HasSuffix(tool.Name, "_print_style") || strings.HasSuffix(tool.Name, "_front_page") || strings.HasSuffix(tool.Name, "_asset") {
			raw, _ := json.Marshal(tool.InputSchema)
			if strings.Contains(string(raw), "previewHash") {
				t.Errorf("%s asks for a preview hash", tool.Name)
			}
		}
	}
}
