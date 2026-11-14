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

	must(t, f.svc.SaveResults(f.ctx, ui, final.Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.10"}, {Position: 2, Mark: "12.30"}}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if r := o.Steps[4]; r.State != StepDone || r.Done != 1 || r.Extra != 0 || len(o.Next) != 0 {
		t.Fatalf("finished = %+v, next %+v", r, o.Next)
	}
}
