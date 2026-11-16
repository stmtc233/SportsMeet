package service

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// A meet made from another keeps the choice of print style, the front
// pages and the images; none of them is shared with the source.
func TestCreateMeetFromExistingCopiesPrintContent(t *testing.T) {
	f := newFixture(t, 2, track())
	ctx := f.ctx
	style := newStyle(t, f.svc, "校运会", ".copied{color:red}").Style.ID
	must(t, f.svc.SetMeetPrintStyle(ctx, ui, f.meet, style))
	settings, _ := f.svc.MeetSettings(ctx, f.meet)
	settings.MinRestMinutes = 30
	must(t, f.svc.SaveMeetSettings(ctx, ui, f.meet, settings))
	_, err := f.svc.SaveAsset(ctx, ui, "meet", f.meet, "logo", "image/png", pngBytes(1))
	must(t, err)
	a, err := f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "![x](asset:logo)", Reports: []string{"startlist"}})
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "规程", BodyMD: "第二页"})
	must(t, err)

	in := MeetInput{SourceMeetID: f.meet, Name: "春季运动会", StartDate: "2027-04-20"}
	plan, err := f.svc.PreviewMutation(ctx, Mutation{Action: "create_meet", MeetInput: &in})
	must(t, err)
	if len(plan.Errors) > 0 {
		t.Fatalf("preview: %+v", plan)
	}
	meet, err := f.svc.CreateMeet(ctx, ui, in)
	must(t, err)
	prefs, err := f.svc.MeetSettings(ctx, meet.ID)
	must(t, err)
	if prefs.PrintStyleID != style || prefs.MinRestMinutes != 0 || !prefs.SeparateUnits {
		t.Fatalf("settings = %+v", prefs)
	}
	pages, err := f.svc.ListFrontPages(ctx, meet.ID)
	must(t, err)
	if len(pages) != 2 || pages[0].ID == a.Page.ID || pages[0].Title != "致辞" || pages[0].Position != 1 || pages[1].Title != "规程" ||
		strings.Join(pages[0].Reports, ",") != "startlist" || pages[0].MeetID != meet.ID {
		t.Fatalf("pages = %+v", pages)
	}
	assets, err := f.svc.ListAssets(ctx, "meet", meet.ID)
	must(t, err)
	if len(assets) != 1 || assets[0].Name != "logo" || assets[0].SHA256 != assetHash(pngBytes(1)) {
		t.Fatalf("assets = %+v", assets)
	}
	// Independent: change the copy, the source stays.
	_, err = f.svc.SaveAsset(ctx, ui, "meet", meet.ID, "logo", "image/png", pngBytes(2))
	must(t, err)
	must(t, f.svc.DeleteFrontPage(ctx, ui, meet.ID, pages[1].ID))
	src, _ := f.svc.ListAssets(ctx, "meet", f.meet)
	srcPages, _ := f.svc.ListFrontPages(ctx, f.meet)
	if src[0].SHA256 != assetHash(pngBytes(1)) || len(srcPages) != 2 {
		t.Fatal("the source meet changed")
	}
	// The preview covers them: changing a front page of the source changes the hash.
	p1, err := f.svc.PreviewMutation(ctx, Mutation{Action: "create_meet", MeetInput: &in})
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, srcPages[1].ID, FrontPageInput{Title: "规程", BodyMD: "改了"})
	must(t, err)
	p2, err := f.svc.PreviewMutation(ctx, Mutation{Action: "create_meet", MeetInput: &in})
	must(t, err)
	if p1.PreviewHash == p2.PreviewHash {
		t.Fatal("the preview hash ignores front pages")
	}
	// Deleting the source leaves the new meet's own copies and the style.
	must(t, f.svc.DeleteMeet(ctx, ui, f.meet))
	if pages, _ = f.svc.ListFrontPages(ctx, meet.ID); len(pages) != 1 {
		t.Fatalf("pages after source delete = %+v", pages)
	}
	if d, err := f.svc.GetPrintStyle(ctx, style); err != nil || len(d.Meets) != 1 {
		t.Fatalf("style = %+v, %v", d, err)
	}
}

