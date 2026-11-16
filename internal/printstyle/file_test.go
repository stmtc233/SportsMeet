package printstyle

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func sampleFile() File {
	return File{
		Name:   "蓝色",
		Params: DefaultParams(),
		CSS:    "h1{color:#036}",
		Slots:  map[string]string{"cover": "<h1>{{.Meet}}</h1>", "footer": "{{.Meet}} 第 {page} 页"},
		Templates: map[string]string{
			"entries": "<p>{{.Meet}}</p>",
		},
		DataVersion: 1,
		Assets:      []FileAsset{{Name: "校徽", MIME: "image/png", Data: base64.StdEncoding.EncodeToString(pngData)}},
	}
}

func TestFileRoundTrip(t *testing.T) {
	data, err := Encode(sampleFile())
	if err != nil {
		t.Fatal(err)
	}
	f, warns, err := Decode(data)
	if err != nil || len(warns) != 0 {
		t.Fatalf("Decode: %v %v", err, warns)
	}
	if f.Format != FileFormat || f.Version != FileVersion || f.Name != "蓝色" || strings.TrimSpace(f.CSS) != "h1{color:#036;}" || f.Params != DefaultParams() ||
		f.Slots["cover"] == "" || f.Templates["entries"] == "" || len(f.Assets) != 1 || f.DataVersion != 1 {
		t.Errorf("file = %+v", f)
	}
	assets, err := f.AssetMap()
	if err != nil || ValidateAsset(assets["校徽"]) != nil {
		t.Errorf("assets = %v %v", assets, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"format", "version", "name", "params", "css", "slots", "templates", "dataVersion", "assets"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("the file lacks %q", k)
		}
	}
}

func TestDecodeCleansCSS(t *testing.T) {
	f := sampleFile()
	f.CSS = "h1{color:red} @import 'https://evil.example/a.css'; p{background:url(https://evil.example/x)}"
	data, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	got, warns, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.CSS, "evil") || !strings.Contains(got.CSS, "color:red") || len(warns) != 2 {
		t.Errorf("css = %q, warnings = %v", got.CSS, warns)
	}
}

func TestDecodeDefaultsParams(t *testing.T) {
	f, _, err := Decode([]byte(`{"format":"sportsmeet-print-style","version":1,"name":"小","params":{"paper":"a3"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Params.Paper != PaperA3 || f.Params.FontSize != 9.5 {
		t.Errorf("params = %+v", f.Params)
	}
}

func TestDecodeErrors(t *testing.T) {
	valid := func(mod func(m map[string]any)) []byte {
		data, _ := Encode(sampleFile())
		var m map[string]any
		json.Unmarshal(data, &m)
		mod(m)
		out, _ := json.Marshal(m)
		return out
	}
	for name, c := range map[string]struct {
		data []byte
		want string
	}{
		"not json":      {[]byte("hello"), "JSON"},
		"wrong format":  {valid(func(m map[string]any) { m["format"] = "other" }), "不是打印样式文件"},
		"newer version": {valid(func(m map[string]any) { m["version"] = 2 }), "太新"},
		"version zero":  {valid(func(m map[string]any) { m["version"] = 0 }), "太新"},
		"no name":       {valid(func(m map[string]any) { m["name"] = "" }), "名称"},
		"bad params":    {valid(func(m map[string]any) { m["params"] = map[string]any{"fontSize": 1} }), "字号"},
		"unknown param": {valid(func(m map[string]any) { m["params"] = map[string]any{"fontsz": 10} }), "版面参数"},
		"bad slot name": {valid(func(m map[string]any) { m["slots"] = map[string]any{"sidebar": "x"} }), "插槽"},
		"bad slot tmpl": {valid(func(m map[string]any) { m["slots"] = map[string]any{"cover": "{{if}}"} }), "语法"},
		"bad kind":      {valid(func(m map[string]any) { m["templates"] = map[string]any{"cards": "x"} }), "报表"},
		"bad template":  {valid(func(m map[string]any) { m["templates"] = map[string]any{"entries": "{{range}}"} }), "语法"},
		"no data ver":   {valid(func(m map[string]any) { delete(m, "dataVersion") }), "dataVersion"},
		"bad asset": {valid(func(m map[string]any) {
			m["assets"] = []any{map[string]any{"name": "a", "mime": "image/png", "data": base64.StdEncoding.EncodeToString([]byte("<svg>"))}}
		}), "png"},
		"asset b64": {valid(func(m map[string]any) {
			m["assets"] = []any{map[string]any{"name": "a", "mime": "image/png", "data": "***"}}
		}), "base64"},
		"css too long": {valid(func(m map[string]any) { m["css"] = strings.Repeat("a", MaxCSSBytes+1) }), "CSS"},
		"dup asset": {valid(func(m map[string]any) {
			a := map[string]any{"name": "a", "mime": "image/png", "data": base64.StdEncoding.EncodeToString(pngData)}
			m["assets"] = []any{a, a}
		}), "重名"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Decode(c.data)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestDecodeReportsEveryProblem(t *testing.T) {
	_, _, err := Decode([]byte(`{"format":"sportsmeet-print-style","version":1,"name":"","params":{"fontSize":1},"slots":{"x":"y"}}`))
	if err == nil || strings.Count(err.Error(), "\n") < 2 {
		t.Errorf("err = %v", err)
	}
}
