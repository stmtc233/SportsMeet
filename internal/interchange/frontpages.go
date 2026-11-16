package interchange

import (
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

// Limits of a meet's front pages and images (spec §5.15).
const (
	MaxFrontPages     = 20
	MaxFrontPageTitle = 60
)

// validateFrontPages checks the v11 fields; older files carry none (the
// reader removed them before decoding).
func validateFrontPages(f *File) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(f.FrontPages) > MaxFrontPages {
		add("frontPages 最多 %d 页", MaxFrontPages)
	}
	for i, page := range f.FrontPages {
		at := fmt.Sprintf("frontPages[%d]", i)
		if n := utf8.RuneCountInString(strings.TrimSpace(page.Title)); n < 1 || n > MaxFrontPageTitle {
			add("%s.title 需要 1–%d 个字", at, MaxFrontPageTitle)
		}
		if _, err := printstyle.RenderMarkdown(page.BodyMD); err != nil {
			add("%s.bodyMd %v", at, err)
		}
		seen := map[string]bool{}
		for _, k := range page.Reports {
			if !slices.Contains(printstyle.ReportKinds, k) || seen[k] {
				add("%s.reports 含有未知或重复的报表「%s」", at, k)
			}
			seen[k] = true
		}
	}
	if len(f.Assets) > printstyle.MaxAssets {
		add("assets 最多 %d 张", printstyle.MaxAssets)
	}
	names := map[string]bool{}
	total := 0
	for i, a := range f.Assets {
		at := fmt.Sprintf("assets[%d]", i)
		if names[a.Name] {
			add("%s.name 「%s」重复", at, a.Name)
		}
		names[a.Name] = true
		data, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil {
			add("%s.data 不是 base64", at)
			continue
		}
		total += len(data)
		if err := printstyle.ValidateAsset(printstyle.Asset{Name: a.Name, MIME: a.MIME, Data: data}); err != nil {
			add("%s %v", at, err)
		}
	}
	if total > printstyle.MaxAssetsBytes {
		add("assets 合计不能超过 %d MB", printstyle.MaxAssetsBytes>>20)
	}
	return p
}
