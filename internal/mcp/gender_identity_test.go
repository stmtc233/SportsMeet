package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func TestSDKIndependentAthleteGenderFieldsAndLegacyUpdates(t *testing.T) {
	manager, svc := setup(t)
	enable(t, manager)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	meet, err := svc.CreateMeet(ctx, store.SourceUI, service.MeetInput{Name: "性别资料测试", StartDate: "2026-11-16"})
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "gender-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: manager.Status().Address, HTTPClient: &http.Client{Transport: bearerTransport{manager.Token()}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	id := ""
	save := func(input map[string]any, fail bool) *service.AthleteSave {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "save_athlete", Arguments: map[string]any{"meetId": meet.ID, "id": id, "input": input}})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError != fail {
			t.Fatalf("save_athlete: %+v", res.Content)
		}
		if fail {
			return nil
		}
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var saved service.AthleteSave
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		id = saved.Athlete.ID
		return &saved
	}
	saved := save(map[string]any{"name": "甲", "competitionGender": "女", "genderIdentity": "非二元"}, false)
	if saved.Athlete.CompetitionGender != "女" || saved.Athlete.GenderIdentity != "非二元" {
		t.Fatal(saved)
	}
	saved = save(map[string]any{"name": "甲", "gender": "男"}, false)
	if saved.Athlete.CompetitionGender != "男" || saved.Athlete.GenderIdentity != "非二元" {
		t.Fatal("legacy update lost identity", saved)
	}
	saved = save(map[string]any{"name": "甲", "genderIdentity": "不愿透露"}, false)
	if saved.Athlete.CompetitionGender != "男" {
		t.Fatal("omission cleared category", saved)
	}
	saved = save(map[string]any{"name": "甲", "gender": ""}, false)
	if saved.Athlete.CompetitionGender != "" || saved.Athlete.GenderIdentity != "不愿透露" {
		t.Fatal("explicit legacy clearing lost", saved)
	}
	saved = save(map[string]any{"name": "甲", "competitionGender": "女子"}, false)
	if saved.Athlete.CompetitionGender != "女" || saved.Athlete.GenderIdentity != "不愿透露" {
		t.Fatal(saved)
	}
	save(map[string]any{"name": "甲", "gender": "", "competitionGender": "女"}, true)
	saved = save(map[string]any{"name": "甲", "competitionGender": "", "genderIdentity": ""}, false)
	if saved.Athlete.CompetitionGender != "" || saved.Athlete.GenderIdentity != "" {
		t.Fatal("explicit clearing was ignored", saved)
	}
}
