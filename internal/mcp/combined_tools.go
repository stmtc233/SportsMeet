package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type combinedArg struct {
	CombinedID string `json:"combinedId"`
}
type combinedWriteArg struct {
	MeetID      string                `json:"meetId"`
	CombinedID  string                `json:"combinedId,omitempty"`
	Input       service.CombinedInput `json:"input"`
	PreviewHash string                `json:"previewHash,omitempty"`
	Confirmed   bool                  `json:"confirmed,omitempty"`
}
type combinedEntriesArg struct {
	CombinedID  string   `json:"combinedId"`
	AthleteIDs  []string `json:"athleteIds"`
	PreviewHash string   `json:"previewHash,omitempty"`
	Confirmed   bool     `json:"confirmed,omitempty"`
}
type combinedDeleteArg struct {
	CombinedID  string `json:"combinedId"`
	PreviewHash string `json:"previewHash,omitempty"`
	Confirmed   bool   `json:"confirmed,omitempty"`
}
type combinedRulesArg struct {
	Request     service.CombinedRulesRequest `json:"request"`
	PreviewHash string                       `json:"previewHash,omitempty"`
	Confirmed   bool                         `json:"confirmed,omitempty"`
}

func registerCombined(s *sdk.Server, svc *service.Service) {
	addTool(s, "list_combined", "列出赛事的全能、分项数量和报名进度。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListCombined(ctx, in.MeetID) })
	addTool(s, "get_combined", "读取全能、分项、计分规则、统一报名和总成绩；complete=false 时名次为暂列，excluded 不列名次。", true, func(ctx context.Context, in combinedArg) (any, error) { return svc.GetCombined(ctx, in.CombinedID) })
	addTool(s, "parse_point_rule", "校验成绩与积分两列的计分表，不写入。档位从高标准到低标准排列，invalidPolicies 须指定 DNS/DNF/DQ/NM 为 zero 或 exclude。", true, func(ctx context.Context, in struct {
		Event service.EventInput     `json:"event"`
		Rule  service.PointRuleInput `json:"rule"`
	}) (any, error) {
		return svc.ParsePointRule(in.Event, in.Rule)
	})
	addTool(s, "preview_save_combined", "预览创建或编辑全能及其独立分项，不写入；combinedId 留空创建。已有计分表修改须走 preview_combined_rules。", true, func(ctx context.Context, in combinedWriteArg) (any, error) {
		return svc.PreviewMutation(ctx, service.Mutation{Action: "save_combined", MeetID: in.MeetID, CombinedID: in.CombinedID, Combined: in.Input})
	})
	addTool(s, "save_combined", "按已确认预览保存全能及分项。", false, func(ctx context.Context, in combinedWriteArg) (any, error) {
		return svc.ApplyMutation(ctx, service.Mutation{Action: "save_combined", MeetID: in.MeetID, CombinedID: in.CombinedID, Combined: in.Input}, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_combined_entries", "预览全能完整报名名单；所有分项同时报名，移除已有排位者会拒绝。", true, func(ctx context.Context, in combinedEntriesArg) (any, error) {
		return svc.PreviewMutation(ctx, service.Mutation{Action: "combined_entries", CombinedID: in.CombinedID, IDs: in.AthleteIDs})
	})
	addTool(s, "set_combined_entries", "按确认预览设置全能完整名单，原子更新所有分项。", false, func(ctx context.Context, in combinedEntriesArg) (any, error) {
		return svc.ApplyMutation(ctx, service.Mutation{Action: "combined_entries", CombinedID: in.CombinedID, IDs: in.AthleteIDs}, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_delete_combined", "预览删除全能、分项和报名；有成绩时拒绝。", true, func(ctx context.Context, in combinedDeleteArg) (any, error) {
		return svc.PreviewMutation(ctx, service.Mutation{Action: "delete_combined", CombinedID: in.CombinedID})
	})
	addTool(s, "delete_combined", "按已确认预览删除无成绩的全能及分项。", false, func(ctx context.Context, in combinedDeleteArg) (any, error) {
		return svc.ApplyMutation(ctx, service.Mutation{Action: "delete_combined", CombinedID: in.CombinedID}, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_combined_rules", "预览全部分项规则变更，返回原积分、总分与名次和重新计算后的结果，不改原始成绩。", true, func(ctx context.Context, in combinedRulesArg) (any, error) {
		return svc.PreviewCombinedRules(ctx, in.Request)
	})
	addTool(s, "apply_combined_rules", "按确认预览保存计分规则并增加规则版本；数据变化后须重新预览。", false, func(ctx context.Context, in combinedRulesArg) (any, error) {
		return svc.ApplyCombinedRules(ctx, store.SourceMCP, in.Request, in.PreviewHash, in.Confirmed)
	})
}
