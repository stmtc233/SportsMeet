package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

const ui = store.SourceUI

// fixture is a meet with two units, athletes and one event.
type fixture struct {
	t        *testing.T
	ctx      context.Context
	svc      *Service
	changes  *[]Change
	meet     string
	units    []string
	athletes []string
	event    string
}

func newFixture(t *testing.T, athletes int, in EventInput) *fixture {
	t.Helper()
	svc, changes := newTestService(t)
	f := &fixture{t: t, ctx: context.Background(), svc: svc, changes: changes}
	m, err := svc.CreateMeet(f.ctx, ui, MeetInput{Name: "秋季运动会", StartDate: "2026-11-12"})
	must(t, err)
	f.meet = m.ID
	for _, name := range []string{"一班", "二班"} {
		u, err := svc.SaveUnit(f.ctx, ui, f.meet, "", UnitInput{Name: name})
		must(t, err)
		f.units = append(f.units, u.ID)
	}
	for i := range athletes {
		a, err := svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: fmt.Sprintf("选手%02d", i+1), Gender: "男",
			Bib: fmt.Sprintf("%03d", i+1), UnitID: f.units[i%2]})
		must(t, err)
		f.athletes = append(f.athletes, a.ID)
	}
	if in.Discipline != "" {
		e, err := svc.SaveEvent(f.ctx, ui, f.meet, "", in)
		must(t, err)
		f.event = e.ID
	}
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantInvalid(t *testing.T, err error, part string) {
	t.Helper()
	var ie *InputError
	if !errors.As(err, &ie) || !strings.Contains(ie.Msg, part) {
		t.Fatalf("err = %v, want an InputError containing %q", err, part)
	}
}

func (f *fixture) detail() *EventDetail {
	f.t.Helper()
	d, err := f.svc.GetEvent(f.ctx, f.event)
	must(f.t, err)
	return d
}

func (f *fixture) enter(ids ...string) {
	f.t.Helper()
	list := make([]EntryInput, len(ids))
	for i, id := range ids {
		list[i] = EntryInput{AthleteID: id}
	}
	must(f.t, f.svc.AddEntries(f.ctx, ui, f.event, list))
}

// addHeat saves one new heat with athletes in lanes 1, 2, 3 … of round.
func (f *fixture) addHeat(round int, ids ...string) {
	f.t.Helper()
	d := f.detail()
	r := d.Rounds[round]
	slots := make([]SlotSave, len(ids))
	for i, id := range ids {
		slots[i] = SlotSave{Position: i + 1, AthleteID: id}
	}
	order := []string{}
	for _, h := range r.Heats {
		order = append(order, h.ID)
	}
	must(f.t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{{
		RoundID: r.ID, Order: append(order, "new"), Added: []HeatSave{{Key: "new", Slots: slots}},
	}}}))
}

func track() EventInput {
	return EventInput{Gender: "男", Discipline: "100m", Kind: "track", LaneCount: 8}
}

func TestAthleteIdentityAndDelete(t *testing.T) {
	f := newFixture(t, 2, track())
	_, err := f.svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: "新人", Bib: "001"})
	wantInvalid(t, err, "改为更新")
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: "新人", Gender: "x"})
	wantInvalid(t, err, "性别")
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "改名", Bib: "001", StudentNumber: "S1", UnitID: f.units[0]})
	must(t, err) // keeping its own bib is not a clash
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[1], AthleteInput{Name: "选手02", StudentNumber: " s1 "})
	wantInvalid(t, err, "学号")

	f.addHeat(0, f.athletes[0])
	wantInvalid(t, f.svc.DeleteAthletes(f.ctx, ui, f.meet, f.athletes), "已排进组")
	must(t, f.svc.DeleteAthletes(f.ctx, ui, f.meet, f.athletes[1:]))
	list, err := f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if len(list) != 1 || !list[0].Placed || list[0].Entries != 1 || list[0].UnitName != "一班" {
		t.Fatalf("athletes = %+v", list)
	}
}

