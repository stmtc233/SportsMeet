package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// ScheduleRequest assigns local wall-clock times to the unscored heats of
// one round, or of several rounds one after another (docs/agents/arrange.md
// 「批量排时间」).
type ScheduleRequest struct {
	EventID         string     `json:"eventId,omitempty" jsonschema:"单个轮次时的项目 ID；填了 rounds 时不用"`
	RoundID         string     `json:"roundId,omitempty" jsonschema:"单个轮次时的轮次 ID；填了 rounds 时不用"`
	Rounds          []RoundRef `json:"rounds,omitempty" jsonschema:"按顺序连排的多个轮次，须属于同一赛事；不填时只排 eventId / roundId 这一个轮次"`
	StartAt         string     `json:"startAt" jsonschema:"首个未比赛组的本地时间，YYYY-MM-DDTHH:mm，无时区"`
	IntervalMinutes int        `json:"intervalMinutes" jsonschema:"相邻未比赛组的间隔，1 至 1440 分钟，同时写为每个被排组的预计用时；已比赛的组不占间隔"`
	RoundGapMinutes int        `json:"roundGapMinutes,omitempty" jsonschema:"连排时上一轮次最后一组结束到下一轮次第一组开始之间另留的分钟数，0 至 1440"`
}

// RoundRef names one round of a multi-round schedule.
type RoundRef struct {
	EventID string `json:"eventId"`
	RoundID string `json:"roundId"`
}

// targets is the rounds the request schedules, in order.
func (in ScheduleRequest) targets() []RoundRef {
	if len(in.Rounds) > 0 {
		return in.Rounds
	}
	return []RoundRef{{EventID: in.EventID, RoundID: in.RoundID}}
}

// ScheduleChange is one heat whose own time or expected duration changes,
// with display text computed in Go. Before is the heat's own time ("" =
// it follows the round's start); BeforeText shows the time it actually had.
type ScheduleChange struct {
	EventID       string `json:"eventId"`
	RoundID       string `json:"roundId"`
	Round         string `json:"round"` // event name and round label
	HeatID        string `json:"heatId"`
	Label         string `json:"label"`
	Before        string `json:"before"`
	After         string `json:"after"`
	BeforeMinutes *int   `json:"beforeMinutes"`
	AfterMinutes  int    `json:"afterMinutes"`
	BeforeText    string `json:"beforeText"`
	AfterText     string `json:"afterText"`
}

type ScheduleSkip struct {
	EventID string `json:"eventId"`
	RoundID string `json:"roundId"`
	Round   string `json:"round"`
	HeatID  string `json:"heatId"`
	Label   string `json:"label"`
	Reason  string `json:"reason"`
}

type SchedulePreview struct {
	PreviewHash string           `json:"previewHash"`
	Errors      []string         `json:"errors"`
	Warnings    []string         `json:"warnings"` // shown, never blocking
	Changes     []ScheduleChange `json:"changes"`
	Skipped     []ScheduleSkip   `json:"skipped"`
	// Conflicts are the athlete time conflicts that involve a changed heat
	// once the plan is applied; warnings only.
	Conflicts []ConflictView `json:"conflicts"`
}

// PreviewSchedule and ApplySchedule are shared by desktop bindings and MCP.
func (s *Service) PreviewSchedule(ctx context.Context, in ScheduleRequest) (*SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.planSchedule(ctx, in)
	return p, err
}

// ApplySchedule recomputes and checks the preview under the same write
// lock as the commit, then writes every change in one transaction.
func (s *Service) ApplySchedule(ctx context.Context, src store.Source, in ScheduleRequest, hash string, confirmed bool) (*SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	p, plan, err := s.planSchedule(ctx, in)
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
		stmts = append(stmts, store.Stmt(`UPDATE heats SET scheduled_at = ?, duration_minutes = ? WHERE id = ? AND round_id = ?`,
			c.After, c.AfterMinutes, c.HeatID, c.RoundID))
	}
	targets := in.targets()
	eventID, message := targets[0].EventID, p.Changes[0].Round
	for _, t := range targets[1:] {
		if t.EventID != eventID {
			eventID = ""
		}
	}
	if len(targets) > 1 {
		message = fmt.Sprintf("%d 个轮次", len(targets))
		if eventID != "" {
			message = plan.eventBy[eventID].Name + " " + message
		}
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "heats.scheduled", MeetID: plan.meet.ID, EventID: eventID,
		Message: message, Detail: struct {
			Request ScheduleRequest  `json:"request"`
			Changes []ScheduleChange `json:"changes"`
		}{in, p.Changes}},
		Change{Kind: "heats", MeetID: plan.meet.ID, EventID: eventID}, stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// planSchedule reads without writing. Its caller holds the service write
