package printstyle

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// MaxMarkdownBytes is the most text a front page may hold.
const MaxMarkdownBytes = 256 << 10

// PageBreakLine, on a line of its own, starts a new printed page.
const PageBreakLine = "---pagebreak---"

// The markers stand for the two special forms while goldmark reads the text.
const pageBreakToken = "SMPAGEBREAKTOKEN8f3a"

var (
	fence  = regexp.MustCompile("^ {0,3}(```|~~~)")
	center = regexp.MustCompile(`(?s)<p>-&gt;\s*(.*?)\s*&lt;-</p>`)
	pageBr = regexp.MustCompile(`<p>` + pageBreakToken + `</p>`)
)

var md = goldmark.New(
	// Tables and strikethrough only; raw HTML in the text is never
	// rendered (html.WithUnsafe is not set), and links are not made of bare
	// addresses.
	goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// RenderMarkdown turns a front page's text into HTML for a report.
//
//   - GitHub-style tables and ~~strikethrough~~ work; a line break in a
//     paragraph is a line break on paper.
//   - A line of ---pagebreak--- starts a new printed page
//     (<div class="page-break">).
//   - A paragraph that starts with -> and ends with <- is centred
//     (<p class="center">), the way "-> 校运会 <-" reads.
//   - Raw HTML is not rendered. Images are ![说明](asset:名称).
//
// The result is cleaned with SanitizeFragment.
func RenderMarkdown(src string) (string, error) {
	if len(src) > MaxMarkdownBytes {
		return "", fmt.Errorf("前置页文字超过 %d KB", MaxMarkdownBytes>>10)
	}
	if strings.Contains(src, pageBreakToken) {
		return "", errors.New("前置页文字含有保留的内容")
	}
	var lines []string
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		if fence.MatchString(line) {
			inFence = !inFence
		}
		if !inFence && strings.TrimSpace(line) == PageBreakLine {
			lines = append(lines, "", pageBreakToken, "")
			continue
		}
		lines = append(lines, line)
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(strings.Join(lines, "\n")), &buf); err != nil {
		return "", fmt.Errorf("前置页文字无法转换：%v", err)
	}
	out := pageBr.ReplaceAllString(buf.String(), `<div class="page-break"></div>`)
	out = center.ReplaceAllString(out, `<p class="center">$1</p>`)
	clean, _ := SanitizeFragment(out)
	return clean, nil
}
