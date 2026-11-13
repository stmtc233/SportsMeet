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

func TestSDKTeamScoringConfirmationAndUIStaleness(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "team-scoring-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args, out any, fail bool) {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError != fail {
			t.Fatalf("%s: %+v", name, res.Content)
		}
		if fail || out == nil {
			return
		}
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
	meet, err := svc.CreateMeet(ctx, store.SourceUI, service.MeetInput{Name: "团体计分", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	var detail service.TeamScoreDetail
	call("get_team_scores", map[string]any{"meetId": meet.ID, "groupLabel": ""}, &detail, false)
	if detail.Rules.Enabled {
		t.Fatal("enabled by default")
	}
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	request := service.TeamScoringRequest{MeetID: meet.ID, Rules: rules}
	args := map[string]any{"request": request}
	var preview service.TeamScoringPreview
	call("preview_team_scoring", args, &preview, false)
	call("apply_team_scoring", args, nil, true)
	args["confirmed"], args["previewHash"] = true, preview.PreviewHash
	_, err = svc.SaveUnit(ctx, store.SourceUI, meet.ID, "", service.UnitInput{Name: "一班"})
	if err != nil {
		t.Fatal(err)
	}
	call("apply_team_scoring", args, nil, true)
	call("preview_team_scoring", args, &preview, false)
	args["previewHash"] = preview.PreviewHash
	call("apply_team_scoring", args, nil, false)
	call("get_team_scores", map[string]any{"meetId": meet.ID}, &detail, false)
	if !detail.Rules.Enabled || len(detail.Rows) != 1 {
		t.Fatal("rules not shared")
	}
	call("apply_team_scoring", args, nil, true)
	call("render_report", map[string]any{"meetId": meet.ID, "kind": "team_scores"}, nil, false)
	call("get_team_scores", map[string]any{"meetId": meet.ID, "targetIds": []string{"missing"}}, nil, true)
}
