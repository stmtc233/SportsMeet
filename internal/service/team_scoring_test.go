package service

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/scoring"
)

func enableTeamScores(t *testing.T, f *fixture, rules domain.TeamScoringRules) *TeamScoringPreview {
	t.Helper()
	rules.Enabled = true
	req := TeamScoringRequest{MeetID: f.meet, Rules: rules}
	p, err := f.svc.PreviewTeamScoring(f.ctx, req)
	must(t, err)
	p, err = f.svc.ApplyTeamScoring(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	return p
}
func teamScores(t *testing.T, f *fixture) *TeamScoreDetail {
	t.Helper()
	d, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: f.meet})
	must(t, err)
	return d
}
func unitScore(t *testing.T, d *TeamScoreDetail, id string) scoring.UnitStanding {
	t.Helper()
	for _, r := range d.Rows {
		if r.UnitID == id {
			return r
		}
	}
	t.Fatal("unit not found")
	return scoring.UnitStanding{}
}

func TestTeamScoresFinalAcrossHeatsPendingAndUnitChanges(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 2}}
	f := newFixture(t, 4, in)
	if d := teamScores(t, f); d.Rules.Enabled || d.Counted != 0 {
		t.Fatal("new meet enabled")
	}
	enableTeamScores(t, f, domain.DefaultTeamScoringRules())
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, h, []ResultInput{{Position: 1, Mark: "10"}, {Position: 2, Mark: "11"}, {Position: 3, Mark: "12"}, {Position: 4, Mark: "13"}}))
	if d := teamScores(t, f); d.Counted != 0 {
		t.Fatal("prelim counted")
	}
	f.addHeat(1, f.athletes[:2]...)
	f.addHeat(1, f.athletes[2:]...)
	detail := f.detail()
	h1, h2 := detail.Rounds[1].Heats[0].ID, detail.Rounds[1].Heats[1].ID
	must(t, f.svc.SaveResults(f.ctx, ui, h1, []ResultInput{{Position: 1, Mark: "12"}, {Position: 2, Mark: "13"}}))
	if d := teamScores(t, f); d.Counted != 0 || d.Rows[0].Rank != nil {
		t.Fatal("partial final counted")
	}
	must(t, f.svc.SaveResults(f.ctx, ui, h2, []ResultInput{{Position: 1, Mark: "11"}, {Position: 2, Status: "DQ"}}))
	d := teamScores(t, f)
	if !d.Complete || d.Counted != 1 || unitScore(t, d, f.units[0]).Total.Text != "16" || unitScore(t, d, f.units[1]).Total.Text != "6" {
		t.Fatalf("cross-heat ranking: %+v", d)
	}
	// A cleared result withdraws the whole project until it is complete again.
	must(t, f.svc.SaveResults(f.ctx, ui, h2, []ResultInput{{Position: 1}}))
	if d := teamScores(t, f); d.Counted != 0 || d.Rows[0].Total.Text != "0" {
		t.Fatal("cleared result kept points")
	}
	must(t, f.svc.SaveResults(f.ctx, ui, h2, []ResultInput{{Position: 1, Mark: "11"}}))
	_, err := f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[2], AthleteInput{Name: "选手03", Gender: "男", Bib: "003"})
	must(t, err)
	d = teamScores(t, f)
	if len(d.Unassigned) != 1 || unitScore(t, d, f.units[0]).Total.Text != "7" || unitScore(t, d, f.units[1]).Total.Text != "6" {
		t.Fatal("unit removal changed places or failed to recalculate")
	}
}

