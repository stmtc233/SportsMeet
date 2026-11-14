package service

import (
	"context"
	"encoding/json"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// MeetSettings are a meet's own arrange and print settings (赛事设置).
type MeetSettings struct {
	// SeparateUnits and Balance are 自动编排's defaults.
	SeparateUnits bool `json:"separateUnits"`
	Balance       bool `json:"balance"`
	// MinRestMinutes is the shortest rest an athlete needs between two
	// heats; 0 checks overlaps only (docs/agents/arrange.md「冲突检查」).
	MinRestMinutes int `json:"minRestMinutes"`
	// CheckInLeadMinutes is how long before a heat's planned start its
	// athletes check in, 0–240; 0 prints no check-in time.
	CheckInLeadMinutes int  `json:"checkInLeadMinutes"`
	PrintNewPage       bool `json:"printNewPage"`
	// PrintSchedule puts the competition schedule at the start of the 秩序册.
	PrintSchedule bool `json:"printSchedule"`
	// PrintRoster puts the units' athletes at the start of the 秩序册.
	PrintRoster bool `json:"printRoster"`
	// PrintCheckInBreak and PrintRecordBreak are where the 检录单 and the
	// 成绩记录表 start a new page (SheetBreak*).
	PrintCheckInBreak string `json:"printCheckInBreak"`
	PrintRecordBreak  string `json:"printRecordBreak"`
	// PrintResultsTime prints each heat's results time in the 成绩册.
	PrintResultsTime bool `json:"printResultsTime"`
	PrintTeamScores  bool `json:"printTeamScores"`
}

// DefaultMeetSettings are a new meet's settings.
func DefaultMeetSettings() MeetSettings {
	return MeetSettings{SeparateUnits: true, Balance: true, PrintSchedule: true, PrintRoster: true, PrintTeamScores: true,
		CheckInLeadMinutes: domain.DefaultCheckInLeadMinutes, PrintCheckInBreak: SheetBreakRound, PrintRecordBreak: SheetBreakRound}
}

// parseMeetSettings reads the stored JSON over the defaults. Settings kept
// with the 检录单's「每组另起一页」switch on keep a page per heat.
func parseMeetSettings(raw string) (MeetSettings, error) {
	m := DefaultMeetSettings()
	if raw == "" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return DefaultMeetSettings(), err
	}
	var old struct {
		Break   *string `json:"printCheckInBreak"`
		NewPage bool    `json:"printCheckInNewPage"`
	}
	if json.Unmarshal([]byte(raw), &old) == nil && old.Break == nil && old.NewPage {
		m.PrintCheckInBreak = SheetBreakHeat
	}
	return m, nil
}

// MeetSettings returns a meet's settings.
func (s *Service) MeetSettings(ctx context.Context, meetID string) (MeetSettings, error) {
	raw, err := s.store.MeetSettings(ctx, meetID)
	if err != nil {
		return DefaultMeetSettings(), err
	}
	return parseMeetSettings(raw)
}

// SaveMeetSettings checks and keeps a meet's settings.
func (s *Service) SaveMeetSettings(ctx context.Context, src store.Source, meetID string, in MeetSettings) error {
	if in.MinRestMinutes < 0 || in.MinRestMinutes > domain.MaxDurationMinutes {
		return invalid("最短休息时间须为 0–%d 分钟", domain.MaxDurationMinutes)
	}
	if !validSheetBreak(in.PrintCheckInBreak) || !validSheetBreak(in.PrintRecordBreak) {
		return invalid("请选择分页方式")
	}
	if !domain.ValidCheckInLead(in.CheckInLeadMinutes) {
		return invalid("检录提前时间须为 0–%d 分钟", domain.MaxCheckInLeadMinutes)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return s.write(ctx, store.Audit{Source: src, Action: "meet.settings", MeetID: meetID, Message: meet.Name, Detail: in},
		Change{Kind: "meets", MeetID: meetID}, store.Stmt(`UPDATE meets SET settings = ? WHERE id = ?`, string(raw), meetID))
}
