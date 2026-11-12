package mcp

import (
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"net/http"
	"testing"
	"time"
)

func TestSDKEventConfiguration(t *testing.T) {
	m, _ := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "event-config-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any) json.RawMessage {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("%s: %+v", name, res.Content)
		}
		// The SDK retains the array in structured content.
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	data := call("list_event_presets", map[string]any{})
	var list []service.EventPreset
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 30 {
		t.Fatalf("catalogue: %s", data)
	}
	var meet map[string]any
	if err := json.Unmarshal(call("create_meet", map[string]any{"input": map[string]any{"name": "配置验证", "startDate": "2026-11-12"}}), &meet); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1500m", "铅球", "混合4×400m"} {
		var preset service.EventPreset
		for _, p := range list {
			if p.ID == id {
				preset = p
			}
		}
		var event struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(call("save_event", map[string]any{"meetId": meet["id"], "input": preset.Input}), &event); err != nil {
			t.Fatal(err)
		}
		var detail service.EventDetail
		if err := json.Unmarshal(call("get_event", map[string]any{"eventId": event.ID}), &detail); err != nil {
			t.Fatal(err)
		}
		if string(detail.Event.StartMode) != preset.Input.StartMode || string(detail.Event.DistanceMode) != preset.Input.DistanceMode || detail.Event.RelayLegCount != preset.Input.RelayLegCount {
			t.Fatalf("config not persisted: %+v", detail.Event)
		}
		if id == "1500m" && (detail.Rules.UsesLanes || !detail.Rules.Timed) {
			t.Fatal("group start lost through MCP")
		}
		if id == "铅球" && detail.Rules.AttemptLabel != "试投" {
			t.Fatal("throwing mode lost through MCP")
		}
	}
}
