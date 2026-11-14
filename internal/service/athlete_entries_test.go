package service

import "testing"

func TestAthleteEntries(t *testing.T) {
	f := newFixture(t, 2, track())
	girls, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "跳远", Kind: "distance"})
	must(t, err)
	relay, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "4×100m", Kind: "relay", LaneCount: 8})
	must(t, err)
	jump, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "混合", Discipline: "跳高", Kind: "height"})
	must(t, err)
	a := f.athletes[0]

	opts, err := f.svc.AthleteEntryOptions(f.ctx, f.meet, "", "男", "")
	must(t, err)
	if len(opts) != 2 || opts[0].EventID != f.event || opts[1].EventID != jump.ID {
		t.Fatalf("new athlete = %+v", opts)
	}
	must(t, f.svc.SetAthleteEntries(f.ctx, ui, f.meet, a, []string{f.event, jump.ID}))
	wantInvalid(t, f.svc.SetAthleteEntries(f.ctx, ui, f.meet, a, []string{relay.ID}), "按队报名")
	f.addHeat(0, a)
	wantInvalid(t, f.svc.SetAthleteEntries(f.ctx, ui, f.meet, a, []string{jump.ID}), "请先移出")
	must(t, f.svc.SetAthleteEntries(f.ctx, ui, f.meet, a, []string{f.event, girls.ID}))

	opts, err = f.svc.AthleteEntryOptions(f.ctx, f.meet, a, "男", "")
	must(t, err)
	// The girls' event they were put in stays listed so it can be undone.
	if len(opts) != 3 || !opts[0].Entered || !opts[0].Placed || !opts[1].Entered || opts[2].Entered {
		t.Fatalf("entered = %+v", opts)
	}
}
