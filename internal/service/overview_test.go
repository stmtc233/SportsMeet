package service

import (
	"testing"
)

func TestOverview(t *testing.T) {
	f := newFixture(t, 4, track()) // one direct final, 一班 二班
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	state := func(o *Overview) string {
		s := ""
		for _, st := range o.Steps {
			s += st.Key + ":" + st.State + " "
		}
		return s
	}
	if got := state(o); got != "events:done entries:todo arrange:todo schedule:todo results:todo " {
		t.Fatalf("blank = %s", got)
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
	if got := state(o); got != "events:done entries:doing arrange:done schedule:done results:doing " {
		t.Fatalf("started = %s", got)
	}
	if r := o.Steps[4]; r.Done != 0 || r.Total != 1 || r.Extra != 1 {
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
	if r := o.Steps[4]; r.State != StepDone || r.Done != 1 || r.Extra != 0 || len(o.Next) != 0 {
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
	must(t, f.svc.ChangeGroup(f.ctx, ui, GroupChangeRequest{MeetID: f.meet, GroupID: g.ID, EventIDs: []string{f.event}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if b := o.Structure; len(b[0].Events) != 1 || b[0].Events[0].Name != "男子100m" || len(b[1].Events) != 0 {
		t.Fatalf("moved = %+v", b)
	}
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
