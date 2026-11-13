package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func previewApply(t *testing.T, svc *Service, op Mutation) *MutationPreview {
	t.Helper()
	p, err := svc.PreviewMutation(context.Background(), op)
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatalf("preview: %+v", p)
	}
	result, err := svc.ApplyMutation(context.Background(), op, p.PreviewHash, true)
	must(t, err)
	return result
}

func TestMCPWorkflowRoundTrip(t *testing.T) {
	// Complete flow on real SQLite: table -> entries -> prelim -> results -> final -> reports -> interchange.
	f := newFixture(t, 0, EventInput{Gender: "男", Discipline: "100m", Kind: "track", Rounds: []RoundInput{{Stage: "heat", AdvanceByPlace: 1, AdvanceByMark: 1}}})
	table, err := ParseTable("姓名,性别,号码,单位,项目\n甲,男,101,一班,100m\n乙,男,102,二班,100m\n丙,男,103,一班,100m\n丁,男,104,二班,100m")
	must(t, err)
	previewApply(t, f.svc, Mutation{Action: "import_athletes", MeetID: f.meet, Table: AthleteImport{Rows: table.Rows[1:], Mapping: table.Mapping}})
	d := f.detail()
	if len(d.Entries) != 4 {
		t.Fatalf("entries: %+v", d.Entries)
	}
	op := Mutation{Action: "auto_arrange", Plan: PlanRequest{EventID: f.event, RoundID: d.Rounds[0].ID, PerHeat: 2, Order: arrange.OrderSeed, Lanes: arrange.LanesCenter, Seed: 19}}
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	if len(f.detail().Rounds[0].Heats) != 0 {
		t.Fatal("preview wrote heats")
	}
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, false)
	wantInvalid(t, err, "预览")
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	must(t, err)
	d = f.detail()
	if len(d.Rounds[0].Heats) != 2 {
		t.Fatal("missing preliminary heats")
	}
	// Repeat auto-arrange: persisted placements must be excluded even with an empty Exclude list.
	previewApply(t, f.svc, op)
	if len(f.detail().Rounds[0].Heats) != 2 {
		t.Fatal("auto-arrange duplicated entrants")
	}
	for i, h := range d.Rounds[0].Heats {
		inputs := []ResultInput{}
		for j, sl := range h.Slots {
			inputs = append(inputs, ResultInput{Position: sl.Position, Mark: []string{"12.10", "13.20", "12.30", "14.40"}[i*2+j]})
		}
		_, err := f.svc.ApplyMutation(f.ctx, Mutation{Action: "set_results", HeatID: h.ID, Results: inputs}, "", false)
		must(t, err)
	}
	d = f.detail()
	previewApply(t, f.svc, Mutation{Action: "advance", Plan: PlanRequest{EventID: f.event, RoundID: d.Rounds[1].ID, PerHeat: 8, Order: arrange.OrderSeed, Lanes: arrange.LanesCenter}})
	d = f.detail()
	if len(d.Rounds[1].Heats) != 1 || len(d.Rounds[1].Heats[0].Slots) != 3 {
		t.Fatal("advance missing")
	}
	for _, kind := range []string{ReportStartList, ReportResults} {
		html, err := f.svc.RenderReport(f.ctx, kind, f.meet, nil)
		must(t, err)
		if !strings.Contains(html, "男子100m") || !strings.Contains(html, "tabular-nums") {
			t.Fatal("report incomplete")
		}
	}
	original, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(original)
	must(t, err)
	imported := previewApply(t, f.svc, Mutation{Action: "import_meet", Data: string(data)})
	importedID := imported.Result.(*ImportPreview).MeetID
	if importedID == "" || importedID == f.meet {
		t.Fatal("import must return the committed meet id")
	}
	clone, err := f.svc.ExportMeet(f.ctx, importedID)
	must(t, err)
	if len(clone.Events) != 1 || len(clone.Athletes) != 4 {
		t.Fatal("roundtrip missing data")
	}
	// Identity IDs are remapped; compare full results, including ranks and Q/q.
	for i, r := range original.Events[0].Rounds {
		for j, h := range r.Heats {
			for k, sl := range h.Slots {
				a, _ := json.Marshal(sl.Result)
				b, _ := json.Marshal(clone.Events[0].Rounds[i].Heats[j].Slots[k].Result)
				if string(a) != string(b) {
					t.Fatalf("results roundtrip: %s != %s", a, b)
				}
			}
		}
	}
	logs, err := f.svc.store.Query(f.ctx, `SELECT count(*) FROM audit_log WHERE source = 'mcp'`)
	must(t, err)
	if logs.Values[0][0].(int64) < 5 {
		t.Fatal("missing MCP audits")
	}
}

