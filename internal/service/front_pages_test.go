package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
)

func TestFrontPagesOrderAndKinds(t *testing.T) {
	f := newFixture(t, 1, track())
	ctx := f.ctx
	var ids []string
	for _, title := range []string{"致辞", "规程", "须知"} {
		r, err := f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: " " + title + " ", BodyMD: "# " + title})
		must(t, err)
		ids = append(ids, r.Page.ID)
		if r.Page.Title != title || len(r.Page.Reports) != 0 || r.Page.MeetID != f.meet {
			t.Fatalf("saved = %+v", r.Page)
		}
	}
	list, err := f.svc.ListFrontPages(ctx, f.meet)
	must(t, err)
	if len(list) != 3 || list[0].Position != 1 || list[2].Position != 3 || list[0].Title != "致辞" {
		t.Fatalf("list = %+v", list)
	}
	// Reports come in the reports' own order, once each.
	r, err := f.svc.SaveFrontPage(ctx, ui, f.meet, ids[1], FrontPageInput{Title: "规程", BodyMD: "新的", Reports: []string{"checkin", "results", "checkin"}})
	must(t, err)
	if strings.Join(r.Page.Reports, ",") != "results,checkin" || r.Page.Position != 2 || r.Page.BodyMD != "新的" {
		t.Fatalf("updated = %+v", r.Page)
	}
	notified := len(*f.changes)
	must(t, f.svc.ReorderFrontPages(ctx, ui, f.meet, []string{ids[2], ids[0], ids[1]}))
	if got := (*f.changes)[notified:]; len(got) != 1 || got[0] != (Change{Kind: "meets", MeetID: f.meet}) {
		t.Fatalf("changes = %+v", got)
	}
	list, _ = f.svc.ListFrontPages(ctx, f.meet)
	if list[0].ID != ids[2] || list[1].ID != ids[0] || list[2].ID != ids[1] || list[2].Position != 3 {
		t.Fatalf("after reorder = %+v", list)
	}
	wantInvalid(t, f.svc.ReorderFrontPages(ctx, ui, f.meet, ids[:2]), "全部")
	wantInvalid(t, f.svc.ReorderFrontPages(ctx, ui, f.meet, []string{ids[0], ids[0], ids[1]}), "重复")
	wantInvalid(t, f.svc.ReorderFrontPages(ctx, ui, f.meet, []string{ids[0], ids[1], "nope"}), "重复")
	// Deleting closes the gap.
	must(t, f.svc.DeleteFrontPage(ctx, ui, f.meet, ids[2]))
	list, _ = f.svc.ListFrontPages(ctx, f.meet)
	if len(list) != 2 || list[0].ID != ids[0] || list[0].Position != 1 || list[1].Position != 2 {
		t.Fatalf("after delete = %+v", list)
	}
	wantInvalid(t, f.svc.DeleteFrontPage(ctx, ui, f.meet, ids[2]), "不存在")
	// Validation writes nothing.
	for name, c := range map[string]struct {
		in   FrontPageInput
		want string
	}{
		"no title":     {FrontPageInput{BodyMD: "x"}, "标题"},
		"long title":   {FrontPageInput{Title: strings.Repeat("长", 61)}, "标题"},
		"unknown kind": {FrontPageInput{Title: "a", Reports: []string{"cover"}}, "没有名为"},
		"huge text":    {FrontPageInput{Title: "a", BodyMD: strings.Repeat("字", printstyle.MaxMarkdownBytes)}, "KB"},
	} {
		_, err := f.svc.SaveFrontPage(ctx, ui, f.meet, "", c.in)
		if err == nil {
			t.Errorf("%s accepted", name)
			continue
		}
		wantInvalid(t, err, c.want)
	}
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "nope", FrontPageInput{Title: "a"})
	wantInvalid(t, err, "不存在")
	_, err = f.svc.SaveFrontPage(ctx, ui, "nope", "", FrontPageInput{Title: "a"})
	wantInvalid(t, err, "赛事")
	if list, _ = f.svc.ListFrontPages(ctx, f.meet); len(list) != 2 {
		t.Fatalf("a refused save wrote: %d pages", len(list))
	}
	// At most 20.
	for len(list) < maxFrontPages {
		_, err := f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "页"})
		must(t, err)
		list = append(list, FrontPageView{})
	}
	_, err = f.svc.SaveFrontPage(ctx, ui, f.meet, "", FrontPageInput{Title: "页"})
	wantInvalid(t, err, "最多")
}

func TestFrontPageWarnsOfMissingImages(t *testing.T) {
	f := newFixture(t, 1, track())
	r, err := f.svc.SaveFrontPage(f.ctx, ui, f.meet, "", FrontPageInput{Title: "致辞", BodyMD: "![校徽](asset:logo) ![x](asset:%E6%A0%A1%E5%BE%BD)"})
	must(t, err)
	if len(r.Warnings) != 2 || !strings.Contains(r.Warnings[0]+r.Warnings[1], "校徽") {
		t.Fatalf("warnings = %v", r.Warnings)
	}
	_, err = f.svc.SaveAsset(f.ctx, ui, "meet", f.meet, "logo", "image/png", pngBytes(1))
	must(t, err)
	_, err = f.svc.SaveAsset(f.ctx, ui, "meet", f.meet, "校徽", "image/png", pngBytes(2))
	must(t, err)
	r, err = f.svc.SaveFrontPage(f.ctx, ui, f.meet, r.Page.ID, FrontPageInput{Title: "致辞", BodyMD: "![校徽](asset:logo) ![x](asset:%E6%A0%A1%E5%BE%BD)"})
	must(t, err)
	if len(r.Warnings) != 0 {
		t.Fatalf("warnings = %v", r.Warnings)
	}
}
