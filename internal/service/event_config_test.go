package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func TestEventPresetsAndCustomConfiguration(t *testing.T) {
	svc, _ := newTestService(t)
	presets := svc.EventPresets()
	if len(presets) != 35 {
		t.Fatalf("presets: %d", len(presets))
	}
	seen := map[string]bool{}
	for _, p := range presets {
		if seen[p.ID] {
			t.Fatalf("duplicate preset %s", p.ID)
		}
		seen[p.ID] = true
		if _, err := p.Input.event(); err != nil {
			t.Fatalf("%s: %v", p.ID, err)
		}
	}
	// A typed name does not apply a preset, including a name matching the catalogue.
	e, err := (EventInput{Gender: "男", Discipline: "1500m", Kind: "track"}).event()
	must(t, err)
	if !e.UsesLanes() {
		t.Fatal("name inferred starting mode")
	}
	e, err = (EventInput{Gender: "男", Discipline: "铅球", Kind: "distance"}).event()
	must(t, err)
	if e.Rules().AttemptLabel != "试跳" {
		t.Fatal("name inferred distance mode")
	}
	for _, in := range []EventInput{
		{Gender: "男", Discipline: "x", Kind: "height", StartMode: "group"},
		{Gender: "男", Discipline: "x", Kind: "track", StartMode: "other"},
		{Gender: "男", Discipline: "x", Kind: "distance", DistanceMode: "other"},
		{Gender: "男", Discipline: "x", Kind: "track", DistanceMode: "throw"},
		{Gender: "男", Discipline: "x", Kind: "track", RelayLegCount: 4},
	} {
		if _, err := in.event(); err == nil {
			t.Fatalf("accepted invalid config %+v", in)
		}
	}
}

func TestGroupStartFlowAndConfigProtection(t *testing.T) {
	in := EventInput{Gender: "男", Discipline: "1500m", Kind: "track", StartMode: "group", LaneCount: 8}
	f := newFixture(t, 16, in)
	f.enter(f.athletes...)
	d := f.detail()
	plan, err := f.svc.PlanHeats(f.ctx, PlanRequest{EventID: f.event, RoundID: d.Rounds[0].ID, PerHeat: 16, Order: arrange.OrderSeed, Lanes: arrange.LanesSequential, Seed: 7})
	must(t, err)
	if len(plan) != 1 || len(plan[0].Slots) != 16 || plan[0].Slots[15].Position != 16 {
		t.Fatalf("group plan %+v", plan)
	}
	f.addHeat(0, f.athletes...)
	d = f.detail()
	if d.Rules.UsesLanes || d.Rules.PositionLabel != "顺序" || !d.Rules.Timed {
		t.Fatalf("rules %+v", d.Rules)
	}
	changed := in
	changed.StartMode = "lanes"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, changed)
	wantInvalid(t, err, "先移出")
	must(t, f.svc.SaveResults(f.ctx, ui, d.Rounds[0].Heats[0].ID, []ResultInput{{Position: 16, Mark: "5:12.34"}, {Position: 1, Mark: "5:10.00"}}))
	d = f.detail()
	if *d.Rounds[0].Heats[0].Slots[15].Result.Mark != 312340 {
		t.Fatal("group result not milliseconds")
	}
	html, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "顺序") || !strings.Contains(html, "不分道") || strings.Contains(html, ">道次<") {
		t.Fatal("group report uses lanes")
	}
	// Import/export retains the actual mode rather than deriving it from the name.
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Version != interchange.Version || file.Events[0].StartMode != "group" {
		t.Fatalf("export %+v", file.Events[0])
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
	events, err := f.svc.ListEvents(f.ctx, imported.MeetID)
	must(t, err)
	again, err := f.svc.GetEvent(f.ctx, events[0].Event.ID)
	must(t, err)
	if again.Event.StartMode != domain.StartGroup || len(again.Rounds[0].Heats[0].Slots) != 16 || *again.Rounds[0].Heats[0].Slots[15].Result.Mark != 312340 {
		t.Fatal("group roundtrip changed data")
	}
	// Conflicting configuration is rejected without modifying the destination.
	file.Events[0].StartMode = "lanes"
	file.Events[0].Rounds = nil
	data, err = interchange.Encode(file)
	must(t, err)
	preview, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(preview.Errors) == 0 {
		t.Fatal("config conflict accepted")
	}
}

