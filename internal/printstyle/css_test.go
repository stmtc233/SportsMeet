package printstyle

import (
	"strings"
	"testing"
)

func TestSanitizeCSS(t *testing.T) {
	const png = "data:image/png;base64,iVBORw0KGgo="
	cases := []struct {
		name    string
		in      string
		keep    []string // substrings the output has
		drop    []string // substrings it lacks
		warning bool
	}{
		{"plain", "body{color:#111;font-size:10pt}", []string{"color:#111", "font-size:10pt"}, nil, false},
		{"import", `@import url("https://evil.example/a.css"); p{color:red}`, []string{"color:red"}, []string{"@import", "evil"}, true},
		{"import escaped", `@\69mport "x.css"; p{color:red}`, []string{"color:red"}, []string{"import", "x.css"}, true},
		{"namespace and charset", `@charset "utf-8"; @namespace svg url(http://x); p{a:b}`, []string{"a:b"}, []string{"namespace", "charset"}, true},
		{"unknown at-rule", `@document url-prefix(){p{color:red}} q{a:b}`, []string{"a:b"}, []string{"document", "color:red"}, true},
		{"external url", `p{background:url(https://evil.example/x.png)}`, nil, []string{"evil", "background"}, true},
		{"external url quoted", `p{background:url("//evil.example/x.png"); color:red}`, []string{"color:red"}, []string{"evil"}, true},
		{"escaped url function", `p{background:u\72l(http://evil.example/x.png)}`, nil, []string{"evil"}, true},
		{"escaped url function hex", `p{background:\75\72\6c(http://evil.example/x.png)}`, nil, []string{"evil"}, true},
		{"uppercase url", `p{background:URL(http://evil.example/x.png)}`, nil, []string{"evil"}, true},
		{"url with spaces", "p{background:url(   http://evil.example/x.png   )}", nil, []string{"evil"}, true},
		{"url with tab in scheme", "p{background:url(\"java\tscript:alert(1)\")}", nil, []string{"alert"}, true},
		{"data image png", `p{background:url(` + png + `)}`, []string{png}, nil, false},
		{"data image svg", `p{background:url(data:image/svg+xml;base64,PHN2Zz4=)}`, nil, []string{"svg"}, true},
		{"data html", `p{background:url("data:text/html,<script>x</script>")}`, nil, []string{"script"}, true},
		{"asset", `p{background:url(asset:校徽)}`, []string{"asset:校徽"}, nil, false},
		{"asset quoted", `p{background:url("asset:logo")}`, []string{"asset:logo"}, nil, false},
		{"image-set external", `p{background:image-set("https://evil.example/a.png" 1x)}`, nil, []string{"evil"}, true},
		{"image-set data", `p{background:image-set("` + png + `" 1x)}`, []string{"image-set"}, nil, false},
		{"image()", `p{background:image("https://evil.example/a.png")}`, nil, []string{"evil"}, true},
		{"webkit image-set", `p{background:-webkit-image-set(url(http://evil.example/a.png) 1x)}`, nil, []string{"evil"}, true},
		{"cross-fade", `p{background:cross-fade(url(http://evil.example/a.png), url(` + png + `), 50%)}`, nil, []string{"evil"}, true},
		{"expression", `p{width:expression(alert(1))}`, nil, []string{"expression"}, true},
		{"expression escaped", `p{width:expr\65ssion(alert(1))}`, nil, []string{"alert"}, true},
		{"behavior", `p{behavior:url(x.htc)}`, nil, []string{"behavior"}, true},
		{"moz-binding", `p{-moz-binding:url(x.xml#a)}`, nil, []string{"binding"}, true},
		{"javascript", `p{background:"javascript:alert(1)"}`, nil, []string{"javascript"}, true},
		{"javascript ident", `a{x:javascript:alert(1)}`, nil, []string{"javascript"}, true},
		{"javascript escaped", `a{x:"\6a avascript:alert(1)"}`, nil, []string{"alert"}, true},
		{"style end tag", `p{content:"</style><script>alert(1)</script>"}`, nil, []string{"</style", "<script"}, true},
		{"style end tag in selector", `a</style><script>{x:y}`, nil, []string{"</style", "<script"}, true},
		{"font-face external", `@font-face{font-family:A;src:url(https://evil.example/a.woff2)} p{a:b}`, []string{"a:b"}, []string{"evil"}, true},
		{"font-face data font", `@font-face{font-family:A;src:url(data:font/woff2;base64,AAAA)}`, []string{"data:font/woff2"}, nil, false},
		{"font-face data image", `@font-face{font-family:A;src:url(data:image/png;base64,AAAA)}`, nil, []string{"data:image"}, true},
		{"font-face local", `@font-face{font-family:A;src:local("SimSun")}`, []string{"local"}, nil, false},
		{"media keeps", `@media print{p{color:red}}`, []string{"@media print", "color:red"}, nil, false},
		{"media with bad inside", `@media print{p{background:url(http://evil.example)} q{a:b}}`, []string{"a:b"}, []string{"evil"}, true},
		// A comment splits the word: u and rl( are not the url function, and
		// nothing is fetched.
		{"comment between", `p{background:u/**/rl(http://evil.example/x)}`, []string{"u rl("}, nil, false},
		{"bad string", "p{content:\"abc\n}; q{a:b}", []string{"a:b"}, nil, true},
		{"unterminated string", `p{content:"abc`, nil, []string{"abc"}, true},
		{"unterminated url", `p{background:url(http://evil.example`, nil, []string{"evil"}, true},
		{"nested parens hide semicolon", `p{x:foo(;background:url(http://evil.example))}`, nil, []string{"evil"}, true},
		{"cdo cdc", `<!-- p{a:b} -->`, []string{"a:b"}, []string{"<!--", "-->"}, false},
		{"variables kept", `:root{--accent:#c00} h1{color:var(--accent)}`, []string{"--accent:#c00", "var(--accent)"}, nil, false},
		{"unicode selector", `.标题{color:red}`, []string{".标题"}, nil, false},
		{"nul bytes", "p{background:u\x00rl(http://evil.example)}", []string{"u\uFFFDrl("}, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, warns, err := SanitizeCSS(c.in)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			for _, s := range c.keep {
				if !strings.Contains(got, s) {
					t.Errorf("output %q lacks %q", got, s)
				}
			}
			for _, s := range c.drop {
				if strings.Contains(strings.ToLower(got), strings.ToLower(s)) {
					t.Errorf("output %q still has %q", got, s)
				}
			}
			if c.warning != (len(warns) > 0) {
				t.Errorf("warnings = %v, want some: %v", warns, c.warning)
			}
			assertCSSClean(t, got)
		})
	}
}