// lock. Rounds follow one another: the next round's first unscored heat
// starts one interval after the previous round's last scheduled heat, plus
// the round gap.
func (s *Service) planSchedule(ctx context.Context, in ScheduleRequest) (*SchedulePreview, *meetPlan, error) {
	p := &SchedulePreview{Errors: []string{}, Warnings: []string{}, Changes: []ScheduleChange{}, Skipped: []ScheduleSkip{}, Conflicts: []ConflictView{}}
	start, ok := domain.ParseScheduled(in.StartAt)
	if !ok {
		p.Errors = append(p.Errors, "请填写有效的日期与时间")
	}
	if in.IntervalMinutes < 1 || in.IntervalMinutes > domain.MaxDurationMinutes {
		p.Errors = append(p.Errors, fmt.Sprintf("间隔须为 1–%d 分钟", domain.MaxDurationMinutes))
	}
	if in.RoundGapMinutes < 0 || in.RoundGapMinutes > domain.MaxDurationMinutes {
		p.Errors = append(p.Errors, fmt.Sprintf("轮次间隔须为 0–%d 分钟", domain.MaxDurationMinutes))
	}
	if len(p.Errors) > 0 {
		return p, nil, nil
	}
	targets := in.targets()
	meetID, err := s.meetOfEvent(ctx, targets[0].EventID)
	if err != nil {
		return nil, nil, err
	}
	plan, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, nil, err
	}
	type target struct {
		event domain.Event
		round domain.Round
	}
	list := make([]target, 0, len(targets))
	seen := map[string]bool{}
	for _, t := range targets {
		e, ok := plan.eventBy[t.EventID]
		if !ok {
			return nil, nil, invalid("连排的轮次须属于同一赛事")
		}
		var round *domain.Round
		for i, r := range plan.rounds[e.ID] {
			if r.ID == t.RoundID {
				round = &plan.rounds[e.ID][i]
			}
		}
		if round == nil {
			return nil, nil, invalid("轮次不存在或已删除")
		}
		if seen[round.ID] {
			return nil, nil, invalid("轮次重复")
		}
		seen[round.ID] = true
		list = append(list, target{e, *round})
	}

	// Wall-clock arithmetic (domain time rules): no zone, no daylight-saving shift.
	step := time.Duration(in.IntervalMinutes) * time.Minute
	next := start
	placed := false // an earlier round scheduled a heat
	outside := 0
	changed := map[string]planned{}
	for _, t := range list {
		name := t.event.Name + " " + t.round.Label()
		heats := append([]domain.Heat(nil), t.round.Heats...)
		sort.Slice(heats, func(i, j int) bool {
			if heats[i].Ordinal != heats[j].Ordinal {
				return heats[i].Ordinal < heats[j].Ordinal
			}
			return heats[i].ID < heats[j].ID
		})
		first := true
		for _, h := range heats {
			label := domain.HeatLabel(h.Ordinal, h.Name)
			if reason := h.Facts().KeepReason(); reason != "" {
				p.Skipped = append(p.Skipped, ScheduleSkip{EventID: t.event.ID, RoundID: t.round.ID, Round: name, HeatID: h.ID, Label: label, Reason: reason})
				continue
			}
			if first && placed {
				next = next.Add(time.Duration(in.RoundGapMinutes) * time.Minute)
			}
			first, placed = false, true
			after, ok := domain.FormatScheduled(next)
			if !ok {
				p.Errors = append(p.Errors, "计划时间超出日期范围")
				p.Changes, p.Conflicts = []ScheduleChange{}, []ConflictView{}
				return p, plan, nil
			}
			minutes := in.IntervalMinutes
			if h.ScheduledAt != after || !samePtr(h.DurationMinutes, &minutes) {
				p.Changes = append(p.Changes, ScheduleChange{EventID: t.event.ID, RoundID: t.round.ID, Round: name, HeatID: h.ID, Label: label,
					Before: h.ScheduledAt, After: after, BeforeMinutes: h.DurationMinutes, AfterMinutes: minutes,
					BeforeText: heatPlanText(t.round, h), AfterText: planText(after, false, &minutes)})
				changed[h.ID] = planned{after, &minutes}
				if domain.OutsideMeet(after, plan.meet) {
					outside++
				}
			}
			next = next.Add(step)
		}
	}
	if outside > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d 组%s", outside, outsideMeetWarning))
	}
	if len(changed) > 0 {
		var touching []arrange.Conflict
		for _, c := range plan.conflicts(changed) {
			if _, a := changed[c.First]; a {
				touching = append(touching, c)
			} else if _, b := changed[c.Second]; b {
				touching = append(touching, c)
			}
		}
		p.Conflicts = plan.conflictViews(touching, changed)
		if n := len(p.Conflicts); n > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("排完后有 %d 处时间冲突", n))
		}
	}
	type stamp struct {
		Event domain.Event
		Round domain.Round
	}
	stamps := make([]stamp, 0, len(list))
	for _, t := range list {
		stamps = append(stamps, stamp{t.event, t.round})
	}
	raw, err := json.Marshal(struct {
		Request ScheduleRequest
		Meet    domain.Meet
		Rounds  []stamp
	}{in, plan.meet, stamps})
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(raw)
	p.PreviewHash = hex.EncodeToString(sum[:])
	return p, plan, nil
}