func TestMCPResultsConfirmationAndStale(t *testing.T) {
	f := newFixture(t, 2, track())
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0]
	op := Mutation{Action: "set_results", HeatID: h.ID, Results: []ResultInput{{Position: 1, Mark: "12.00"}, {Position: 2, Status: "DNS"}}}
	_, err := f.svc.ApplyMutation(f.ctx, op, "", false)
	must(t, err)
	op.Results = []ResultInput{{Position: 1, Mark: "11.00"}}
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	_, err = f.svc.ApplyMutation(f.ctx, op, "", false)
	wantInvalid(t, err, "确认")
	must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 2, Status: "DNF"}}))
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("stale: %v", err)
	}
	previewApply(t, f.svc, op)
	if f.detail().Rounds[0].Heats[0].Slots[0].Result.MarkText != "11.00" {
		t.Fatal("overwrite not committed")
	}
	op.Results = []ResultInput{{Position: 1}}
	p, err = f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	if len(p.Errors) == 0 || p.PreviewHash != "" {
		t.Fatal("MCP must not clear results")
	}
	// Changing only the note is audited with full previous and next records.
	op.Results = []ResultInput{{Position: 1, Mark: "11.00", Note: "裁判复核"}}
	previewApply(t, f.svc, op)
	rows, err := f.svc.store.Query(f.ctx, `SELECT detail_json FROM audit_log WHERE action = 'results.saved' ORDER BY rowid DESC LIMIT 1`)
	must(t, err)
	if !strings.Contains(rows.Values[0][0].(string), "裁判复核") || !strings.Contains(rows.Values[0][0].(string), "beforeRecord") {
		t.Fatal("incomplete result audit")
	}
}

func TestMCPDeletesAndRace(t *testing.T) {
	f := newFixture(t, 2, track())
	f.addHeat(0, f.athletes[0])
	h := f.detail().Rounds[0].Heats[0]
	op := Mutation{Action: "delete_heat", HeatID: h.ID}
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 1, Mark: "12.0"}}))
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if err == nil {
		t.Fatal("deleted a heat after results arrived")
	}
	for _, op := range []Mutation{{Action: "delete_meet", MeetID: f.meet}, {Action: "delete_event", EventID: f.event}, {Action: "delete_heat", HeatID: h.ID}, {Action: "delete_athletes", MeetID: f.meet, IDs: []string{f.athletes[0]}}} {
		p, err := f.svc.PreviewMutation(f.ctx, op)
		must(t, err)
		if len(p.Errors) == 0 {
			t.Fatalf("unsafe delete preview: %+v", op)
		}
	}
	// Two simultaneous confirmations of one deletion: exactly one can commit.
	raceSvc := New(f.svc.store, nil)
	del := Mutation{Action: "delete_athletes", MeetID: f.meet, IDs: []string{f.athletes[1]}}
	p, err = raceSvc.PreviewMutation(f.ctx, del)
	must(t, err)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := raceSvc.ApplyMutation(f.ctx, del, p.PreviewHash, true); results <- e }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent delete successes: %d", successes)
	}
}

func TestPreferencesPersistAndPrint(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	must(t, err)
	svc := New(db, nil)
	prefs, err := svc.Preferences(ctx)
	must(t, err)
	if prefs.Theme != "system" || !prefs.Balance || !prefs.SeparateUnits {
		t.Fatal("defaults")
	}
	prefs.Theme = "dark"
	prefs.PrintNewPage = true
	must(t, svc.SavePreferences(ctx, prefs))
	cfg, err := PrepareMCPConfig(MCPConfig{Enabled: true, Port: 18765}, false)
	must(t, err)
	must(t, svc.SaveMCPConfig(ctx, cfg))
	must(t, db.Close())
	db, err = store.Open(ctx, dir)
	must(t, err)
	defer db.Close()
	svc = New(db, nil)
	got, err := svc.Preferences(ctx)
	must(t, err)
	if got != prefs {
		t.Fatal("preferences lost")
	}
	saved, err := svc.MCPConfig(ctx)
	must(t, err)
	if saved != cfg || len(saved.Token) != 64 {
		t.Fatal("MCP settings lost")
	}
	logs, err := db.Query(ctx, `SELECT detail_json, message FROM audit_log`)
	must(t, err)
	for _, r := range logs.Values {
		if strings.Contains(r[0].(string)+r[1].(string), cfg.Token) {
			t.Fatal("token leaked to audit")
		}
	}
	m, err := svc.CreateMeet(ctx, ui, MeetInput{Name: "打印", StartDate: "2026-11-12"})
	must(t, err)
	html, err := svc.RenderReport(ctx, ReportStartList, m.ID, nil)
	must(t, err)
	if !strings.Contains(html, "break-before: page") {
		t.Fatal("print preference ignored")
	}
}

