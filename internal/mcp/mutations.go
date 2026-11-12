package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type confirmation struct {
	PreviewHash string `json:"previewHash,omitempty" jsonschema:"对应预览的指纹；提交需保持同一组参数"`
	Confirmed   bool   `json:"confirmed,omitempty" jsonschema:"只有用户明确确认预览后才能设为 true"`
}
type entriesArg struct {
	EventID string               `json:"eventId"`
	Entries []service.EntryInput `json:"entries"`
	confirmation
}
type removeEntriesArg struct {
	EventID    string   `json:"eventId"`
	AthleteIDs []string `json:"athleteIds,omitempty"`
	TeamIDs    []string `json:"teamIds,omitempty"`
	confirmation
}
type resultsArg struct {
	HeatID  string                `json:"heatId"`
	Results []service.ResultInput `json:"results"`
	confirmation
}
type planArg struct {
	Plan service.PlanRequest `json:"plan"`
	confirmation
}
type importArg struct {
	MeetID string `json:"meetId,omitempty" jsonschema:"留空导入为新赛事"`
	Data   string `json:"data" jsonschema:"完整交换格式 JSON 文本，包含 format、version、meet、units、athletes、events"`
	confirmation
}
type tableArg struct {
	MeetID string                `json:"meetId"`
	Table  service.AthleteImport `json:"table"`
	confirmation
}
type scheduleArg struct {
	Request service.ScheduleRequest `json:"request"`
	confirmation
}

type deleteArg struct {
	MeetID     string   `json:"meetId,omitempty"`
	EventID    string   `json:"eventId,omitempty"`
	HeatID     string   `json:"heatId,omitempty"`
	ID         string   `json:"id,omitempty" jsonschema:"参赛单位 ID，仅 delete_unit 使用"`
	AthleteIDs []string `json:"athleteIds,omitempty"`
	confirmation
}

func mutationPair[In any](s *sdk.Server, svc *service.Service, previewName, name, description string, decode func(In) (service.Mutation, confirmation)) {
	addTool(s, previewName, "预览："+description+"。返回完整影响范围、errors 与 previewHash；向用户展示并取得确认后才能提交。", true, func(ctx context.Context, in In) (any, error) {
		op, _ := decode(in)
		return svc.PreviewMutation(ctx, op)
	})
	addTool(s, name, description+"。批量操作、删除与成绩覆盖必须先预览，携带相同参数、previewHash 与 confirmed=true；只填空成绩可直接提交。", false, func(ctx context.Context, in In) (any, error) {
		op, c := decode(in)
		return svc.ApplyMutation(ctx, op, c.PreviewHash, c.Confirmed)
	})
}

func registerMutations(s *sdk.Server, svc *service.Service) {
	mutationPair(s, svc, "preview_entries", "add_entries", "批量报名", func(in entriesArg) (service.Mutation, confirmation) {
		return service.Mutation{Action: "add_entries", EventID: in.EventID, Entries: in.Entries}, in.confirmation
	})
	mutationPair(s, svc, "preview_remove_entries", "remove_entries", "取消报名，已排进组的运动员须先在界面移出", func(in removeEntriesArg) (service.Mutation, confirmation) {
		return service.Mutation{Action: "remove_entries", EventID: in.EventID, IDs: append(in.AthleteIDs, in.TeamIDs...)}, in.confirmation
	})
	mutationPair(s, svc, "preview_results", "set_results", "录入成绩和状态、远度逐次记录或高度逐高度记录；位置不能为空，不可删除成绩或指定名次", func(in resultsArg) (service.Mutation, confirmation) {
		return service.Mutation{Action: "set_results", HeatID: in.HeatID, Results: in.Results}, in.confirmation
	})
	mutationPair(s, svc, "preview_arrange", "auto_arrange", "只为未分组的报名者追加新组，seed 控制随机结果", func(in planArg) (service.Mutation, confirmation) {
		in.Plan.FromAdvance = false
		return service.Mutation{Action: "auto_arrange", Plan: in.Plan}, in.confirmation
	})
	mutationPair(s, svc, "preview_advance", "advance", "只为上一轮 Q/q 晋级者追加下一轮组，来源成绩必须录完；seed 控制随机结果", func(in planArg) (service.Mutation, confirmation) {
		in.Plan.FromAdvance = true
		return service.Mutation{Action: "advance", Plan: in.Plan}, in.confirmation
	})
	mutationPair(s, svc, "preview_import", "import_meet", "导入交换格式赛事，含接力队伍与棒次；成绩覆盖时展示原成绩与新成绩", func(in importArg) (service.Mutation, confirmation) {
		return service.Mutation{Action: "import_meet", MeetID: in.MeetID, Data: in.Data}, in.confirmation
	})
	mutationPair(s, svc, "preview_athletes", "import_athletes", "批量导入表格运动员、所属单位与报名；先 parse_table 获取 rows 和 mapping", func(in tableArg) (service.Mutation, confirmation) {
		return service.Mutation{Action: "import_athletes", MeetID: in.MeetID, Table: in.Table}, in.confirmation
	})
	addTool(s, "preview_schedule", "预览一个轮次的批量计划时间：按组序号排列，已有成绩的组跳过且不占间隔，已有计划时间可覆盖；返回 changes、skipped、errors 和 previewHash。向用户展示原时间与新时间后取得明确确认。", true, func(ctx context.Context, in scheduleArg) (any, error) {
		return svc.PreviewSchedule(ctx, in.Request)
	})
	addTool(s, "schedule_heats", "提交批量计划时间。必须先 preview_schedule 并获得用户确认，携带相同 request、previewHash 和 confirmed=true；任何目标轮次数据变化后必须重新预览。", false, func(ctx context.Context, in scheduleArg) (any, error) {
		return svc.ApplySchedule(ctx, store.SourceMCP, in.Request, in.PreviewHash, in.Confirmed)
	})

	mutationPair(s, svc, "preview_delete_team", "delete_team", "删除接力队伍及其报名；传 eventId 与 id，已排进组的队伍须先移出", func(in struct {
		EventID string `json:"eventId"`
		ID      string `json:"id"`
		confirmation
	}) (service.Mutation, confirmation) {
		return service.Mutation{Action: "delete_team", EventID: in.EventID, ID: in.ID}, in.confirmation
	})
	for _, action := range []string{"delete_meet", "delete_unit", "delete_event", "delete_athletes", "delete_heat"} {
		mutationPair(s, svc, "preview_"+action, action, "删除指定数据。赛事用 meetId，参赛单位用 meetId+id，项目用 eventId，运动员用 meetId+athleteIds，组用 heatId；有成绩的赛事、项目或组禁止删除，已排进组的运动员禁止删除", func(in deleteArg) (service.Mutation, confirmation) {
			return service.Mutation{Action: action, MeetID: in.MeetID, EventID: in.EventID, HeatID: in.HeatID, ID: in.ID, IDs: in.AthleteIDs}, in.confirmation
		})
	}
}
