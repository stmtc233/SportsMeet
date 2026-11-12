package service

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// HeatStartInput sets when a heat actually started; StartedAt "" clears it.
type HeatStartInput struct {
	HeatID    string `json:"heatId"`
	StartedAt string `json:"startedAt" jsonschema:"实际开赛的本地时间，YYYY-MM-DDTHH:mm，无时区；空字符串表示清除"`
}

// HeatStartResult is the heat's actual start after the write, shown as
// the event detail shows it.
type HeatStartResult struct {
	HeatID string `json:"heatId"`
	HeatStart
}

// SetHeatStarted records, changes or clears a heat's actual start. It is
// not arranging: no fingerprint, no lock, and results stay as they are
// (docs/agents/domain.md「时间」). An unchanged value writes nothing.
func (s *Service) SetHeatStarted(ctx context.Context, src store.Source, in HeatStartInput) (*HeatStartResult, error) {
	if in.StartedAt != "" && !domain.ValidScheduled(in.StartedAt) {
		return nil, invalid("开赛时间格式不正确")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.store.Query(ctx, `SELECT r.event_id FROM heats h JOIN rounds r ON r.id = h.round_id WHERE h.id = ?`, in.HeatID)
	if err != nil {
		return nil, err
	}
	if len(rows.Values) == 0 {
		return nil, invalid("组不存在或已删除")
	}
	st, err := s.loadEvent(ctx, rows.Values[0][0].(string))
	if err != nil {
		return nil, err
	}
	meet, err := s.store.GetMeet(ctx, st.event.MeetID)
	if err != nil {
		return nil, err
	}
	start := ""
	if meet != nil {
		start = meet.StartDate
	}
	day := domain.PlannedDay(st.event, st.rounds, start)
	for _, r := range st.rounds {
		for _, h := range r.Heats {
			if h.ID != in.HeatID {
				continue
			}
			effective, _ := domain.EffectiveScheduled(r, h)
			out := &HeatStartResult{HeatID: h.ID, HeatStart: HeatStartOf(effective, in.StartedAt, day)}
			if h.StartedAt == in.StartedAt {
				return out, nil
			}
			text := domain.ScheduledText(in.StartedAt, "")
			if text == "" {
				text = "清除"
			}
			err := s.write(ctx,
				store.Audit{Source: src, Action: "heat.started", MeetID: st.event.MeetID, EventID: st.event.ID, HeatID: h.ID,
					Message: st.event.Name + " " + r.Label() + " " + domain.HeatLabel(h.Ordinal, h.Name) + " " + text,
					Detail: struct {
						Before string `json:"before"`
						After  string `json:"after"`
					}{h.StartedAt, in.StartedAt}},
				Change{Kind: "heats", MeetID: st.event.MeetID, EventID: st.event.ID},
				store.Stmt(`UPDATE heats SET started_at = ? WHERE id = ?`, in.StartedAt, h.ID))
			if err != nil {
				return nil, err
			}
			return out, nil
		}
	}
	return nil, invalid("组不存在或已删除")
}
