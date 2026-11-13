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

func TestSDKGroupsConfirmationAndSharedReports(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "groups-test", Version: "1"}, nil)
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
	meet, err := svc.CreateMeet(ctx, store.SourceUI, service.MeetInput{Name: "分年级", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	var g1, g2 domain.CompetitionGroup
	call("save_group", map[string]any{"meetId": meet.ID, "input": service.GroupInput{Name: "初一"}}, &g1, false)
	call("save_group", map[string]any{"meetId": meet.ID, "input": service.GroupInput{Name: "初二"}}, &g2, false)
	call("list_groups", map[string]any{"meetId": meet.ID}, nil, false)
	e, err := svc.SaveEvent(ctx, store.SourceUI, meet.ID, "", service.EventInput{Gender: "男", Discipline: "100m", Kind: "track", GroupID: g1.ID})
	if err != nil {
		t.Fatal(err)
	}
	req := service.GroupChangeRequest{MeetID: meet.ID, GroupID: g2.ID, EventIDs: []string{e.ID}}
	args := map[string]any{"request": req}
	var preview service.GroupReview
	call("preview_group_change", args, &preview, false)
	call("apply_group_change", args, nil, true)
	args["confirmed"] = true
	args["previewHash"] = preview.PreviewHash
	if _, err = svc.SaveGroup(ctx, store.SourceUI, meet.ID, g2.ID, service.GroupInput{Name: "八年级"}); err != nil {
		t.Fatal(err)
	}
	call("apply_group_change", args, nil, true)
	call("preview_group_change", args, &preview, false)
	args["previewHash"] = preview.PreviewHash
	call("apply_group_change", args, nil, false)
	detail, err := svc.GetEvent(ctx, e.ID)
	if err != nil || detail.Event.GroupID != g2.ID || detail.Event.GroupLabel != "八年级" {
		t.Fatal(detail, err)
	}
	call("get_team_score_boards", map[string]any{"meetId": meet.ID, "groupId": g2.ID}, nil, false)
	call("suggest_groups", map[string]any{"meetId": meet.ID}, nil, false)
	call("render_report", map[string]any{"meetId": meet.ID, "kind": "results", "groupIds": []string{g2.ID}}, nil, false)
	call("render_report", map[string]any{"meetId": meet.ID, "kind": "results", "groupIds": []string{"missing"}}, nil, true)
	for _, g := range []string{g2.ID, g1.ID} {
		var deletion service.MutationPreview
		del := map[string]any{"meetId": meet.ID, "id": g}
		call("preview_delete_group", del, &deletion, false)
		if len(deletion.Errors) > 0 {
			t.Fatal(deletion.Errors)
		}
		del["confirmed"] = true
		del["previewHash"] = deletion.PreviewHash
		call("delete_group", del, nil, false)
	}
	detail, err = svc.GetEvent(ctx, e.ID)
	if err != nil || detail.Event.GroupID != "" || detail.Event.GroupLabel != "" {
		t.Fatal("deleted group still referenced", detail, err)
	}
}