func TestEventRules(t *testing.T) {
	f := newFixture(t, 0, track())
	_, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男子", Discipline: "100 米", Kind: "track"})
	wantInvalid(t, err, "已有相同的项目")
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Discipline: "100m", Kind: "track"})
	wantInvalid(t, err, "性别")
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "4×100m", Kind: "relay"})
	wantInvalid(t, err, "接力")

	d := f.detail()
	if d.Event.Name != "男子100m" || len(d.Rounds) != 1 || d.Rounds[0].Stage != domain.StageFinal {
		t.Fatalf("new event = %+v", d)
	}
	in := track()
	in.Prelim, in.AdvanceByPlace, in.AdvanceByMark = true, 1, 2
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	must(t, err)
	d = f.detail()
	if len(d.Rounds) != 2 || d.Rounds[0].Stage != domain.StageHeat || d.Rounds[0].Advance.ByMark != 2 || d.Rounds[1].Ordinal != 2 {
		t.Fatalf("with prelim = %+v", d.Rounds)
	}
	in.Prelim = false
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, in)
	must(t, err)
	if d = f.detail(); len(d.Rounds) != 1 || d.Rounds[0].Ordinal != 1 {
		t.Fatalf("prelim removed = %+v", d.Rounds)
	}
	rows, err := f.svc.ListEvents(f.ctx, f.meet)
	must(t, err)
	if len(rows) != 1 || !rows[0].AutoName || rows[0].Prelim {
		t.Fatalf("ListEvents = %+v", rows)
	}
}

func TestEntries(t *testing.T) {
	f := newFixture(t, 3, track())
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[0], Seed: "12.5"}, {AthleteID: f.athletes[1]}}))
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[0], Seed: "11.0"}})) // already entered: kept
	wantInvalid(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[2], Seed: "abc"}}), "格式")
	must(t, f.svc.SetSeed(f.ctx, ui, f.event, f.athletes[1], "13.01"))
	d := f.detail()
	if len(d.Entries) != 2 || d.Entries[0].SeedText != "12.50" || *d.Entries[1].SeedMark != 13010 || d.Entries[0].UnitName != "一班" {
		t.Fatalf("entries = %+v", d.Entries)
	}
	f.addHeat(0, f.athletes[0], f.athletes[2]) // athletes[2] was not entered: placing enters
	d = f.detail()
	if len(d.Entries) != 3 || !d.Entries[0].Placed || d.Entries[1].Placed {
		t.Fatalf("after placing = %+v", d.Entries)
	}
	wantInvalid(t, f.svc.RemoveEntries(f.ctx, ui, f.event, []string{f.athletes[0]}), "先移出")
	must(t, f.svc.RemoveEntries(f.ctx, ui, f.event, []string{f.athletes[1]}))
	if d = f.detail(); len(d.Entries) != 2 {
		t.Fatalf("after removing = %+v", d.Entries)
	}
}

