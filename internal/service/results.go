package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// ResultInput is what was typed for one slot. A row with nothing in it
// (no status, mark, attempts or trials) removes the slot's result.
type ResultInput struct {
	Position int    `json:"position"`
	Status   string `json:"status,omitempty"` // "" = from the marks: OK, or NM for a field record without a valid mark
	Mark     string `json:"mark,omitempty"`   // timed events, and field events without a record
	// Attempts are a distance result's tries in order: a mark, X or -.
	Attempts []string `json:"attempts,omitempty"`
	// Bars are a height result's heights; blank trials are skipped.
	Bars []BarInput `json:"bars,omitempty"`
	Note string     `json:"note,omitempty"`
}

// BarInput is one height as typed.
type BarInput struct {
	Height string `json:"height"`
	Trials string `json:"trials"`
}

func positionLabel(kind domain.EventKind, pos int) string {
	if kind.Timed() {
		return fmt.Sprintf("第 %d 道", pos)
	}
	return fmt.Sprintf("第 %d 位", pos)
}

// buildResult turns typed input into a result, or nil to remove it.
func buildResult(kind domain.EventKind, in ResultInput) (*domain.Result, error) {
	r := &domain.Result{Status: domain.Status(strings.ToUpper(strings.TrimSpace(in.Status))), Note: strings.TrimSpace(in.Note)}
	if r.Status != "" && !r.Status.Valid() {
		return nil, invalid("状态不正确")
	}
	attempts := append([]string(nil), in.Attempts...)
	for len(attempts) > 0 && strings.TrimSpace(attempts[len(attempts)-1]) == "" {
		attempts = attempts[:len(attempts)-1]
	}
	for i, text := range attempts {
		if strings.TrimSpace(text) == "" {
			return nil, invalid("第 %d 次未填", i+1)
		}
		a, err := scoring.ParseAttempt(kind, text)
		if err != nil {
			return nil, invalid("第 %d 次「%s」格式不正确", i+1, text)
		}
		r.Attempts = append(r.Attempts, a)
	}
	for _, b := range in.Bars {
		if strings.TrimSpace(b.Trials) == "" {
			continue
		}
		h, err := scoring.ParseMark(kind, b.Height)
		if err != nil {
			return nil, invalid("高度「%s」格式不正确", b.Height)
		}
		trials, err := scoring.ParseTrials(b.Trials)
		if err != nil {
			return nil, invalid("%s:%s", b.Height, err)
		}
		r.Bars = append(r.Bars, domain.Bar{Height: h, Trials: trials})
	}
	for i := 1; i < len(r.Bars); i++ {
		if r.Bars[i].Height <= r.Bars[i-1].Height {
			return nil, invalid("高度应从低到高")
		}
	}
	if len(r.Attempts) > 0 && kind != domain.KindDistance || len(r.Bars) > 0 && kind != domain.KindHeight {
		return nil, invalid("逐次记录与项目类型不符")
	}

	mark, hasMark := int64(0), false
	switch {
	case len(r.Attempts) > 0:
		mark, hasMark = scoring.BestAttempt(r.Attempts)
	case len(r.Bars) > 0:
		mark, hasMark = scoring.ClearedHeight(r.Bars)
	case strings.TrimSpace(in.Mark) != "":
		m, err := scoring.ParseMark(kind, in.Mark)
		if err != nil {
			return nil, invalid("成绩「%s」格式不正确", in.Mark)
		}
		mark, hasMark = m, true
	}
	record := len(r.Attempts) > 0 || len(r.Bars) > 0
	switch {
	case r.Status == "" && hasMark:
		r.Status = domain.StatusOK
	case r.Status == "" && record:
		r.Status = domain.StatusNM
	case r.Status == "":
		return nil, nil
	case r.Status == domain.StatusOK && !hasMark:
		return nil, invalid("请填写成绩")
	}
	if r.Status == domain.StatusOK {
		r.Mark = &mark
	}
	return r, nil
}

func upsertResult(heatID string, pos int, r domain.Result, now int64) sqlite.Statement {
	return store.Stmt(`INSERT INTO results(heat_id, position, status, mark, rank, qualify, wind, detail_json, note, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(heat_id, position) DO UPDATE SET status = excluded.status, mark = excluded.mark,
		rank = excluded.rank, qualify = excluded.qualify, wind = excluded.wind, detail_json = excluded.detail_json,
		note = excluded.note, updated_at = excluded.updated_at`,
		heatID, pos, string(r.Status), optArg(r.Mark), optArg(r.Rank), r.Qualify, optArg(r.Wind), store.ResultDetailJSON(r), r.Note, now)
}

