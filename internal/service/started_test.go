package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func heatStart(t *testing.T, f *fixture, index int) HeatView {
	t.Helper()
	return f.detail().Rounds[0].Heats[index]
}

func TestStartedRecordedWithFirstResults(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T09:30")
	f.at(time.Date(2026, 11, 12, 1, 42, 0, 0, time.UTC)) // 09:42 in UTC+8
	heat := heatStart(t, f, 0)
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	got := heatStart(t, f, 0)
	if got.StartedAt != "2026-11-12T09:42" || got.StartedText != "09:42" || got.StartDeltaText != "晚 12 分钟" {
		t.Fatalf("after the first results: %+v", got.HeatStart)
	}
	if got.Base != heat.Base {
		t.Fatal("the actual start changed the heat's fingerprint")
	}

	// Later saves and clearing the results keep it.
	f.at(time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC))
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, []ResultInput{{Position: 1, Mark: "12.40"}}))
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, []ResultInput{{Position: 1}}))
	got = heatStart(t, f, 0)
	if got.StartedAt != "2026-11-12T09:42" || got.Slots[0].Result != nil {
		t.Fatalf("after editing and clearing: %+v, %+v", got.HeatStart, got.Slots[0].Result)
	}

	// A save that writes no result records nothing.
	other := heatStart(t, f, 1)
	must(t, f.svc.SaveResults(f.ctx, ui, other.ID, []ResultInput{{Position: 1}}))
	if got := heatStart(t, f, 1); got.StartedAt != "" || got.StartDeltaText != "" {
		t.Fatalf("a heat without results: %+v", got.HeatStart)
	}
}

func TestSetHeatStarted(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T09:30")
	heat := heatStart(t, f, 0)
	before := len(*f.changes)

	out, err := f.svc.SetHeatStarted(f.ctx, store.SourceMCP, HeatStartInput{HeatID: heat.ID, StartedAt: "2026-11-13T10:35"})
	must(t, err)
	if out.HeatID != heat.ID || out.StartedText != "2026-11-13 10:35" || out.StartDeltaText != "晚 1 天 1 小时 5 分钟" {
		t.Fatalf("result = %+v", out)
	}
	got := heatStart(t, f, 0)
	if got.HeatStart != out.HeatStart || got.Base != heat.Base || got.KeepReason != "" {
		t.Fatalf("detail = %+v, base %q → %q, keep %q", got.HeatStart, heat.Base, got.Base, got.KeepReason)
	}
	if n := len(*f.changes); n != before+1 || (*f.changes)[n-1].Kind != "heats" || (*f.changes)[n-1].EventID != f.event {
		t.Fatalf("changes = %+v", (*f.changes)[before:])
	}
	log, err := f.svc.ListAudit(f.ctx, AuditQuery{Source: "mcp"})
	must(t, err)
	if len(log.Rows) != 1 || log.Rows[0].Action != "heat.started" || log.Rows[0].ActionText != "修改实际开赛时间" || log.Rows[0].HeatID != heat.ID {
		t.Fatalf("log = %+v", log.Rows)
	}

	// The same value writes nothing.
	_, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: heat.ID, StartedAt: "2026-11-13T10:35"})
	must(t, err)
	if len(*f.changes) != before+1 {
		t.Fatal("an unchanged start was written")
	}

	// Earlier than planned, and on the schedule.
	_, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: heat.ID, StartedAt: "2026-11-12T09:25"})
	must(t, err)
	tt, err := f.svc.Schedule(f.ctx, f.meet)
	must(t, err)
	found := false
	for _, day := range tt.Days {
		for _, r := range day.Rounds {
			for _, h := range r.Heats {
				if h.ID == heat.ID {
					found = true
					if h.StartedText != "09:25" || h.StartDeltaText != "早 5 分钟" {
						t.Fatalf("schedule heat = %+v", h.HeatStart)
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("heat missing from the schedule")
	}

	// Without a planned time there is no difference.
	unplanned := heatStart(t, f, 1)
	out, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: unplanned.ID, StartedAt: "2026-11-12T10:00"})
	must(t, err)
	if out.StartedText == "" || out.StartDeltaText != "" {
		t.Fatalf("unplanned = %+v", out)
	}

	// Clearing.
	out, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: heat.ID})
	must(t, err)
	if got := heatStart(t, f, 0); out.StartedAt != "" || got.StartedAt != "" || got.StartedText != "" || got.StartDeltaText != "" {
		t.Fatalf("cleared = %+v, detail %+v", out, got.HeatStart)
	}

	_, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: heat.ID, StartedAt: "2026-11-12 09:30"})
	wantInvalid(t, err, "开赛时间格式")
	_, err = f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: "missing", StartedAt: "2026-11-12T09:30"})
	wantInvalid(t, err, "组不存在")
}

