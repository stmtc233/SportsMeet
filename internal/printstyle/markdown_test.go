package printstyle

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		keep []string
		drop []string
	}{
		{"heading", "# 校运会\n\n正文", []string{"<h1>校运会</h1>", "<p>正文</p>"}, nil},
		{"table", "| 项目 | 人数 |\n|---|---|\n| 100m | 8 |", []string{"<table>", "<th>项目</th>", "<td>8</td>"}, nil},
		{"strikethrough", "~~错~~", []string{"<del>错</del>"}, nil},
		{"pagebreak", "第一页\n\n---pagebreak---\n\n第二页", []string{`<div class="page-break"></div>`, "第一页", "第二页"}, []string{"pagebreak"}},
		{"pagebreak glued to text", "第一页\n---pagebreak---\n第二页", []string{`<div class="page-break"></div>`}, []string{"pagebreak"}},
		{"pagebreak in code fence", "```\n---pagebreak---\n```", []string{"---pagebreak---"}, []string{"page-break"}},
		{"center", "-> 第一届校运会 <-", []string{`<p class="center">第一届校运会</p>`}, []string{"&gt;", "&lt;"}},
		{"center multiline", "->\n第一行  \n第二行\n<-", []string{`class="center"`, "第一行", "第二行"}, nil},
		{"raw html is not rendered", "<script>alert(1)</script>\n\n<div onclick=\"x\">a</div>\n\n<img src=x onerror=alert(1)>", nil, []string{"<script", "onclick", "onerror", "<div"}},
		{"javascript link", "[点我](javascript:alert(1))", []string{"点我"}, []string{"javascript", "href"}},
		{"link is text", "[官网](https://example.com)", []string{"官网"}, []string{"href", "https"}},
		{"asset image", "![校徽](asset:校徽)", []string{"<img", "asset:", "alt=\"校徽\""}, nil},
		{"external image", "![x](https://evil.example/a.png)", nil, []string{"evil", "<img"}},
		{"data image", "![x](data:image/png;base64,iVBORw0KGgo=)", []string{"data:image/png"}, nil},
		{"svg data image", "![x](data:image/svg+xml;base64,PHN2Zz4=)", nil, []string{"svg", "<img"}},
		{"hard wrap", "第一行\n第二行", []string{"<br"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := RenderMarkdown(c.in)
			if err != nil {
				t.Fatal(err)
			}
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
			assertHTMLClean(t, "<body>"+got+"</body>")
		})
	}
}

func TestRenderMarkdownLimits(t *testing.T) {
	if _, err := RenderMarkdown(strings.Repeat("字", MaxMarkdownBytes)); err == nil {
		t.Error("an over-long text was accepted")
	}
	if _, err := RenderMarkdown("SMPAGEBREAKTOKEN8f3a"); err == nil {
		t.Error("the reserved token was accepted")
	}
}
