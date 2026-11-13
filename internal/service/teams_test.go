package service

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func relay() EventInput {
	return EventInput{Gender: "男", Discipline: "4×100m", Kind: "relay", LaneCount: 8}
}
func team(t *testing.T, f *fixture, name string, members ...string) *domain.Team {
	t.Helper()
	got, err := f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: name, UnitID: f.units[0], MemberIDs: members})
	must(t, err)
	return got
}
func confirm(t *testing.T, f *fixture, op Mutation) *MutationPreview {
	t.Helper()
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatalf("preview: %v", p.Errors)
	}
	got, err := f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	must(t, err)
	return got
}

func TestRelayCompleteWorkflow(t *testing.T) {
	in := relay()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1, AdvanceByMark: 1}}
	f := newFixture(t, 24, in)
	ids := []string{}
	for i := range 6 {
		got := team(t, f, fmt.Sprintf("一班%d队", i+1), f.athletes[i*4:i*4+4]...)
		ids = append(ids, got.ID)
	}
	inputs := []EntryInput{}
	for i, id := range ids {
		inputs = append(inputs, EntryInput{TeamID: id, Seed: fmt.Sprintf("%d.00", 50+i)})
	}
	confirm(t, f, Mutation{Action: "add_entries", EventID: f.event, Entries: inputs})
	d := f.detail()
	if len(d.Teams) != 6 || len(d.Entries) != 6 || d.Entries[0].AthleteID != "" || d.Entries[0].TeamID != ids[0] {
		t.Fatalf("detail: %+v", d)
	}
	must(t, f.svc.SetSeed(f.ctx, ui, f.event, ids[0], "49.50"))
	confirm(t, f, Mutation{Action: "auto_arrange", Plan: PlanRequest{EventID: f.event, RoundID: d.Rounds[0].ID, PerHeat: 2, Order: arrange.OrderSeed, Lanes: arrange.LanesCenter, SeparateUnits: true, Balance: true, Seed: 42}})
	d = f.detail()
	if len(d.Rounds[0].Heats) != 3 {
		t.Fatalf("heats: %+v", d.Rounds[0])
	}
	for i, h := range d.Rounds[0].Heats {
		if len(h.Slots) != 2 || h.Slots[0].TeamID == "" || h.Slots[0].AthleteID != "" || h.Slots[0].Position != 4 {
			t.Fatalf("team slots: %+v", h.Slots)
		}
		// Team ranks and Q/q are calculated only once every heat is complete.
		confirm(t, f, Mutation{Action: "set_results", HeatID: h.ID, Results: []ResultInput{{Position: 4, Mark: fmt.Sprintf("%d.00", 50+i)}, {Position: 5, Mark: fmt.Sprintf("%d.00", 55+i)}}})
	}
	d = f.detail()
	qualified := 0
	for _, h := range d.Rounds[0].Heats {
		for _, sl := range h.Slots {
			if sl.Result == nil || sl.Result.Rank == nil {
				t.Fatal("missing ranks")
			}
			if sl.Result.Qualify != "" {
				qualified++
			}
		}
	}
	if qualified != 4 || !d.Rounds[0].Complete || len(d.Rounds[0].Standings) != 6 || d.Rounds[0].Standings[0].TeamID == "" {
		t.Fatalf("qualification: %+v", d.Rounds[0])
	}
	confirm(t, f, Mutation{Action: "advance", Plan: PlanRequest{EventID: f.event, RoundID: d.Rounds[1].ID, PerHeat: 8, Order: "seed", Lanes: "center", Seed: 42}})
	d = f.detail()
	final := d.Rounds[1].Heats[0]
	if len(final.Slots) != 4 {
		t.Fatal("wrong finalists")
	}
	results := []ResultInput{}
	for _, sl := range final.Slots {
		results = append(results, ResultInput{Position: sl.Position, Mark: "48.20"})
	}
	must(t, f.svc.SaveResults(f.ctx, ui, final.ID, results))
	for _, sl := range f.detail().Rounds[1].Heats[0].Slots {
		if *sl.Result.Rank != 1 {
			t.Fatal("tied teams must share rank")
		}
	}
	for _, kind := range []string{ReportStartList, ReportResults} {
		html, err := f.svc.RenderReport(f.ctx, kind, f.meet, nil)
		must(t, err)
		for _, want := range []string{"一班1队", "1棒 选手01", "4棒 选手04", "队伍 / 棒次"} {
			if !strings.Contains(html, want) {
				t.Fatalf("report missing %s", want)
			}
		}
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(file)
	must(t, err)
	if _, err = interchange.Decode(data); err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(p.Errors) > 0 || p.Teams.New != 6 || p.Entries != 6 || p.ResultsNew != 10 {
		t.Fatalf("preview: %+v", p)
	}
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", p.Hash)
	must(t, err)
	again, err := f.svc.ExportMeet(f.ctx, imported.MeetID)
	must(t, err)
	if len(again.Events[0].Teams) != 6 || len(again.Events[0].Rounds[1].Heats[0].Slots) != 4 {
		t.Fatalf("roundtrip: %+v", again.Events)
	}
	for i, t1 := range file.Events[0].Teams {
		t2 := again.Events[0].Teams[i]
		if t1.Name != t2.Name || len(t2.MemberIDs) != 4 {
			t.Fatal("lost lineup")
		}
		for j, id := range t1.MemberIDs {
			if athleteName(fileAthletes(file), id) != athleteName(fileAthletes(again), t2.MemberIDs[j]) {
				t.Fatal("leg order changed")
			}
		}
	}
	p, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) > 0 || p.Teams.Reused != 6 || p.Teams.New != 0 || p.ResultsNew != 0 || p.ResultsOverwritten != 0 {
		t.Fatalf("reuse: %+v", p)
	}
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p.Hash)
	must(t, err)
	if len(f.detail().Teams) != 6 {
		t.Fatal("duplicate teams on reimport")
	}
	usage, err := f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if usage[0].Entries != 1 || !usage[0].Placed || !usage[0].TeamMember {
		t.Fatal("relay athlete usage missing")
	}
}
func fileAthletes(f *interchange.File) []domain.Athlete {
	out := []domain.Athlete{}
	for _, a := range f.Athletes {
		out = append(out, domain.Athlete{ID: a.ID, Name: a.Name})
	}
	return out
}

