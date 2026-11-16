package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// pngBytes is a small image that passes printstyle.ValidateAsset; seed makes
// two of them differ.
func pngBytes(seed byte) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{seed}, 40)...)
}

func strp(s string) *string { return &s }

func newStyle(t *testing.T, svc *Service, name, css string) *PrintStyleSaved {
	t.Helper()
	in := PrintStyleInput{Name: &name}
	if css != "" {
		in.CSS = &css
	}
	r, err := svc.SavePrintStyle(t.Context(), ui, "", in)
	must(t, err)
	return r
}

// cleaned is the CSS as it is stored.
func cleaned(t *testing.T, css string) string {
	t.Helper()
	out, _, err := printstyle.SanitizeCSS(css)
	must(t, err)
	return out
}

func styleCount(t *testing.T, svc *Service) int {
	t.Helper()
	list, err := svc.ListPrintStyles(t.Context())
	must(t, err)
	return len(list) - 1 // the built-in one
}

func TestPrintStyleListStartsWithDefault(t *testing.T) {
	svc, changes := newTestService(t)
	list, err := svc.ListPrintStyles(t.Context())
	must(t, err)
	if len(list) != 1 || list[0].ID != DefaultPrintStyleID || !list[0].Builtin || list[0].Name != "默认" {
		t.Fatalf("list = %+v", list)
	}
	d, err := svc.GetPrintStyle(t.Context(), DefaultPrintStyleID)
	must(t, err)
	if d.Params != printstyle.DefaultParams() || d.CSS != "" || len(d.Assets) != 0 {
		t.Fatalf("default = %+v", d)
	}
	r := newStyle(t, svc, "校运会", ".x{color:red}")
	if r.Version != 1 || !r.Changed || !r.Style.HasCustomCSS || r.Style.Builtin {
		t.Fatalf("created = %+v", r)
	}
	if n := len(*changes); n != 1 || (*changes)[0] != (Change{Kind: "printStyles"}) {
		t.Fatalf("changes = %+v", *changes)
	}
	list, _ = svc.ListPrintStyles(t.Context())
	if len(list) != 2 || list[1].Name != "校运会" || list[1].Version != 1 || list[1].UsedBy != 0 || list[0].UsedBy != 0 {
		t.Fatalf("list = %+v", list)
	}
	if _, err := svc.SavePrintStyle(t.Context(), ui, DefaultPrintStyleID, PrintStyleInput{CSS: strp("a{}")}); err == nil || !strings.Contains(err.Error(), "内置") {
		t.Fatalf("built-in saved: %v", err)
	}
	if err := svc.DeletePrintStyle(t.Context(), ui, DefaultPrintStyleID); err == nil {
		t.Fatal("built-in deleted")
	}
	// An action in the log, with no meet.
	page, err := svc.ListAudit(t.Context(), AuditQuery{})
	must(t, err)
	if len(page.Rows) != 1 || page.Rows[0].Action != "printstyle.created" || page.Rows[0].MeetID != "" || page.Rows[0].ActionText == "" {
		t.Fatalf("audit = %+v", page.Rows)
	}
}