func TestSaveBoard(t *testing.T) {
	f := newFixture(t, 6, track())
	a := f.athletes
	f.addHeat(0, a[0], a[1], a[2])
	f.addHeat(0, a[3], a[4])
	d := f.detail()
	r := d.Rounds[0]
	h1, h2 := r.Heats[0], r.Heats[1]
	if h1.Ordinal != 1 || h2.Ordinal != 2 || h1.Label != "第1组" || len(h1.Slots) != 3 {
		t.Fatalf("heats = %+v", r.Heats)
	}
	save := func(rs RoundSave, more ...func(*BoardSave)) error {
		rs.RoundID = r.ID
		in := BoardSave{EventID: f.event, Rounds: []RoundSave{rs}}
		for _, fn := range more {
			fn(&in)
		}
		return f.svc.SaveBoard(f.ctx, ui, in)
	}

	// A stale fingerprint is refused, and so is an order missing a heat.
	err := save(RoundSave{Order: []string{h1.ID, h2.ID}, Changed: []HeatSave{{ID: h1.ID, Base: "old", Slots: nil}}})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("stale base: %v", err)
	}
	if err := save(RoundSave{Order: []string{h1.ID}}); !errors.Is(err, ErrStale) {
		t.Fatalf("short order: %v", err)
	}

	// Moving a[3] into heat 1 takes it out of heat 2, which was not submitted.
	slots := []SlotSave{{4, a[0]}, {5, a[1]}, {3, a[2]}, {6, a[3]}}
	must(t, save(RoundSave{Order: []string{h1.ID, h2.ID}, Changed: []HeatSave{{ID: h1.ID, Base: h1.Base, Name: "第1组", ScheduledAt: "2026-11-12T09:30", Slots: slots}}}))
	d = f.detail()
	h1, h2 = d.Rounds[0].Heats[0], d.Rounds[0].Heats[1]
	if len(h1.Slots) != 4 || h1.Slots[0].Position != 3 || h1.Slots[0].AthleteID != a[2] || h1.Name != "" || h1.ScheduledAt != "2026-11-12T09:30" {
		t.Fatalf("heat 1 = %+v", h1)
	}
	if len(h2.Slots) != 1 || h2.Slots[0].AthleteID != a[4] {
		t.Fatalf("heat 2 = %+v", h2)
	}

	// Lane 9 does not exist; the same athlete twice is refused.
	wantInvalid(t, save(RoundSave{Order: []string{h1.ID, h2.ID}, Changed: []HeatSave{{ID: h2.ID, Base: h2.Base, Slots: []SlotSave{{9, a[4]}}}}}), "超过 8 道")
	wantInvalid(t, save(RoundSave{Order: []string{h1.ID, h2.ID}, Changed: []HeatSave{{ID: h2.ID, Base: h2.Base, Slots: []SlotSave{{1, a[4]}, {2, a[4]}}}}}), "两次")

	// Reordering renumbers; a new heat in between takes the next ordinal.
	must(t, save(RoundSave{Order: []string{h2.ID, "k", h1.ID}, Added: []HeatSave{{Key: "k", Name: "加赛", Slots: []SlotSave{{1, a[5]}}}}}))
	d = f.detail()
	hs := d.Rounds[0].Heats
	if len(hs) != 3 || hs[0].ID != h2.ID || hs[0].Ordinal != 2 || hs[1].Label != "加赛" || hs[1].Ordinal != 3 || hs[2].ID != h1.ID || hs[2].Ordinal != 4 {
		t.Fatalf("after reorder = %+v", hs)
	}

	// Results lock a heat.
	h1 = hs[2]
	must(t, f.svc.SaveResults(f.ctx, ui, h1.ID, []ResultInput{{Position: 4, Mark: "12.34"}, {Position: 5, Mark: "12.50"}}))
	d = f.detail()
	hs = d.Rounds[0].Heats
	h1 = hs[2]
	if h1.KeepReason == "" || hs[0].KeepReason != "" {
		t.Fatalf("lock = %q / %q", h1.KeepReason, hs[0].KeepReason)
	}
	order := []string{hs[0].ID, hs[1].ID, h1.ID}
	wantInvalid(t, save(RoundSave{Order: order[:2], Deleted: []HeatRef{{h1.ID, h1.Base}}}), "不能删除")
	swapped := []SlotSave{{5, a[0]}, {4, a[1]}, {3, a[2]}, {6, a[3]}}
	wantInvalid(t, save(RoundSave{Order: order, Changed: []HeatSave{{ID: h1.ID, Base: h1.Base, Slots: swapped}}}), "先解锁")
	// A locked heat also keeps its athletes when another heat claims one.
	wantInvalid(t, save(RoundSave{Order: order, Changed: []HeatSave{{ID: hs[0].ID, Base: hs[0].Base, Slots: []SlotSave{{1, a[4]}, {2, a[0]}}}}}), "先解锁")
	// Renaming it is fine, and unlocked the swap carries each result along.
	must(t, save(RoundSave{Order: order, Changed: []HeatSave{{ID: h1.ID, Base: h1.Base, Name: "A 组", Slots: swapped}}},
		func(in *BoardSave) { in.Unlocked = []string{h1.ID} }))
	d = f.detail()
	h1 = d.Rounds[0].Heats[2]
	got := map[string]string{}
	for _, sl := range h1.Slots {
		if sl.Result != nil {
			got[sl.AthleteID] = fmt.Sprintf("%d:%s:%d", sl.Position, sl.Result.MarkText, *sl.Result.Rank)
		}
	}
	if h1.Label != "A 组" || len(got) != 2 || got[a[0]] != "5:12.34:1" || got[a[1]] != "4:12.50:2" {
		t.Fatalf("after unlock swap: %q %v", h1.Label, got)
	}

	// 「全部重置」 deletes everything, results included.
	var del []HeatRef
	for _, h := range d.Rounds[0].Heats {
		del = append(del, HeatRef{h.ID, h.Base})
	}
	must(t, save(RoundSave{Order: []string{}, Deleted: del}, func(in *BoardSave) { in.Reset = true }))
	if d = f.detail(); len(d.Rounds[0].Heats) != 0 {
		t.Fatalf("after reset = %+v", d.Rounds[0].Heats)
	}
	counts, _ := f.svc.CountEvent(f.ctx, f.event)
	if counts.Heats != 0 || counts.Results != 0 {
		t.Fatalf("counts = %+v", counts)
	}
	last := (*f.changes)[len(*f.changes)-1]
	if last.Kind != "heats" || last.EventID != f.event {
		t.Fatalf("last change = %+v", last)
	}
}

