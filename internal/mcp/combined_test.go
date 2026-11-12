package mcp

import (
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
	"net/http"
	"testing"
	"time"
)

func TestSDKCombinedConfirmationAndConfiguredResults(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "combined-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args any, out any, fail bool) {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError != fail {
			t.Fatalf("%s: %+v", name, res.Content)
		}
		if fail {
			return
		}
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			if err := json.Unmarshal(data, out); err != nil {
				t.Fatalf("%s: %s: %v", name, data, err)
			}
		}
	}
	var meet struct {
		ID string `json:"id"`
	}
	call("create_meet", map[string]any{"input": service.MeetInput{Name: "MCP全能", StartDate: "2026-11-12"}}, &meet, false)
	var athlete struct {
		Athlete struct {
			ID string `json:"id"`
		} `json:"athlete"`
	}
	call("save_athlete", map[string]any{"meetId": meet.ID, "input": service.AthleteInput{Name: "甲", Gender: "男"}}, &athlete, false)
	policy := map[string]string{"DNS": "zero", "DNF": "zero", "DQ": "exclude", "NM": "zero"}
	input := service.CombinedInput{Name: "校园两项", Gender: "男", Components: []service.CombinedComponentInput{
		{Event: service.EventInput{Gender: "男", Discipline: "跳绳", Kind: "count"}, Rule: service.PointRuleInput{Table: "120\t100\n100\t80", InvalidPolicies: policy}},
		{Event: service.EventInput{Gender: "男", Discipline: "技巧评分", Kind: "score", ScoreDecimals: 2}, Rule: service.PointRuleInput{Table: "12.50\t100\n10.00\t80", InvalidPolicies: policy}},
	}}
	args := map[string]any{"meetId": meet.ID, "input": input}
	var preview service.MutationPreview
	call("preview_save_combined", args, &preview, false)
	call("save_combined", args, nil, true)
	args["previewHash"], args["confirmed"] = preview.PreviewHash, true
	var saved struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	call("save_combined", args, &saved, false)
	id := saved.Result.ID
	if id == "" {
		t.Fatal("missing combined id")
	}
	roster := map[string]any{"combinedId": id, "athleteIds": []string{athlete.Athlete.ID}}
	call("preview_combined_entries", roster, &preview, false)
	roster["previewHash"], roster["confirmed"] = preview.PreviewHash, true
	call("set_combined_entries", roster, nil, false)
	var detail service.CombinedDetail
	call("get_combined", map[string]any{"combinedId": id}, &detail, false)
	heats := []string{}
	for i, part := range detail.Components {
		var event service.EventDetail
		call("get_event", map[string]any{"eventId": part.Event.ID}, &event, false)
		plan := map[string]any{"plan": service.PlanRequest{EventID: part.Event.ID, RoundID: event.Rounds[0].ID, PerHeat: 12, Order: "seed", Lanes: "sequential"}}
		call("preview_arrange", plan, &preview, false)
		plan["previewHash"], plan["confirmed"] = preview.PreviewHash, true
		call("auto_arrange", plan, nil, false)
		call("get_event", map[string]any{"eventId": part.Event.ID}, &event, false)
		heat := event.Rounds[0].Heats[0]
		heats = append(heats, heat.ID)
		mark := []string{"120", "12.50"}[i]
		call("set_results", map[string]any{"heatId": heat.ID, "results": []service.ResultInput{{Position: 1, Mark: mark}}}, nil, false)
	}
	call("get_combined", map[string]any{"combinedId": id}, &detail, false)
	if !detail.Complete || detail.Rows[0].Total != 200 {
		t.Fatal("configured MCP scores")
	}
	req := service.CombinedRulesRequest{CombinedID: id}
	for i, c := range detail.Components {
		r := c.Input
		if i == 0 {
			r.Table = "120\t120\n100\t80"
		}
		req.Rules = append(req.Rules, service.CombinedRuleChange{EventID: c.Event.ID, Rule: r})
	}
	var rp service.CombinedRulesPreview
	call("preview_combined_rules", map[string]any{"request": req}, &rp, false)
	if rp.After[0].Total != 220 {
		t.Fatal("MCP rule preview")
	}
	// A UI score arriving after the MCP preview invalidates confirmation.
	if err := svc.SaveResults(ctx, store.SourceUI, heats[0], []service.ResultInput{{Position: 1, Mark: "110"}}); err != nil {
		t.Fatal(err)
	}
	call("apply_combined_rules", map[string]any{"request": req, "previewHash": rp.PreviewHash, "confirmed": true}, nil, true)
	call("preview_combined_rules", map[string]any{"request": req}, &rp, false)
	call("apply_combined_rules", map[string]any{"request": req, "previewHash": rp.PreviewHash, "confirmed": true}, nil, false)
	call("get_combined", map[string]any{"combinedId": id}, &detail, false)
	if detail.Combined.RuleVersion != 2 || detail.Rows[0].Total != 180 {
		t.Fatal("MCP rule commit")
	}
	call("preview_delete_combined", map[string]any{"combinedId": id}, &preview, false)
	if len(preview.Errors) == 0 {
		t.Fatal("scored combined deletion accepted")
	}
}