func TestPrintStylePartialUpdate(t *testing.T) {
	svc, changes := newTestService(t)
	ctx := t.Context()
	first := newStyle(t, svc, "甲", ".a{color:red}")
	id := first.Style.ID
	patch := `{"fontSize":11,"paper":"a3","orientation":"landscape"}`
	r, err := svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{ParamsPatch: patch})
	must(t, err)
	p := r.Style.Params
	if r.Version != 2 || p.FontSize != 11 || p.Paper != "a3" || p.Orientation != "landscape" || p.MarginTop != 15 || r.Style.CSS != cleaned(t, ".a{color:red}") || r.Style.Name != "甲" {
		t.Fatalf("patched = %+v", r.Style)
	}
	// Only the name: the rest stays.
	r, err = svc.RenamePrintStyle(ctx, ui, id, "乙")
	must(t, err)
	if r.Version != 3 || r.Style.Name != "乙" || r.Style.Params.FontSize != 11 {
		t.Fatalf("renamed = %+v", r.Style)
	}
	// A slot is added by key; a white-space value removes it; others stay.
	r, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Slots: map[string]string{"cover": `<p id="covermark">{{.Meet}}</p>`, "footer": "页 {page}"}})
	must(t, err)
	if r.Version != 4 || len(r.Style.Slots) != 2 || r.Style.DataVersion != 1 || !r.Style.HasTemplates {
		t.Fatalf("slots = %+v", r.Style)
	}
	r, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Slots: map[string]string{"footer": " "}})
	must(t, err)
	if len(r.Style.Slots) != 1 || r.Style.Slots["cover"] == "" || r.Style.CSS != cleaned(t, ".a{color:red}") {
		t.Fatalf("slot removed = %+v", r.Style.Slots)
	}
	// Nothing changes: no version, no write, no notification.
	before := len(*changes)
	r, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{CSS: strp(".a{color:red}"), ParamsPatch: `{"fontSize":11}`})
	must(t, err)
	if r.Changed || r.Version != 5 || len(*changes) != before {
		t.Fatalf("no-op save: %+v, %d changes", r, len(*changes)-before)
	}
	// Params and a patch together are refused.
	pp := printstyle.DefaultParams()
	_, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Params: &pp, ParamsPatch: patch})
	wantInvalid(t, err, "之一")
	// Full replacement.
	pp.FontSize = 12
	r, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Params: &pp})
	must(t, err)
	if r.Style.Params.FontSize != 12 || r.Style.Params.Paper != "a4" {
		t.Fatalf("replaced = %+v", r.Style.Params)
	}
}

func TestPrintStyleValidationWritesNothing(t *testing.T) {
	svc, changes := newTestService(t)
	ctx := t.Context()
	newStyle(t, svc, "甲", "")
	good := newStyle(t, svc, "乙", ".keep{color:red}")
	id, notified := good.Style.ID, len(*changes)
	bad := []struct {
		name string
		id   string
		in   PrintStyleInput
		want string
	}{
		{"empty name", "", PrintStyleInput{Name: strp("  ")}, "样式名称"},
		{"long name", "", PrintStyleInput{Name: strp(strings.Repeat("长", 41))}, "样式名称"},
		{"no name", "", PrintStyleInput{}, "名称"},
		{"taken", "", PrintStyleInput{Name: strp(" 甲 ")}, "已有名为"},
		{"default's name", "", PrintStyleInput{Name: strp("默认")}, "已有名为"},
		{"rename onto another", id, PrintStyleInput{Name: strp("甲")}, "已有名为"},
		{"params", id, PrintStyleInput{ParamsPatch: `{"fontSize":99}`}, "字号"},
		{"unknown param", id, PrintStyleInput{ParamsPatch: `{"fontSizes":9}`}, "格式不对"},
		{"css too big", id, PrintStyleInput{CSS: strp(strings.Repeat("a{}", printstyle.MaxCSSBytes))}, "KB"},
		{"unknown slot", id, PrintStyleInput{Slots: map[string]string{"nowhere": "x"}}, "没有名为"},
		{"slot syntax", id, PrintStyleInput{Slots: map[string]string{"cover": "{{"}}, "插槽「cover」"},
		{"slot field", id, PrintStyleInput{Slots: map[string]string{"cover": "{{.Nope}}"}}, "插槽「cover」"},
		{"unknown template", id, PrintStyleInput{Templates: map[string]string{"x": "y"}}, "报表模板「x」"},
		{"template syntax", id, PrintStyleInput{Templates: map[string]string{"entries": "{{if}}"}}, "报表模板「entries」"},
		{"missing style", "nope", PrintStyleInput{CSS: strp("a{}")}, "不存在"},
	}
	for _, c := range bad {
		_, err := svc.SavePrintStyle(ctx, ui, c.id, c.in)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		wantInvalid(t, err, c.want)
	}
	// The checks run in order: a bad name is reported before bad parameters.
	_, err := svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{Name: strp(""), ParamsPatch: `{"fontSize":99}`})
	wantInvalid(t, err, "样式名称")
	_, err = svc.SavePrintStyle(ctx, ui, id, PrintStyleInput{ParamsPatch: `{"fontSize":99}`, Slots: map[string]string{"cover": "{{"}, CSS: strp(strings.Repeat("a{}", printstyle.MaxCSSBytes))})
	wantInvalid(t, err, "字号")
	if styleCount(t, svc) != 2 || len(*changes) != notified {
		t.Fatalf("a refused save wrote: %d styles, %d changes", styleCount(t, svc), len(*changes)-notified)
	}
	d, err := svc.GetPrintStyle(ctx, id)
	must(t, err)
	if d.Version != 1 || d.CSS != cleaned(t, ".keep{color:red}") {
		t.Fatalf("style changed: %+v", d)
	}
	rows, err := svc.store.ListPrintStyleVersions(ctx, id)
	must(t, err)
	if len(rows) != 1 {
		t.Fatalf("versions = %d", len(rows))
	}
}