func TestTrackResultsRankAndQualify(t *testing.T) {
	in := track()
	in.Prelim, in.AdvanceByPlace, in.AdvanceByMark = true, 1, 1
	f := newFixture(t, 6, in)
	a := f.athletes
	f.addHeat(0, a[0], a[1], a[2])
	f.addHeat(0, a[3], a[4], a[5])
	hs := f.detail().Rounds[0].Heats

	wantInvalid(t, f.svc.SaveResults(f.ctx, ui, hs[0].ID, []ResultInput{{Position: 1, Mark: "abc"}}), "第 1 道")
	wantInvalid(t, f.svc.SaveResults(f.ctx, ui, hs[0].ID, []ResultInput{{Position: 1, Status: "OK"}}), "请填写成绩")
	must(t, f.svc.SaveResults(f.ctx, ui, hs[0].ID, []ResultInput{
		{Position: 1, Mark: "12.50"}, {Position: 2, Mark: "12.30"}, {Position: 3, Mark: "12.50"}}))
	d := f.detail()
	res := func(heat, slot int) *ResultView { return d.Rounds[0].Heats[heat].Slots[slot].Result }
	if *res(0, 0).Rank != 2 || *res(0, 1).Rank != 1 || *res(0, 2).Rank != 2 {
		t.Fatalf("ranks = %d %d %d", *res(0, 0).Rank, *res(0, 1).Rank, *res(0, 2).Rank)
	}
	if res(0, 1).Qualify != "" || d.Rounds[0].Complete {
		t.Fatal("Q given before the round is complete")
	}
	if _, err := f.svc.PlanHeats(f.ctx, PlanRequest{EventID: f.event, RoundID: d.Rounds[1].ID, FromAdvance: true, PerHeat: 8,
		Order: arrange.OrderSeed, Lanes: arrange.LanesCenter}); err == nil {
		t.Fatal("advanced from an unfinished round")
	}

	must(t, f.svc.SaveResults(f.ctx, ui, hs[1].ID, []ResultInput{
		{Position: 1, Mark: "12.40"}, {Position: 2, Status: "dns"}, {Position: 3, Mark: "13.00"}}))
	d = f.detail()
	q := []string{res(0, 0).Qualify, res(0, 1).Qualify, res(0, 2).Qualify, res(1, 0).Qualify, res(1, 1).Qualify, res(1, 2).Qualify}
	// Heat winners Q; a[0] and a[2] tie on the one q place, so both advance.
	if strings.Join(q, ",") != "q,Q,q,Q,," || !d.Rounds[0].Complete {
		t.Fatalf("qualify = %v complete = %v", q, d.Rounds[0].Complete)
	}
	if res(1, 1).Status != domain.StatusDNS || res(1, 1).Mark != nil || res(1, 1).Rank != nil {
		t.Fatalf("DNS = %+v", res(1, 1))
	}
	st := d.Rounds[0].Standings
	if len(st) != 6 || st[0].AthleteID != a[1] || *st[1].Rank != 2 || *st[2].Rank != 3 || *st[3].Rank != 3 || st[5].Rank != nil {
		t.Fatalf("standings = %+v", st)
	}

	plan, err := f.svc.PlanHeats(f.ctx, PlanRequest{EventID: f.event, RoundID: d.Rounds[1].ID, FromAdvance: true, PerHeat: 8,
		Order: arrange.OrderSeed, Lanes: arrange.LanesCenter, Exclude: []string{a[2]}})
	must(t, err)
	if len(plan) != 1 || len(plan[0].Slots) != 3 {
		t.Fatalf("final plan = %+v", plan)
	}
	for _, sl := range plan[0].Slots {
		if sl.ID == a[1] && sl.Position != 4 { // the fastest qualifier takes lane 4
			t.Fatalf("final lanes = %+v", plan[0].Slots)
		}
	}

	// Clearing a result takes Q / q away again.
	must(t, f.svc.SaveResults(f.ctx, ui, hs[1].ID, []ResultInput{{Position: 3}}))
	d = f.detail()
	if res(1, 2) != nil || res(0, 1).Qualify != "" || *res(1, 0).Rank != 1 {
		t.Fatalf("after clearing: %+v / %q", res(1, 2), res(0, 1).Qualify)
	}
}

