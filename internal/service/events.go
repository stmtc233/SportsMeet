package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func itoa(n int) string { return strconv.Itoa(n) }

// EventInput creates or updates an event. Prelim asks for a 预赛 before
// the final, advancing by AdvanceByPlace and AdvanceByMark.
type EventInput struct {
	Name           string `json:"name"` // "" = composed from the fields below
	Gender         string `json:"gender"`
	Discipline     string `json:"discipline"`
	GroupLabel     string `json:"groupLabel"`
	Kind           string `json:"kind"`
	Date           string `json:"date"`
	LaneCount      int    `json:"laneCount"`
	AttemptCount   int    `json:"attemptCount"`
	Note           string `json:"note"`
	Prelim         bool   `json:"prelim"`
	AdvanceByPlace int    `json:"advanceByPlace"`
	AdvanceByMark  int    `json:"advanceByMark"`
}

// EventRow is an event in the meet's list, with its progress.
type EventRow struct {
	Event    domain.Event    `json:"event"`
	AutoName bool            `json:"autoName"` // the name is the composed one
	Prelim   bool            `json:"prelim"`
	Advance  *domain.Advance `json:"advance"`
	Entries  int             `json:"entries"`
	Heats    int             `json:"heats"`
	Placed   int             `json:"placed"`
	Results  int             `json:"results"`
}

// ListEvents returns a meet's events.
func (s *Service) ListEvents(ctx context.Context, meetID string) ([]EventRow, error) {
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}
	entered := map[string]int{}
	for _, e := range entries {
		entered[e.EventID]++
	}
	out := make([]EventRow, len(events))
	for i, e := range events {
		row := EventRow{Event: e, Entries: entered[e.ID],
			AutoName: e.Name == domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)}
		for _, r := range rounds[e.ID] {
			if r.Stage == domain.StageHeat {
				row.Prelim, row.Advance = true, r.Advance
			}
			row.Heats += len(r.Heats)
			for _, h := range r.Heats {
				row.Placed += len(h.Slots)
				row.Results += h.Facts().Results
			}
		}
		out[i] = row
	}
	return out, nil
}

func (in EventInput) event() (domain.Event, error) {
	e := domain.Event{
		Name:         strings.TrimSpace(in.Name),
		Discipline:   strings.TrimSpace(in.Discipline),
		GroupLabel:   strings.TrimSpace(in.GroupLabel),
		Kind:         domain.EventKind(in.Kind),
		Date:         strings.TrimSpace(in.Date),
		LaneCount:    in.LaneCount,
		AttemptCount: in.AttemptCount,
		Note:         strings.TrimSpace(in.Note),
	}
	g, ok := domain.NormalizeEventGender(in.Gender)
	if e.LaneCount == 0 {
		e.LaneCount = 8
	}
	if e.AttemptCount == 0 {
		e.AttemptCount = 3
	}
	switch {
	case !ok:
		return e, invalid("请选择性别")
	case e.Discipline == "":
		return e, invalid("请填写项目名")
	case e.Kind == domain.KindRelay:
		return e, invalid("暂不支持接力项目")
	case !e.Kind.Valid():
		return e, invalid("请选择项目类型")
	case e.Date != "" && !dateRe.MatchString(e.Date):
		return e, invalid("日期格式不正确")
	case e.LaneCount < 1 || e.LaneCount > 12:
		return e, invalid("道数应在 1 到 12 之间")
	case e.AttemptCount < 1 || e.AttemptCount > 6:
		return e, invalid("试跳次数应在 1 到 6 之间")
	case in.Prelim && (in.AdvanceByPlace < 0 || in.AdvanceByMark < 0 || in.AdvanceByPlace+in.AdvanceByMark == 0):
		return e, invalid("请填写晋级人数")
	}
	e.Gender = g
	if e.Name == "" {
		e.Name = domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)
	}
	return e, nil
}