func TestImportFingerprintIncludesFileAndTarget(t *testing.T) {
	f := newFixture(t, 1, track())
	original, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(original)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	original.Meet.Note = "文件已修改"
	changed, err := interchange.Encode(original)
	must(t, err)
	_, err = f.svc.ImportMeet(f.ctx, ui, changed, f.meet, p.Hash)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("changed file: %v", err)
	}
	// A target note change does not change summary counts, but must invalidate the preview.
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "秋季运动会", StartDate: "2026-11-12", Note: "赛事已修改"}))
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p.Hash)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("changed target: %v", err)
	}
}

func TestMCPImportRejectsGenderMixing(t *testing.T) {
	f := newFixture(t, 1, track())
	f.enter(f.athletes[0])
	f.addHeat(0, f.athletes[0])
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	file.Athletes[0].Gender = "女"
	file.Events[0].Rounds[0].Heats[0].Name = "女子第一组"
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewMutation(f.ctx, Mutation{Action: "import_meet", Data: string(data)})
	must(t, err)
	if len(p.Errors) == 0 || p.PreviewHash != "" {
		t.Fatal("gender-mixed import can be confirmed")
	}
	normal, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(normal.Errors) != 0 || len(normal.Warnings) == 0 {
		t.Fatal("general interchange import must warn")
	}
}

func TestMCPFieldRecordsAndRollback(t *testing.T) {
	for _, kind := range []string{"distance", "height"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "田赛", Kind: kind})
			f.addHeat(0, f.athletes...)
			h := f.detail().Rounds[0].Heats[0]
			result := ResultInput{Position: 1, Attempts: []string{"5.10", "X", "5.30"}}
			if kind == "height" {
				result = ResultInput{Position: 1, Bars: []BarInput{{Height: "1.50", Trials: "O"}, {Height: "1.55", Trials: "XO"}}}
			}
			op := Mutation{Action: "set_results", HeatID: h.ID, Results: []ResultInput{result, {Position: 2, Mark: "invalid"}}}
			p, err := f.svc.PreviewMutation(f.ctx, op)
			must(t, err)
			if len(p.Errors) == 0 || f.detail().Rounds[0].Heats[0].Slots[0].Result != nil {
				t.Fatal("invalid batch partially wrote")
			}
			op.Results = op.Results[:1]
			_, err = f.svc.ApplyMutation(f.ctx, op, "", false)
			must(t, err)
			got := f.detail().Rounds[0].Heats[0].Slots[0].Result
			if got == nil || *got.Rank != 1 {
				t.Fatal("field result not ranked")
			}
		})
	}
}

func TestRelayImportIncludesTeams(t *testing.T) {
	f := newFixture(t, 0, EventInput{})
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	file.Events = []interchange.Event{{ID: "relay", Gender: "男", Discipline: "4×100m", Kind: "relay", Teams: []interchange.Team{{ID: "team", Name: "一班", MemberIDs: []string{}}}, Entries: []interchange.Entry{{TeamID: "team"}}, Rounds: []interchange.Round{}}}
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(p.Errors) != 0 || p.Teams.New != 1 || p.Entries != 1 {
		t.Fatalf("relay preview: %+v", p)
	}
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", p.Hash)
	must(t, err)
	events, err := f.svc.ListEvents(f.ctx, imported.MeetID)
	must(t, err)
	if len(events) != 1 || events[0].Event.Kind != domain.KindRelay {
		t.Fatal("relay not imported")
	}
}

func TestDuplicateAthleteNameWarnsButIdentityClashRefuses(t *testing.T) {
	f := newFixture(t, 1, track())
	saved, err := f.svc.SaveAthleteWithWarnings(f.ctx, store.SourceMCP, f.meet, "", AthleteInput{Name: "选手01", Gender: "男", Bib: "002"})
	must(t, err)
	if len(saved.Warnings) != 1 || saved.Athlete.ID == f.athletes[0] {
		t.Fatal("name warning must allow a distinct athlete")
	}
	_, err = f.svc.SaveAthleteWithWarnings(f.ctx, store.SourceMCP, f.meet, "", AthleteInput{Name: "选手01", Gender: "男", Bib: "001"})
	wantInvalid(t, err, "改为更新")
}
