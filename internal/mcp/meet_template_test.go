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

func TestSDKCreateMeetFromExisting(t *testing.T) {
	m, svc := setup(t)
	enable(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	source, err := svc.CreateMeet(ctx, store.SourceUI, service.MeetInput{Name: "来源赛事", StartDate: "2026-11-12"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveEvent(ctx, store.SourceUI, source.ID, "", service.EventInput{Gender: "女", Discipline: "100m", Kind: "track", GroupLabel: "高一", Rounds: []service.RoundInput{{Stage: "semifinal", AdvanceByPlace: 2, AdvanceByMark: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "meet-template-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: m.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{m.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	args := map[string]any{"input": map[string]any{"sourceMeetId": source.ID, "name": "新赛事", "startDate": "2027-04-20"}}
	res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "create_meet", Arguments: args})
	if err != nil || !res.IsError {
		t.Fatalf("creation without confirmation: %+v, %v", res, err)
	}
	res, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "preview_create_meet", Arguments: args})
	if err != nil || res.IsError {
		t.Fatalf("preview_create_meet: %+v, %v", res, err)
	}
	previewData, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var preview struct {
		PreviewHash string `json:"previewHash"`
		Result      struct {
			Projects struct {
				Events []domain.Event `json:"events"`
			} `json:"projects"`
		} `json:"result"`
	}
	if err := json.Unmarshal(previewData, &preview); err != nil || preview.PreviewHash == "" || len(preview.Result.Projects.Events) != 1 {
		t.Fatalf("incomplete preview: %s, %v", previewData, err)
	}
	args["previewHash"], args["confirmed"] = preview.PreviewHash, true
	res, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "create_meet", Arguments: args})
	if err != nil || res.IsError {
		t.Fatalf("create_meet: %+v, %v", res, err)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var created domain.Meet
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListEvents(ctx, created.ID)
	if err != nil || len(list) != 1 || list[0].Event.GroupLabel != "高一" || list[0].Entries != 0 || list[0].Heats != 0 || list[0].Results != 0 || len(list[0].Rounds) != 1 || list[0].Rounds[0].Stage != "semifinal" || list[0].Rounds[0].AdvanceByMark != 4 {
		t.Fatalf("copied events: %+v, %v", list, err)
	}
	audit, err := svc.ListAudit(ctx, service.AuditQuery{MeetID: created.ID})
	if err != nil || len(audit.Rows) != 1 || audit.Rows[0].Source != store.SourceMCP {
		t.Fatalf("audit: %+v, %v", audit, err)
	}
	res, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "create_meet", Arguments: args})
	if err != nil || !res.IsError {
		t.Fatalf("reused preview must be stale: %+v, %v", res, err)
	}
	res, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "update_meet", Arguments: map[string]any{"meetId": created.ID, "input": map[string]any{"sourceMeetId": source.ID, "name": "覆盖", "startDate": "2027-04-20"}}})
	if err != nil || !res.IsError {
		t.Fatalf("update must reject sourceMeetId: %+v, %v", res, err)
	}
}
