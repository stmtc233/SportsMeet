package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// planPrintContent adds the statements for a file's front pages and images
// (interchange v11) to an import plan.
//
// A new meet takes them all, in the file's order. Importing into an
// existing meet only adds, as for everything else the meet already has:
//   - a front page identical in title and text to one the meet has is
//     reused, the others go to the end of the meet's pages (together at
//     most 20; more is an error, the file is refused);
//   - an image whose name the meet lacks is added, one with the same name
//     and the same bytes is reused, one with the same name and other bytes
//     keeps the meet's image and warns (the meet's image is the one in
//     use, and the front pages of the file may then show it);
//   - the meet's own pages, their order and images are never changed or
//     removed.
func (s *Service) planPrintContent(ctx context.Context, plan *importPlan, f *interchange.File, now int64) error {
	p := &plan.preview
	if len(f.FrontPages) == 0 && len(f.Assets) == 0 {
		return nil
	}
	var havePages []store.FrontPageRow
	var haveAssets []store.AssetRow
	if !p.NewMeet {
		var err error
		if havePages, err = s.store.ListFrontPages(ctx, plan.meetID); err != nil {
			return err
		}
		if haveAssets, err = s.store.ListAssets(ctx, store.AssetOfMeet, plan.meetID, false); err != nil {
			return err
		}
	}
	position := len(havePages)
	for _, page := range f.FrontPages {
		reused := false
		for _, e := range havePages {
			if e.Title == strings.TrimSpace(page.Title) && e.BodyMD == page.BodyMD {
				reused = true
			}
		}
		if reused {
			p.FrontPages.Reused++
			continue
		}
		position++
		if position > maxFrontPages {
			p.Errors = append(p.Errors, fmt.Sprintf("前置页合计超过 %d 页", maxFrontPages))
			break
		}
		in, err := checkFrontPage(FrontPageInput{Title: page.Title, BodyMD: page.BodyMD, Reports: page.Reports})
		if err != nil {
			p.Errors = append(p.Errors, err.Error())
			continue
		}
		plan.stmts = append(plan.stmts, store.InsertFrontPage(store.FrontPageRow{ID: newID(), MeetID: plan.meetID, Position: position, Title: in.Title,
			BodyMD: in.BodyMD, ReportsJSON: jsonValue(in.Reports), CreatedAt: now, UpdatedAt: now}))
		p.FrontPages.New++
	}

	count, total := len(haveAssets), 0
	for _, a := range haveAssets {
		total += a.Size
	}
	for _, a := range f.Assets {
		data, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("图片「%s」的数据不是 base64", a.Name))
			continue
		}
		sha := assetHash(data)
		var have *store.AssetRow
		for i := range haveAssets {
			if haveAssets[i].Name == a.Name {
				have = &haveAssets[i]
			}
		}
		switch {
		case have != nil && have.SHA256 == sha:
			p.Assets.Reused++
		case have != nil:
			p.Assets.Reused++
			p.Warnings = append(p.Warnings, fmt.Sprintf("图片「%s」与赛事里的同名图片内容不同，保留赛事里的", a.Name))
		default:
			count++
			total += len(data)
			plan.stmts = append(plan.stmts, store.UpsertAsset(store.AssetOfMeet, store.AssetRow{ID: newID(), OwnerID: plan.meetID, Name: a.Name, MIME: a.MIME, SHA256: sha, Data: data}))
			p.Assets.New++
		}
	}
	if count > printstyle.MaxAssets {
		p.Errors = append(p.Errors, fmt.Sprintf("图片合计超过 %d 张", printstyle.MaxAssets))
	}
	if total > printstyle.MaxAssetsBytes {
		p.Errors = append(p.Errors, fmt.Sprintf("图片合计超过 %d MB", printstyle.MaxAssetsBytes>>20))
	}
	return nil
}
