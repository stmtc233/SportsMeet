package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func TestCountScoreAndGenericTeamsFlow(t *testing.T) {
	in := EventInput{Gender: "混合", Discipline: "集体跳绳", Kind: "count", ParticipantMode: "team", TeamSize: 2, AttemptMode: "sum", AttemptCount: 3}
	f := newFixture(t, 4, in)
	teams := []string{}
	for i := range 2 {
		team, err := f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: []string{"A队", "B队"}[i], MemberIDs: f.athletes[i*2 : i*2+2]})
		must(t, err)
		teams = append(teams, team.ID)
		must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{TeamID: team.ID}}))
	}
	d := f.detail()
	if !d.Rules.Team || !d.Rules.MultipleAttempts {
		t.Fatal(d.Rules)
	}
	plan, err := f.svc.PlanHeats(f.ctx, PlanRequest{EventID: f.event, RoundID: d.Rounds[0].ID, PerHeat: 12, Order: arrange.OrderSeed, Lanes: arrange.LanesSequential})
	must(t, err)
	slots := []SlotSave{}
	for _, sl := range plan[0].Slots {
		slots = append(slots, SlotSave{ID: sl.ID, Position: sl.Position})
	}
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: d.Rounds[0].ID, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: slots}}}}}))
	d = f.detail()
	h := d.Rounds[0].Heats[0]
	must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 1, Attempts: []string{"120", "124", "122"}}, {Position: 2, Attempts: []string{"0", "X", "-"}}}))
	d = f.detail()
	if d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "366" || d.Rounds[0].Heats[0].Slots[1].Result.Status != domain.StatusOK {
		t.Fatal("count aggregation/zero")
	}
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, teams[0], TeamInput{Name: "A队", MemberIDs: []string{f.athletes[0]}})
	wantInvalid(t, err, "比赛记录")
	changed := in
	changed.AttemptMode = "best"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, changed)
	wantInvalid(t, err, "计分规则")
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "366") || !strings.Contains(html, "成员") {
		t.Fatal("team count report")
	}
	score, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "技巧评分", Kind: "score", ScoreDecimals: 2})
	must(t, err)
	f.event = score.ID
	f.addHeat(0, f.athletes[0])
	d = f.detail()
	must(t, f.svc.SaveResults(f.ctx, ui, d.Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.50"}}))
	d = f.detail()
	if d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "12.50" {
		t.Fatal("score precision")
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	// A compatible roster cannot silently change the number of visible attempts.
	for i := range file.Events {
		if file.Events[i].Kind == "count" {
			file.Events[i].AttemptCount = ptr(4)
			changed, err := interchange.Encode(file)
			must(t, err)
			conflict, err := f.svc.PreviewImport(f.ctx, changed, f.meet)
			must(t, err)
			if len(conflict.Errors) == 0 {
				t.Fatal("attempt configuration conflict accepted")
			}
			file.Events[i].AttemptCount = ptr(3)
		}
	}
	data, err := interchange.Encode(file)
	must(t, err)
	preview, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(preview.Errors) > 0 {
		t.Fatal(preview.Errors)
	}
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", preview.Hash)
	must(t, err)
	rows, err := f.svc.ListEvents(f.ctx, imported.MeetID)
	must(t, err)
	for _, row := range rows {
		d, err := f.svc.GetEvent(f.ctx, row.Event.ID)
		must(t, err)
		if row.Event.Kind == domain.KindCount && (!d.Rules.Team || d.Rounds[0].Heats[0].Slots[0].Result.MarkText != "366") {
			t.Fatal("team/scalar roundtrip")
		}
	}
}

func TestScalarPrecisionCannotReinterpretEntryMarks(t *testing.T) {
	in := EventInput{Gender: "男", Discipline: "评分", Kind: "score", ScoreDecimals: 2}
	f := newFixture(t, 1, in)
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[0], Seed: "12.50"}}))
	in.ScoreDecimals = 0
	_, err := f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	wantInvalid(t, err, "报名成绩")
	if f.detail().Entries[0].SeedText != "12.50" {
		t.Fatal("entry mark changed precision")
	}
}
