package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Schedule a final before entries or heats exist, then fill its qualifiers
// without losing its planned time. Overview and timetable must agree throughout.
func TestPlanScheduleBeforeQualifiers(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1, AdvanceByMark: 1}}
	f := newFixture(t, 4, in)
	rs := f.detail().Rounds
	req := ScheduleRequest{Mode: "rounds", Rounds: []RoundRef{{f.event, rs[0].ID}, {f.event, rs[1].ID}},
		StartAt: "2026-11-12T09:00", IntervalMinutes: 120}
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if st := overviewStep(o, "schedule"); st.Total != 2 || st.Done != 0 || o.Badges.Schedule != 2 {
		t.Fatalf("empty rounds must need times: %+v / %+v", st, o.Badges)
	}
	p, err := f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	if len(p.Errors) != 0 || len(p.Changes) != 0 || len(p.RoundChanges) != 2 || p.RoundChanges[1].After != "2026-11-12T11:00" || len(p.Warnings) != 1 {
		t.Fatalf("round preview: %+v", p)
	}
	if f.detail().Rounds[1].ScheduledAt != "" {
		t.Fatal("preview wrote a round")
	}
	_, err = f.svc.ApplySchedule(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	tt, err := f.svc.Schedule(f.ctx, f.meet)
	must(t, err)
	final := tt.Days[0].Rounds[1]
	if tt.Pending != 0 || final.TimeState != StepDone || final.Arrange != RoundWaiting || final.PeopleText != "名单待定" || !final.ConflictPending || final.CanArrange || final.HeatCount != 0 {
		t.Fatalf("planned final awaiting qualifiers: %+v", final)
	}
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes...)
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if st := overviewStep(o, "schedule"); st.Done != 2 || o.Badges.Schedule != 0 || o.Badges.Arrange != 0 {
		t.Fatalf("final should only wait for qualifiers: %+v / %+v", st, o.Badges)
	}
	must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, []ResultInput{
		{Position: 1, Mark: "12.00"}, {Position: 2, Mark: "12.10"}, {Position: 3, Mark: "12.20"}, {Position: 4, Mark: "12.30"},
	}))
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if o.Badges.Arrange != 1 || o.Current != "advance" || overviewStep(o, "advance").Hint != "男子100m 决赛待编排 2 人" {
		t.Fatalf("qualifiers need arrangement: %+v", o)
	}
	// A placeholder group and then only one of two qualifiers must not finish it.
	f.addHeat(1)
	f.addHeat(1, f.athletes[0])
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if st := overviewStep(o, "advance"); st.Done != 0 || st.Extra != 1 || o.Badges.Arrange != 1 || st.Hint != "男子100m 决赛待编排 1 人" {
		t.Fatalf("partial advancement: %+v / %+v", st, o.Badges)
	}
	// Complete the q qualifier through the same batch arrangement used by the UI.
	batch := BatchArrangeRequest{MeetID: f.meet, Rounds: []RoundRef{{f.event, rs[1].ID}}, Order: "seed", Lanes: "sequential"}
	bp, err := f.svc.PreviewBatchArrange(f.ctx, batch)
	must(t, err)
	if len(bp.Errors) != 0 || bp.People != 1 {
		t.Fatalf("remaining qualifiers: %+v", bp)
	}
	_, err = f.svc.ApplyBatchArrange(f.ctx, ui, batch, bp.PreviewHash, true)
	must(t, err)
	d := f.detail().Rounds[1]
	if d.ScheduledAt != "2026-11-12T11:00" {
		t.Fatalf("final time changed: %s", d.ScheduledAt)
	}
	for _, h := range d.Heats {
		if h.ScheduledAt != "" || h.EffectiveAt != d.ScheduledAt || !h.ScheduleInherited {
			t.Fatalf("new heat did not inherit the plan: %+v", h)
		}
	}
	o, err = f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if overviewStep(o, "advance").State != StepDone || o.Badges.Arrange != 0 || o.Badges.Schedule != 0 {
		t.Fatalf("complete advancement: %+v", o)
	}
}