func TestRelayValidationAndGuards(t *testing.T) {
	f := newFixture(t, 5, relay())
	a := team(t, f, "A队", f.athletes[:4]...)
	_, err := f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: "重复", MemberIDs: []string{f.athletes[4], f.athletes[4]}})
	wantInvalid(t, err, "重复")
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: "其他队", MemberIDs: []string{f.athletes[0]}})
	wantInvalid(t, err, "其他队伍")
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: "A队", UnitID: f.units[0]})
	wantInvalid(t, err, "同名")
	other := newFixture(t, 1, relay())
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: "跨赛事", MemberIDs: other.athletes})
	wantInvalid(t, err, "本赛事")
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: "跨单位", UnitID: other.units[0]})
	wantInvalid(t, err, "本赛事")
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{TeamID: a.ID}}))
	wantInvalid(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[4]}}), "类型")
	wantInvalid(t, f.svc.DeleteAthletes(f.ctx, ui, f.meet, []string{f.athletes[0]}), "接力队")
	in := relay()
	in.Kind = "track"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	wantInvalid(t, err, "接力类型")
	// Typed team placement implicitly enters an unregistered team.
	b := team(t, f, "B队", f.athletes[4])
	d := f.detail()
	r := d.Rounds[0]
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{"new"}, Added: []HeatSave{{Key: "new", Slots: []SlotSave{{Position: 2, TeamID: a.ID}, {Position: 3, ID: b.ID}}}}}}}))
	d = f.detail()
	h := d.Rounds[0].Heats[0]
	if len(d.Entries) != 2 {
		t.Fatal("placement must enter team")
	}
	wantInvalid(t, f.svc.RemoveEntries(f.ctx, ui, f.event, []string{a.ID}), "已排进组")
	wantInvalid(t, f.svc.DeleteTeam(f.ctx, ui, f.event, a.ID), "已排进组")
	must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 2, Mark: "50.12"}}))
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, b.ID, TeamInput{Name: "B队", MemberIDs: nil})
	wantInvalid(t, err, "棒次")
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, a.ID, TeamInput{Name: "A队", MemberIDs: []string{f.athletes[1], f.athletes[0], f.athletes[2], f.athletes[3]}})
	wantInvalid(t, err, "棒次")
	// Name edits keep results, and stale confirmation includes team definitions.
	op := Mutation{Action: "set_results", HeatID: h.ID, Results: []ResultInput{{Position: 2, Mark: "49.12"}}}
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	_, err = f.svc.SaveTeam(f.ctx, ui, f.event, a.ID, TeamInput{Name: "A队更名", UnitID: a.UnitID, MemberIDs: a.MemberIDs})
	must(t, err)
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("stale team preview: %v", err)
	}
	// Only an explicitly unlocked arrangement can move a scored team's result.
	h = f.detail().Rounds[0].Heats[0]
	save := BoardSave{EventID: f.event, Rounds: []RoundSave{{RoundID: r.ID, Order: []string{h.ID}, Changed: []HeatSave{{ID: h.ID, Base: h.Base, Slots: []SlotSave{{Position: 3, TeamID: a.ID}, {Position: 2, TeamID: b.ID}}}}}}}
	wantInvalid(t, f.svc.SaveBoard(f.ctx, ui, save), "解锁")
	save.Unlocked = []string{h.ID}
	must(t, f.svc.SaveBoard(f.ctx, ui, save))
	if got := f.detail().Rounds[0].Heats[0].Slots[1]; got.TeamID != a.ID || got.Result == nil || *got.Result.Mark != 50120 {
		t.Fatal("result did not follow team")
	}
	// Import roster conflicts cannot rewrite the lineup.
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	file.Events[0].Teams[0].MemberIDs[0], file.Events[0].Teams[0].MemberIDs[1] = file.Events[0].Teams[0].MemberIDs[1], file.Events[0].Teams[0].MemberIDs[0]
	data, err := interchange.Encode(file)
	must(t, err)
	p2, err := f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p2.Errors) == 0 {
		t.Fatal("roster conflict accepted")
	}
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p2.Hash)
	wantInvalid(t, err, "文件有误")
	if !reflect.DeepEqual(f.detail().Teams[0].MemberIDs, a.MemberIDs) {
		t.Fatal("roster conflict wrote")
	}
}

