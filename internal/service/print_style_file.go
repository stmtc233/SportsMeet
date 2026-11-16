package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// ExportPrintStyle returns a style as a style file (printstyle.FileExt;
// its version is its own, independent of the interchange format) with its
// images. The built-in default can be exported too.
func (s *Service) ExportPrintStyle(ctx context.Context, id string) ([]byte, error) {
	f := printstyle.File{Assets: []printstyle.FileAsset{}}
	if id == DefaultPrintStyleID {
		c := builtinContent()
		f.Name, f.Params, f.Slots, f.Templates = c.Name, c.Params, c.Slots, c.Templates
	} else {
		row, err := s.store.GetPrintStyle(ctx, id)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, errNoStyle()
		}
		c, _ := contentOf(*row)
		f.Name, f.Params, f.CSS, f.Slots, f.Templates, f.DataVersion = c.Name, c.Params, c.CSS, nonNil(c.Slots), nonNil(c.Templates), c.DataVersion
		assets, err := s.store.ListAssets(ctx, store.AssetOfStyle, id, true)
		if err != nil {
			return nil, err
		}
		for _, a := range assets {
			f.Assets = append(f.Assets, printstyle.FileAsset{Name: a.Name, MIME: a.MIME, Data: base64.StdEncoding.EncodeToString(a.Data)})
		}
	}
	data, err := printstyle.Encode(f)
	if err != nil {
		return nil, invalid("样式无法导出：%v", err)
	}
	return data, nil
}

// ImportPrintStyle reads a style file and always creates a new style from
// it, at once: the name gets (2), (3) … added when it is taken (the
// built-in 默认 is taken), and no existing style is touched. The file is
// checked as a save is (parameters, CSS cleaned, templates against the
// sample data); templates written for another report data version are kept
// but not used, with a warning.
func (s *Service) ImportPrintStyle(ctx context.Context, src store.Source, data []byte) (*PrintStyleSaved, error) {
	f, warns, err := printstyle.Decode(data)
	if err != nil {
		return nil, invalid("%s", err)
	}
	name, err := validStyleName(f.Name)
	if err != nil {
		return nil, err
	}
	c := styleContent{Name: name, Params: f.Params, CSS: f.CSS, Slots: nonNil(f.Slots), Templates: nonNil(f.Templates), DataVersion: f.DataVersion}
	// An empty template is no template.
	for k, v := range c.Slots {
		if strings.TrimSpace(v) == "" {
			delete(c.Slots, k)
		}
	}
	for k, v := range c.Templates {
		if strings.TrimSpace(v) == "" {
			delete(c.Templates, k)
		}
	}
	if !c.hasTemplates() {
		c.DataVersion = 0
	} else if c.DataVersion == report.DataVersion {
		for _, slot := range sortedStrings(c.Slots) {
			if err := report.ValidateSlot(slot, c.Slots[slot]); err != nil {
				return nil, invalid("插槽「%s」：%v", slot, err)
			}
		}
		for _, kind := range sortedStrings(c.Templates) {
			if err := report.ValidateTemplate(kind, c.Templates[kind]); err != nil {
				return nil, invalid("报表模板「%s」：%v", kind, err)
			}
		}
	} else {
		warns = append(warns, "自定义模板写于旧的报表数据版本，不会使用，请更新模板")
	}
	assetMap, err := f.AssetMap()
	if err != nil {
		return nil, invalid("%s", err)
	}
	var assets []store.AssetRow
	total := 0
	for _, n := range slices.Sorted(mapKeys(assetMap)) {
		a := assetMap[n]
		total += len(a.Data)
		assets = append(assets, store.AssetRow{Name: a.Name, MIME: a.MIME, SHA256: assetHash(a.Data), Size: len(a.Data), Data: a.Data})
	}
	if total > printstyle.MaxAssetsBytes {
		return nil, invalid("图片合计不能超过 %d MB", printstyle.MaxAssetsBytes>>20)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) >= maxPrintStyles {
		return nil, invalid("样式最多 %d 个", maxPrintStyles)
	}
	if unique := uniqueStyleName(rows, c.Name); unique != c.Name {
		warns = append(warns, fmt.Sprintf("已有名为「%s」的样式，导入的样式命名为「%s」", c.Name, unique))
		c.Name = unique
	}
	return s.createStyle(ctx, src, rows, c, assets, "printstyle.imported", "导入", warns)
}

func mapKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
