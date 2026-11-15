package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func TestCreateMeetFromExisting(t *testing.T) {
	in := track()
	in.Name, in.GroupLabel, in.Date, in.Note = "百米赛", "高一", "2026-11-13", "项目备注"
	in.StartMode, in.LaneCount = "break", 6
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 2, AdvanceByMark: 3}, {Stage: "quarterfinal", AdvanceByMark: 8}, {Stage: "semifinal", AdvanceByPlace: 3}}
	f := newFixture(t, 3, in)
	f.addHeat(0, f.athletes...)
	detail := f.detail()
	must(t, f.svc.SaveResults(f.ctx, ui, detail.Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	_, err := f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: detail.Rounds[0].ID, ScheduledAt: "2026-11-13T09:00"})
	must(t, err)
	// Imported custom round names belong to the format too.
	must(t, f.svc.store.Write(f.ctx, store.Stmt(`UPDATE rounds SET name=? WHERE id=?`, "第一轮", detail.Rounds[0].ID)))
	_, err = f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "未设项目组"})
	must(t, err)
	for _, event := range []EventInput{
		{Gender: "男", Discipline: "铅球", Kind: "distance", DistanceMode: "throw", AttemptCount: 6},
		{Gender: "女", Discipline: "跳高", Kind: "height"},
		{Gender: "混合", Discipline: "跳绳", Kind: "count", RankDirection: "asc", AttemptMode: "sum", AttemptCount: 3},
		{Gender: "混合", Discipline: "集体评分", Kind: "score", ParticipantMode: "team", TeamSize: 6, ScoreDecimals: 3, AttemptMode: "best", AttemptCount: 2},
	} {
		_, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", event)
		must(t, err)
	}
	relay, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "混合", Discipline: "4×100m", Kind: "relay", StartMode: "lanes", RelayLegCount: 4})
	must(t, err)
	team, err := f.svc.SaveTeam(f.ctx, ui, relay.ID, "", TeamInput{Name: "接力队", UnitID: f.units[0], MemberIDs: f.athletes})
	must(t, err)
	must(t, f.svc.AddEntries(f.ctx, ui, relay.ID, []EntryInput{{TeamID: team.ID}}))
	ci := combinedInput()
	ci.GroupLabel, ci.Note = "高一", "全能备注"
	combined, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", ci)
	must(t, err)
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, combined.ID, f.athletes[:2]))
	// Source rule versions describe its own editing history.
	cd, err := f.svc.GetCombined(f.ctx, combined.ID)
	must(t, err)
	req := CombinedRulesRequest{CombinedID: combined.ID}
	for _, c := range cd.Components {
		rule := c.Input
		rule.InvalidPolicies["DQ"] = "exclude"
		req.Rules = append(req.Rules, CombinedRuleChange{EventID: c.Event.ID, Rule: rule})
	}
	p, err := f.svc.PreviewCombinedRules(f.ctx, req)
	must(t, err)
	_, err = f.svc.ApplyCombinedRules(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	deleted, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "200m", Kind: "track"})
	must(t, err)
	rules := domain.DefaultTeamScoringRules()
	rules.Places, rules.TiePolicy, rules.RelayMultiplier = []int64{10, 8, 6}, "same", 3
	rules.Overrides = []domain.TeamScoreOverride{{EventID: f.event, Multiplier: 0}, {CombinedID: combined.ID, Multiplier: 2}, {EventID: deleted.ID, Multiplier: 4}}
	enableTeamScores(t, f, rules)
	must(t, f.svc.DeleteEvent(f.ctx, ui, deleted.ID, false))
	settings, err := f.svc.MeetSettings(f.ctx, f.meet)
	must(t, err)
	settings.MinRestMinutes = 30
	must(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, settings))
	before, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	sourceEvents, err := f.svc.store.ListEvents(f.ctx, store.Scope{MeetID: f.meet})
	must(t, err)
	sourceRounds, err := f.svc.store.ListRounds(f.ctx, store.Scope{MeetID: f.meet})
	must(t, err)
	sourceGroups, err := f.svc.store.ListGroups(f.ctx, f.meet)
	must(t, err)

	notified := len(*f.changes)
	input := MeetInput{SourceMeetID: f.meet, Name: "春季运动会", StartDate: "2027-04-20", Location: "新操场", EntryLimits: &domain.EntryLimits{AthleteEvents: 2}}
	meet, err := f.svc.CreateMeet(f.ctx, ui, input)
	must(t, err)
	if meet.ID == f.meet || meet.Name != input.Name || meet.StartDate != input.StartDate || meet.Location != input.Location || meet.EntryLimits.AthleteEvents != 2 {
		t.Fatalf("new meet: %+v", meet)
	}
	if len(*f.changes) != notified+1 || (*f.changes)[notified] != (Change{Kind: "meets", MeetID: meet.ID}) {
		t.Fatal("creation must notify once, after the full write")
	}
	events, err := f.svc.store.ListEvents(f.ctx, store.Scope{MeetID: meet.ID})
	must(t, err)
	rounds, err := f.svc.store.LoadRounds(f.ctx, store.Scope{MeetID: meet.ID})
	must(t, err)
	groups, err := f.svc.store.ListGroups(f.ctx, meet.ID)
	must(t, err)
	if len(events) != len(sourceEvents) || len(groups) != len(sourceGroups) {
		t.Fatalf("definitions missing: events=%d groups=%d", len(events), len(groups))
	}
	groupIDs, eventIDs := map[string]string{}, map[string]string{}
	for i, g := range groups {
		old := sourceGroups[i]
		if g.ID == old.ID || g.MeetID != meet.ID || g.Name != old.Name || g.Ordinal != old.Ordinal {
			t.Fatalf("group mapping: %+v", g)
		}
		groupIDs[old.ID] = g.ID
	}
	for i, e := range events {
		old := sourceEvents[i]
		eventIDs[old.ID] = e.ID
		if old.ID == e.ID || e.MeetID != meet.ID || e.GroupID != groupIDs[old.GroupID] || e.CombinedID != "" && e.CombinedID == old.CombinedID {
			t.Fatalf("event mapping: %+v", e)
		}
		old.ID, old.MeetID, old.GroupID, old.CombinedID, old.Date = e.ID, meet.ID, e.GroupID, e.CombinedID, ""
		if !reflect.DeepEqual(old, e) {
			t.Fatalf("event configuration changed: got %+v, want %+v", e, old)
		}
		if len(rounds[e.ID]) != len(sourceRounds[sourceEvents[i].ID]) {
			t.Fatal("rounds missing")
		}
		for j, r := range rounds[e.ID] {
			was := sourceRounds[sourceEvents[i].ID][j]
			if r.ID == was.ID || r.EventID != e.ID || r.ScheduledAt != "" || len(r.Heats) > 0 {
				t.Fatalf("round mapping or schedule copied: %+v", r)
			}
			was.ID, was.EventID, was.ScheduledAt = r.ID, e.ID, ""
			if !reflect.DeepEqual(was, r) {
				t.Fatalf("round format changed: got %+v, want %+v", r, was)
			}
		}
	}
	parents, err := f.svc.store.ListCombined(f.ctx, meet.ID)
	must(t, err)
	if len(parents) != 1 || parents[0].Combined.ID == combined.ID || parents[0].Combined.RuleVersion != 1 || len(parents[0].AthleteIDs) != 0 {
		t.Fatalf("combined mapping: %+v", parents)
	}
	copied, err := f.svc.GetCombined(f.ctx, parents[0].Combined.ID)
	must(t, err)
	original, err := f.svc.GetCombined(f.ctx, combined.ID)
	must(t, err)
	wantCombined := original.Combined
	wantCombined.ID, wantCombined.MeetID, wantCombined.GroupID, wantCombined.RuleVersion = copied.Combined.ID, meet.ID, groupIDs[original.Combined.GroupID], 1
	if !reflect.DeepEqual(copied.Combined, wantCombined) {
		t.Fatalf("combined configuration changed: %+v", copied.Combined)
	}
	for i, c := range copied.Components {
		if c.Event.ID != eventIDs[original.Components[i].Event.ID] || c.Event.CombinedID != copied.Combined.ID || c.Ordinal != original.Components[i].Ordinal || !reflect.DeepEqual(c.Rule, original.Components[i].Rule) {
			t.Fatalf("combined rule mapping: %+v", c)
		}
	}
	scores, err := f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: meet.ID})
	must(t, err)
	rules.Enabled = true
	rules.Overrides = []domain.TeamScoreOverride{{CombinedID: copied.Combined.ID, Multiplier: 2}, {EventID: eventIDs[f.event], Multiplier: 0}}
	if !reflect.DeepEqual(scores.Rules, rules) {
		t.Fatalf("team scoring rules: %+v", scores.Rules)
	}
	counts, err := f.svc.CountMeet(f.ctx, meet.ID)
	must(t, err)
	if counts.Athletes != 0 || counts.Heats != 0 || counts.Results != 0 {
		t.Fatalf("participant data copied: %+v", counts)
	}
	units, err := f.svc.store.ListUnits(f.ctx, meet.ID)
	must(t, err)
	entries, err := f.svc.store.ListEntries(f.ctx, store.Scope{MeetID: meet.ID})
	must(t, err)
	if len(units) != 0 || len(entries) != 0 {
		t.Fatal("units or entries copied")
	}
	teams, err := f.svc.store.ListTeams(f.ctx, store.Scope{MeetID: meet.ID})
	must(t, err)
	if len(teams) != 0 {
		t.Fatal("teams copied")
	}
	newSettings, err := f.svc.MeetSettings(f.ctx, meet.ID)
	must(t, err)
	if newSettings.MinRestMinutes != 0 {
		t.Fatal("meet settings copied")
	}
	audit, err := f.svc.ListAudit(f.ctx, AuditQuery{MeetID: meet.ID})
	must(t, err)
	if len(audit.Rows) != 1 || audit.Rows[0].Action != "meet.created" {
		t.Fatalf("creation audit: %+v", audit)
	}
	log, err := f.svc.store.Query(f.ctx, `SELECT detail_json FROM audit_log WHERE meet_id=?`, meet.ID)
	must(t, err)
	if !strings.Contains(log.Values[0][0].(string), f.meet) {
		t.Fatal("source meet missing from audit detail")
	}
	after, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("source meet was modified")
	}
	// Removing the source must leave the new definitions and all references intact.
	must(t, f.svc.DeleteMeet(f.ctx, ui, f.meet))
	_, err = f.svc.ExportMeet(f.ctx, meet.ID)
	must(t, err)
	_, err = f.svc.GetCombined(f.ctx, copied.Combined.ID)
	must(t, err)
	_, err = f.svc.GetTeamScores(f.ctx, TeamScoreFilter{MeetID: meet.ID})
	must(t, err)
}

