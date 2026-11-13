package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/store"
)

func scheduleFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t, 4, track())
	for _, id := range f.athletes {
		f.addHeat(0, id)
	}
	return f
}

func setHeatTime(t *testing.T, f *fixture, index int, value string) {
	t.Helper()
	d := f.detail()
	r := d.Rounds[0]
	h := r.Heats[index]
	rs := RoundSave{RoundID: r.ID, Order: []string{}}
	for _, heat := range r.Heats {
		rs.Order = append(rs.Order, heat.ID)
	}
	hs := HeatSave{ID: h.ID, Base: h.Base, Name: h.Name, ScheduledAt: value, Slots: []SlotSave{}}
	for _, slot := range h.Slots {
		hs.Slots = append(hs.Slots, SlotSave{Position: slot.Position, AthleteID: slot.AthleteID})
	}
	rs.Changed = []HeatSave{hs}
	must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{rs}}))
}

func scheduleRequest(f *fixture) ScheduleRequest {
	return ScheduleRequest{EventID: f.event, RoundID: f.detail().Rounds[0].ID, StartAt: "2026-11-12T23:58", IntervalMinutes: 3}
}

func TestSchedulePreviewCommitAndNoop(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 1, "2026-11-12T10:25")
	setHeatTime(t, f, 2, "2026-11-12T10:30")
	h2 := f.detail().Rounds[0].Heats[1]
	must(t, f.svc.SaveResults(f.ctx, ui, h2.ID, []ResultInput{{Position: 1, Status: "DNS"}}))
	before := f.detail()
	changesBefore := len(*f.changes)
	in := scheduleRequest(f)
	p, err := f.svc.PreviewSchedule(f.ctx, in)
	must(t, err)
	if len(p.Errors) != 0 || len(p.Changes) != 3 || len(p.Skipped) != 1 || p.Skipped[0].HeatID != h2.ID {
		t.Fatalf("preview: %+v", p)
	}
	want := []string{"2026-11-12T23:58", "2026-11-13T00:01", "2026-11-13T00:04"}
	for i, c := range p.Changes {
		if c.After != want[i] {
			t.Fatalf("time %d: %s", i, c.After)
		}
	}
	if p.Changes[0].BeforeText != "未安排" || p.Changes[1].Before != "2026-11-12T10:30" || p.Changes[1].AfterText != "2026-11-13 00:01 · 3 分钟" ||
		p.Changes[0].BeforeMinutes != nil || p.Changes[0].AfterMinutes != 3 {
		t.Fatal("missing before/after")
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(f.detail())
	if string(a) != string(b) || len(*f.changes) != changesBefore {
		t.Fatal("preview wrote or notified")
	}
	_, err = f.svc.ApplySchedule(f.ctx, ui, in, p.PreviewHash, false)
	wantInvalid(t, err, "确认")
	changed := in
	changed.IntervalMinutes = 4
	_, err = f.svc.ApplySchedule(f.ctx, ui, changed, p.PreviewHash, true)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("changed parameters: %v", err)
	}
	_, err = f.svc.ApplySchedule(f.ctx, ui, in, p.PreviewHash, true)
	must(t, err)
	after := f.detail()
	if after.Rounds[0].Heats[1].ScheduledAt != h2.ScheduledAt || after.Rounds[0].Heats[1].DurationMinutes != nil {
		t.Fatal("scored heat time changed")
	}
	if d := after.Rounds[0].Heats[2].DurationMinutes; d == nil || *d != 3 || after.Rounds[0].Heats[2].EndAt != "2026-11-13T00:04" {
		t.Fatal("interval not written as the heats' duration")
	}
	if len(*f.changes) != changesBefore+1 || (*f.changes)[changesBefore].Kind != "heats" {
		t.Fatal("missing change notification")
	}
	for i, h := range after.Rounds[0].Heats {
		a, _ := json.Marshal(before.Rounds[0].Heats[i].Slots)
		b, _ := json.Marshal(h.Slots)
		if string(a) != string(b) {
			t.Fatal("scheduling changed positions or results")
		}
		if h.ID != h2.ID && h.Base == before.Rounds[0].Heats[i].Base {
			t.Fatal("time did not change the board fingerprint")
		}
	}
	html, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "2026-11-13 00:01") {
		t.Fatal("report lost the next-day schedule")
	}
	exported, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	scheduledAt := exported.Events[0].Rounds[0].Heats[2].ScheduledAt
	if scheduledAt == nil || *scheduledAt != "2026-11-13T00:01" {
		t.Fatal("export lost the next-day schedule")
	}
	rows, err := f.svc.store.Query(f.ctx, `SELECT source, detail_json FROM audit_log WHERE action = 'heats.scheduled'`)
	must(t, err)
	if len(rows.Values) != 1 || rows.Values[0][0] != "ui" || !strings.Contains(rows.Values[0][1].(string), "2026-11-12T10:30") {
		t.Fatalf("audit: %+v", rows.Values)
	}
	p, err = f.svc.PreviewSchedule(f.ctx, in)
	must(t, err)
	if len(p.Changes) != 0 || len(p.Skipped) != 1 {
		t.Fatal("identical schedule must be a no-op")
	}
	_, err = f.svc.ApplySchedule(f.ctx, ui, in, p.PreviewHash, true)
	must(t, err)
	if len(*f.changes) != changesBefore+1 {
		t.Fatal("no-op notified")
	}
}

