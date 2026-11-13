package mcp

import (
	"context"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type groupChangeArg struct {
	Request     service.GroupChangeRequest `json:"request"`
	PreviewHash string                     `json:"previewHash,omitempty"`
	Confirmed   bool                       `json:"confirmed,omitempty"`
}
type groupAssignmentsArg struct {
	Request     service.GroupAssignmentsRequest `json:"request"`
	PreviewHash string                          `json:"previewHash,omitempty"`
	Confirmed   bool                            `json:"confirmed,omitempty"`
}
type groupDeleteArg struct {
	MeetID      string `json:"meetId"`
	ID          string `json:"id"`
	PreviewHash string `json:"previewHash,omitempty"`
	Confirmed   bool   `json:"confirmed,omitempty"`
}
type groupOrderArg struct {
	MeetID      string   `json:"meetId"`
	IDs         []string `json:"ids"`
	PreviewHash string   `json:"previewHash,omitempty"`
	Confirmed   bool     `json:"confirmed,omitempty"`
}

func registerGroups(s *sdk.Server, svc *service.Service) {
	addTool(s, "list_groups", "列出赛事统一组别、显示顺序及项目、单位、运动员数量。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListGroups(ctx, in.MeetID) })
	addTool(s, "save_group", "创建或改名赛事组别；id 留空创建。自动项目名称随改名更新，自定义名称保留。", false, func(ctx context.Context, in struct {
		MeetID string             `json:"meetId"`
		ID     string             `json:"id,omitempty"`
		Input  service.GroupInput `json:"input"`
	}) (any, error) {
		return svc.SaveGroup(ctx, store.SourceMCP, in.MeetID, in.ID, in.Input)
	})
	addTool(s, "suggest_groups", "按已有报名、年级文字及单位归属建议组别，不写入；多个候选须明确选择。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.SuggestGroups(ctx, in.MeetID) })
	addTool(s, "get_team_score_boards", "按组别分别读取团体榜，每组独立排名与进度；groupId 省略为全部组别分别出榜，空字符串为未分组。", true, func(ctx context.Context, in service.TeamScoreFilter) (any, error) {
		return svc.GetTeamScoreBoards(ctx, in)
	})
	addTool(s, "preview_group_change", "预览调整普通项目或全能组别，含前后各组团体排名；全能全部分项一起调整，不移动报名或改成绩。", true, func(ctx context.Context, in groupChangeArg) (any, error) {
		return svc.PreviewGroupChange(ctx, in.Request)
	})
	addTool(s, "apply_group_change", "按已确认预览调整项目组别，数据变化后重新预览。", false, func(ctx context.Context, in groupChangeArg) (any, error) {
		return svc.ApplyGroupChange(ctx, store.SourceMCP, in.Request, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_group_assignments", "预览单位和运动员归属。kind 为 unit（多个 groupIds）或 athlete（至多一个 groupId，空数组清除）；不移动已有报名。", true, func(ctx context.Context, in groupAssignmentsArg) (any, error) {
		return svc.PreviewGroupAssignments(ctx, in.Request)
	})
	addTool(s, "apply_group_assignments", "按确认预览保存组别归属，数据变化后重新预览。", false, func(ctx context.Context, in groupAssignmentsArg) (any, error) {
		return svc.ApplyGroupAssignments(ctx, store.SourceMCP, in.Request, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_delete_group", "预览删除组别；引用它的项目、全能、单位与运动员改为不分组别，成绩与编排不变。", true, func(ctx context.Context, in groupDeleteArg) (any, error) {
		return svc.PreviewMutation(ctx, service.Mutation{Action: "delete_group", MeetID: in.MeetID, ID: in.ID})
	})
	addTool(s, "delete_group", "按确认预览删除组别，引用改为不分组别。", false, func(ctx context.Context, in groupDeleteArg) (any, error) {
		return svc.ApplyMutation(ctx, service.Mutation{Action: "delete_group", MeetID: in.MeetID, ID: in.ID}, in.PreviewHash, in.Confirmed)
	})
	addTool(s, "preview_reorder_groups", "预览完整组别顺序，ids 必须恰好包含赛事所有组别。", true, func(ctx context.Context, in groupOrderArg) (any, error) {
		return svc.PreviewMutation(ctx, service.Mutation{Action: "reorder_groups", MeetID: in.MeetID, IDs: in.IDs})
	})
	addTool(s, "reorder_groups", "按确认预览保存完整组别顺序。", false, func(ctx context.Context, in groupOrderArg) (any, error) {
		return svc.ApplyMutation(ctx, service.Mutation{Action: "reorder_groups", MeetID: in.MeetID, IDs: in.IDs}, in.PreviewHash, in.Confirmed)
	})
}