func TestExportImportFrontPagesAndAssets(t *testing.T) {
	f := newFixture(t, 2, track())
	ctx := f.ctx
	_, err := f.svc.SaveAsset(ctx, ui, "meet", f.meet, "logo", "image/png", pngBytes(1))
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "![x](asset:logo)"})
	must(t, err)
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "须知", BodyMD: "请检录", Reports: []string{"checkin", "startlist"}})
	must(t, err)
	file, err := f.svc.ExportMeet(ctx, f.meet)
	must(t, err)
	if file.Version != interchange.Version || len(file.FrontPages) != 2 || file.FrontPages[1].Reports[0] != "startlist" || len(file.Assets) != 1 || file.Assets[0].MIME != "image/png" {
		t.Fatalf("export = %+v %+v", file.FrontPages, file.Assets)
	}
	data, err := interchange.Encode(file)
	must(t, err)

	// A new meet: everything, in order; exporting it again gives the same.
	p, err := f.svc.PreviewImport(ctx, data, "")
	must(t, err)
	if len(p.Errors) != 0 || p.FrontPages != (Tally{New: 2}) || p.Assets != (Tally{New: 1}) {
		t.Fatalf("preview = %+v", p)
	}
	done, err := f.svc.ImportMeet(ctx, ui, data, "", p.Hash)
	must(t, err)
	again, err := f.svc.ExportMeet(ctx, done.MeetID)
	must(t, err)
	if strip(t, file) != strip(t, again) {
		t.Fatal("front pages and images changed in the round trip")
	}
	// Importing a style's choice is not part of the file.
	if prefs, _ := f.svc.MeetSettings(ctx, done.MeetID); prefs.PrintStyleID != "" {
		t.Fatal("print style chosen by an import")
	}

	// Into the meet it came from: everything is reused, nothing is written.
	p, err = f.svc.PreviewImport(ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) != 0 || p.FrontPages != (Tally{Reused: 2}) || p.Assets != (Tally{Reused: 1}) || len(p.Warnings) != 0 {
		t.Fatalf("re-import preview = %+v", p)
	}

	// Into another meet: pages go after the meet's own, its images are kept.
	g := newFixture(t, 2, track())
	_, err = g.svc.SaveFrontPage(g.ctx, ui, g.meet, "", FrontPageInput{Title: "本赛事的页", BodyMD: "自己的"})
	must(t, err)
	_, err = g.svc.SaveAsset(g.ctx, ui, "meet", g.meet, "logo", "image/png", pngBytes(9))
	must(t, err)
	p, err = g.svc.PreviewImport(g.ctx, data, g.meet)
	must(t, err)
	if len(p.Errors) != 0 || p.FrontPages != (Tally{New: 2}) || p.Assets != (Tally{Reused: 1}) || len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "logo") {
		t.Fatalf("preview into another meet = %+v", p)
	}
	_, err = g.svc.ImportMeet(g.ctx, ui, data, g.meet, p.Hash)
	must(t, err)
	pages, _ := g.svc.ListFrontPages(g.ctx, g.meet)
	if len(pages) != 3 || pages[0].Title != "本赛事的页" || pages[1].Title != "致辞" || pages[2].Title != "须知" || pages[2].Position != 3 {
		t.Fatalf("pages = %+v", pages)
	}
	if a, _ := g.svc.store.GetAsset(g.ctx, store.AssetOfMeet, g.meet, "logo"); a == nil || a.SHA256 != assetHash(pngBytes(9)) {
		t.Fatal("the meet's own image was replaced")
	}
	// Imported twice: no more pages.
	p, err = g.svc.PreviewImport(g.ctx, data, g.meet)
	must(t, err)
	if p.FrontPages != (Tally{Reused: 2}) {
		t.Fatalf("second preview = %+v", p)
	}
	// Past 20 pages the file is refused.
	for len(pages) < maxFrontPages {
		_, err = g.svc.SaveFrontPage(g.ctx, ui, g.meet, "", FrontPageInput{Title: "页", BodyMD: "x"})
		must(t, err)
		pages = append(pages, FrontPageView{})
	}
	h := newFixture(t, 1, track())
	for i := range maxFrontPages {
		_, err = h.svc.SaveFrontPage(h.ctx, ui, h.meet, "", FrontPageInput{Title: "文件页", BodyMD: fmt.Sprint("别处的", i)})
		must(t, err)
	}
	full, _ := h.svc.ExportMeet(h.ctx, h.meet)
	fullData, _ := interchange.Encode(full)
	p, err = g.svc.PreviewImport(g.ctx, fullData, g.meet)
	must(t, err)
	if len(p.Errors) == 0 {
		t.Fatalf("20+ front pages accepted: %+v", p)
	}
	if _, err = g.svc.ImportMeet(g.ctx, ui, fullData, g.meet, p.Hash); err == nil {
		t.Fatal("imported past the limit")
	}
}

func TestImportOldVersionsLeavesFrontPagesAlone(t *testing.T) {
	f := newFixture(t, 1, track())
	_, err := f.svc.SaveFrontPage(f.ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "你好"})
	must(t, err)
	v10, err := os.ReadFile("../../testdata/interchange/sample-meet-v10.json")
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, v10, f.meet)
	must(t, err)
	if p.FrontPages != (Tally{}) || p.Assets != (Tally{}) {
		t.Fatalf("a v10 file touched them: %+v", p)
	}
	// A v10 file as a new meet has none; the v11 golden has two pages and an image.
	p, err = f.svc.PreviewImport(f.ctx, v10, "")
	must(t, err)
	done, err := f.svc.ImportMeet(f.ctx, ui, v10, "", p.Hash)
	must(t, err)
	if pages, _ := f.svc.ListFrontPages(f.ctx, done.MeetID); len(pages) != 0 {
		t.Fatalf("pages = %+v", pages)
	}
	v11, err := os.ReadFile("../../testdata/interchange/sample-meet-v11.json")
	must(t, err)
	p, err = f.svc.PreviewImport(f.ctx, v11, "")
	must(t, err)
	if len(p.Errors) != 0 || p.FrontPages.New != 2 || p.Assets.New != 1 {
		t.Fatalf("v11 golden preview = %+v", p)
	}
	done, err = f.svc.ImportMeet(f.ctx, ui, v11, "", p.Hash)
	must(t, err)
	pages, _ := f.svc.ListFrontPages(f.ctx, done.MeetID)
	if len(pages) != 2 || pages[0].Title != "致辞" || strings.Join(pages[1].Reports, ",") != "checkin,startlist" && strings.Join(pages[1].Reports, ",") != "startlist,checkin" {
		t.Fatalf("pages = %+v", pages)
	}
}
