package service

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// auditActions names every log action as the settings page shows it. A new
// write action is added here together with its write (docs/agents/domain.md
// 「操作日志」); TestAuditActionTexts checks that none is missing.
var auditActions = map[string]string{
	"group.created":        "新建组别",
	"group.updated":        "修改组别",
	"group.deleted":        "删除组别",
	"group.reordered":      "调整组别顺序",
	"group.projects":       "调整项目组别",
	"group.assignments":    "调整参赛组别",
	"meet.created":         "新建赛事",
	"meet.updated":         "修改赛事",
	"meet.settings":        "修改赛事设置",
	"meet.deleted":         "删除赛事",
	"meet.imported":        "导入赛事",
	"unit.created":         "新建参赛单位",
	"unit.updated":         "修改参赛单位",
	"unit.deleted":         "删除参赛单位",
	"athlete.created":      "新建运动员",
	"athlete.updated":      "修改运动员",
	"athlete.deleted":      "删除运动员",
	"athlete.imported":     "导入运动员",
	"event.created":        "新建项目",
	"event.updated":        "修改项目",
	"event.deleted":        "删除项目",
	"team.created":         "新建队伍",
	"team.updated":         "修改队伍",
	"team.deleted":         "删除队伍",
	"entry.added":          "报名",
	"entry.removed":        "取消报名",
	"entry.seed":           "修改报名成绩",
	"entry.athlete_set":    "修改运动员报名",
	"board.saved":          "保存编排",
	"board.reset":          "重置编排",
	"board.batch_arranged": "批量自动编排",
	"heats.scheduled":      "排时间",
	"round.scheduled":      "修改轮次开始时间",
	"results.saved":        "录入成绩",
	"heat.started":         "修改实际开赛时间",
	"results.tie":          "裁定并列",
	"settings.preferences": "修改设置",
	"settings.mcp":         "修改 MCP 设置",
	"combined.created":     "新建全能",
	"combined.updated":     "修改全能",
	"combined.deleted":     "删除全能",
	"combined.entries":     "全能报名",
	"team_scoring.rules":   "修改团体计分规则",
	"combined.rules":       "修改计分规则",
}

var auditSources = map[store.Source]string{
	store.SourceUI:     "界面",
	store.SourceMCP:    "MCP",
	store.SourceSystem: "系统",
}

// AuditQuery selects a page of the operation log. MeetID and Source ""
// match everything; After continues from the Next of a previous page.
type AuditQuery struct {
	MeetID string       `json:"meetId"`
	Source string       `json:"source"` // ui | mcp | system, "" = all
	After  *AuditCursor `json:"after"`
	Limit  int          `json:"limit"` // 1–200, 0 = 50
}

// AuditCursor is where a page ended: the time and id of its last row.
type AuditCursor struct {
	At int64 `json:"at"`
	ID int64 `json:"id"`
}

// AuditRow is one log row with its texts as the page shows them.
type AuditRow struct {
	ID         int64        `json:"id"`
	At         int64        `json:"at"` // ms
	AtText     string       `json:"atText"`
	Source     store.Source `json:"source"`
	SourceText string       `json:"sourceText"`
	Action     string       `json:"action"`
	ActionText string       `json:"actionText"`
	MeetID     string       `json:"meetId"`
	MeetName   string       `json:"meetName"` // "" when the meet was deleted or the row has none
	EventID    string       `json:"eventId"`
	HeatID     string       `json:"heatId"`
	Message    string       `json:"message"`
}

// AuditPage is a page of the log, newest first; Next is nil on the last page.
type AuditPage struct {
	Rows []AuditRow   `json:"rows"`
	Next *AuditCursor `json:"next"`
}

// ListAudit reads the operation log, read-only, in two queries whatever
// the page size: the rows, and the meet names.
func (s *Service) ListAudit(ctx context.Context, q AuditQuery) (*AuditPage, error) {
	source := store.Source(q.Source)
	if source != "" && auditSources[source] == "" {
		return nil, invalid("来源不正确")
	}
	limit := q.Limit
	switch {
	case limit == 0:
		limit = 50
	case limit < 1 || limit > 200:
		return nil, invalid("每页条数须为 1–200")
	}
	f := store.AuditFilter{MeetID: q.MeetID, Source: source, Limit: limit + 1}
	if q.After != nil {
		f.BeforeAt, f.BeforeID = q.After.At, q.After.ID
	}
	rows, err := s.store.ListAudit(ctx, f)
	if err != nil {
		return nil, err
	}
	meets, err := s.store.ListMeets(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(meets))
	for _, m := range meets {
		names[m.ID] = m.Name
	}
	page := &AuditPage{Rows: make([]AuditRow, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			last := page.Rows[limit-1]
			page.Next = &AuditCursor{At: last.At, ID: last.ID}
			break
		}
		page.Rows = append(page.Rows, AuditRow{ID: r.ID, At: r.At, AtText: domain.RecordedText(r.At, s.zone),
			Source: r.Source, SourceText: textOr(auditSources[r.Source], string(r.Source)),
			Action: r.Action, ActionText: textOr(auditActions[r.Action], r.Action),
			MeetID: r.MeetID, MeetName: names[r.MeetID], EventID: r.EventID, HeatID: r.HeatID, Message: r.Message})
	}
	return page, nil
}

func textOr(text, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