// outsideMeetWarning follows what is outside: "时间" or "N 组".
const outsideMeetWarning = "不在赛事日期内"

// planText shows a planned time with the day and, when given, the
// expected duration: "2026-11-12 09:00 · 5 分钟"; inherited times are
// marked as the round's; "未安排" when there is no time.
func planText(at string, inherited bool, minutes *int) string {
	text := domain.ScheduledText(at, "")
	switch {
	case text == "":
		text = "未安排"
	case inherited:
		text += "（沿用轮次）"
	}
	if d := domain.DurationText(minutes); d != "" {
		text += " · " + d
	}
	return text
}

func heatPlanText(r domain.Round, h domain.Heat) string {
	at, inherited := domain.EffectiveScheduled(r, h)
	return planText(at, inherited, h.DurationMinutes)
}

// eventMeet is the event's meet; a missing meet reads as one without days.
func (s *Service) eventMeet(ctx context.Context, e domain.Event) (domain.Meet, error) {
	m, err := s.store.GetMeet(ctx, e.MeetID)
	if err != nil || m == nil {
		return domain.Meet{}, err
	}
	return *m, nil
}

// ScheduleWarning is what the round-start and heat dialogs show for a
// planned time before saving: "" or a warning that never blocks the save.
// MCP and the batch preview use the same rule (domain.OutsideMeet).
func (s *Service) ScheduleWarning(ctx context.Context, eventID, at string) (string, error) {
	events, err := s.store.ListEvents(ctx, store.Scope{EventID: eventID})
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", invalid("项目不存在或已删除")
	}
	meet, err := s.eventMeet(ctx, events[0])
	if err != nil {
		return "", err
	}
	if domain.OutsideMeet(at, meet) {
		return "时间" + outsideMeetWarning, nil
	}
	return "", nil
}

// RoundStart sets when a round starts; ScheduledAt "" clears it. Heats
// without their own time start then.
type RoundStart struct {
	EventID     string `json:"eventId"`
	RoundID     string `json:"roundId"`
	ScheduledAt string `json:"scheduledAt" jsonschema:"轮次开始的本地时间，YYYY-MM-DDTHH:mm，无时区；空字符串表示清除"`
}

// RoundStartResult carries the warnings the new time raises; they never
// block the write.
type RoundStartResult struct {
	Warnings []string `json:"warnings"`
}

// SetRoundStart writes a round's start time. It is a single field with no
// results attached, so it writes directly; heats that have competed keep
// their slots and results either way.
func (s *Service) SetRoundStart(ctx context.Context, src store.Source, in RoundStart) (*RoundStartResult, error) {
	if in.ScheduledAt != "" && !domain.ValidScheduled(in.ScheduledAt) {
		return nil, invalid("计划时间格式不正确")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, in.EventID)
	if err != nil {
		return nil, err
	}
	round := st.round(in.RoundID)
	if round == nil {
		return nil, invalid("轮次不存在或已删除")
	}
	meet, err := s.eventMeet(ctx, st.event)
	if err != nil {
		return nil, err
	}
	out := &RoundStartResult{Warnings: []string{}}
	if domain.OutsideMeet(in.ScheduledAt, meet) {
		out.Warnings = append(out.Warnings, "时间"+outsideMeetWarning)
	}
	if round.ScheduledAt == in.ScheduledAt {
		return out, nil
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "round.scheduled", MeetID: st.event.MeetID, EventID: in.EventID,
		Message: st.event.Name + " " + round.Label() + " " + planText(in.ScheduledAt, false, nil), Detail: struct {
			Before string `json:"before"`
			After  string `json:"after"`
		}{round.ScheduledAt, in.ScheduledAt}},
		Change{Kind: "heats", MeetID: st.event.MeetID, EventID: in.EventID},
		store.Stmt(`UPDATE rounds SET scheduled_at = ? WHERE id = ? AND event_id = ?`, in.ScheduledAt, in.RoundID, in.EventID))
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HeatPlan is a heat's own time and expected duration as the heat dialog
// submits them, checked as SaveBoard checks them and shown as the event
// detail shows them.
type HeatPlan struct {
	ScheduledAt     string `json:"scheduledAt"`
	DurationMinutes *int   `json:"durationMinutes"`
	HeatTimes
}

// CheckHeatPlan reads the dialog's fields: at "" follows the round's
// start, minutes is the typed text ("" = not given); day is the event's
// shown day.
func CheckHeatPlan(roundStart, at, minutes, day string) (*HeatPlan, error) {
	if at != "" && !domain.ValidScheduled(at) {
		return nil, invalid("计划时间格式不正确")
	}
	p := &HeatPlan{ScheduledAt: at}
	if text := strings.TrimSpace(minutes); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || !domain.ValidDuration(&n) {
			return nil, invalid("预计用时须为 1–%d 分钟", domain.MaxDurationMinutes)
		}
		p.DurationMinutes = &n
	}
	p.HeatTimes = HeatTimesOf(roundStart, at, p.DurationMinutes, day)
	return p, nil
}