func insertRound(r domain.Round) sqlite.Statement {
	var byPlace, byMark any
	if r.Advance != nil {
		byPlace, byMark = r.Advance.ByPlace, r.Advance.ByMark
	}
	return store.Stmt(`INSERT INTO rounds(id, event_id, stage, ordinal, name, advance_by_place, advance_by_mark)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, r.ID, r.EventID, string(r.Stage), r.Ordinal, r.Name, byPlace, byMark)
}

func insertEvent(e domain.Event, now int64) sqlite.Statement {
	return store.Stmt(`INSERT INTO events(id, meet_id, name, gender, discipline, group_label, kind, date,
		lane_count, attempt_count, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.MeetID, e.Name, string(e.Gender), e.Discipline, e.GroupLabel, string(e.Kind), e.Date,
		e.LaneCount, e.AttemptCount, e.Note, now)
}

// SaveEvent creates an event (id "") or updates one. A new event gets its
// final round, and a 预赛 before it when asked.
func (s *Service) SaveEvent(ctx context.Context, src store.Source, meetID, id string, in EventInput) (*domain.Event, error) {
	e, err := in.event()
	if err != nil {
		return nil, err
	}
	e.ID, e.MeetID = id, meetID
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	var old *domain.Event
	key := domain.EventKey(e.Gender, e.Discipline, e.GroupLabel)
	for _, o := range events {
		if o.ID == id {
			old = &o
		} else if domain.EventKey(o.Gender, o.Discipline, o.GroupLabel) == key {
			return nil, invalid("已有相同的项目「%s」", o.Name)
		}
	}
	var advance *domain.Advance
	if in.Prelim {
		advance = &domain.Advance{ByPlace: in.AdvanceByPlace, ByMark: in.AdvanceByMark}
	}
	audit := store.Audit{Source: src, Action: "event.updated", MeetID: meetID, EventID: id, Message: e.Name}
	var stmts []sqlite.Statement

	if id == "" {
		e.ID = newID()
		audit.Action, audit.EventID = "event.created", e.ID
		stmts = append(stmts, insertEvent(e, s.store.Now()))
		final := domain.Round{ID: newID(), EventID: e.ID, Stage: domain.StageFinal, Ordinal: 1}
		if in.Prelim {
			stmts = append(stmts, insertRound(domain.Round{ID: newID(), EventID: e.ID, Stage: domain.StageHeat, Ordinal: 1, Advance: advance}))
			final.Ordinal = 2
		}
		stmts = append(stmts, insertRound(final))
	} else {
		if old == nil {
			return nil, invalid("项目不存在或已删除")
		}
		rounds, err := s.store.LoadRounds(ctx, store.Scope{EventID: id})
		if err != nil {
			return nil, err
		}
		results := 0
		var prelim *domain.Round
		for _, r := range rounds[id] {
			if r.Stage == domain.StageHeat {
				prelim = &r
			}
			for _, h := range r.Heats {
				results += h.Facts().Results
			}
		}
		if e.Kind != old.Kind && results > 0 {
			return nil, invalid("已有成绩，不能改项目类型")
		}
		stmts = append(stmts, store.Stmt(`UPDATE events SET name = ?, gender = ?, discipline = ?, group_label = ?,
			kind = ?, date = ?, lane_count = ?, attempt_count = ?, note = ? WHERE id = ?`,
			e.Name, string(e.Gender), e.Discipline, e.GroupLabel, string(e.Kind), e.Date, e.LaneCount, e.AttemptCount, e.Note, id))
		switch {
		case in.Prelim && prelim == nil:
			// Later rounds move up by one to make room for the 预赛.
			stmts = append(stmts,
				store.Stmt(`UPDATE rounds SET ordinal = ordinal + 1 WHERE event_id = ?`, id),
				insertRound(domain.Round{ID: newID(), EventID: id, Stage: domain.StageHeat, Ordinal: 1, Advance: advance}))
		case in.Prelim:
			stmts = append(stmts, store.Stmt(`UPDATE rounds SET advance_by_place = ?, advance_by_mark = ? WHERE id = ?`,
				advance.ByPlace, advance.ByMark, prelim.ID))
		case prelim != nil:
			if len(prelim.Heats) > 0 {
				return nil, invalid("预赛已有组，请先删除这些组")
			}
			stmts = append(stmts,
				store.Stmt(`DELETE FROM rounds WHERE id = ?`, prelim.ID),
				store.Stmt(`UPDATE rounds SET ordinal = ordinal - 1 WHERE event_id = ? AND ordinal > ?`, id, prelim.Ordinal))
		}
	}
	if err := s.write(ctx, audit, Change{Kind: "events", MeetID: meetID, EventID: e.ID}, stmts...); err != nil {
		return nil, err
	}
	return &e, nil
}

// CountEvent returns what deleting the event would remove.
func (s *Service) CountEvent(ctx context.Context, id string) (store.MeetCounts, error) {
	return s.store.Count(ctx, store.Scope{EventID: id})
}

// DeleteEvent removes an event with its rounds, heats, results and
// entries. allowResults is false for callers that must not erase results
// (MCP): an event holding any is then refused.
func (s *Service) DeleteEvent(ctx context.Context, src store.Source, id string, allowResults bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	events, err := s.store.ListEvents(ctx, store.Scope{EventID: id})
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return invalid("项目不存在或已删除")
	}
	e := events[0]
	counts, err := s.store.Count(ctx, store.Scope{EventID: id})
	if err != nil {
		return err
	}
	if counts.Results > 0 && !allowResults {
		return invalid("「%s」已有成绩，只能在界面上删除", e.Name)
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "event.deleted", MeetID: e.MeetID, EventID: id, Message: e.Name, Detail: counts},
		Change{Kind: "events", MeetID: e.MeetID, EventID: id},
		store.Stmt(`DELETE FROM events WHERE id = ?`, id))
}
