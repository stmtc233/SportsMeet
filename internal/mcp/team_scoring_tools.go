package mcp

import (
	"context"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type teamScoringArg struct {
	Request     service.TeamScoringRequest `json:"request"`
	PreviewHash string                     `json:"previewHash,omitempty"`
	Confirmed   bool                       `json:"confirmed,omitempty"`
}

func registerTeamScoring(s *sdk.Server, svc *service.Service) {
	addTool(s, "get_team_scores", "读取团体计分规则、单位排名、逐项得分、无单位得分及项目计分进度。可按项目性别、groupLabel（省略为全部，空字符串为无组别）和 targetIds（普通项目或全能 ID）筛选。总分同分并列，exact 为精确分数，text 为显示值。", true, func(ctx context.Context, in service.TeamScoreFilter) (any, error) { return svc.GetTeamScores(ctx, in) })
	addTool(s, "preview_team_scoring", "预览团体计分规则变更及各单位原、新总分与名次。places 为按名次排列的非负整数分值；tiePolicy 为 average（占用名次分值均分）或 same（各得该名次分值）；零倍数不计分。全能分项不能单独计分。", true, func(ctx context.Context, in teamScoringArg) (any, error) {
		return svc.PreviewTeamScoring(ctx, in.Request)
	})
	addTool(s, "apply_team_scoring", "按已确认预览保存团体计分规则；须展示预览并取得用户确认，数据变化后重新预览。", false, func(ctx context.Context, in teamScoringArg) (any, error) {
		return svc.ApplyTeamScoring(ctx, store.SourceMCP, in.Request, in.PreviewHash, in.Confirmed)
	})
}
