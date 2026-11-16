package printstyle

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const pngURI = "data:image/png;base64,iVBORw0KGgo="

func TestSanitizeDocument(t *testing.T) {
	cases := []struct {
		name string
		in   string
		keep []string
		drop []string
	}{
		{"script", `<p>a</p><script>alert(1)</script>`, []string{"<p>a</p>"}, []string{"script", "alert"}},
		{"onclick", `<p onclick="alert(1)" class="x">a</p>`, []string{`class="x"`}, []string{"onclick", "alert"}},
		{"onerror img", `<img src="x" onerror="alert(1)">`, nil, []string{"onerror", "<img"}},
		{"img data", `<img src="` + pngURI + `" alt="校徽" width="40">`, []string{pngURI, `alt="校徽"`, `width="40"`}, nil},
		{"img asset", `<img src="asset:校徽">`, []string{`src="asset:校徽"`}, nil},
		{"img external", `<img src="https://evil.example/a.png">`, nil, []string{"evil", "<img"}},
		{"img svg data", `<img src="data:image/svg+xml;base64,PHN2Zz4=">`, nil, []string{"svg", "<img"}},
		{"img javascript", `<img src="javascript:alert(1)">`, nil, []string{"javascript", "<img"}},
		{"iframe", `<iframe src="https://evil.example"></iframe><p>a</p>`, []string{"<p>a</p>"}, []string{"iframe", "evil"}},
		{"object embed", `<object data="x"></object><embed src="x">`, nil, []string{"object", "embed"}},
		{"link base", `<link rel="stylesheet" href="https://evil.example/a.css"><base href="https://evil.example/">`, nil, []string{"link", "base", "evil"}},
		{"form", `<form action="https://evil.example"><input name="a"><button>x</button></form>`, nil, []string{"form", "input", "button", "evil"}},
		{"svg", `<svg onload="alert(1)"><script>alert(1)</script></svg><p>a</p>`, []string{"<p>a</p>"}, []string{"svg", "alert"}},
		{"math", `<math><mi xlink:href="javascript:alert(1)">x</mi></math>`, nil, []string{"math", "javascript"}},
		{"noscript template", `<noscript><p title="</noscript><img src=x onerror=alert(1)>"></noscript><template><script>alert(1)</script></template>`, nil, []string{"noscript", "template", "alert", "onerror"}},
		{"anchor unwrapped", `<a href="javascript:alert(1)">名单</a>`, []string{"名单"}, []string{"<a", "href", "javascript"}},
		{"comment", `<!-- <script>alert(1)</script> --><p>a</p>`, []string{"<p>a</p>"}, []string{"<!--", "alert"}},
		{"style attr", `<p style="color:red;background:url(https://evil.example/x)">a</p>`, []string{"color:red"}, []string{"evil", "url"}},
		{"style attr all bad", `<p style="background:url(https://evil.example/x)">a</p>`, []string{"<p>a</p>"}, []string{"style", "evil"}},
		{"style element", `<style>p{color:red} @import "https://evil.example/a.css";</style><p>a</p>`, []string{"color:red"}, []string{"import", "evil"}},
		{"style end tag", `<style>p{content:"</style><script>alert(1)</script>"}</style>`, nil, []string{"<script", "alert"}},
		{"meta refresh", `<meta http-equiv="refresh" content="0;url=https://evil.example">`, nil, []string{"refresh", "evil"}},
		{"meta csp own", `<meta http-equiv="Content-Security-Policy" content="` + CSP + `">`, []string{"Content-Security-Policy", "default-src &#39;none&#39;"}, nil},
		{"meta csp other", `<meta http-equiv="Content-Security-Policy" content="default-src *">`, nil, []string{"default-src *"}},
		{"meta charset", `<meta charset="x-evil">`, []string{`charset="UTF-8"`}, []string{"x-evil"}},
		{"data attrs", `<body data-meet="赛事" data-title="成绩册" data-paper="210x297" data-onclick="x">`, []string{`data-meet="赛事"`, `data-paper="210x297"`}, nil},
		{"table", `<table><thead><tr><th scope="col" colspan="2" onclick="x">a</th></tr></thead><tbody><tr><td rowspan="2">b</td></tr></tbody></table>`, []string{`scope="col"`, `colspan="2"`, `rowspan="2"`}, []string{"onclick"}},
		{"colspan junk", `<table><tr><td colspan="javascript:1">b</td></tr></table>`, nil, []string{"javascript"}},
		{"font unwrapped", `<font color="red">字</font>`, []string{"字"}, []string{"<font", "color"}},
		{"svg use", `<p><svg><use href="data:image/svg+xml;base64,x#a"></use></svg></p>`, nil, []string{"svg", "use"}},
		{"mxss textarea", `<textarea></textarea><p title="</textarea><img src=x onerror=alert(1)>">`, nil, []string{"onerror", "<img"}},
		{"xmp", `<xmp><img src=x onerror=alert(1)></xmp>`, nil, []string{"onerror", "xmp"}},
		{"entities in text", `<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>`, []string{"&lt;script&gt;"}, []string{"<script"}},
		{"title", `<title>赛事 成绩册</title>`, []string{"<title>赛事 成绩册</title>"}, nil},
		{"img data in style", `<div style="background:url(` + pngURI + `)">x</div>`, []string{pngURI}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := SanitizeDocument("<!doctype html><html><head></head><body>" + c.in + "</body></html>")
			for _, s := range c.keep {
				if !strings.Contains(got, s) {
					t.Errorf("output lacks %q:\n%s", s, got)
				}
			}
			for _, s := range c.drop {
				if strings.Contains(strings.ToLower(got), strings.ToLower(s)) {
					t.Errorf("output still has %q:\n%s", s, got)
				}
			}
			assertHTMLClean(t, got)
		})
	}
}

