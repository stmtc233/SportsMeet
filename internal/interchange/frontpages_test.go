package interchange

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestV11FrontPagesAndAssets(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v11.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 11 || len(f.FrontPages) != 2 || len(f.Assets) != 1 || f.Assets[0].Name != "logo" {
		t.Fatalf("v11 fields: %+v %+v", f.FrontPages, f.Assets)
	}
	if f.FrontPages[0].Title != "致辞" || len(f.FrontPages[0].Reports) != 0 || !reflect.DeepEqual(f.FrontPages[1].Reports, []string{"startlist", "checkin"}) {
		t.Fatalf("front pages: %+v", f.FrontPages)
	}
	encoded, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(f, again) {
		t.Fatal("v11 roundtrip", err)
	}

	// A file written before v11 never had them: whatever it carries there is ignored.
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["version"] = 10
	doc["assets"] = "future"
	doc["frontPages"] = 5
	legacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	old, err := Decode(legacy)
	if err != nil || old.FrontPages != nil || old.Assets != nil {
		t.Fatal("v10 front pages interpreted", err)
	}
	v10, err := os.ReadFile("../../testdata/interchange/sample-meet-v10.json")
	if err != nil {
		t.Fatal(err)
	}
	old, err = Decode(v10)
	if err != nil || old.Version != 10 || old.FrontPages != nil {
		t.Fatal("v10 golden", err)
	}
	// ... and it keeps the rulings the v10 golden carries.
	if _, err := Encode(old); err != nil {
		t.Fatal(err)
	}
}

func TestV11FrontPagesRejects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v11.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	cases := []struct{ name, from, to, want string }{
		{"empty title", `"title": "致辞"`, `"title": " "`, "title"},
		{"unknown report", `"startlist"`, `"startlisp"`, "未知或重复的报表"},
		{"duplicate report", `"checkin"`, `"startlist"`, "未知或重复的报表"},
		{"not base64", `"data": "iVBOR`, `"data": "!!iVBOR`, "不是 base64"},
		{"not an image", `"data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="`, `"data": "aGVsbG8="`, "不是 png"},
		{"mime mismatch", `"image/png"`, `"image/jpeg"`, "不一致"},
		{"bad asset name", `"name": "logo"`, `"name": "lo go"`, "图片名称"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := strings.Replace(valid, c.from, c.to, 1)
			if text == valid {
				t.Fatalf("pattern %q not found", c.from)
			}
			_, err := Decode([]byte(text))
			p, ok := err.(Problems)
			if !ok || !strings.Contains(p.Error(), c.want) {
				t.Fatalf("err = %v, want a problem containing %q", err, c.want)
			}
		})
	}
	f, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	f.Assets = append(f.Assets, Asset{Name: "logo", MIME: "image/png", Data: f.Assets[0].Data})
	if len(Validate(f)) == 0 {
		t.Fatal("duplicate asset name accepted")
	}
	f.Assets = f.Assets[:1]
	for range 20 {
		f.FrontPages = append(f.FrontPages, FrontPage{Title: "页"})
	}
	if len(Validate(f)) == 0 {
		t.Fatal("more than 20 front pages accepted")
	}
}