func TestScheduleStalePreview(t *testing.T) {
	for _, change := range []string{"results", "time", "new heat", "positions", "name and order"} {
		t.Run(change, func(t *testing.T) {
			f := scheduleFixture(t)
			in := scheduleRequest(f)
			p, err := f.svc.PreviewSchedule(f.ctx, in)
			must(t, err)
			d := f.detail()
			r := d.Rounds[0]
			h := r.Heats[0]
			switch change {
			case "results":
				must(t, f.svc.SaveResults(f.ctx, ui, h.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
			case "time":
				setHeatTime(t, f, 0, "2026-11-12T11:00")
			case "new heat":
				f.addHeat(0)
			case "positions", "name and order":
				rs := RoundSave{RoundID: r.ID, Order: []string{}}
				for _, heat := range r.Heats {
					rs.Order = append(rs.Order, heat.ID)
				}
				hs := HeatSave{ID: h.ID, Base: h.Base, Slots: []SlotSave{{Position: 1, AthleteID: h.Slots[0].AthleteID}}}
				if change == "positions" {
					hs.Slots[0].Position = 2
				} else {
					hs.Name = "加赛"
					rs.Order[0], rs.Order[1] = rs.Order[1], rs.Order[0]
				}
				rs.Changed = []HeatSave{hs}
				must(t, f.svc.SaveBoard(f.ctx, ui, BoardSave{EventID: f.event, Rounds: []RoundSave{rs}}))
			}
			before, _ := json.Marshal(f.detail())
			_, err = f.svc.ApplySchedule(f.ctx, store.SourceMCP, in, p.PreviewHash, true)
			if !errors.Is(err, ErrStale) {
				t.Fatalf("stale after %s: %v", change, err)
			}
			after, _ := json.Marshal(f.detail())
			if string(before) != string(after) {
				t.Fatal("stale submission wrote data")
			}
		})
	}
}

func TestScheduleInvalidDatesAndRanges(t *testing.T) {
	f := scheduleFixture(t)
	for _, start := range []string{"", "2026-02-30T09:00", "2026-11-12T24:00", "2026-11-12T9:00", "2026-11-12T09:00:01", "2026-11-12T09:00Z", "0000-01-01T09:00"} {
		in := scheduleRequest(f)
		in.StartAt = start
		p, err := f.svc.PreviewSchedule(f.ctx, in)
		must(t, err)
		if len(p.Errors) == 0 || p.PreviewHash != "" || len(p.Changes) > 0 {
			t.Fatalf("accepted start %q: %+v", start, p)
		}
	}
	for _, interval := range []int{-1, 0, 1441, int(^uint(0) >> 1)} {
		in := scheduleRequest(f)
		in.IntervalMinutes = interval
		p, err := f.svc.PreviewSchedule(f.ctx, in)
		must(t, err)
		if len(p.Errors) == 0 || p.PreviewHash != "" {
			t.Fatalf("accepted interval %d", interval)
		}
	}
	in := scheduleRequest(f)
	in.StartAt = "9999-12-31T23:59"
	p, err := f.svc.PreviewSchedule(f.ctx, in)
	must(t, err)
	if len(p.Errors) == 0 || len(p.Changes) != 0 || p.PreviewHash != "" {
		t.Fatal("overflow must not produce a committable partial plan")
	}
	_, err = f.svc.ApplySchedule(f.ctx, ui, in, "invalid", true)
	wantInvalid(t, err, "日期范围")
	in = scheduleRequest(f)
	in.RoundID = "missing"
	_, err = f.svc.PreviewSchedule(f.ctx, in)
	wantInvalid(t, err, "轮次")
}

func TestScheduleLocalCalendar(t *testing.T) {
	f := scheduleFixture(t)
	for _, tc := range []struct {
		start    string
		interval int
		second   string
	}{
		{"2028-02-28T23:59", 1, "2028-02-29T00:00"},
		{"2026-03-08T01:59", 2, "2026-03-08T02:01"},
		{"2026-12-31T23:59", 1440, "2027-01-01T23:59"},
	} {
		in := scheduleRequest(f)
		in.StartAt = tc.start
		in.IntervalMinutes = tc.interval
		p, err := f.svc.PreviewSchedule(f.ctx, in)
		must(t, err)
		if len(p.Errors) > 0 || p.Changes[1].After != tc.second {
			t.Fatalf("calendar: %+v", p)
		}
	}
}

func TestScheduleRoundScopeAndScoredOnly(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 1, in)
	f.addHeat(0, f.athletes[0])
	f.addHeat(1, f.athletes[0])
	req := scheduleRequest(f)
	p, err := f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	final := f.detail().Rounds[1].Heats[0]
	must(t, f.svc.SaveResults(f.ctx, ui, final.ID, []ResultInput{{Position: 1, Status: "DNS"}}))
	_, err = f.svc.ApplySchedule(f.ctx, store.SourceMCP, req, p.PreviewHash, true)
	must(t, err)
	if f.detail().Rounds[1].Heats[0].ScheduledAt != "" {
		t.Fatal("scheduled the wrong round")
	}
	req.RoundID = f.detail().Rounds[1].ID
	p, err = f.svc.PreviewSchedule(f.ctx, req)
	must(t, err)
	if len(p.Changes) != 0 || len(p.Skipped) != 1 || len(p.Errors) != 0 {
		t.Fatal("scored-only round preview")
	}
	// An empty round is a valid no-op.
	empty := newFixture(t, 0, track())
	p, err = empty.svc.PreviewSchedule(empty.ctx, scheduleRequest(empty))
	must(t, err)
	if len(p.Errors) != 0 || len(p.Changes) != 0 || p.PreviewHash == "" {
		t.Fatal("empty round preview")
	}
}

func TestScheduleConcurrentConfirmationAndRollback(t *testing.T) {
	f := scheduleFixture(t)
	in := scheduleRequest(f)
	p, err := f.svc.PreviewSchedule(f.ctx, in)
	must(t, err)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.svc.ApplySchedule(f.ctx, store.SourceMCP, in, p.PreviewHash, true)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrStale) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent successes: %d", success)
	}
	// Inject a failure in the second update and check the real SQLite transaction rolls back the first.
	in.StartAt = "2026-11-13T10:00"
	p, err = f.svc.PreviewSchedule(f.ctx, in)
	must(t, err)
	second := f.detail().Rounds[0].Heats[1].ID
	must(t, f.svc.store.Write(f.ctx, store.Stmt(fmt.Sprintf(`CREATE TRIGGER fail_schedule BEFORE UPDATE OF scheduled_at ON heats WHEN NEW.id = '%s' BEGIN SELECT RAISE(ABORT, 'blocked'); END`, second))))
	before, _ := json.Marshal(f.detail())
	notified := len(*f.changes)
	_, err = f.svc.ApplySchedule(f.ctx, ui, in, p.PreviewHash, true)
	if err == nil {
		t.Fatal("injected write failure succeeded")
	}
	after, _ := json.Marshal(f.detail())
	if string(before) != string(after) || len(*f.changes) != notified {
		t.Fatal("failed schedule partially wrote or notified")
	}
}