func TestPrintStyleCSSIsCleaned(t *testing.T) {
	svc, _ := newTestService(t)
	r := newStyle(t, svc, "甲", `@import url(http://evil.example/x.css); .a{color:red}`)
	if strings.Contains(r.Style.CSS, "import") || !strings.Contains(r.Style.CSS, ".a") || len(r.Warnings) == 0 {
		t.Fatalf("css = %q, warnings = %v", r.Style.CSS, r.Warnings)
	}
	row, err := svc.store.GetPrintStyle(t.Context(), r.Style.ID)
	must(t, err)
	if strings.Contains(row.CSS, "import") {
		t.Fatal("the stored CSS is not the cleaned one")
	}
}

func TestPrintStyleVersionsTrimmedAndRestored(t *testing.T) {
	svc, changes := newTestService(t)
	ctx := t.Context()
	id := newStyle(t, svc, "甲", ".v1{color:red}").Style.ID
	for v := 2; v <= 36; v++ {
		_, err := svc.SavePrintStyle(ctx, store.SourceMCP, id, PrintStyleInput{CSS: strp(fmt.Sprintf(".v%d{color:red}", v))})
		must(t, err)
	}
	list, err := svc.ListPrintStyleVersions(ctx, id)
	must(t, err)
	if len(list) != 30 || list[0].Version != 36 || !list[0].Current || list[29].Version != 7 || list[1].Current || list[0].Source != "mcp" || list[0].Reason == "" {
		t.Fatalf("versions = %+v", list)
	}
	if want := domain.RecordedText(list[0].At, svc.zone); list[0].AtText == "" || list[0].AtText != want {
		t.Fatalf("atText = %q, want %q", list[0].AtText, want)
	}
	if _, err := svc.RestorePrintStyleVersion(ctx, ui, id, 6); err == nil {
		t.Fatal("a trimmed version was restored")
	}
	notified := len(*changes)
	r, err := svc.RestorePrintStyleVersion(ctx, ui, id, 20)
	must(t, err)
	if r.Version != 37 || !strings.Contains(r.Style.CSS, ".v20{") || !r.Changed || len(*changes) != notified+1 {
		t.Fatalf("restored = %+v", r)
	}
	list, _ = svc.ListPrintStyleVersions(ctx, id)
	if len(list) != 30 || list[0].Version != 37 || list[0].Reason != "恢复第 20 版" || list[29].Version != 8 {
		t.Fatalf("versions after restore: first %+v last %+v (%d)", list[0], list[len(list)-1], len(list))
	}
	// Restoring the version that is current changes nothing.
	r, err = svc.RestorePrintStyleVersion(ctx, ui, id, 37)
	must(t, err)
	if r.Changed || r.Version != 37 {
		t.Fatalf("restore of the current version: %+v", r)
	}
	// The name stays when an older version had another one.
	_, err = svc.RenamePrintStyle(ctx, ui, id, "乙")
	must(t, err)
	r, err = svc.RestorePrintStyleVersion(ctx, ui, id, 37)
	must(t, err)
	if r.Style.Name != "乙" {
		t.Fatalf("restore changed the name: %q", r.Style.Name)
	}
}