func TestMeetTemplateFailureAndEmptySource(t *testing.T) {
	ctx := context.Background()
	s, changes := newTestService(t)
	in := MeetInput{SourceMeetID: "missing", Name: "新赛事", StartDate: "2027-04-20"}
	_, err := s.CreateMeet(ctx, ui, in)
	wantInvalid(t, err, "不存在")
	meets, err := s.ListMeets(ctx)
	must(t, err)
	if len(meets) != 0 || len(*changes) != 0 {
		t.Fatal("invalid source left a meet or emitted a change")
	}
	source, err := s.CreateMeet(ctx, ui, MeetInput{Name: "空赛事", StartDate: "2026-11-12"})
	must(t, err)
	in.SourceMeetID = source.ID
	created, err := s.CreateMeet(ctx, store.SourceMCP, in)
	must(t, err)
	counts, err := s.CountMeet(ctx, created.ID)
	must(t, err)
	if counts.Events != 0 {
		t.Fatal("empty source created events")
	}
	err = s.UpdateMeet(ctx, ui, source.ID, in)
	wantInvalid(t, err, "仅新建")
	_, err = s.SaveEvent(ctx, ui, source.ID, "", track())
	must(t, err)
	// Fail at the last definition write, after the meet, event and round inserts.
	must(t, s.store.Write(ctx, store.Stmt(`CREATE TRIGGER fail_template BEFORE INSERT ON meet_team_scoring BEGIN SELECT RAISE(ABORT, 'blocked'); END`)))
	notified := len(*changes)
	_, err = s.CreateMeet(ctx, ui, in)
	if err == nil {
		t.Fatal("injected write failure succeeded")
	}
	meets, err = s.ListMeets(ctx)
	must(t, err)
	if len(meets) != 2 || len(*changes) != notified {
		t.Fatal("failed copy left partial data or emitted a change")
	}
	rows, err := s.store.Query(ctx, `PRAGMA foreign_key_check`)
	must(t, err)
	if len(rows.Values) != 0 {
		t.Fatal("foreign key violation")
	}
}