func TestSanitizeCSSTooLong(t *testing.T) {
	_, _, err := SanitizeCSS(strings.Repeat("a{b:c}", MaxCSSBytes/6+10))
	if err == nil {
		t.Fatal("no error for an over-long stylesheet")
	}
	if _, _, err := SanitizeCSS(strings.Repeat("a", MaxCSSBytes)); err != nil {
		t.Fatalf("a stylesheet of exactly the limit: %v", err)
	}
}

func TestSanitizeCSSDeepNesting(t *testing.T) {
	in := strings.Repeat("@media print{", 200) + "p{a:b}" + strings.Repeat("}", 200)
	got, _, err := SanitizeCSS(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "{") > 2*maxCSSDepth+2 {
		t.Errorf("nesting kept: %d blocks", strings.Count(got, "{"))
	}
}

func TestSanitizeCSSWarningsAreBounded(t *testing.T) {
	_, warns, _ := SanitizeCSS(strings.Repeat("p{background:url(http://evil.example)}", 500))
	if len(warns) > maxWarnings+1 {
		t.Errorf("%d warnings", len(warns))
	}
}

func TestSanitizeDeclarations(t *testing.T) {
	got, _ := SanitizeDeclarations("color: red; background: url(http://evil.example); font-size: 10pt")
	if !strings.Contains(got, "color: red") || !strings.Contains(got, "font-size: 10pt") || strings.Contains(got, "evil") {
		t.Errorf("got %q", got)
	}
	if got, _ := SanitizeDeclarations("a{color:red}"); got != "" {
		t.Errorf("a rule in a style attribute stayed: %q", got)
	}
}

func TestAllowedImageURL(t *testing.T) {
	for in, want := range map[string]bool{
		"data:image/png;base64,AAAA":  true,
		"DATA:image/JPEG;base64,AAAA": true,
		"data:image/webp;base64,AAAA": true,
		"data:image/gif;base64,AAAA":  false,
		"data:image/svg+xml,<svg>":    false,
		"asset:校徽":                    true,
		"asset:":                      false,
		"asset:a b":                   false,
		"https://x.example/a.png":     false,
		"javascript:alert(1)":         false,
		" \tdata:image/png,AA":        true,
		"da\nta:image/png,AA":         true,
		"//x.example/a.png":           false,
		"":                            false,
	} {
		if got := AllowedImageURL(in); got != want {
			t.Errorf("AllowedImageURL(%q) = %v, want %v", in, got, want)
		}
	}
}

// assertCSSClean is what every output of the sanitizer must satisfy; the
// fuzz test uses it too.
func assertCSSClean(t testing.TB, out string) {
	t.Helper()
	low := strings.ToLower(out)
	for _, bad := range []string{"<script", "</style", "javascript:"} {
		if strings.Contains(low, bad) {
			t.Errorf("output has %q: %q", bad, out)
		}
	}
	for _, tok := range tokenizeCSS(out) {
		if tok.kind == tkAtKeyword && !atRules[strings.ToLower(tok.val)] {
			t.Errorf("output has @%s: %q", tok.val, out)
		}
		if tok.kind == tkURL && !allowedCSSURL(tok.val, false) {
			t.Errorf("output has url(%q): %q", tok.val, out)
		}
	}
	// Reading the output again must find nothing more to remove.
	again, warns, err := SanitizeCSS(out)
	if err != nil || len(warns) > 0 {
		t.Errorf("output is not stable: %v %v\n%q\n%q", err, warns, out, again)
	}
}

func FuzzSanitizeCSS(f *testing.F) {
	for _, s := range []string{
		`p{background:url(http://evil.example)}`, `@import "x";`, `u\72l(x)`, `a{b:image-set("x" 1x)}`,
		`</style><script>alert(1)</script>`, `@font-face{src:url(x)}`, `p{width:expression(x)}`, "p{a:\"b\n}",
		`@media print{p{color:red}}`, `a{b:c(;d:url(x))}`, "url(\x00x)", `\`, `"`, `url(`, `/*`, `<!--`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, _, err := SanitizeCSS(in)
		if err != nil {
			return
		}
		assertCSSClean(t, out)
	})
}