func TestFieldResults(t *testing.T) {
	f := newFixture(t, 3, EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"})
	a := f.athletes
	f.addHeat(0, a[0], a[1], a[2])
	heat := f.detail().Rounds[0].Heats[0].ID
	wantInvalid(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{{Position: 1, Attempts: []string{"5.10", "", "5.30"}}}), "第 2 次未填")
	must(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{
		{Position: 1, Attempts: []string{"5.10", "X", "5.30", ""}},
		{Position: 2, Attempts: []string{"5.30", "5.20", "-"}},
		{Position: 3, Attempts: []string{"x", "X", "x"}},
	}))
	d := f.detail()
	s := d.Rounds[0].Heats[0].Slots
	if s[0].Result.MarkText != "5.30" || *s[0].Result.Rank != 2 || *s[1].Result.Rank != 1 || len(s[0].Result.Attempts) != 3 || s[0].Result.Attempts[1] != "X" {
		t.Fatalf("distance = %+v %+v", s[0].Result, s[1].Result)
	}
	if s[2].Result.Status != domain.StatusNM || s[2].Result.Rank != nil {
		t.Fatalf("three fouls = %+v", s[2].Result)
	}

	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "跳高", Kind: "height"})
	must(t, err)
	f.event = e.ID
	f.addHeat(0, a[0], a[1])
	heat = f.detail().Rounds[0].Heats[0].ID
	wantInvalid(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{{Position: 1, Bars: []BarInput{{"1.50", "OOOO"}}}}), "O、X、-")
	must(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{
		{Position: 1, Bars: []BarInput{{"1.50", "o"}, {"1.55", "xo"}, {"1.60", "xxx"}}},
		{Position: 2, Bars: []BarInput{{"1.50", "xo"}, {"1.55", "o"}, {"1.60", ""}}},
	}))
	s = f.detail().Rounds[0].Heats[0].Slots
	if s[0].Result.MarkText != "1.55" || *s[0].Result.Rank != 2 || *s[1].Result.Rank != 1 || len(s[1].Result.Bars) != 2 || s[0].Result.Bars[1].Trials != "XO" {
		t.Fatalf("height = %+v %+v", s[0].Result, s[1].Result)
	}
	_, err = f.svc.SaveEvent(f.ctx, ui, f.meet, f.event, EventInput{Gender: "男", Discipline: "跳高", Kind: "distance"})
	wantInvalid(t, err, "不能改项目类型")
	wantInvalid(t, f.svc.DeleteEvent(f.ctx, store.SourceMCP, f.event, false), "只能在界面上删除")
	must(t, f.svc.DeleteEvent(f.ctx, ui, f.event, true))
}