func TestStartedInReports(t *testing.T) {
	f := scheduleFixture(t)
	setHeatTime(t, f, 0, "2026-11-12T09:30")
	f.at(time.Date(2026, 11, 12, 1, 42, 0, 0, time.UTC))
	heat := heatStart(t, f, 0)
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))

	html, err := f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "实际开赛 09:42") || strings.Contains(html, "成绩时间") {
		t.Fatalf("results book without the results time:\n%s", html)
	}
	prefs, err := f.svc.MeetSettings(f.ctx, f.meet)
	must(t, err)
	prefs.PrintResultsTime = true
	must(t, f.svc.SaveMeetSettings(f.ctx, ui, f.meet, prefs))
	html, err = f.svc.RenderReport(f.ctx, ReportResults, f.meet, nil)
	must(t, err)
	if !strings.Contains(html, "成绩时间 2026-11-12 09:42") {
		t.Fatalf("results book with the results time:\n%s", html)
	}
	html, err = f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if strings.Contains(html, "实际开赛") || strings.Contains(html, "成绩时间") {
		t.Fatal("the start list prints the actual start")
	}
}

func TestStartedInterchange(t *testing.T) {
	f := scheduleFixture(t)
	heat := heatStart(t, f, 0)
	_, err := f.svc.SetHeatStarted(f.ctx, ui, HeatStartInput{HeatID: heat.ID, StartedAt: "2026-11-12T09:42"})
	must(t, err)
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Version != interchange.Version {
		t.Fatalf("version = %d", file.Version)
	}
	if got := file.Events[0].Rounds[0].Heats[0].StartedAt; got == nil || *got != "2026-11-12T09:42" {
		t.Fatalf("exported startedAt = %v", got)
	}
	if file.Events[0].Rounds[0].Heats[1].StartedAt != nil {
		t.Fatal("an unrecorded start was exported")
	}
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	done, err := f.svc.ImportMeet(f.ctx, ui, data, "", p.Hash)
	must(t, err)
	again, err := f.svc.ExportMeet(f.ctx, done.MeetID)
	must(t, err)
	if got := again.Events[0].Rounds[0].Heats[0].StartedAt; got == nil || *got != "2026-11-12T09:42" {
		t.Fatalf("imported startedAt = %v", got)
	}

	// Into the meet it came from, a changed start keeps the meet's own.
	*file.Events[0].Rounds[0].Heats[0].StartedAt = "2026-11-12T10:00"
	data, err = interchange.Encode(file)
	must(t, err)
	p, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p.Hash)
	must(t, err)
	if got := heatStart(t, f, 0); got.StartedAt != "2026-11-12T09:42" {
		t.Fatalf("existing heat start = %q", got.StartedAt)
	}

	// A v4 file imports with no start recorded.
	v4, err := os.ReadFile("../../testdata/interchange/sample-meet-v4.json")
	must(t, err)
	p, err = f.svc.PreviewImport(f.ctx, v4, "")
	must(t, err)
	if len(p.Errors) != 0 {
		t.Fatalf("v4 preview errors: %v", p.Errors)
	}
	done, err = f.svc.ImportMeet(f.ctx, ui, v4, "", p.Hash)
	must(t, err)
	out, err := f.svc.ExportMeet(f.ctx, done.MeetID)
	must(t, err)
	for _, e := range out.Events {
		for _, r := range e.Rounds {
			for _, h := range r.Heats {
				if h.StartedAt != nil {
					t.Fatalf("v4 heat with a start: %+v", h)
				}
			}
		}
	}
}
