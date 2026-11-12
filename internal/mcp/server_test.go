package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func setup(t *testing.T) (*Manager, *service.Service) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, nil)
	m := New(svc)
	t.Cleanup(func() { _ = m.Close(); _ = db.Close() })
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m, svc
}
func port(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p
}
func enable(t *testing.T, m *Manager) {
	t.Helper()
	if err := m.Configure(context.Background(), true, port(t), false); err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticationAndLimits(t *testing.T) {
	m, _ := setup(t)
	if m.Status().Running || m.Status().Enabled || m.Token() != "" {
		t.Fatal("must default off")
	}
	enable(t, m)
	token := m.Token()
	cases := []struct {
		name, host, origin, auth string
		body                     string
		want                     int
	}{
		{name: "missing token", host: "127.0.0.1", want: 401},
		{name: "wrong token", host: "127.0.0.1", auth: "Bearer incorrect", want: 401},
		{name: "external origin", host: "127.0.0.1", origin: "https://example.com", auth: "Bearer " + token, want: 403},
		{name: "null origin", host: "127.0.0.1", origin: "null", auth: "Bearer " + token, want: 403},
		{name: "suffix origin", host: "127.0.0.1", origin: "http://localhost.evil.test", auth: "Bearer " + token, want: 403},
		{name: "userinfo origin", host: "127.0.0.1", origin: "http://evil@localhost", auth: "Bearer " + token, want: 403},
		{name: "rebinding host", host: "evil.test", auth: "Bearer " + token, want: 403},
		{name: "oversized body", host: "127.0.0.1", auth: "Bearer " + token, body: strings.Repeat("x", MaxBody+1), want: 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://127.0.0.1/mcp", strings.NewReader(tc.body))
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			req.Header.Set("Authorization", tc.auth)
			w := httptest.NewRecorder()
			m.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("code %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
	// Local browser origins pass the gate (SDK may reject malformed protocol requests).
	for _, origin := range []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://[::1]:5173"} {
		req := httptest.NewRequest("POST", "http://127.0.0.1/mcp", strings.NewReader("{}"))
		req.Host = "127.0.0.1"
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, req)
		if w.Code == 401 || w.Code == 403 {
			t.Fatalf("local origin rejected: %s %d", origin, w.Code)
		}
	}
}

func TestLifecycleRotationAndBusyPort(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	old := m.Status()
	token := m.Token()
	busy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if err := m.Configure(context.Background(), true, busy.Addr().(*net.TCPAddr).Port, false); err == nil {
		t.Fatal("accepted busy port")
	}
	if m.Status() != old || m.Token() != token {
		t.Fatal("busy port changed live state")
	}
	if err := m.Configure(context.Background(), true, old.Port, true); err != nil {
		t.Fatal(err)
	}
	if m.Token() == token {
		t.Fatal("rotation did not change token")
	}
	req, _ := http.NewRequest("POST", old.Address, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("old token accepted: %d", resp.StatusCode)
	}
	cfg, err := svc.MCPConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != m.Token() {
		t.Fatal("token not persisted")
	}
	next := port(t)
	if err := m.Configure(context.Background(), true, next, false); err != nil {
		t.Fatal(err)
	}
	if m.Status().Port != next {
		t.Fatal("port not changed")
	}
	if err := m.Configure(context.Background(), false, next, false); err != nil {
		t.Fatal(err)
	}
	c, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(next)), 100*time.Millisecond)
	if err == nil {
		c.Close()
		t.Fatal("disabled port still listening")
	}
	// Restart with the persisted enabled configuration.
	if err := m.Configure(context.Background(), true, next, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	restored := New(svc)
	defer restored.Close()
	if err := restored.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !restored.Status().Running || restored.Token() != cfg.Token {
		t.Fatal("restart lost configuration")
	}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func TestSDKClientToolsAndConfirmation(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) < 35 {
		t.Fatalf("missing tools: %d", len(list.Tools))
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	if names["configure_mcp"] || names["save_settings"] {
		t.Fatal("settings exposed")
	}
	call := func(name string, args any, fail bool) map[string]any {
		t.Helper()
		result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != fail {
			t.Fatalf("%s IsError=%v: %+v", name, result.IsError, result.Content)
		}
		if fail {
			return nil
		}
		b, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s: %s %v", name, b, err)
		}
		return out
	}
	created := call("create_meet", map[string]any{"input": map[string]any{"name": "MCP 验证", "startDate": "2026-11-12"}}, false)
	meetID, _ := created["id"].(string)
	if meetID == "" {
		t.Fatalf("missing meet id: %+v", created)
	}

	athlete := call("save_athlete", map[string]any{"meetId": meetID, "input": map[string]any{"name": "甲", "gender": "男", "bib": "1"}}, false)
	event := call("save_event", map[string]any{"meetId": meetID, "input": map[string]any{"gender": "男", "discipline": "100m", "kind": "track"}}, false)
	eventID := event["id"].(string)
	entryArgs := map[string]any{"eventId": eventID, "entries": []map[string]any{{"athleteId": athlete["athlete"].(map[string]any)["id"]}}}
	entryPreview := call("preview_entries", entryArgs, false)
	entryArgs["previewHash"], entryArgs["confirmed"] = entryPreview["previewHash"], true
	call("add_entries", entryArgs, false)
	detail := call("get_event", map[string]any{"eventId": eventID}, false)
	roundID := detail["rounds"].([]any)[0].(map[string]any)["id"]
	planArgs := map[string]any{"plan": map[string]any{"eventId": eventID, "roundId": roundID, "perHeat": 8, "order": "seed", "lanes": "center", "seed": 17}}
	planPreview := call("preview_arrange", planArgs, false)
	planArgs["previewHash"], planArgs["confirmed"] = planPreview["previewHash"], true
	call("auto_arrange", planArgs, false)
	detail = call("get_event", map[string]any{"eventId": eventID}, false)
	heat := detail["rounds"].([]any)[0].(map[string]any)["heats"].([]any)[0].(map[string]any)
	position := heat["slots"].([]any)[0].(map[string]any)["position"]
	request := service.ScheduleRequest{EventID: eventID, RoundID: roundID.(string), StartAt: "2026-11-12T23:58", IntervalMinutes: 3}
	desktopPreview, err := svc.PreviewSchedule(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	scheduleArgs := map[string]any{"request": request}
	schedulePreview := call("preview_schedule", scheduleArgs, false)
	if schedulePreview["previewHash"] != desktopPreview.PreviewHash {
		t.Fatal("desktop and MCP previews differ")
	}
	call("schedule_heats", scheduleArgs, true)
	scheduleArgs["previewHash"], scheduleArgs["confirmed"] = desktopPreview.PreviewHash, true
	call("schedule_heats", scheduleArgs, false)
	scheduled := call("get_event", map[string]any{"eventId": eventID}, false)
	scheduledHeat := scheduled["rounds"].([]any)[0].(map[string]any)["heats"].([]any)[0].(map[string]any)
	if scheduledHeat["scheduledAt"] != "2026-11-12T23:58" {
		t.Fatal("MCP schedule not persisted")
	}

	resultArgs := map[string]any{"heatId": heat["id"], "results": []map[string]any{{"position": position, "mark": "12.34"}}}
	call("set_results", resultArgs, false)
	call("schedule_heats", scheduleArgs, true) // Scores arriving after preview invalidate it.
	scoredPreview := call("preview_schedule", map[string]any{"request": request}, false)
	if len(scoredPreview["changes"].([]any)) != 0 || len(scoredPreview["skipped"].([]any)) != 1 {
		t.Fatal("scored group was not skipped")
	}
	call("set_results", resultArgs, true)
	// Do not delete the meet now that it has competition records.
	protected := call("preview_delete_meet", map[string]any{"meetId": meetID}, false)
	if len(protected["errors"].([]any)) == 0 {
		t.Fatal("MCP delete allowed scored meet")
	}

	// Relay uses the same HTTP tools and typed team identities throughout.
	relayEvent := call("save_event", map[string]any{"meetId": meetID, "input": map[string]any{"gender": "男", "discipline": "4×100m", "kind": "relay"}}, false)
	relayID := relayEvent["id"].(string)
	memberID := athlete["athlete"].(map[string]any)["id"]
	team := call("save_team", map[string]any{"eventId": relayID, "input": map[string]any{"name": "A队", "memberIds": []any{memberID}}}, false)
	teamID := team["id"]
	relayEntryArgs := map[string]any{"eventId": relayID, "entries": []any{map[string]any{"teamId": teamID}}}
	relayPreview := call("preview_entries", relayEntryArgs, false)
	relayEntryArgs["previewHash"], relayEntryArgs["confirmed"] = relayPreview["previewHash"], true
	call("add_entries", relayEntryArgs, false)
	call("set_seed", map[string]any{"eventId": relayID, "teamId": teamID, "seed": "51.50"}, false)
	rd, err := svc.GetEvent(ctx, relayID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rd.Teams) != 1 || rd.Teams[0].MemberIDs[0] != memberID || rd.Entries[0].SeedText != "51.50" || rd.Entries[0].AthleteID != "" {
		t.Fatalf("relay desktop detail: %+v", rd)
	}
	relayPlan := map[string]any{"plan": map[string]any{"eventId": relayID, "roundId": rd.Rounds[0].ID, "perHeat": 8, "order": "seed", "lanes": "center", "seed": 17}}
	rp := call("preview_arrange", relayPlan, false)
	relayPlan["previewHash"], relayPlan["confirmed"] = rp["previewHash"], true
	call("auto_arrange", relayPlan, false)
	rd, err = svc.GetEvent(ctx, relayID)
	if err != nil {
		t.Fatal(err)
	}
	rh := rd.Rounds[0].Heats[0]
	call("set_results", map[string]any{"heatId": rh.ID, "results": []any{map[string]any{"position": rh.Slots[0].Position, "mark": "49.12"}}}, false)
	rd, err = svc.GetEvent(ctx, relayID)
	if err != nil {
		t.Fatal(err)
	}
	if rd.Rounds[0].Standings[0].TeamID != teamID || *rd.Rounds[0].Standings[0].Rank != 1 {
		t.Fatal("relay not ranked")
	}
	call("save_team", map[string]any{"eventId": relayID, "id": teamID, "input": map[string]any{"name": "A队", "memberIds": []any{}}}, true)
	dp := call("preview_delete_team", map[string]any{"eventId": relayID, "id": teamID}, false)
	if len(dp["errors"].([]any)) == 0 {
		t.Fatal("scored team deletion allowed")
	}
	spare := call("save_team", map[string]any{"eventId": relayID, "input": map[string]any{"name": "备用队", "memberIds": []any{}}}, false)
	delArgs := map[string]any{"eventId": relayID, "id": spare["id"]}
	dp = call("preview_delete_team", delArgs, false)
	delArgs["previewHash"], delArgs["confirmed"] = dp["previewHash"], true
	call("delete_team", delArgs, false)
	disposable := call("create_meet", map[string]any{"input": map[string]any{"name": "删除验证", "startDate": "2026-11-12"}}, false)
	meetID = disposable["id"].(string)
	args := map[string]any{"meetId": meetID}
	preview := call("preview_delete_meet", args, false)
	call("delete_meet", args, true)
	args["previewHash"] = preview["previewHash"]
	args["confirmed"] = true
	call("delete_meet", args, false)
	// Legacy protocol initialization remains interoperable.
	req, _ := http.NewRequestWithContext(ctx, "POST", m.Status().Address, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"legacy","version":"1"}}}`))
	req.Header.Set("Authorization", "Bearer "+m.Token())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "SportsMeet") {
		t.Fatal(fmt.Sprintf("legacy: %d %s", resp.StatusCode, body))
	}
}
