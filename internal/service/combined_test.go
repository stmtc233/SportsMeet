package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func allZero() map[string]string {
	return map[string]string{"DNS": "zero", "DNF": "zero", "DQ": "zero", "NM": "zero"}
}
func combinedInput() CombinedInput {
	return CombinedInput{Name: "两项全能", Gender: "男", Components: []CombinedComponentInput{
		{Event: track(), Rule: PointRuleInput{Table: "12.345\t100\n13.000\t80", InvalidPolicies: allZero()}},
		{Event: EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"}, Rule: PointRuleInput{Table: "5.000\t100\n4.000\t80", InvalidPolicies: allZero()}},
	}}
}
func TestCombinedRosterScoringRulesAndRoundtrip(t *testing.T) {
	f := newFixture(t, 3, track())
	ordinary := f.event
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes[:2]))
	d, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if d.Complete || !d.Rows[0].Pending || len(d.Components) != 2 {
		t.Fatal("pending all-around")
	}
	if !strings.Contains(d.Components[0].Input.Table, "12.345") {
		t.Fatal("rule lost precision")
	}
	for _, part := range d.Components {
		f.event = part.Event.ID
		f.addHeat(0, f.athletes[:2]...)
		event := f.detail()
		inputs := []ResultInput{{Position: 1, Mark: "12.345"}, {Position: 2, Mark: "12.9"}}
		if part.Event.Kind == "distance" {
			inputs = []ResultInput{{Position: 1, Attempts: []string{"4.50"}}, {Position: 2, Attempts: []string{"5.10"}}}
		}
		must(t, f.svc.SaveResults(f.ctx, ui, event.Rounds[0].Heats[0].ID, inputs))
	}
	d, err = f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if !d.Complete || d.Rows[0].Total != 180 || d.Rows[1].Total != 180 || *d.Rows[0].Rank != 1 || *d.Rows[1].Rank != 1 {
		t.Fatalf("combined totals %+v", d.Rows)
	}
	ordinaryDetail, err := f.svc.GetEvent(f.ctx, ordinary)
	must(t, err)
	if len(ordinaryDetail.Rounds[0].Heats) > 0 {
		t.Fatal("ordinary event contaminated")
	}
	wantInvalid(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, nil), "已有分项编排")
	wantInvalid(t, f.svc.RemoveEntries(f.ctx, ui, d.Components[0].Event.ID, f.athletes[:1]), "全能")
	wantInvalid(t, f.svc.AddEntries(f.ctx, ui, d.Components[0].Event.ID, []EntryInput{{AthleteID: f.athletes[2]}}), "全能")
	wantInvalid(t, f.svc.DeleteEvent(f.ctx, ui, d.Components[0].Event.ID, true), "全能")
	req := CombinedRulesRequest{CombinedID: c.ID, Rules: []CombinedRuleChange{}}
	for i, p := range d.Components {
		r := p.Input
		if i == 0 {
			r.Table = "12.345\t120\n13\t80"
		}
		req.Rules = append(req.Rules, CombinedRuleChange{EventID: p.Event.ID, Rule: r})
	}
	preview, err := f.svc.PreviewCombinedRules(f.ctx, req)
	must(t, err)
	if !preview.Changed || preview.After[0].Total != 200 || preview.Before[0].Total != 180 {
		t.Fatal("rule preview")
	}
	_, err = f.svc.ApplyCombinedRules(f.ctx, ui, req, preview.PreviewHash, false)
	wantInvalid(t, err, "先预览")
	_, err = f.svc.ApplyCombinedRules(f.ctx, ui, req, preview.PreviewHash, true)
	must(t, err)
	_, err = f.svc.ApplyCombinedRules(f.ctx, ui, req, preview.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatal("reused confirmation", err)
	}
	d, err = f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if d.Combined.RuleVersion != 2 || d.Rows[0].Total != 200 {
		t.Fatal("rule persistence")
	}
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, []string{c.ID})
	must(t, err)
	if !strings.Contains(html, "正式排名") || !strings.Contains(html, "总分") || !strings.Contains(html, "200") {
		t.Fatal("combined report")
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(file)
	must(t, err)
	importPreview, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(importPreview.Errors) > 0 {
		t.Fatal(importPreview.Errors)
	}
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", importPreview.Hash)
	must(t, err)
	parents, err := f.svc.ListCombined(f.ctx, imported.MeetID)
	must(t, err)
	if len(parents) != 1 {
		t.Fatal(parents)
	}
	again, err := f.svc.GetCombined(f.ctx, parents[0].Combined.ID)
	must(t, err)
	if len(again.Rows) != 2 || again.Rows[0].Total != 200 || again.Combined.RuleVersion != 2 {
		t.Fatal("combined roundtrip")
	}
	reuse, err := f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(reuse.Errors) > 0 || reuse.Combineds.Reused != 1 {
		t.Fatal(reuse)
	}
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, reuse.Hash)
	must(t, err)
	wantInvalid(t, f.svc.DeleteCombined(f.ctx, store.SourceMCP, c.ID), "只能在界面")
	must(t, f.svc.DeleteCombined(f.ctx, ui, c.ID))
	events, err := f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	if len(events) != 1 || events[0].Event.ID != ordinary {
		t.Fatal("combined cascade affected standalone")
	}
}
func TestCombinedFailuresAreAtomic(t *testing.T) {
	f := newFixture(t, 2, track())
	in := combinedInput()
	in.Components[1].Rule.Table = "bad"
	_, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", in)
	wantInvalid(t, err, "计分表")
	parents, err := f.svc.ListCombined(f.ctx, f.meet)
	must(t, err)
	events, err := f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	if len(parents) != 0 || len(events) != 1 {
		t.Fatal("partial all-around creation")
	}
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	wantInvalid(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, []string{f.athletes[0], "missing"}), "不属于")
	d, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if len(d.AthleteIDs) != 0 {
		t.Fatal("partial roster")
	}
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes[:1]))
	in = combinedInput()
	in.Name = "新名称"
	_, err = f.svc.SaveCombined(f.ctx, ui, f.meet, c.ID, in)
	must(t, err)
	d, err = f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if len(d.AthleteIDs) != 1 || d.Combined.Name != "新名称" {
		t.Fatal("edit lost roster")
	}
}

