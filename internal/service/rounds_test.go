package service

import (
	"errors"
	"testing"
)

func roundStates(m *MeetRounds) string {
	s := ""
	for _, r := range m.Rounds {
		s += r.RoundLabel + ":" + r.Arrange + "/" + r.Results + " "
	}
	return s
}

func TestMeetRoundsAndBatchArrange(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 4, in)
	jump, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"})
	must(t, err)

	m, err := f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if got := roundStates(m); got != "预赛:empty/ 决赛:waiting/ 决赛:empty/ " {
		t.Fatalf("empty = %s", got)
	}
	if m.Rounds[0].Text != "男子100m 预赛" || m.Rounds[2].Text != "男子跳远" || m.Day != "" || len(m.Days) != 0 {
		t.Fatalf("rows = %+v", m)
	}

	f.enter(f.athletes...)
	must(t, f.svc.AddEntries(f.ctx, ui, jump.ID, []EntryInput{{AthleteID: f.athletes[0]}, {AthleteID: f.athletes[1]}}))
	m, err = f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if got := roundStates(m); got != "预赛:todo/ 决赛:waiting/ 决赛:todo/ " || !m.Rounds[0].CanArrange || m.Rounds[0].Unplaced != 4 || m.Rounds[1].CanArrange {
		t.Fatalf("entered = %s %+v", got, m.Rounds)
	}

	req := BatchArrangeRequest{MeetID: f.meet, Order: "seed", Lanes: "center", SeparateUnits: true, Balance: true, Seed: 7,
		Rounds: []RoundRef{{EventID: f.event, RoundID: m.Rounds[0].RoundID}, {EventID: f.event, RoundID: m.Rounds[1].RoundID}, {EventID: jump.ID, RoundID: m.Rounds[2].RoundID}}}
	p, err := f.svc.PreviewBatchArrange(f.ctx, req)
	must(t, err)
	if len(p.Errors) != 0 || p.Heats != 2 || p.People != 6 || p.Rounds[1].Error == "" || p.Rounds[0].PerHeat != 8 || p.Rounds[2].Source != "报名" {
		t.Fatalf("preview = %+v", p)
	}
	if _, err := f.svc.ApplyBatchArrange(f.ctx, ui, req, "stale", true); !errors.Is(err, ErrStale) {
		t.Fatalf("stale hash = %v", err)
	}
	_, err = f.svc.ApplyBatchArrange(f.ctx, ui, req, p.PreviewHash, true)
	must(t, err)

	m, err = f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if got := roundStates(m); got != "预赛:arranged/none 决赛:waiting/ 决赛:arranged/none " || m.Rounds[0].People != 4 || m.Rounds[0].Unplaced != 0 {
		t.Fatalf("arranged = %s %+v", got, m.Rounds)
	}
	// Nothing left: arranging again is refused.
	p, err = f.svc.PreviewBatchArrange(f.ctx, req)
	must(t, err)
	if len(p.Errors) == 0 {
		t.Fatalf("again = %+v", p)
	}

	heat := f.detail().Rounds[0].Heats[0]
	ids := []ResultInput{}
	for _, sl := range heat.Slots {
		ids = append(ids, ResultInput{Position: sl.Position, Mark: "12.10"})
	}
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, ids[:1]))
	m, err = f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if m.Rounds[0].Results != ResultsDoing {
		t.Fatalf("doing = %s", roundStates(m))
	}
	// A tie at the qualifying place waits for a ruling.
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, ids))
	m, err = f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if m.Rounds[0].Results != ResultsTies || m.Rounds[0].HeatsDone != 1 || m.Rounds[1].Arrange != RoundReady || !m.Rounds[1].CanArrange {
		t.Fatalf("ties = %s %+v", roundStates(m), m.Rounds[:2])
	}

	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: jump.ID, RoundID: m.Rounds[2].RoundID, ScheduledAt: "2026-11-13T09:00"})
	must(t, err)
	_, err = f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: m.Rounds[0].RoundID, ScheduledAt: "2026-11-12T10:00"})
	must(t, err)
	m, err = f.svc.MeetRounds(f.ctx, f.meet)
	must(t, err)
	if len(m.Days) != 2 || m.Days[0].Day != "2026-11-12" || m.Days[1].Rounds[0].EventID != jump.ID || m.Day != "2026-11-12" || len(m.Unscheduled) != 0 {
		t.Fatalf("days = %+v", m)
	}
}