func TestScheduleCompletenessCountsEveryHeat(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T09:00")
	tt, err := f.svc.Schedule(f.ctx, f.meet)
	must(t, err)
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if tt.Pending != 1 || tt.Days[0].Rounds[0].TimeState != StepDoing || overviewStep(o, "schedule").Done != 0 || o.Badges.Schedule != 1 {
		t.Fatalf("one timed heat is not a whole round: %+v / %+v", tt, o)
	}
	roundStart(t, f, "2026-11-12T09:05")
	tt, err = f.svc.Schedule(f.ctx, f.meet)
	must(t, err)
	if tt.Pending != 0 || tt.Days[0].Rounds[0].TimeState != StepDone || tt.Days[0].Rounds[0].Heats[0].EffectiveAt != "2026-11-12T09:00" {
		t.Fatalf("round start should fill missing times and retain overrides: %+v", tt)
	}
}

func TestRoundScheduleChecksInheritedConflicts(t *testing.T) {
	tf := newTimetableFixture(t)
	roundStart(t, tf.on(tf.relay), "2026-11-12T09:00")
	r := tf.on(tf.sprint).detail().Rounds[0]
	req := ScheduleRequest{Mode: "rounds", EventID: tf.sprint, RoundID: r.ID, StartAt: "2026-11-12T09:00", IntervalMinutes: 5}
	p, err := tf.svc.PreviewSchedule(tf.ctx, req)
	must(t, err)
	if len(p.Conflicts) != 1 || !strings.Contains(p.Conflicts[0].First.Text, "09:00") || !strings.Contains(p.Conflicts[0].Second.Text, "09:00") {
		t.Fatalf("inherited conflicts need their proposed times: %+v", p)
	}
	setHeatTime(t, tf.fixture, 0, "2026-11-12T10:00")
	p, err = tf.svc.PreviewSchedule(tf.ctx, req)
	must(t, err)
	if len(p.Conflicts) != 0 {
		t.Fatalf("heat override must be preserved: %+v", p)
	}
}

func TestRoundPlanningWithNoQualifiers(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 2, in)
	f.addHeat(0, f.athletes...)
	must(t, f.svc.SaveResults(f.ctx, ui, f.detail().Rounds[0].Heats[0].ID, []ResultInput{{Position: 1, Status: "DNS"}, {Position: 2, Status: "DNS"}}))
	tt, err := f.svc.Schedule(f.ctx, f.meet)
	must(t, err)
	final := tt.Unscheduled[1]
	o, err := f.svc.Overview(f.ctx, f.meet)
	must(t, err)
	if final.Arrange != RoundReady || final.CanArrange || final.ConflictPending || final.Unplaced != 0 || o.Badges.Arrange != 0 {
		t.Fatalf("no qualifiers should not create an arrangement or conflict-check task: %+v / %+v", final, o.Badges)
	}
}

func TestRoundSchedulePreservesHeatsAndChecksConfirmation(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T08:00")
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[1].ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	before := f.detail().Rounds[0]
	req := ScheduleRequest{Mode: "rounds", EventID: f.event, RoundID: r.ID, StartAt: "2026-11-12T09:00", IntervalMinutes: 5}
	p, err := f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	// Any result arriving between preview and submission invalidates it.
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[2].ID, []ResultInput{{Position: 1, Mark: "12.40"}}))
	_, err = f.svc.ApplySchedule(f.ctx, ui, req, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("stale round schedule: %v", err)
	}
	before = f.detail().Rounds[0]
	p, err = f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	n := len(*f.changes)
	_, err = f.svc.ApplySchedule(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	after := f.detail().Rounds[0]
	for i, h := range after.Heats {
		old := before.Heats[i]
		if h.Base != old.Base || h.ScheduledAt != old.ScheduledAt || !reflect.DeepEqual(h.Slots, old.Slots) || h.StartedAt != old.StartedAt {
			t.Fatalf("round scheduling changed heat: %+v / %+v", old, h)
		}
	}
	if len(*f.changes) != n+1 {
		t.Fatal("expected one change notification")
	}
	p, err = f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	_, err = f.svc.ApplySchedule(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)
	if len(p.RoundChanges) != 0 || len(*f.changes) != n+1 {
		t.Fatal("no-op emitted a notification")
	}
	// Calendar overflow must reject the whole batch, including earlier rows.
	in := track()
	in.Discipline = "200m"
	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", in)
	must(t, err)
	other, err := f.svc.GetEvent(f.ctx, e.ID)
	must(t, err)
	req.Rounds = []RoundRef{{f.event, r.ID}, {e.ID, other.Rounds[0].ID}}
	req.StartAt = "9999-12-31T23:59"
	p, err = f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	if len(p.Errors) == 0 || !strings.Contains(p.Errors[0], "超出") || len(p.RoundChanges) != 0 {
		t.Fatalf("overflow must leave no changes: %+v", p)
	}
}