func TestTeamScoringPreviewStalenessValidationAndAudit(t *testing.T) {
	f := newFixture(t, 2, track())
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, h, []ResultInput{{Position: 1, Mark: "12"}, {Position: 2, Mark: "12"}}))
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	req := TeamScoringRequest{MeetID: f.meet, Rules: rules}
	p, err := f.svc.PreviewTeamScoring(f.ctx, req)
	must(t, err)
	if p.After.Rows[0].Total.Text != "8" || p.After.Rows[1].Total.Text != "8" || *p.After.Rows[0].Rank != 1 || *p.After.Rows[1].Rank != 1 {
		t.Fatal("tied places")
	}
	_, err = f.svc.ApplyTeamScoring(f.ctx, ui, req, p.PreviewHash, false)
	wantInvalid(t, err, "先预览")
	must(t, f.svc.SaveResults(f.ctx, ui, h, []ResultInput{{Position: 2, Mark: "13"}}))
	_, err = f.svc.ApplyTeamScoring(f.ctx, ui, req, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatal("stale result accepted", err)
	}
	enableTeamScores(t, f, rules)
	audit, err := f.svc.ListAudit(f.ctx, AuditQuery{MeetID: f.meet})
	must(t, err)
	if audit.Rows[0].Action != "team_scoring.rules" || audit.Rows[0].ActionText != "修改团体计分规则" || (*f.changes)[len(*f.changes)-1].Kind != "team_scoring" {
		t.Fatal("missing audit/event")
	}
	rules.Overrides = []domain.TeamScoreOverride{{EventID: f.event, Multiplier: 0}}
	enableTeamScores(t, f, rules)
	if d := teamScores(t, f); d.Total != 0 || d.Counted != 0 || d.Complete || d.Sources[0].State != "不计分" {
		t.Fatal("excluded event counted")
	}
	rules.Overrides = []domain.TeamScoreOverride{{EventID: "other-meet", Multiplier: 2}}
	_, err = f.svc.PreviewTeamScoring(f.ctx, TeamScoringRequest{MeetID: f.meet, Rules: rules})
	wantInvalid(t, err, "不属于本赛事")
	_, err = f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: f.meet, TargetIDs: []string{""}})
	wantInvalid(t, err, "不存在")
	for _, text := range []string{"", "9、-1", "9、7.5", "9、x", "7、9"} {
		_, err := ParseTeamScoringInput(TeamScoringInput{Places: text, TiePolicy: "average", RelayMultiplier: "2"})
		if err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	parsed, err := ParseTeamScoringInput(TeamScoringInput{Enabled: true, Places: "9、7,6\n5\t4", TiePolicy: "average", RelayMultiplier: "2"})
	must(t, err)
	if !reflect.DeepEqual(parsed.Places, []int64{9, 7, 6, 5, 4}) {
		t.Fatal("parse places")
	}
}

func TestTeamScoresHeightConfirmation(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "跳高", Kind: "height"})
	f.addHeat(0, f.athletes...)
	enableTeamScores(t, f, domain.DefaultTeamScoringRules())
	h := f.detail().Rounds[0].Heats[0].ID
	inputs := []ResultInput{{Position: 1, Bars: []BarInput{{"1.5", "O"}}}, {Position: 2, Status: "DNS"}}
	must(t, f.svc.SaveResults(f.ctx, ui, h, inputs))
	if teamScores(t, f).Counted != 0 {
		t.Fatal("height progress counted")
	}
	for i := range inputs {
		inputs[i].HeightFinished = true
	}
	must(t, f.svc.SaveResults(f.ctx, ui, h, inputs))
	d := teamScores(t, f)
	if d.Counted != 1 || unitScore(t, d, f.units[0]).Total.Text != "9" || unitScore(t, d, f.units[1]).Total.Text != "0" {
		t.Fatal("confirmed height not counted")
	}
}