func TestRelayConcurrentRosterAndDeletion(t *testing.T) {
	f := newFixture(t, 1, relay())
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := range 2 {
		wg.Go(func() {
			_, err := f.svc.SaveTeam(f.ctx, ui, f.event, "", TeamInput{Name: fmt.Sprint(i), MemberIDs: f.athletes})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("member assigned twice")
	}
	d := f.detail()
	id := d.Teams[0].ID
	confirm(t, f, Mutation{Action: "delete_team", EventID: f.event, ID: id})
	if len(f.detail().Teams) != 0 {
		t.Fatal("team not deleted")
	}
	must(t, f.svc.DeleteAthletes(f.ctx, store.SourceMCP, f.meet, f.athletes))
}

func TestRelayImportMemberConflictsAndGender(t *testing.T) {
	f := newFixture(t, 4, relay())
	team(t, f, "A队", f.athletes...)
	cases := map[string]func(*interchange.File){
		"duplicate leg": func(file *interchange.File) {
			file.Events[0].Teams[0].MemberIDs = append(file.Events[0].Teams[0].MemberIDs, f.athletes[0])
		},
		"other team": func(file *interchange.File) {
			file.Events[0].Teams = append(file.Events[0].Teams, interchange.Team{ID: "extra", Name: "B队", MemberIDs: f.athletes[:1]})
		},
		"unknown member": func(file *interchange.File) { file.Events[0].Teams[0].MemberIDs[0] = "unknown" },
		"duplicate team name": func(file *interchange.File) {
			old := file.Events[0].Teams[0]
			file.Events[0].Teams = append(file.Events[0].Teams, interchange.Team{ID: "extra", Name: " a队 ", UnitID: old.UnitID, MemberIDs: []string{}})
		},
		"aliases match same athlete": func(file *interchange.File) {
			file.Athletes[1].Name = file.Athletes[0].Name
			file.Athletes[1].Bib = ""
			file.Athletes[1].UnitID = file.Athletes[0].UnitID
			file.Athletes[1].Affiliation = file.Athletes[0].Affiliation
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			file, err := f.svc.ExportMeet(f.ctx, f.meet)
			must(t, err)
			change(file)
			data, err := interchange.Encode(file)
			must(t, err)
			p, err := f.svc.PreviewImport(f.ctx, data, f.meet)
			must(t, err)
			if len(p.Errors) == 0 {
				t.Fatal("invalid relay import accepted")
			}
			_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p.Hash)
			wantInvalid(t, err, "文件有误")
			if !reflect.DeepEqual(f.detail().Teams[0].MemberIDs, f.athletes) {
				t.Fatal("invalid import changed roster")
			}
		})
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	file.Athletes[0].Gender = "女"
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(p.Errors) > 0 || !strings.Contains(strings.Join(p.Warnings, ""), "性别") {
		t.Fatalf("UI gender warning: %+v", p)
	}
	mp, err := f.svc.PreviewMutation(f.ctx, Mutation{Action: "import_meet", Data: string(data)})
	must(t, err)
	if len(mp.Errors) == 0 {
		t.Fatal("MCP relay gender mixing accepted")
	}
}