func TestSanitizeDocumentKeepsDoctypeAndLang(t *testing.T) {
	got, _ := SanitizeDocument(`<!doctype html><html lang="zh-CN" onload="x()"><head><meta charset="UTF-8"><title>t</title></head><body class="a"><header><h1>赛事</h1></header></body></html>`)
	for _, s := range []string{"<!DOCTYPE html>", `lang="zh-CN"`, "<h1>赛事</h1>", "<header>"} {
		if !strings.Contains(got, s) {
			t.Errorf("output lacks %q:\n%s", s, got)
		}
	}
	if strings.Contains(got, "onload") {
		t.Errorf("onload kept: %s", got)
	}
}

func TestSanitizeFragment(t *testing.T) {
	got, warns := SanitizeFragment(`<h1>标题</h1><style>p{color:red}</style><script>x</script><p style="text-align:center">字</p>`)
	if !strings.Contains(got, "<h1>标题</h1>") || !strings.Contains(got, "text-align:center") {
		t.Errorf("got %q", got)
	}
	if strings.Contains(got, "<style") || strings.Contains(got, "script") || len(warns) == 0 {
		t.Errorf("got %q, warnings %v", got, warns)
	}
}

// assertHTMLClean walks a sanitized document and checks it holds nothing a
// report may not have; the fuzz test uses it too.
func assertHTMLClean(t testing.TB, out string) {
	t.Helper()
	if low := strings.ToLower(out); strings.Contains(low, "<script") {
		t.Errorf("output has <script: %q", out)
	}
	root, err := html.Parse(strings.NewReader(out))
	if err != nil {
		return
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.CommentNode {
			t.Errorf("a comment is left: %q", out)
		}
		if n.Type == html.ElementNode {
			if n.Namespace != "" || !allowedElements[n.Data] {
				t.Errorf("element <%s> is left: %q", n.Data, out)
			}
			for _, a := range n.Attr {
				key := strings.ToLower(a.Key)
				if strings.HasPrefix(key, "on") {
					t.Errorf("event attribute %s is left: %q", key, out)
				}
				if strings.Contains(strings.ToLower(a.Val), "javascript:") {
					t.Errorf("javascript: in %s=%q", key, a.Val)
				}
				if n.Data == "img" && key == "src" && !AllowedImageURL(a.Val) {
					t.Errorf("img src %q is left", a.Val)
				}
				if key == "style" {
					assertCSSClean(t, "x{"+a.Val+"}")
				}
				switch key {
				case "href", "action", "formaction", "background", "poster", "srcset", "xlink:href":
					t.Errorf("attribute %s is left: %q", key, out)
				}
			}
			if n.Data == "style" {
				var b strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					b.WriteString(c.Data)
				}
				assertCSSClean(t, b.String())
			}
			if n.Data == "meta" {
				for _, a := range n.Attr {
					if a.Key == "content" && a.Val != CSP {
						t.Errorf("meta content %q is left", a.Val)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
}

func FuzzSanitizeDocument(f *testing.F) {
	for _, s := range []string{
		`<p onclick=alert(1)>x</p>`, `<script>alert(1)</script>`, `<img src=x onerror=alert(1)>`,
		`<svg><style><img src=x onerror=alert(1)></style></svg>`, `<style>p{background:url(//evil)}</style>`,
		`<noscript><p title="</noscript><img src=x onerror=alert(1)>">`, `<a href="javascript:alert(1)">x</a>`,
		`<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`, `<iframe srcdoc="<script>alert(1)</script>">`,
		`<meta http-equiv="refresh" content="0;url=//evil">`, `<table><td style="x:expression(1)">`, `<form><math><mtext></form><form><mglyph><style></math><img src onerror=alert(1)>`,
		`<img src="asset:a" width=3>`, `<textarea><img src=x onerror=alert(1)></textarea>`, `<body data-a="1">`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, _ := SanitizeDocument(in)
		assertHTMLClean(t, out)
		frag, _ := SanitizeFragment(in)
		assertHTMLClean(t, "<body>"+frag+"</body>")
	})
}