func TestPrintStyleCopyKeepsAssets(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()
	src := newStyle(t, svc, "甲", ".a{background:url(asset:logo)}")
	_, err := svc.SaveAsset(ctx, ui, "style", src.Style.ID, "logo", "image/png", pngBytes(1))
	must(t, err)
	c, err := svc.CopyPrintStyle(ctx, ui, src.Style.ID, "")
	must(t, err)
	if c.Style.ID == src.Style.ID || c.Style.Name != "甲 副本" || c.Version != 1 || len(c.Style.Assets) != 1 || c.Style.CSS != src.Style.CSS {
		t.Fatalf("copy = %+v", c.Style)
	}
	again, err := svc.CopyPrintStyle(ctx, ui, src.Style.ID, "")
	must(t, err)
	if again.Style.Name != "甲 副本(2)" {
		t.Fatalf("second copy = %q", again.Style.Name)
	}
	_, err = svc.CopyPrintStyle(ctx, ui, src.Style.ID, "甲")
	wantInvalid(t, err, "已有名为")
	// The copy's image is its own: replacing it leaves the source alone.
	_, err = svc.SaveAsset(ctx, ui, "style", c.Style.ID, "logo", "image/png", pngBytes(2))
	must(t, err)
	a, _ := svc.store.GetAsset(ctx, store.AssetOfStyle, src.Style.ID, "logo")
	b, _ := svc.store.GetAsset(ctx, store.AssetOfStyle, c.Style.ID, "logo")
	if a == nil || b == nil || a.SHA256 == b.SHA256 || a.ID == b.ID {
		t.Fatalf("assets shared: %+v %+v", a, b)
	}
	// A copy of the built-in style starts from the default look.
	d, err := svc.CopyPrintStyle(ctx, ui, DefaultPrintStyleID, "新样式")
	must(t, err)
	if d.Style.Params != printstyle.DefaultParams() || d.Style.CSS != "" || d.Style.Name != "新样式" {
		t.Fatalf("default copy = %+v", d.Style)
	}
	first, err := svc.CopyPrintStyle(ctx, ui, DefaultPrintStyleID, "")
	must(t, err)
	if first.Style.Name != "默认 副本" {
		t.Fatalf("name = %q", first.Style.Name)
	}
}

func TestPrintStyleDeleteReadsAsDefault(t *testing.T) {
	f := scheduleFixture(t)
	ctx := f.ctx
	id := newStyle(t, f.svc, "甲", ".gone{color:red}").Style.ID
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, id))
	list, _ := f.svc.ListPrintStyles(ctx)
	if list[1].UsedBy != 1 || list[0].UsedBy != 0 {
		t.Fatalf("used by = %+v", list)
	}
	d, err := f.svc.GetPrintStyle(ctx, id)
	must(t, err)
	if len(d.Meets) != 1 || d.Meets[0].ID != f.meet {
		t.Fatalf("meets = %+v", d.Meets)
	}
	html, err := f.svc.RenderReport(ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, ".gone") {
		t.Fatal("style not used")
	}
	must(t, f.svc.DeletePrintStyle(ctx, ui, id))
	prefs, err := f.svc.MeetSettings(ctx, f.meet)
	must(t, err)
	if prefs.PrintStyleID != "" {
		t.Fatalf("the vanished style still shows: %q", prefs.PrintStyleID)
	}
	html, err = f.svc.RenderReport(ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if strings.Contains(html, ".gone") {
		t.Fatal("deleted style used")
	}
	list, _ = f.svc.ListPrintStyles(ctx)
	if len(list) != 1 || list[0].UsedBy != 1 {
		t.Fatalf("after delete = %+v", list)
	}
	rows, err := f.svc.store.Query(ctx, `SELECT (SELECT count(*) FROM print_style_versions), (SELECT count(*) FROM print_style_assets)`)
	must(t, err)
	if rows.Values[0][0] != int64(0) || rows.Values[0][1] != int64(0) {
		t.Fatalf("leftovers: %v", rows.Values[0])
	}
	// Saving the settings with the vanished id keeps it out.
	prefs.PrintStyleID = id
	must(t, f.svc.SaveMeetSettings(ctx, ui, f.meet, prefs))
	prefs, _ = f.svc.MeetSettings(ctx, f.meet)
	if prefs.PrintStyleID != "" {
		t.Fatal("a vanished style id was stored")
	}
	wantInvalid(t, f.svc.DeletePrintStyle(ctx, ui, id), "不存在")
}

