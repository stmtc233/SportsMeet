package service

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func addGroup(t *testing.T, f *fixture, name string) string {
	t.Helper()
	g, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: name})
	must(t, err)
	return g.ID
}
func setUnitGroups(t *testing.T, f *fixture, id string, ids ...string) {
	t.Helper()
	units, err := f.svc.ListUnits(f.ctx, f.meet)
	must(t, err)
	for _, u := range units {
		if u.ID == id {
			_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, id, UnitInput{Name: u.Name, ShortName: u.ShortName, GroupIDs: ids})
			must(t, err)
			return
		}
	}
	t.Fatal("unit missing")
}
func gradeFixture(t *testing.T) (*fixture, string, string, string, string) {
	t.Helper()
	f := newFixture(t, 4, EventInput{})
	g1, g2 := addGroup(t, f, "初一"), addGroup(t, f, "初二")
	setUnitGroups(t, f, f.units[0], g1)
	setUnitGroups(t, f, f.units[1], g2)
	ids := []string{}
	for _, gid := range []string{g1, g2} {
		in := track()
		in.GroupID = gid
		e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", in)
		must(t, err)
		ids = append(ids, e.ID)
		f.event = e.ID
		index := 0
		if gid == g2 {
			index = 1
		}
		f.addHeat(0, f.athletes[index], f.athletes[index+2])
		must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "12"}, {Position: 2, Mark: "13"}}))
	}
	enableTeamScores(t, f, domain.DefaultTeamScoringRules())
	return f, g1, g2, ids[0], ids[1]
}

func TestGroupBoardsRenameDefaultsAndPrint(t *testing.T) {
	f, g1, g2, e1, e2 := gradeFixture(t)
	zero, err := f.svc.SaveUnit(f.ctx, ui, f.meet, "", UnitInput{Name: "初一3班", ShortName: "3班", GroupIDs: []string{g1}})
	must(t, err)
	boards, err := f.svc.GetTeamScoreBoards(f.ctx, TeamScoreFilter{MeetID: f.meet})
	must(t, err)
	if len(boards) != 2 || boards[0].GroupID != g1 || len(boards[0].Detail.Rows) != 2 || len(boards[1].Detail.Rows) != 1 || boards[0].Detail.Rows[0].Total.Text != "16" || boards[1].Detail.Rows[0].Total.Text != "16" || *boards[1].Detail.Rows[0].Rank != 1 || unitScore(t, boards[0].Detail, zero.ID).Total.Text != "0" {
		t.Fatalf("grade boards %+v", boards)
	}
	a, err := f.svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: "新生", Gender: "男", UnitID: f.units[0]})
	must(t, err)
	if a.GroupID != g1 {
		t.Fatal("unit's unique group not defaulted")
	}
	_, err = f.svc.SaveGroup(f.ctx, ui, f.meet, g1, GroupInput{Name: "七年级"})
	must(t, err)
	d, err := f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if d.Event.GroupID != g1 || d.Event.GroupLabel != "七年级" || d.Event.Name != "男子100m 七年级" || d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "12.00" {
		t.Fatal("rename lost name/results", d.Event)
	}
	in := track()
	in.GroupID = g2
	in.Name = "高年级短跑"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, e2, in)
	must(t, err)
	_, err = f.svc.SaveGroup(f.ctx, ui, f.meet, g2, GroupInput{Name: "八年级"})
	must(t, err)
	d, err = f.svc.GetEvent(f.ctx, e2)
	must(t, err)
	if d.Event.Name != "高年级短跑" {
		t.Fatal("custom name replaced")
	}
	must(t, f.svc.ReorderGroups(f.ctx, ui, f.meet, []string{g2, g1}))
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if strings.Count(html, `class="report-group"`) != 2 || strings.Index(html, `class="report-group__title">八年级`) > strings.Index(html, `class="report-group__title">七年级`) {
		t.Fatal("group print order")
	}
	if path := os.Getenv("SPORTSMEET_GROUP_REPORT_OUTPUT"); path != "" {
		must(t, os.WriteFile(path, []byte(html), 0o644))
	}
	html, err = f.svc.RenderGroupedReport(f.ctx, GroupReportRequest{Kind: ReportResults, MeetID: f.meet, GroupIDs: []string{g1}})
	must(t, err)
	if strings.Contains(html, "高年级短跑") || !strings.Contains(html, "七年级") || !strings.Contains(html, "团体总分表") {
		t.Fatal("group print scope")
	}

	html, err = f.svc.RenderGroupedReport(f.ctx, GroupReportRequest{Kind: ReportTeamScores, MeetID: f.meet, GroupIDs: []string{g1}, EventIDs: []string{e1}})
	must(t, err)
	if !strings.Contains(html, "所选项目") || strings.Contains(html, "八年级") {
		t.Fatal("partial group scores")
	}
	_, err = f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: " 七年级 "})
	wantInvalid(t, err, "同名")
	// Deleting a group in use leaves its events ungrouped with their records.
	must(t, f.svc.DeleteGroup(f.ctx, ui, f.meet, g1))
	d, err = f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if d.Event.GroupID != "" || d.Event.GroupLabel != "" || len(d.Entries) == 0 {
		t.Fatal("deleted group still referenced", d.Event)
	}
	units, err := f.svc.ListUnits(f.ctx, f.meet)
	must(t, err)
	for _, u := range units {
		if slices.Contains(u.GroupIDs, g1) {
			t.Fatal("unit keeps deleted group")
		}
	}
}