func TestTeamScoresCombinedOnlyAndRoundtripReports(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
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
	rules := domain.DefaultTeamScoringRules()
	rules.Overrides = []domain.TeamScoreOverride{{CombinedID: c.ID, Multiplier: 2}}
	enableTeamScores(t, f, rules)
	scores := teamScores(t, f)
	if scores.Counted != 1 || scores.Total != 1 || len(scores.Targets) != 1 || unitScore(t, scores, f.units[0]).Total.Text != "18" || unitScore(t, scores, f.units[1]).Total.Text != "14" {
		t.Fatal("combined duplicated")
	}
	rules.Overrides = []domain.TeamScoreOverride{{EventID: f.event, Multiplier: 2}}
	_, err = f.svc.PreviewTeamScoring(f.ctx, TeamScoringRequest{MeetID: f.meet, Rules: rules})
	wantInvalid(t, err, "不属于本赛事")
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Version != 8 || file.Meet.TeamScoring == nil {
		t.Fatal("missing current rules")
	}
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatal(p.Errors)
	}
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", p.Hash)
	must(t, err)
	again, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: imported.MeetID})
	must(t, err)
	if again.Rows[0].Total.Text != "18" || again.Rows[1].Total.Text != "14" || again.Rules.Overrides[0].CombinedID == c.ID {
		t.Fatal("roundtrip/remapping")
	}
	p, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatal("same rules conflict", p.Errors)
	}
	file.Meet.TeamScoring.Places[0] = 10
	data, err = interchange.Encode(file)
	must(t, err)
	p, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if !strings.Contains(strings.Join(p.Errors, " "), "团体计分规则") {
		t.Fatal("conflicting rules accepted")
	}
	for _, kind := range []string{ReportResults, ReportTeamScores} {
		html, err := f.svc.RenderReport(f.ctx, kind, f.meet, []string{c.ID})
		must(t, err)
		if !strings.Contains(html, "团体总分表") || !strings.Contains(html, "正式排名") || !strings.Contains(html, "所选项目") || !strings.Contains(html, ">18<") {
			t.Fatal("missing score table")
		}
		if kind == ReportTeamScores && strings.Contains(html, "分项 1") {
			t.Fatal("standalone includes events")
		}
		if kind == ReportTeamScores && os.Getenv("SPORTSMEET_TEAM_REPORT_OUTPUT") != "" {
			must(t, os.WriteFile(os.Getenv("SPORTSMEET_TEAM_REPORT_OUTPUT"), []byte(html), 0o644))
		}
	}
	prefs, err := f.svc.Preferences(f.ctx)
	must(t, err)
	prefs.PrintTeamScores = false
	must(t, f.svc.SavePreferences(f.ctx, prefs))
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if strings.Contains(html, "团体总分表") {
		t.Fatal("print switch ignored")
	}
}

func TestTeamScoresRelayAndOrdinaryTeamFiltersAndSelectedPrint(t *testing.T) {
	f := newFixture(t, 4, relay())
	tm := team(t, f, "接力队", f.athletes...)
	round := f.detail().Rounds[0].ID
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: round, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 1, ID: tm.ID}}}}}}}))
	must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "50"}}))
	enableTeamScores(t, f, domain.DefaultTeamScoringRules())
	d := teamScores(t, f)
	if unitScore(t, d, f.units[0]).Total.Text != "18" || len(unitScore(t, d, f.units[0]).Lines) != 1 || unitScore(t, d, f.units[1]).Total.Text != "0" {
		t.Fatal("relay split by members")
	}
	relayID := f.event
	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "混合", Discipline: "集体跳绳", Kind: "count", ParticipantMode: "team", GroupLabel: "初中组"})
	must(t, err)
	f.event = e.ID
	tm, err = f.svc.SaveTeam(f.ctx, ui, e.ID, "", TeamInput{Name: "跳绳队", UnitID: f.units[1], MemberIDs: f.athletes})
	must(t, err)
	round = f.detail().Rounds[0].ID
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: round, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 1, ID: tm.ID}}}}}}}))
	must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "120"}}))
	d = teamScores(t, f)
	if d.Counted != 2 || unitScore(t, d, f.units[1]).Total.Text != "9" || unitScore(t, d, f.units[1]).Mixed.Text != "9" {
		t.Fatal("ordinary team multiplier/gender")
	}
	for _, filter := range []TeamScoreFilter{{MeetID: f.meet, Gender: "混合"}, {MeetID: f.meet, GroupLabel: ptr("初中组")}, {MeetID: f.meet, TargetIDs: []string{e.ID}}} {
		got, err := f.svc.GetTeamScores(f.ctx, filter)
		must(t, err)
		if got.Total != 1 || got.Counted != 1 || unitScore(t, got, f.units[0]).Total.Text != "0" || unitScore(t, got, f.units[1]).Total.Text != "9" {
			t.Fatal("filter does not resummarize")
		}
	}
	got, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: f.meet, GroupLabel: ptr("")})
	must(t, err)
	if got.Total != 1 || got.Sources[0].EventID != relayID {
		t.Fatal("ungrouped filter")
	}
	html, err := f.svc.RenderReport(f.ctx, ReportTeamScores, f.meet, []string{e.ID})
	must(t, err)
	if strings.Contains(html, ">18<") || !strings.Contains(html, ">9<") || !strings.Contains(html, "所选项目") {
		t.Fatal("selected report contains unselected points")
	}
	// An athlete's unit does not determine the team's unit. Updating the team's
	// unit after competing moves its score without editing results or members.
	_, err = f.svc.SaveTeam(f.ctx, ui, e.ID, tm.ID, TeamInput{Name: tm.Name, UnitID: f.units[0], MemberIDs: tm.MemberIDs})
	must(t, err)
	if unitScore(t, teamScores(t, f), f.units[0]).Total.Text != "27" {
		t.Fatal("team unit change")
	}
}