func optArg[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

type slotRef struct {
	heat string
	pos  int
}

// storedResults indexes a round's results as loaded.
func storedResults(heats []domain.Heat) map[slotRef]*domain.Result {
	out := map[slotRef]*domain.Result{}
	for _, h := range heats {
		for _, sl := range h.Slots {
			if sl.Result != nil {
				out[slotRef{h.ID, sl.Position}] = sl.Result
			}
		}
	}
	return out
}

// copyHeats copies heats down to their slots, so a write can change them
// without touching what was loaded.
func copyHeats(heats []domain.Heat) []domain.Heat {
	out := make([]domain.Heat, len(heats))
	for i, h := range heats {
		out[i] = h
		out[i].Slots = append([]domain.Slot(nil), h.Slots...)
	}
	return out
}

// resultWrites returns the statements bringing the stored results of
// heats to what they hold now; rewritten names heats whose slots were
// deleted and inserted again, which lost their result rows.
func resultWrites(before map[slotRef]*domain.Result, heats []domain.Heat, rewritten map[string]bool, now int64) []sqlite.Statement {
	var stmts []sqlite.Statement
	for _, h := range heats {
		for _, sl := range h.Slots {
			old := before[slotRef{h.ID, sl.Position}]
			switch {
			case sl.Result == nil && old != nil && !rewritten[h.ID]:
				stmts = append(stmts, store.Stmt(`DELETE FROM results WHERE heat_id = ? AND position = ?`, h.ID, sl.Position))
			case sl.Result != nil && (rewritten[h.ID] || old == nil || !reflect.DeepEqual(*old, *sl.Result)):
				stmts = append(stmts, upsertResult(h.ID, sl.Position, *sl.Result, now))
			}
		}
	}
	return stmts
}

// ResultChange is one result before and after a write, for the log.
type ResultChange struct {
	Athlete      string         `json:"athlete"`
	Before       string         `json:"before"`
	After        string         `json:"after"`
	BeforeRecord *domain.Result `json:"beforeRecord,omitempty"`
	AfterRecord  *domain.Result `json:"afterRecord,omitempty"`
}

func resultText(kind domain.EventKind, r *domain.Result) string {
	switch {
	case r == nil:
		return ""
	case r.Status == domain.StatusOK && r.Mark != nil:
		return scoring.FormatMark(kind, *r.Mark)
	}
	return string(r.Status)
}

// SaveResults records what was typed for slots of one heat, then
// recomputes the places and Q / q of its round. Places always come from
// scoring; callers cannot set them.
func (s *Service) SaveResults(ctx context.Context, src store.Source, heatID string, inputs []ResultInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.store.Query(ctx, `SELECT r.event_id, r.id FROM heats h JOIN rounds r ON r.id = h.round_id WHERE h.id = ?`, heatID)
	if err != nil {
		return err
	}
	if len(rows.Values) == 0 {
		return ErrStale
	}
	st, err := s.loadEvent(ctx, rows.Values[0][0].(string))
	if err != nil {
		return err
	}
	round := st.round(rows.Values[0][1].(string))
	kind := st.event.Kind
	before := storedResults(round.Heats)
	heats := copyHeats(round.Heats)
	var heat *domain.Heat
	for i := range heats {
		if heats[i].ID == heatID {
			heat = &heats[i]
		}
	}
	var changes []ResultChange
	seen := map[int]bool{}
	for _, in := range inputs {
		if seen[in.Position] {
			return invalid("位置重复")
		}
		seen[in.Position] = true
		var slot *domain.Slot
		for i := range heat.Slots {
			if heat.Slots[i].Position == in.Position {
				slot = &heat.Slots[i]
			}
		}
		if slot == nil || slot.CompetitorID() == "" {
			return ErrStale
		}
		r, err := buildResult(kind, in)
		if err != nil {
			return invalid("%s:%s", positionLabel(kind, in.Position), err)
		}
		if r != nil && slot.Result != nil {
			r.Wind = slot.Result.Wind
		}
		if a, b := resultText(kind, slot.Result), resultText(kind, r); !reflect.DeepEqual(slot.Result, r) {
			changes = append(changes, ResultChange{Athlete: st.competitorName(slot.CompetitorID()), Before: a, After: b, BeforeRecord: slot.Result, AfterRecord: r})
		}
		slot.Result = r
	}
	rescore(kind, round.Advance, heats)
	stmts := resultWrites(before, heats, nil, s.store.Now())
	if len(stmts) == 0 {
		return nil
	}
	label := st.event.Name + " " + round.Label() + " " + domain.HeatLabel(heat.Ordinal, heat.Name)
	return s.write(ctx,
		store.Audit{Source: src, Action: "results.saved", MeetID: st.event.MeetID, EventID: st.event.ID, HeatID: heatID,
			Message: label, Detail: changes},
		Change{Kind: "results", MeetID: st.event.MeetID, EventID: st.event.ID}, stmts...)
}