func TestGroupProjectChangesAreConfirmedAtomicAndKeepResults(t *testing.T) {
	f, g1, g2, e1, _ := gradeFixture(t)
	_, err := f.svc.PreviewGroupChange(f.ctx, GroupChangeRequest{MeetID: f.meet, GroupID: g2, EventIDs: []string{e1}})
	wantInvalid(t, err, "相同项目")
	g3 := addGroup(t, f, "初三")
	req := GroupChangeRequest{MeetID: f.meet, GroupID: g3, EventIDs: []string{e1}}
	p, err := f.svc.PreviewGroupChange(f.ctx, req)
	must(t, err)
	if len(p.Changes) != 1 || p.Changes[0].Before[0] != "初一" || p.After[2].Detail.Rows[0].Total.Text != "16" {
		t.Fatal("move preview")
	}
	_, err = f.svc.ApplyGroupChange(f.ctx, ui, req, p.PreviewHash, false)
	wantInvalid(t, err, "先预览")
	setUnitGroups(t, f, f.units[0], g1, g3)
	_, err = f.svc.ApplyGroupChange(f.ctx, ui, req, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatal("stale group move", err)
	}
	p, err = f.svc.PreviewGroupChange(f.ctx, req)
	must(t, err)
	_, err = f.svc.ApplyGroupChange(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	d, err := f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if d.Event.GroupID != g3 || len(d.Entries) != 2 || len(d.Rounds[0].Heats) != 1 || d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "12.00" {
		t.Fatal("group move modified competition records")
	}
	// Editing the event moves it directly, keeping entries, heats and results.
	in := track()
	in.GroupID = g1
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, e1, in)
	must(t, err)
	d, err = f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if d.Event.GroupID != g1 || len(d.Entries) != 2 || d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "12.00" {
		t.Fatal("event edit modified competition records")
	}
	must(t, f.svc.ChangeGroup(f.ctx, ui, GroupChangeRequest{MeetID: f.meet, GroupID: g3, EventIDs: []string{e1}}))
	d, err = f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if d.Event.GroupID != g3 || len(d.Entries) != 2 {
		t.Fatal("direct group change")
	}
	other := newFixture(t, 0, EventInput{})
	foreign := addGroup(t, other, "异赛事")
	_, err = f.svc.PreviewGroupChange(f.ctx, GroupChangeRequest{MeetID: f.meet, GroupID: foreign, EventIDs: []string{e1}})
	wantInvalid(t, err, "不属于")
}