func TestCombinedEntryDatabaseFailureRollsBackWholeRoster(t *testing.T) {
	f := newFixture(t, 1, track())
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	d, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	// A duplicate in the second component causes the batch to fail after its first writes.
	must(t, f.svc.store.Write(f.ctx, insertEntry(d.Components[1].Event.ID, f.athletes[0], "", nil)))
	if err := f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes); err == nil {
		t.Fatal("injected failure succeeded")
	}
	d, err = f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if len(d.AthleteIDs) != 0 {
		t.Fatal("partial parent roster committed")
	}
	child, err := f.svc.GetEvent(f.ctx, d.Components[0].Event.ID)
	must(t, err)
	if len(child.Entries) != 0 {
		t.Fatal("partial component roster committed")
	}
}

func TestPointRuleHeadersAndOrder(t *testing.T) {
	svc, _ := newTestService(t)
	in := EventInput{Gender: "男", Discipline: "100m", Kind: "track"}
	p, err := svc.ParsePointRule(in, PointRuleInput{Table: "成绩\t积分\n12.345\t100\n13\t80", InvalidPolicies: allZero()})
	must(t, err)
	if p.Rows[0].MarkText != "12.345" || len(p.Rows) != 2 {
		t.Fatal("header or precision")
	}
	_, err = svc.ParsePointRule(in, PointRuleInput{Table: "13\t80\n12\t100", InvalidPolicies: allZero()})
	wantInvalid(t, err, "档位")
}

func TestCombinedReportSplitsWideComponents(t *testing.T) {
	f := newFixture(t, 1, EventInput{})
	in := combinedInput()
	for _, name := range []string{"200m", "400m", "800m"} {
		part := combinedInput().Components[0]
		part.Event.Discipline = name
		in.Components = append(in.Components, part)
	}
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", in)
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes))
	book, err := f.svc.reportBook(f.ctx, f.meet, []string{c.ID}, false)
	must(t, err)
	tables := book.Combineds[0].Tables
	if len(tables) != 2 || len(tables[0].Headers) != 4 || len(tables[1].Headers) != 1 || len(tables[1].Rows[0].Cells) != 1 {
		t.Fatal("wide report not split")
	}
}
