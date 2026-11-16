package printstyle

import (
	"strings"
	"testing"
)

var (
	pngData  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...)
	jpegData = append([]byte("\xff\xd8\xff\xe0"), make([]byte, 20)...)
	webpData = append(append([]byte("RIFF\x10\x00\x00\x00WEBP"), "VP8 "...), make([]byte, 20)...)
)

func TestValidateAsset(t *testing.T) {
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxAssetBytes)...)
	for _, c := range []struct {
		name string
		a    Asset
		ok   bool
	}{
		{"png", Asset{"校徽", "image/png", pngData}, true},
		{"jpeg", Asset{"照片", "image/jpeg", jpegData}, true},
		{"webp", Asset{"logo", "image/webp", webpData}, true},
		{"mime mismatch", Asset{"x", "image/jpeg", pngData}, false},
		{"gif", Asset{"x", "image/gif", []byte("GIF89a....")}, false},
		{"svg", Asset{"x", "image/svg+xml", []byte("<svg></svg>")}, false},
		{"empty", Asset{"x", "image/png", nil}, false},
		{"too big", Asset{"x", "image/png", big}, false},
		{"no name", Asset{"", "image/png", pngData}, false},
		{"name with space", Asset{"a b", "image/png", pngData}, false},
		{"name with quote", Asset{`a"b`, "image/png", pngData}, false},
		{"name with paren", Asset{"a(b)", "image/png", pngData}, false},
		{"name with full-width paren", Asset{"a（b）", "image/png", pngData}, false},
		{"name 40 chars", Asset{strings.Repeat("字", 40), "image/png", pngData}, true},
		{"name 41 chars", Asset{strings.Repeat("字", 41), "image/png", pngData}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateAsset(c.a); (err == nil) != c.ok {
				t.Errorf("err = %v, want ok %v", err, c.ok)
			}
		})
	}
}

func TestResolveAssets(t *testing.T) {
	assets := map[string]Asset{"校徽": {"校徽", "image/png", pngData}, "bg": {"bg", "image/jpeg", jpegData}}
	uri := assets["校徽"].DataURI()
	got, warns := ResolveAssets(`<img src="asset:校徽"><div style="background:url(asset:bg)"></div><style>p{background:url("asset:校徽")}</style>`, assets)
	if len(warns) != 0 {
		t.Errorf("warnings = %v", warns)
	}
	if strings.Count(got, uri) != 2 || !strings.Contains(got, assets["bg"].DataURI()) || strings.Contains(got, "asset:") {
		t.Errorf("got %s", got)
	}

	// Markdown percent-escapes non-ASCII addresses.
	got, warns = ResolveAssets(`<img src="asset:%E6%A0%A1%E5%BE%BD">`, assets)
	if len(warns) != 0 || !strings.Contains(got, uri) {
		t.Errorf("percent-escaped: %s %v", got, warns)
	}

	got, warns = ResolveAssets(`<img src="asset:不存在"><p style="background:url(asset:不存在)">`, assets)
	if len(warns) != 1 || strings.Contains(got, "asset:") || !strings.Contains(warns[0], "不存在") {
		t.Errorf("missing: %s %v", got, warns)
	}

	// Plain text that mentions asset: is not a reference.
	got, _ = ResolveAssets(`<p>写法是 asset:名称</p>`, assets)
	if !strings.Contains(got, "asset:名称") {
		t.Errorf("text was changed: %s", got)
	}

	// An asset that is not a valid image is not embedded.
	got, warns = ResolveAssets(`<img src="asset:bad">`, map[string]Asset{"bad": {"bad", "image/png", []byte("<svg onload=x>")}})
	if len(warns) != 1 || strings.Contains(got, "svg") {
		t.Errorf("invalid asset: %s %v", got, warns)
	}
}