func TestSetMeetPrintStyle(t *testing.T) {
	f := newFixture(t, 2, track())
	ctx := f.ctx
	id := newStyle(t, f.svc, "甲", "").Style.ID
	notified := len(*f.changes)
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, id))
	if got := (*f.changes)[notified:]; len(got) != 1 || got[0] != (Change{Kind: "meets", MeetID: f.meet}) {
		t.Fatalf("changes = %+v", got)
	}
	prefs, _ := f.svc.MeetSettings(ctx, f.meet)
	if prefs.PrintStyleID != id || !prefs.SeparateUnits {
		t.Fatalf("settings = %+v", prefs)
	}
	wantInvalid(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, "nope"), "不存在")
	wantInvalid(t, f.svc.SetMeetPrintStyle(ctx, ui, "nope", id), "赛事")
	// Setting it again writes nothing; the default's id and "" both clear it.
	notified = len(*f.changes)
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, id))
	if len(*f.changes) != notified {
		t.Fatal("an unchanged choice was written")
	}
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, DefaultPrintStyleID))
	if prefs, _ = f.svc.MeetSettings(ctx, f.meet); prefs.PrintStyleID != "" {
		t.Fatal("default not chosen")
	}
	list, _ := f.svc.ListPrintStyles(ctx)
	if list[0].UsedBy != 1 || list[1].UsedBy != 0 {
		t.Fatalf("used by = %+v", list)
	}
}

