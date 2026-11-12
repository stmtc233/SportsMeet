package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func TestSDKHeatStarted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 12, 1, 42, 0, 0, time.UTC).UnixMilli()
	db.Now = func() int64 { return now }
	svc := service.New(db, nil)
	m := New(svc)
	t.Cleanup(func() { _ = m.Close(); _ = db.Close() })
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	enable(t, m)

	meet, err := svc.CreateMeet(ctx, store.SourceUI, service.MeetInput{Name: "开赛验证", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.SaveAthlete(ctx, store.SourceUI, meet.ID, "", service.AthleteInput{Name: "甲", Gender: "男", Bib: "1"})
	if err != nil {
		t.Fatal(err)
	}
	e, err := svc.SaveEvent(ctx, store.SourceUI, meet.ID, "", service.EventInput{Gender: "男", Discipline: "100m", Kind: "track", LaneCount: 8})
	if err != nil {
		t.Fatal(err)
	}
	d, err := svc.GetEvent(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveBoard(ctx, store.SourceUI, service.BoardSave{EventID: e.ID, Rounds: []service.RoundSave{{
		RoundID: d.Rounds[0].ID, Order: []string{"new"},
		Added: []service.HeatSave{{Key: "new", ScheduledAt: "2026-11-12T09:30", Slots: []service.SlotSave{{Position: 4, AthleteID: a.ID}}}},
	}}}); err != nil {
		t.Fatal(err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "started-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any, fail bool) map[string]any {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError != fail {
			t.Fatalf("%s IsError=%v: %+v", name, res.IsError, res.Content)
		}
		if fail {
			return nil
		}
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	firstHeat := func() map[string]any {
		t.Helper()
		detail := call("get_event", map[string]any{"eventId": e.ID}, false)
		return detail["rounds"].([]any)[0].(map[string]any)["heats"].([]any)[0].(map[string]any)
	}

	heat := firstHeat()
	if heat["startedAt"] != "" || heat["startDeltaText"] != "" {
		t.Fatalf("before results: %+v", heat)
	}
	// The first results record the start with the clock.
	call("set_results", map[string]any{"heatId": heat["id"], "results": []map[string]any{{"position": 4, "mark": "12.34"}}}, false)
	recorded := domain.WallClock(now, time.Local)
	heat = firstHeat()
	if heat["startedAt"] != recorded || heat["startDeltaText"] != domain.StartDeltaText("2026-11-12T09:30", recorded) {
		t.Fatalf("after results: %+v, want %s", heat, recorded)
	}

	out := call("set_heat_started", map[string]any{"input": map[string]any{"heatId": heat["id"], "startedAt": "2026-11-12T09:45"}}, false)
	if out["heatId"] != heat["id"] || out["startedAt"] != "2026-11-12T09:45" || out["startedText"] != "09:45" || out["startDeltaText"] != "晚 15 分钟" {
		t.Fatalf("set_heat_started = %+v", out)
	}
	after := firstHeat()
	if after["base"] != heat["base"] || after["keepReason"] != "已有成绩" || after["startDeltaText"] != "晚 15 分钟" {
		t.Fatalf("get_event after the change: %+v", after)
	}
	schedule := call("get_schedule", map[string]any{"meetId": meet.ID}, false)
	sh := schedule["days"].([]any)[0].(map[string]any)["rounds"].([]any)[0].(map[string]any)["heats"].([]any)[0].(map[string]any)
	if sh["startedAt"] != "2026-11-12T09:45" || sh["startedText"] != "09:45" || sh["startDeltaText"] != "晚 15 分钟" {
		t.Fatalf("get_schedule heat: %+v", sh)
	}
	call("set_heat_started", map[string]any{"input": map[string]any{"heatId": heat["id"], "startedAt": "09:45"}}, true)
	cleared := call("set_heat_started", map[string]any{"input": map[string]any{"heatId": heat["id"], "startedAt": ""}}, false)
	if cleared["startedAt"] != "" || cleared["startDeltaText"] != "" {
		t.Fatalf("cleared = %+v", cleared)
	}
	log, err := svc.ListAudit(ctx, service.AuditQuery{Source: "mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Rows) != 3 || log.Rows[0].Action != "heat.started" || log.Rows[1].Action != "heat.started" {
		t.Fatalf("log = %+v", log.Rows)
	}
}
