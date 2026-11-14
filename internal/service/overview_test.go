package service

import (
	"testing"
)

func TestOverview(t *testing.T) {
	f := newFixture(t, 4, track()) // one direct final, 一班 二班
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if got := overviewStates(o); got != "before events:done units:done entries:todo arrange:todo schedule:todo during results:todo after team:off " || o.Current != "entries" {
		t.Fatalf("blank = %s, current %s", got, o.Current)
	}
	if o.Units != 2 || o.Athletes != 4 || len(o.Next) != 0 {
		t.Fatalf("blank = %+v", o)
	}
	if b := o.Structure; len(b) != 1 || b[0].GroupID != "" || b[0].Athletes != 4 || len(b[0].Units) != 2 || b[0].Units[1].Name != "二班" || b[0].Units[1].Athletes != 2 ||
		len(b[0].Events) != 1 || b[0].Events[0].Name != "男子100m" || b[0].Events[0].State != StepTodo {
		t.Fatalf("structure = %+v", b)
	}

	f.enter(f.athletes[0], f.athletes[2]) // 一班 only
	f.addHeat(0, f.athletes[0], f.athletes[2])
	final := f.detail().Rounds[0]
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: final.ID, ScheduledAt: "2026-11-12T09:00"})
	must(t, err)
	must(t, f.svc.SaveResults(f.ctx, ui, final.Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.10"}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if got := overviewStates(o); got != "before events:done units:done entries:doing arrange:done schedule:done during results:doing after team:off " {
		t.Fatalf("started = %s", got)
	}
	if r := overviewStep(o, "results"); r.Done != 0 || r.Total != 1 || r.Extra != 1 {
		t.Fatalf("results = %+v", r)
	}
	if len(o.Next) != 1 || o.Next[0].StartText != "09:00" || !o.Next[0].Started || o.NextTotal != 1 {
		t.Fatalf("next = %+v", o.Next)
	}
	if u, e := o.Structure[0].Units[0], o.Structure[0].Events[0]; !u.Entered || o.Structure[0].Units[1].Entered || e.State != StepDoing {
		t.Fatalf("started structure = %+v", o.Structure)
	}

	must(t, f.svc.SaveResults(f.ctx, ui, final.Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.10"}, {Position: 2, Mark: "12.30"}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if r := overviewStep(o, "results"); r.State != StepDone || r.Done != 1 || r.Extra != 0 || len(o.Next) != 0 {
		t.Fatalf("finished = %+v, next %+v", r, o.Next)
	}
	if e := o.Structure[0].Events[0]; e.State != StepDone {
		t.Fatalf("finished event = %+v", e)
	}

	// With a group: its branch takes the unit and, once moved, the event
	// by its name without the group; the rest is 未分组.
	g, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高一"})
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[0], UnitInput{Name: "一班", GroupIDs: []string{g.ID}})
	must(t, err)
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if b := o.Structure; len(b) != 2 || b[0].Name != "高一" || len(b[0].Units) != 1 || b[0].Units[0].Athletes != 0 || len(b[0].Events) != 0 ||
		b[1].GroupID != "" || b[1].Name != "未分组" || len(b[1].Units) != 2 || len(b[1].Events) != 1 {
		t.Fatalf("grouped = %+v", b)
	}
	if u := overviewStep(o, "units"); u.Warnings != 1 {
		t.Fatalf("units of no group = %+v", u)
	}
	must(t, f.svc.ChangeGroup(f.ctx, ui, GroupChangeRequest{MeetID: f.meet, GroupID: g.ID, EventIDs: []string{f.event}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if b := o.Structure; len(b[0].Events) != 1 || b[0].Events[0].Name != "男子100m" || len(b[1].Events) != 0 {
		t.Fatalf("moved = %+v", b)
	}
}

// A heat and a final: 晋级 waits for the finished heat, with the tie on
// the qualifying place to rule, until the final has heats; 团体总分 counts
// the event once enabled, done with its final.
func TestOverviewAdvanceAndTeam(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 4, in)
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes[0], f.athletes[1])
	f.addHeat(0, f.athletes[2], f.athletes[3])
	_, err := f.svc.SaveTeamScoring(f.ctx, ui, f.meet, TeamScoringInput{Enabled: true, Places: "7、5", TiePolicy: "average", RelayMultiplier: "2", Overrides: []TeamScoreOverrideInput{}})
	must(t, err)
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if got := overviewStates(o); got != "before events:done units:done entries:done arrange:done schedule:todo during results:todo advance:todo after team:todo " || o.Current != "schedule" {
		t.Fatalf("arranged = %s, current %s", got, o.Current)
	}
	if a, r, tm := overviewStep(o, "advance"), overviewStep(o, "results"), overviewStep(o, "team"); a.Total != 1 || a.Done != 0 || a.Extra != 0 || r.Total != 2 || tm.Total != 1 || tm.Done != 0 ||
		r.Hint != "男子100m 预赛 待录成绩" || overviewStep(o, "schedule").Hint != "男子100m 预赛等 2 个未排时间" {
		t.Fatalf("arranged steps = %+v %+v %+v", a, r, tm)
	}

	heats := f.detail().Rounds[0].Heats
	must(t, f.svc.SaveResults(f.ctx, ui, heats[0].ID, []ResultInput{{Position: 1, Mark: "12.10"}, {Position: 2, Mark: "12.10"}}))
	must(t, f.svc.SaveResults(f.ctx, ui, heats[1].ID, []ResultInput{{Position: 1, Mark: "12.20"}, {Position: 2, Mark: "12.30"}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if a, r := overviewStep(o, "advance"), overviewStep(o, "results"); a.State != StepTodo || a.Extra != 1 || a.Warnings != 1 || r.Done != 1 || r.State != StepDoing ||
		a.Hint != "男子100m 编排决赛" || r.Hint != "" {
		t.Fatalf("heat finished = %+v %+v", a, r)
	}
	if o.Badges != (OverviewBadges{Arrange: 1, Schedule: 0, Results: 0}) {
		t.Fatalf("badges = %+v", o.Badges)
	}

	f.addHeat(1, f.athletes[0], f.athletes[1], f.athletes[2])
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if a := overviewStep(o, "advance"); a.State != StepDone || a.Extra != 0 || a.Warnings != 0 {
		t.Fatalf("advanced = %+v", a)
	}
	must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[1].Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.00"}, {Position: 2, Mark: "12.10"}, {Position: 3, Mark: "12.20"}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if tm, r := overviewStep(o, "team"), overviewStep(o, "results"); tm.State != StepDone || tm.Done != 1 || r.State != StepDone {
		t.Fatalf("final finished = %+v %+v", tm, r)
	}
}

// overviewStates reads "phase step:state …" in order.
func overviewStates(o *Overview) string {
	s := ""
	for _, ph := range o.Phases {
		s += ph.Key + " "
		for _, st := range ph.Steps {
			s += st.Key + ":" + st.State + " "
		}
	}
	return s
}

func overviewStep(o *Overview, key string) OverviewStep {
	for _, ph := range o.Phases {
		for _, st := range ph.Steps {
			if st.Key == key {
				return st
			}
		}
	}
	return OverviewStep{}
}

func TestCombinedProgress(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, StepTodo},
		{[]string{StepTodo, StepTodo}, StepTodo},
		{[]string{EventArranged, StepTodo}, EventArranged},
		{[]string{StepDone, StepTodo}, StepDoing},
		{[]string{StepDone, StepDone}, StepDone},
	} {
		if got := combinedProgress(c.in); got != c.want {
			t.Errorf("%v = %s, want %s", c.in, got, c.want)
		}
	}
}