func TestPlanHeatsFromEntries(t *testing.T) {
	f := newFixture(t, 10, track())
	for i, id := range f.athletes {
		must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: id, Seed: fmt.Sprintf("12.%02d", i)}}))
	}
	round := f.detail().Rounds[0].ID
	req := PlanRequest{EventID: f.event, RoundID: round, PerHeat: 4, Order: arrange.OrderSeed, Lanes: arrange.LanesCenter,
		SeparateUnits: true, Balance: true, Seed: 1, Exclude: f.athletes[:2]}
	plan, err := f.svc.PlanHeats(f.ctx, req)
	must(t, err)
	n := 0
	for _, h := range plan {
		n += len(h.Slots)
		for _, sl := range h.Slots {
			if sl.ID == f.athletes[0] || sl.ID == f.athletes[1] {
				t.Fatal("an excluded athlete was planned")
			}
		}
	}
	if len(plan) != 2 || n != 8 {
		t.Fatalf("plan = %+v", plan)
	}
	req.PerHeat = 9
	_, err = f.svc.PlanHeats(f.ctx, req)
	wantInvalid(t, err, "超过道数")
}

func TestReports(t *testing.T) {
	f := newFixture(t, 3, track())
	f.addHeat(0, f.athletes...)
	heat := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{{Position: 1, Mark: "12.34"}, {Position: 2, Status: "DQ"}}))
	html, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	for _, want := range []string{"秋季运动会", "秩序册", "男子100m", "第1组", "选手01", "一班", "tabular-nums", "道次"} {
		if !strings.Contains(html, want) {
			t.Errorf("start list lacks %q", want)
		}
	}
	html, err = f.svc.RenderReport(f.ctx, ReportResults, f.meet, []string{f.event})
	must(t, err)
	for _, want := range []string{"成绩册", "12.34", "犯规取消", "选手02"} {
		if !strings.Contains(html, want) {
			t.Errorf("results lack %q", want)
		}
	}
	html, _ = f.svc.RenderReport(f.ctx, ReportResults, f.meet, []string{"other"})
	if strings.Contains(html, "男子100m") {
		t.Error("an event outside the selection was printed")
	}
	if _, err := f.svc.RenderReport(f.ctx, "x", f.meet, nil); err == nil {
		t.Error("unknown report kind rendered")
	}
}

func TestMeetUpdateDeleteCascade(t *testing.T) {
	f := newFixture(t, 2, track())
	f.addHeat(0, f.athletes...)
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "春季运动会", StartDate: "2027-04-01", EndDate: "2027-04-02"}))
	counts, err := f.svc.CountMeet(f.ctx, f.meet)
	must(t, err)
	if counts.Events != 1 || counts.Athletes != 2 || counts.Heats != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, "", UnitInput{Name: " 一班"})
	wantInvalid(t, err, "同名")
	must(t, f.svc.DeleteMeet(f.ctx, ui, f.meet))
	rows, err := f.svc.store.Query(f.ctx, `SELECT (SELECT count(*) FROM athletes) + (SELECT count(*) FROM slots) + (SELECT count(*) FROM entries) + (SELECT count(*) FROM units)`)
	must(t, err)
	if rows.Values[0][0].(int64) != 0 {
		t.Fatalf("rows left after deleting the meet: %v", rows.Values)
	}
	logs, _ := f.svc.store.Query(f.ctx, `SELECT count(*) FROM audit_log WHERE action = 'meet.deleted' AND source = 'ui'`)
	if logs.Values[0][0].(int64) != 1 {
		t.Fatal("deleting the meet was not logged")
	}
}