func TestPrintStyleExportImportRoundTrip(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()
	in := PrintStyleInput{Name: strp("校运会"), ParamsPatch: `{"fontSize":10.5,"tableLines":"grid"}`, CSS: strp(".h{background:url(asset:logo)}"),
		Slots: map[string]string{"cover": `<p id="covermark">{{.Meet}}</p>`}}
	made, err := svc.SavePrintStyle(ctx, ui, "", in)
	must(t, err)
	id := made.Style.ID
	_, err = svc.SaveAsset(ctx, ui, "style", id, "logo", "image/png", pngBytes(7))
	must(t, err)
	data, err := svc.ExportPrintStyle(ctx, id)
	must(t, err)
	a, err := svc.ImportPrintStyle(ctx, store.SourceMCP, data)
	must(t, err)
	b, err := svc.ImportPrintStyle(ctx, ui, data)
	must(t, err)
	if a.Style.Name != "校运会(2)" || b.Style.Name != "校运会(3)" || len(a.Warnings) == 0 || a.Style.ID == id {
		t.Fatalf("imported names %q %q, warnings %v", a.Style.Name, b.Style.Name, a.Warnings)
	}
	orig, _ := svc.GetPrintStyle(ctx, id)
	if a.Version != 1 || a.Style.Params != orig.Params || a.Style.CSS != orig.CSS || a.Style.Slots["cover"] != orig.Slots["cover"] || a.Style.DataVersion != orig.DataVersion ||
		len(a.Style.Assets) != 1 || a.Style.Assets[0].SHA256 != orig.Assets[0].SHA256 {
		t.Fatalf("imported = %+v\noriginal = %+v", a.Style, orig)
	}
	// Exporting the copy gives the same file apart from the name.
	again, err := svc.ExportPrintStyle(ctx, a.Style.ID)
	must(t, err)
	if strings.Replace(string(again), "校运会(2)", "校运会", 1) != string(data) {
		t.Fatal("export of the imported style differs")
	}
	// The built-in one exports and imports under another name.
	def, err := svc.ExportPrintStyle(ctx, DefaultPrintStyleID)
	must(t, err)
	d, err := svc.ImportPrintStyle(ctx, ui, def)
	must(t, err)
	if d.Style.Name != "默认(2)" || d.Style.Params != printstyle.DefaultParams() {
		t.Fatalf("default imported = %+v", d.Style)
	}
	// An import never touches an existing style.
	if now, _ := svc.GetPrintStyle(ctx, id); now.Version != orig.Version {
		t.Fatal("import changed an existing style")
	}
	// Bad files: nothing is created.
	count := styleCount(t, svc)
	for name, text := range map[string]string{
		"not json":  "nope",
		"not ours":  `{"format":"other"}`,
		"newer":     `{"format":"sportsmeet-print-style","version":99}`,
		"bad param": strings.Replace(string(data), `"fontSize": 10.5`, `"fontSize": 99`, 1),
		"bad image": strings.Replace(string(data), `"mime": "image/png"`, `"mime": "image/jpeg"`, 1),
	} {
		if _, err := svc.ImportPrintStyle(ctx, ui, []byte(text)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if styleCount(t, svc) != count {
		t.Fatal("a refused import created a style")
	}
	// A style whose templates are for another data version imports with a warning, unused.
	stale := strings.Replace(string(data), `"dataVersion": 1`, `"dataVersion": 7`, 1)
	s, err := svc.ImportPrintStyle(ctx, ui, []byte(stale))
	must(t, err)
	if s.Style.DataVersion != 7 || len(s.Warnings) < 2 {
		t.Fatalf("stale import: %+v", s)
	}
	// The form saves its params with empty slots and templates; that does
	// not mark the stale templates as current.
	css := ".x{}"
	saved, err := svc.SavePrintStyle(ctx, ui, s.Style.ID, PrintStyleInput{CSS: &css, Slots: map[string]string{}, Templates: map[string]string{}})
	must(t, err)
	if saved.Style.DataVersion != 7 {
		t.Fatalf("an empty patch restamped the templates: %d", saved.Style.DataVersion)
	}
}

func TestAssetsOverwriteAndLimits(t *testing.T) {
	f := newFixture(t, 2, track())
	ctx := f.ctx
	style := newStyle(t, f.svc, "甲", "").Style.ID
	for _, owner := range []struct{ scope, id string }{{"style", style}, {"meet", f.meet}} {
		info, err := f.svc.SaveAsset(ctx, ui, owner.scope, owner.id, "logo", "", pngBytes(1))
		must(t, err)
		if info.MIME != "image/png" || info.Size != 48 || info.SHA256 == "" {
			t.Fatalf("%s: %+v", owner.scope, info)
		}
		_, err = f.svc.SaveAsset(ctx, ui, owner.scope, owner.id, "logo", "image/png", pngBytes(2))
		must(t, err)
		list, err := f.svc.ListAssets(ctx, owner.scope, owner.id)
		must(t, err)
		if len(list) != 1 || list[0].SHA256 != assetHash(pngBytes(2)) {
			t.Fatalf("%s: overwrite left %+v", owner.scope, list)
		}
		for name, c := range map[string]struct {
			name, mime string
			data       []byte
			want       string
		}{
			"not an image": {"a", "image/png", []byte("hello"), "不是 png"},
			"wrong type":   {"a", "image/jpeg", pngBytes(1), "不一致"},
			"bad name":     {"a b", "image/png", pngBytes(1), "图片名称"},
			"too big":      {"a", "image/png", append(pngBytes(1), make([]byte, printstyle.MaxAssetBytes)...), "MB"},
			"empty":        {"a", "image/png", nil, "空"},
		} {
			if _, err := f.svc.SaveAsset(ctx, ui, owner.scope, owner.id, c.name, c.mime, c.data); err == nil {
				t.Errorf("%s/%s accepted", owner.scope, name)
			} else {
				wantInvalid(t, err, c.want)
			}
		}
	}
	// The style's version went up for the image saves and nothing else did.
	d, _ := f.svc.GetPrintStyle(ctx, style)
	if d.Version != 3 {
		t.Fatalf("style version = %d", d.Version)
	}
	// The same image again: no write, no version.
	notified := len(*f.changes)
	_, err := f.svc.SaveAsset(ctx, ui, "style", style, "logo", "image/png", pngBytes(2))
	must(t, err)
	if d, _ = f.svc.GetPrintStyle(ctx, style); d.Version != 3 || len(*f.changes) != notified {
		t.Fatal("an unchanged image was written")
	}
	// The count limit.
	for i := 0; i < printstyle.MaxAssets-1; i++ {
		_, err := f.svc.SaveAsset(ctx, ui, "meet", f.meet, fmt.Sprintf("p%d", i), "image/png", pngBytes(3))
		must(t, err)
	}
	_, err = f.svc.SaveAsset(ctx, ui, "meet", f.meet, "one-too-many", "image/png", pngBytes(3))
	wantInvalid(t, err, "最多")
	_, err = f.svc.SaveAsset(ctx, ui, "meet", f.meet, "p0", "image/png", pngBytes(4)) // replacing is fine
	must(t, err)
	// Delete.
	must(t, f.svc.DeleteAsset(ctx, ui, "style", style, "logo"))
	wantInvalid(t, f.svc.DeleteAsset(ctx, ui, "style", style, "logo"), "不存在")
	must(t, f.svc.DeleteAsset(ctx, ui, "meet", f.meet, "p0"))
	if d, _ = f.svc.GetPrintStyle(ctx, style); len(d.Assets) != 0 || d.Version != 4 {
		t.Fatalf("after delete: %+v", d)
	}
	// Scopes and owners are checked.
	_, err = f.svc.SaveAsset(ctx, ui, "other", style, "a", "image/png", pngBytes(1))
	wantInvalid(t, err, "范围")
	_, err = f.svc.SaveAsset(ctx, ui, "style", DefaultPrintStyleID, "a", "image/png", pngBytes(1))
	wantInvalid(t, err, "内置")
	_, err = f.svc.SaveAsset(ctx, ui, "meet", "nope", "a", "image/png", pngBytes(1))
	wantInvalid(t, err, "赛事")
	uri, err := f.svc.AssetDataURI(ctx, "meet", f.meet, "p1")
	must(t, err)
	if want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes(3)); uri != want {
		t.Fatalf("uri = %s", uri)
	}
}

// Deleting a meet takes its images and front pages with it.
func TestMeetDeleteCascadesPrintContent(t *testing.T) {
	f := newFixture(t, 1, track())
	_, err := f.svc.SaveAsset(f.ctx, ui, "meet", f.meet, "logo", "image/png", pngBytes(1))
	must(t, err)
	_, err = f.svc.SaveFrontPage(f.ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "你好"})
	must(t, err)
	must(t, f.svc.DeleteMeet(f.ctx, ui, f.meet))
	rows, err := f.svc.store.Query(f.ctx, `SELECT (SELECT count(*) FROM meet_assets), (SELECT count(*) FROM meet_front_pages)`)
	must(t, err)
	if rows.Values[0][0] != int64(0) || rows.Values[0][1] != int64(0) {
		t.Fatalf("leftovers: %v", rows.Values[0])
	}
}

func TestPrintStyleLongNamesStayInTheLimit(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()
	long := strings.Repeat("长", printstyle.MaxNameRunes)
	id := newStyle(t, svc, long, "").Style.ID
	c, err := svc.CopyPrintStyle(ctx, ui, id, "")
	must(t, err)
	if n := len([]rune(c.Style.Name)); n > printstyle.MaxNameRunes || c.Style.Name == long {
		t.Fatalf("copy name has %d characters: %q", n, c.Style.Name)
	}
	data, err := svc.ExportPrintStyle(ctx, id)
	must(t, err)
	a, err := svc.ImportPrintStyle(ctx, ui, data)
	must(t, err)
	if a.Style.Name != strings.Repeat("长", printstyle.MaxNameRunes-4)+"(3)" { // (2) is the copy
		t.Fatalf("import name = %q", a.Style.Name)
	}
}