func TestTeamScoringLegacyImportAndDeletedTargets(t *testing.T) {
	f := newFixture(t, 1, track())
	rules := domain.DefaultTeamScoringRules()
	rules.Overrides = []domain.TeamScoreOverride{{EventID: f.event, Multiplier: 2}}
	enableTeamScores(t, f, rules)
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	file.Version = 6
	data, err := interchange.Encode(file)
	must(t, err)
	for _, target := range []string{f.meet, ""} {
		p, err := f.svc.PreviewImport(f.ctx, data, target)
		must(t, err)
		if len(p.Errors) > 0 {
			t.Fatal(p.Errors)
		}
		p, err = f.svc.ImportMeet(f.ctx, ui, data, target, p.Hash)
		must(t, err)
		id := target
		if id == "" {
			id = p.MeetID
		}
		got, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: id})
		must(t, err)
		if got.Rules.Enabled != (target != "") {
			t.Fatal("legacy import reset/enabled scoring")
		}
	}
	must(t, f.svc.DeleteEvent(f.ctx, ui, f.event, true))
	got := teamScores(t, f)
	if len(got.Rules.Overrides) != 0 || len(got.Targets) != 0 {
		t.Fatal("deleted target kept rule")
	}
	file, err = f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if len(file.Meet.TeamScoring.Overrides) != 0 || len(interchange.Validate(file)) > 0 {
		t.Fatal("dangling exported rule")
	}
}

func TestTeamScoringSaveParsesInputDirectly(t *testing.T) {
	f := newFixture(t, 2, EventInput{})
	rules, err := f.svc.SaveTeamScoring(f.ctx, ui, f.meet, TeamScoringInput{Enabled: true, Places: "7、5，3 1", TiePolicy: "average", RelayMultiplier: "2", Overrides: []TeamScoreOverrideInput{}})
	must(t, err)
	if !rules.Enabled || len(rules.Places) != 4 || rules.Places[0] != 7 || rules.Places[3] != 1 {
		t.Fatal("saved rules", rules)
	}
	detail, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: f.meet})
	must(t, err)
	if !detail.Rules.Enabled || detail.Rules.Places[0] != 7 {
		t.Fatal("rules not stored", detail.Rules)
	}
	_, err = f.svc.SaveTeamScoring(f.ctx, ui, f.meet, TeamScoringInput{Enabled: true, Places: "1、x", TiePolicy: "average", RelayMultiplier: "2"})
	wantInvalid(t, err, "非负整数")
}