func TestGroupAssignmentsSuggestionsImportMatchingAndCrossGroup(t *testing.T) {
	f, g1, g2, e1, _ := gradeFixture(t)
	req := GroupAssignmentsRequest{MeetID: f.meet, Assignments: []GroupAssignment{{Kind: "athlete", ID: f.athletes[0], GroupIDs: []string{g2}}}}
	p, err := f.svc.PreviewGroupAssignments(f.ctx, req)
	must(t, err)
	_, err = f.svc.ApplyGroupAssignments(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	d, err := f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if !d.Entries[0].GroupMismatch || len(d.Rounds[0].Heats) != 1 {
		t.Fatal("cross group membership warning")
	}
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[0], UnitInput{Name: "初一1班", ShortName: "1班", GroupIDs: []string{g1}})
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[1], UnitInput{Name: "初二1班", ShortName: "1班", GroupIDs: []string{g2}})
	must(t, err)
	input := AthleteImport{Mapping: []string{ColName, ColGender, ColUnit, ColGroup, ColEvents}, Rows: [][]string{{"新选手", "男", "初一1班", "初一", "100m"}}}
	preview, err := f.svc.PreviewAthleteImport(f.ctx, f.meet, input)
	must(t, err)
	if preview.Errors != 0 || preview.Entries != 1 {
		t.Fatal("group matching", preview.Rows)
	}
	_, err = f.svc.ImportAthletes(f.ctx, ui, f.meet, input)
	must(t, err)
	d, err = f.svc.GetEvent(f.ctx, e1)
	must(t, err)
	if len(d.Entries) != 3 {
		t.Fatal("wrong grade event matched")
	}
	input.Rows = [][]string{{"未知单位", "男", "1班", "初一", "100m"}}
	preview, err = f.svc.PreviewAthleteImport(f.ctx, f.meet, input)
	must(t, err)
	if preview.Errors != 1 {
		t.Fatal("ambiguous short unit accepted")
	}
	suggestions, err := f.svc.SuggestGroups(f.ctx, f.meet)
	must(t, err)
	if !slices.ContainsFunc(suggestions, func(g GroupSuggestion) bool {
		return g.Kind == "athlete" && g.ID == f.athletes[1] && slices.Contains(g.Candidates, g2)
	}) {
		t.Fatal("missing roster suggestion")
	}
}

