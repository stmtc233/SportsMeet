package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// ScheduleRequest assigns local wall-clock times to the unscored heats of one round.
type ScheduleRequest struct {
	EventID         string `json:"eventId"`
	RoundID         string `json:"roundId"`
	StartAt         string `json:"startAt" jsonschema:"首个未比赛组的本地时间，YYYY-MM-DDTHH:mm，无时区"`
	IntervalMinutes int    `json:"intervalMinutes" jsonschema:"相邻未比赛组的间隔，1 至 1440 分钟；已比赛的组不占间隔"`
}

// ScheduleChange is one actual time change, with display text computed in Go.
type ScheduleChange struct {
	HeatID     string `json:"heatId"`
	Label      string `json:"label"`
	Before     string `json:"before"`
	After      string `json:"after"`
	BeforeText string `json:"beforeText"`
	AfterText  string `json:"afterText"`
}

type ScheduleSkip struct {
	HeatID string `json:"heatId"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type SchedulePreview struct {
	PreviewHash string           `json:"previewHash"`
	Errors      []string         `json:"errors"`
	Changes     []ScheduleChange `json:"changes"`
	Skipped     []ScheduleSkip   `json:"skipped"`
}

// PreviewSchedule and ApplySchedule are shared by desktop bindings and MCP.
func (s *Service) PreviewSchedule(ctx context.Context, in ScheduleRequest) (*SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.planSchedule(ctx, in)
	return p, err
}

// ApplySchedule recomputes and checks the preview under the same write lock as the commit.
func (s *Service) ApplySchedule(ctx context.Context, src store.Source, in ScheduleRequest, hash string, confirmed bool) (*SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	p, st, err := s.planSchedule(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(p.Errors) > 0 {
		return nil, invalid("%s", p.Errors[0])
	}
	if p.PreviewHash != hash {
		return nil, ErrStale
	}
	if len(p.Changes) == 0 {
		return p, nil
	}
	stmts := make([]sqlite.Statement, 0, len(p.Changes))
	for _, c := range p.Changes {
		stmts = append(stmts, store.Stmt(`UPDATE heats SET scheduled_at = ? WHERE id = ? AND round_id = ?`, c.After, c.HeatID, in.RoundID))
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "heats.scheduled", MeetID: st.event.MeetID, EventID: in.EventID,
		Message: st.event.Name + " " + st.round(in.RoundID).Label(), Detail: struct {
			Request ScheduleRequest  `json:"request"`
			Changes []ScheduleChange `json:"changes"`
		}{in, p.Changes}},
		Change{Kind: "heats", MeetID: st.event.MeetID, EventID: in.EventID}, stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// planSchedule reads without writing. Its caller holds the service write lock.
func (s *Service) planSchedule(ctx context.Context, in ScheduleRequest) (*SchedulePreview, *eventState, error) {
	p := &SchedulePreview{Errors: []string{}, Changes: []ScheduleChange{}, Skipped: []ScheduleSkip{}}
	start, err := time.Parse("2006-01-02T15:04", in.StartAt)
	if !minuteRe.MatchString(in.StartAt) || err != nil || start.Year() < 1 {
		p.Errors = append(p.Errors, "请填写有效的日期与时间")
	}
	if in.IntervalMinutes < 1 || in.IntervalMinutes > 1440 {
		p.Errors = append(p.Errors, "间隔须为 1–1440 分钟")
	}
	if len(p.Errors) > 0 {
		return p, nil, nil
	}
	st, err := s.loadEvent(ctx, in.EventID)
	if err != nil {
		return nil, nil, err
	}
	round := st.round(in.RoundID)
	if round == nil {
		return nil, nil, invalid("轮次不存在或已删除")
	}
	heats := append([]domain.Heat(nil), round.Heats...)
	sort.Slice(heats, func(i, j int) bool {
		if heats[i].Ordinal != heats[j].Ordinal {
			return heats[i].Ordinal < heats[j].Ordinal
		}
		return heats[i].ID < heats[j].ID
	})
	next := start
	for _, h := range heats {
		label := domain.HeatLabel(h.Ordinal, h.Name)
		if reason := h.Facts().KeepReason(); reason != "" {
			p.Skipped = append(p.Skipped, ScheduleSkip{HeatID: h.ID, Label: label, Reason: reason})
			continue
		}
		if next.Year() > 9999 {
			p.Errors = append(p.Errors, "计划时间超出日期范围")
			p.Changes = []ScheduleChange{}
			return p, st, nil
		}
		after := next.Format("2006-01-02T15:04")
		if h.ScheduledAt != after {
			beforeText := strings.ReplaceAll(h.ScheduledAt, "T", " ")
			if beforeText == "" {
				beforeText = "未安排"
			}
			p.Changes = append(p.Changes, ScheduleChange{HeatID: h.ID, Label: label, Before: h.ScheduledAt, After: after,
				BeforeText: beforeText, AfterText: strings.ReplaceAll(after, "T", " ")})
		}
		// Parse and arithmetic deliberately use no local timezone conversion: these are wall-clock fields.
		next = next.Add(time.Duration(in.IntervalMinutes) * time.Minute)
	}
	raw, err := json.Marshal(struct {
		Request ScheduleRequest
		Event   domain.Event
		Round   domain.Round
	}{in, st.event, *round})
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(raw)
	p.PreviewHash = hex.EncodeToString(sum[:])
	return p, st, nil
}