func TestThrowingAndRelayConfiguration(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "铅球", Kind: "distance", DistanceMode: "throw", AttemptCount: 3})
	f.addHeat(0, f.athletes...)
	d := f.detail()
	h := d.Rounds[0].Heats[0]
	must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 1, Attempts: []string{"8.50", "X", "免投"}}, {Position: 2, Attempts: []string{"8.40", "8.50", "8.10"}}}))
	d = f.detail()
	if d.Rules.AttemptLabel != "试投" || *d.Rounds[0].Heats[0].Slots[1].Result.Rank != 1 || *d.Rounds[0].Heats[0].Slots[0].Result.Rank != 2 {
		t.Fatal("throwing tie break or label")
	}
	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, `<th class="try">第3次</th>`) || !strings.Contains(html, `<td class="try best">8.50</td><td class="try">X</td><td class="try">-</td>`) {
		t.Fatal("throwing attempts not printed")
	}
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, EventInput{Gender: "男", Discipline: "铅球", Kind: "distance", DistanceMode: "throw", AttemptCount: 2})
	wantInvalid(t, err, "不能减少")
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Events[0].DistanceMode != "throw" {
		t.Fatal("lost throwing mode")
	}
	data, err := interchange.Encode(file)
	must(t, err)
	preview, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", preview.Hash)
	must(t, err)
	rows, err := f.svc.ListEvents(f.ctx, imported.MeetID)
	must(t, err)
	if rows[0].Event.DistanceMode != domain.DistanceThrow {
		t.Fatal("import lost throwing mode")
	}
	relay, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "混合", Discipline: "4×400m", Kind: "relay", StartMode: "break", RelayLegCount: 4})
	must(t, err)
	// A partial lineup is allowed, and exposes a warning to every read client.
	team, err := f.svc.SaveTeam(f.ctx, ui, relay.ID, "", TeamInput{Name: "A队", MemberIDs: f.athletes})
	must(t, err)
	rd, err := f.svc.GetEvent(f.ctx, relay.ID)
	must(t, err)
	if rd.Teams[0].ID != team.ID || rd.Teams[0].SizeWarning == "" || !rd.Rules.UsesLanes {
		t.Fatal("relay recommendation or break start")
	}

	file, err = f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err = interchange.Encode(file)
	must(t, err)
	preview, err = f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	imported, err = f.svc.ImportMeet(f.ctx, ui, data, "", preview.Hash)
	must(t, err)
	rows, err = f.svc.ListEvents(f.ctx, imported.MeetID)
	must(t, err)
	found := false
	for _, row := range rows {
		if row.Event.Kind == domain.KindRelay {
			found = true
			if row.Event.StartMode != domain.StartBreak || row.Event.RelayLegCount != 4 {
				t.Fatal("relay configuration changed during roundtrip")
			}
			again, err := f.svc.GetEvent(f.ctx, row.Event.ID)
			must(t, err)
			if len(again.Teams) != 1 || again.Teams[0].SizeWarning == "" {
				t.Fatal("partial roster changed during roundtrip")
			}
		}
	}
	if !found {
		t.Fatal("relay lost during roundtrip")
	}
}

func TestLaneAndScoredStartModeEditProtection(t *testing.T) {
	f := newFixture(t, 8, track())
	f.addHeat(0, f.athletes...)
	in := track()
	in.LaneCount = 4
	_, err := f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	wantInvalid(t, err, "已有道次超过")
	in = track()
	in.StartMode = "break"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	must(t, err)
	heat := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{{Position: 1, Mark: "12.34"}}))
	in.StartMode = "lanes"
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	wantInvalid(t, err, "不能改起跑方式")
}