func TestMeetTemplatePreviewConfirmation(t *testing.T) {
	f := newFixture(t, 2, track())
	in := MeetInput{SourceMeetID: f.meet, Name: "新赛事", StartDate: "2027-04-20"}
	op := Mutation{Action: "create_meet", MeetInput: &in}
	notified := len(*f.changes)
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	if p.PreviewHash == "" || len(p.Errors) > 0 || len(*f.changes) != notified {
		t.Fatalf("invalid preview: %+v", p)
	}
	meets, err := f.svc.ListMeets(f.ctx)
	must(t, err)
	if len(meets) != 1 {
		t.Fatal("preview created a meet")
	}
	plan := p.Result.(*MeetCreationPlan)
	if len(plan.Projects.Events) != 1 || plan.Projects.Events[0].ID != f.event || len(plan.Projects.Rounds[f.event][0].Heats) != 0 {
		t.Fatalf("preview must contain only definitions: %+v", plan)
	}
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, false)
	wantInvalid(t, err, "先预览")
	in.Name = "另一名称"
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("changed request: %v", err)
	}
	in.Name = "新赛事"
	event := track()
	event.Note = "新规程"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, event)
	must(t, err)
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("changed source: %v", err)
	}
	p, err = f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	committed, err := f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	must(t, err)
	created := committed.Result.(*MeetCreationPlan).Meet
	if created.ID == f.meet || created.Name != in.Name {
		t.Fatalf("created meet: %+v", created)
	}
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("reused confirmation: %v", err)
	}
}
