package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func addTool[In any](s *sdk.Server, name, description string, readOnly bool, fn func(context.Context, In) (any, error)) {
	closed := false
	sdk.AddTool(s, &sdk.Tool{Name: name, Description: description, Annotations: &sdk.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &closed}},
		func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
			out, err := fn(ctx, in)
			return nil, out, err
		})
}

type empty struct{}
type meetArg struct {
	MeetID string `json:"meetId"`
}
type eventArg struct {
	EventID string `json:"eventId"`
}
type saveUnitArg struct {
	MeetID string            `json:"meetId"`
	ID     string            `json:"id,omitempty"`
	Input  service.UnitInput `json:"input"`
}
type saveAthleteArg struct {
	MeetID string               `json:"meetId"`
	ID     string               `json:"id,omitempty"`
	Input  service.AthleteInput `json:"input"`
}
type saveEventArg struct {
	MeetID string             `json:"meetId"`
	ID     string             `json:"id,omitempty"`
	Input  service.EventInput `json:"input"`
}

func NewServer(svc *service.Service) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "SportsMeet", Version: service.Version}, &sdk.ServerOptions{Instructions: "SportsMeet 是单机田径运动会软件。先查询再修改，使用返回的 ID。批量操作与删除必须先 preview_*，展示完整预览并取得用户明确确认后，携带相同参数、previewHash 和 confirmed=true 提交；errors 非空不得提交。数据变化时重新预览。空成绩位置可直接 set_results，覆盖已有成绩必须 preview_results 并确认。不得把获取预览当作用户确认。成绩名次与晋级由服务计算。设置、令牌、文件系统与网络不向 MCP 开放。"})
	addTool(s, "list_meets", "列出赛事。", true, func(ctx context.Context, _ empty) (any, error) { return svc.ListMeets(ctx) })
	addTool(s, "get_meet", "读取赛事。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.MeetDetail(ctx, in.MeetID) })
	addTool(s, "list_units", "列出参赛单位。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListUnits(ctx, in.MeetID) })
	addTool(s, "list_athletes", "列出运动员、所属单位及报名和排位状态。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListAthletes(ctx, in.MeetID) })
	addTool(s, "list_events", "列出项目及比赛进度。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListEvents(ctx, in.MeetID) })
	addTool(s, "list_event_presets", "列出常用项目预设。input 可作为 save_event 的初始字段，再调整性别、组别与配置；项目保存后独立于预设。", true, func(ctx context.Context, in empty) (any, error) { return svc.EventPresets(), nil })
	addTool(s, "get_event", "读取项目、接力队伍与棒次、报名、轮次（含开始时间 scheduledAt）、组、位置、成绩、排名与 Q/q 晋级。组的 scheduledAt 是单独指定的时间（空 = 沿用轮次），effectiveAt 是生效计划时间，scheduleInherited 表示沿用轮次，durationMinutes 是预计用时，endAt 是预计结束时间；startedAt 是实际开赛时间（空 = 未记录），startedText 是其显示文本，startDeltaText 是与生效计划时间的差（如「晚 12 分钟」「早 5 分钟」「准时」）。", true, func(ctx context.Context, in eventArg) (any, error) { return svc.GetEvent(ctx, in.EventID) })
	addTool(s, "get_schedule", "读取一个赛事的竞赛日程：days 按天、按生效时间列出各项目的轮次与组（startText / effectiveAt / endAt / durationText，scheduleInherited 表示沿用轮次开始时间，keepReason 非空表示已比赛；startedAt / startedText / startDeltaText 是实际开赛时间及与计划的差），unscheduled 是没有任何计划时间的轮次，conflicts 是全部运动员时间冲突。只读。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.Schedule(ctx, in.MeetID) })
	addTool(s, "list_conflicts", "列出一个赛事的运动员时间冲突：同一运动员（队伍展开为队员，含全能分项）的两个组时间重叠，或前一组预计结束到后一组开始不足设置里的最短休息时间。没有预计用时的组按时间点计，没有计划时间的组不参与。只提示，不拦截任何保存。只读。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.MeetConflicts(ctx, in.MeetID) })
	addTool(s, "export_meet", "返回一个赛事的完整交换格式对象，含成绩。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ExportMeet(ctx, in.MeetID) })
	addTool(s, "create_meet", "创建赛事。", false, func(ctx context.Context, in struct {
		Input service.MeetInput `json:"input"`
	}) (any, error) {
		return svc.CreateMeet(ctx, store.SourceMCP, in.Input)
	})
	addTool(s, "update_meet", "更新一个赛事。input 是全部字段，省略的可选字段会清空。", false, func(ctx context.Context, in struct {
		MeetID string            `json:"meetId"`
		Input  service.MeetInput `json:"input"`
	}) (any, error) {
		err := svc.UpdateMeet(ctx, store.SourceMCP, in.MeetID, in.Input)
		return map[string]bool{"saved": err == nil}, err
	})
	addTool(s, "save_unit", "创建或更新一个参赛单位。id 留空创建；更新时 input 是全部字段。", false, func(ctx context.Context, in saveUnitArg) (any, error) {
		return svc.SaveUnit(ctx, store.SourceMCP, in.MeetID, in.ID, in.Input)
	})
	addTool(s, "save_athlete", "创建或更新一个运动员。id 留空创建；学号、号码布赛事内唯一。更新时 input 是全部字段。", false, func(ctx context.Context, in saveAthleteArg) (any, error) {
		return svc.SaveAthleteWithWarnings(ctx, store.SourceMCP, in.MeetID, in.ID, in.Input)
	})
	addTool(s, "save_team", "创建或更新队伍。eventId 指定队伍项目，id 留空创建；input 是完整队名、参赛单位和按棒次顺序的 memberIds（可为空）。同一运动员在一个项目只能属于一队，已有比赛记录不能改棒次。保存队伍后用 add_entries 的 teamId 报名。", false, func(ctx context.Context, in struct {
		EventID string            `json:"eventId"`
		ID      string            `json:"id,omitempty"`
		Input   service.TeamInput `json:"input"`
	}) (any, error) {
		return svc.SaveTeam(ctx, store.SourceMCP, in.EventID, in.ID, in.Input)
	})
	addTool(s, "save_event", "创建或更新一个项目。id 留空创建；必须有性别、项目名与类型。更新时 input 是全部字段。startMode 为 lanes（分道）、group（不分道）、break（分道后抢道）；distanceMode 为 jump 或 throw；relayLegCount 是建议棒数，0 为未指定。participantMode 为 athlete 或 team；count/score 支持 rankDirection（asc/desc）、attemptMode（single/best/sum）；scoreDecimals 为计分小数位 0–3。rounds 列出决赛之前的轮次，留空即直接决赛：stage 为 heat（预赛）、quarterfinal（次赛）、semifinal（半决赛），各带 advanceByPlace（各组前几名晋级）与 advanceByMark（其余按成绩晋级人数），两者至少一个大于 0；更新时未列出的轮次会被删除，已有组的轮次不能删除。", false, func(ctx context.Context, in saveEventArg) (any, error) {
		return svc.SaveEvent(ctx, store.SourceMCP, in.MeetID, in.ID, in.Input)
	})
	addTool(s, "set_seed", "设置一个报名成绩；seed 留空清除。", false, func(ctx context.Context, in struct {
		EventID   string `json:"eventId"`
		AthleteID string `json:"athleteId,omitempty"`
		TeamID    string `json:"teamId,omitempty"`
		Seed      string `json:"seed"`
	}) (any, error) {
		err := svc.SetEntrySeed(ctx, store.SourceMCP, in.EventID, service.EntryInput{AthleteID: in.AthleteID, TeamID: in.TeamID, Seed: in.Seed})
		return map[string]bool{"saved": err == nil}, err
	})
	addTool(s, "parse_table", "解析粘贴的 CSV/TSV，返回列映射与行，不写入。", true, func(_ context.Context, in struct {
		Text string `json:"text"`
	}) (any, error) {
		return service.ParseTable(in.Text)
	})
	addTool(s, "render_report", "生成秩序册或成绩册 HTML。kind 为 startlist 或 results；eventIds 留空选全部。不操作系统打印窗口。", true, func(ctx context.Context, in struct {
		MeetID   string   `json:"meetId"`
		Kind     string   `json:"kind"`
		EventIDs []string `json:"eventIds,omitempty"`
	}) (any, error) {
		html, err := svc.RenderReport(ctx, in.Kind, in.MeetID, in.EventIDs)
		return map[string]string{"html": html}, err
	})
	registerCombined(s, svc)
	registerMutations(s, svc)
	return s
}