func TestGroupCombinedMovesAndV8RoundTrip(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	g1, g2 := addGroup(t, f, "初一"), addGroup(t, f, "初二")
	setUnitGroups(t, f, f.units[0], g1, g2)
	setUnitGroups(t, f, f.units[1], g2)
	in := combinedInput()
	in.GroupID = g1
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", in)
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, f.athletes))
	d, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	for _, part := range d.Components {
		f.event = part.Event.ID
		f.addHeat(0, f.athletes...)
		inputs := []ResultInput{{Position: 1, Mark: "12"}, {Position: 2, Mark: "13"}}
		if part.Event.Kind == domain.KindDistance {
			inputs = []ResultInput{{Position: 1, Attempts: []string{"5.1"}}, {Position: 2, Attempts: []string{"4.5"}}}
		}
		must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, inputs))
	}
	enableTeamScores(t, f, domain.DefaultTeamScoringRules())
	req := GroupChangeRequest{MeetID: f.meet, GroupID: g2, CombinedIDs: []string{c.ID}}
	p, err := f.svc.PreviewGroupChange(f.ctx, req)
	must(t, err)
	_, err = f.svc.ApplyGroupChange(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	after, err := f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if !after.Complete || after.Combined.GroupID != g2 {
		t.Fatal("combined move")
	}
	for i, part := range after.Components {
		if part.Event.GroupID != g2 || part.Event.ID != d.Components[i].Event.ID {
			t.Fatal("component lost identity")
		}
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Version != interchange.Version || len(file.Groups) != 2 || len(file.Units[0].GroupIDs)+len(file.Units[1].GroupIDs) != 3 {
		t.Fatal("missing v8 groups")
	}
	data, err := interchange.Encode(file)
	must(t, err)
	imp, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(imp.Errors) > 0 {
		t.Fatal(imp.Errors)
	}
	imp, err = f.svc.ImportMeet(f.ctx, ui, data, "", imp.Hash)
	must(t, err)
	boards, err := f.svc.GetTeamScoreBoards(f.ctx, TeamScoreFilter{MeetID: imp.MeetID})
	must(t, err)
	if len(boards) != 2 || boards[1].GroupID == g2 || !boards[1].Detail.Complete || boards[1].Detail.Rows[0].Total.Text != "9" {
		t.Fatal("group remapping/ranking")
	}
	imp, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(imp.Errors) > 0 {
		t.Fatal("same-meet groups", imp.Errors)
	}
	// Editing the combined moves it with its components, keeping the results.
	in.GroupID = g1
	_, err = f.svc.SaveCombined(f.ctx, ui, f.meet, c.ID, in)
	must(t, err)
	after, err = f.svc.GetCombined(f.ctx, c.ID)
	must(t, err)
	if !after.Complete || after.Combined.GroupID != g1 {
		t.Fatal("combined edit move")
	}
	for _, part := range after.Components {
		if part.Event.GroupID != g1 {
			t.Fatal("component group not moved")
		}
	}
}

func TestGroupDeletedOrderAndImportConflicts(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	g1, g2, g3 := addGroup(t, f, "初一"), addGroup(t, f, "初二"), addGroup(t, f, "初三")
	must(t, f.svc.DeleteGroup(f.ctx, ui, f.meet, g2))
	g4 := addGroup(t, f, "高一")
	groups, err := f.svc.ListGroups(f.ctx, f.meet)
	must(t, err)
	if len(groups) != 3 || groups[2].ID != g4 || groups[2].Ordinal <= groups[1].Ordinal {
		t.Fatal("ordinal reused", groups)
	}
	setUnitGroups(t, f, f.units[0], g1)
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(file)
	must(t, err)
	setUnitGroups(t, f, f.units[0], g3)
	preview, err := f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(preview.Errors) == 0 {
		t.Fatal("conflicting membership silently overwritten")
	}
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, preview.Hash)
	if err == nil {
		t.Fatal("conflicting membership imported")
	}
	units, err := f.svc.ListUnits(f.ctx, f.meet)
	must(t, err)
	if !slices.Equal(units[0].GroupIDs, []string{g3}) {
		t.Fatal("failed import changed group", units)
	}
}

func TestGroupAmbiguousDisplayNamesUseContext(t *testing.T) {
	events := []domain.Event{
		{ID: "a", Gender: domain.GenderMale, Discipline: "100m", Name: "短跑", GroupID: "g1", Kind: domain.KindTrack},
		{ID: "b", Gender: domain.GenderMale, Discipline: "100m", Name: "短跑", GroupID: "g2", Kind: domain.KindTrack},
	}
	if matchEvent(events, "短跑", domain.GenderMale) != "" || matchEvent(events, "短跑", domain.GenderMale, "g2") != "b" || matchEvent(events, "100m", domain.GenderMale, "g1") != "a" {
		t.Fatal("ambiguous display name matched first event")
	}
}

func TestGroupRulePreviewTracksEntriesAndEmptyFilters(t *testing.T) {
	f, _, _, e1, _ := gradeFixture(t)
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	rules.Places[0] = 10
	req := TeamScoringRequest{MeetID: f.meet, Rules: rules}
	preview, err := f.svc.PreviewTeamScoring(f.ctx, req)
	must(t, err)
	must(t, f.svc.AddEntries(f.ctx, ui, e1, []EntryInput{{AthleteID: f.athletes[1]}}))
	_, err = f.svc.ApplyTeamScoring(f.ctx, ui, req, preview.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatal("entries did not invalidate per-group rule preview", err)
	}
	_, err = f.svc.GetTeamScoreBoards(f.ctx, TeamScoreFilter{MeetID: f.meet, GroupID: ptr(""), TargetIDs: []string{"missing"}})
	wantInvalid(t, err, "不存在")
}
