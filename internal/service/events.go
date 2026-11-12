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
	ParticipantMode string `json:"participantMode,omitempty"`
	TeamSize        int    `json:"teamSize,omitempty"`
	ScoreDecimals   int    `json:"scoreDecimals,omitempty"`
	RankDirection   string `json:"rankDirection,omitempty"`
	AttemptMode     string `json:"attemptMode,omitempty"`
	StartMode       string `json:"startMode,omitempty"`
	DistanceMode    string `json:"distanceMode,omitempty"`
	RelayLegCount   int    `json:"relayLegCount,omitempty"`
	Name            string `json:"name,omitempty"` // "" = composed from the fields below
	Gender          string `json:"gender"`
	Discipline      string `json:"discipline"`
	GroupLabel      string `json:"groupLabel,omitempty"`
	Kind            string `json:"kind"`
	Date            string `json:"date,omitempty"`
	LaneCount       int    `json:"laneCount,omitempty"`
	AttemptCount    int    `json:"attemptCount,omitempty"`
	Note            string `json:"note,omitempty"`
	Prelim          bool   `json:"prelim,omitempty"`
	AdvanceByPlace  int    `json:"advanceByPlace,omitempty"`
	AdvanceByMark   int    `json:"advanceByMark,omitempty"`
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
	// TimeText is when the event starts: its earliest planned time, a
	// round's start or a heat's own (HH:mm on the meet's first day, else
	// with the date), or its date when nothing has a time and the date is
	// not the first day; "" otherwise.
	TimeText string `json:"timeText"`
}

// ListEvents returns a meet's events in schedule order (domain.ScheduleOrder).
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
	meet, err := s.store.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	start := ""
	if meet != nil {
		start = meet.StartDate
	}
	entered := map[string]int{}
	for _, e := range entries {
		entered[e.EventID]++
	}
	events = domain.ScheduleOrder(events, rounds, start)
	out := make([]EventRow, len(events))
	for i, e := range events {
		row := EventRow{Event: e, Entries: entered[e.ID],
			AutoName: e.Name == domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)}
		if first := domain.EarliestScheduled(rounds[e.ID]); first != "" {
			row.TimeText = domain.ScheduledText(first, start)
		} else if day := domain.EventDay(e, start); day != start {
			row.TimeText = day
		}
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
		CompetitionConfig: domain.CompetitionConfig{ParticipantMode: in.ParticipantMode, TeamSize: in.TeamSize, ScoreDecimals: in.ScoreDecimals, RankDirection: in.RankDirection, AttemptMode: in.AttemptMode},
		Name:              strings.TrimSpace(in.Name),
		Discipline:        strings.TrimSpace(in.Discipline),
		GroupLabel:        strings.TrimSpace(in.GroupLabel),
		Kind:              domain.EventKind(in.Kind),
		Date:              strings.TrimSpace(in.Date),
		LaneCount:         in.LaneCount,
		AttemptCount:      in.AttemptCount,
		Note:              strings.TrimSpace(in.Note),
		StartMode:         domain.StartMode(in.StartMode), DistanceMode: domain.DistanceMode(in.DistanceMode), RelayLegCount: in.RelayLegCount,
	}
	g, ok := domain.NormalizeEventGender(in.Gender)
	if e.LaneCount == 0 {
		e.LaneCount = 8
	}
	if e.AttemptCount == 0 {
		if e.Kind.Scalar() {
			e.AttemptCount = 1
		} else {
			e.AttemptCount = 3
		}
	}
	switch {
	case !ok:
		return e, invalid("请选择性别")
	case e.Discipline == "":
		return e, invalid("请填写项目名")
	case !e.Kind.Valid():
		return e, invalid("请选择项目类型")
	case e.Date != "" && !domain.ValidDate(e.Date):
		return e, invalid("日期格式不正确")
	case e.LaneCount < 1 || e.LaneCount > 12:
		return e, invalid("道数应在 1 到 12 之间")
	case e.AttemptCount < 1 || e.AttemptCount > 6:
		return e, invalid("尝试次数应在 1 到 6 之间")
	case in.Prelim && (in.AdvanceByPlace < 0 || in.AdvanceByMark < 0 || in.AdvanceByPlace+in.AdvanceByMark == 0):
		return e, invalid("请填写晋级人数")
	}
	if err := e.NormalizeConfig(); err != nil {
		return e, invalid("%s", err)
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
	return store.Stmt(`INSERT INTO rounds(id, event_id, stage, ordinal, name, advance_by_place, advance_by_mark, scheduled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, r.ID, r.EventID, string(r.Stage), r.Ordinal, r.Name, byPlace, byMark, r.ScheduledAt)
}

func insertEvent(e domain.Event, now int64) sqlite.Statement {
	return store.Stmt(`INSERT INTO events(id, meet_id, name, gender, discipline, group_label, kind, date,
		lane_count, attempt_count, note, start_mode, distance_mode, relay_leg_count, config_json, combined_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.MeetID, e.Name, string(e.Gender), e.Discipline, e.GroupLabel, string(e.Kind), e.Date,
		e.LaneCount, e.AttemptCount, e.Note, string(e.StartMode), string(e.DistanceMode), e.RelayLegCount, store.EventConfigJSON(e), nullable(e.CombinedID), now)
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
	for _, existing := range events {
		if existing.ID == id && existing.CombinedID != "" {
			return nil, invalid("请在全能中编辑分项")
		}
	}
	var old *domain.Event
	key := domain.EventKey(e.Gender, e.Discipline, e.GroupLabel)
	for _, o := range events {
		if o.ID == id {
			old = &o
		} else if o.CombinedID == "" && domain.EventKey(o.Gender, o.Discipline, o.GroupLabel) == key {
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
		if old.CombinedID != "" {
			return nil, invalid("请在全能中编辑分项")
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
		if e.IsTeam() != old.IsTeam() || (e.Kind == domain.KindRelay) != (old.Kind == domain.KindRelay) {
			entries, err := s.store.ListEntries(ctx, store.Scope{EventID: id})
			if err != nil {
				return nil, err
			}
			teams, err := s.store.ListTeams(ctx, store.Scope{EventID: id})
			if err != nil {
				return nil, err
			}
			occupied := false
			for _, r := range rounds[id] {
				for _, h := range r.Heats {
					occupied = occupied || len(h.Slots) > 0
				}
			}
			if len(entries) > 0 || len(teams) > 0 || occupied {
				if (e.Kind == domain.KindRelay) != (old.Kind == domain.KindRelay) {
					return nil, invalid("已有报名或队伍，不能切换接力类型")
				}
				return nil, invalid("已有报名，不能改参赛方式")
			}
		}
		if e.Kind != old.Kind || e.ScoreDecimals != old.ScoreDecimals {
			entries, err := s.store.ListEntries(ctx, store.Scope{EventID: id})
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if entry.SeedMark != nil {
					return nil, invalid("已有报名成绩，不能改单位")
				}
			}
		}
		if e.Kind != old.Kind && results > 0 {
			return nil, invalid("已有成绩，不能改项目类型")
		}
		for _, r := range rounds[id] {
			for _, h := range r.Heats {
				if len(h.Slots) > 0 && e.UsesLanes() != old.UsesLanes() {
					return nil, invalid("已排进组，请先移出再改起跑方式")
				}
				for _, sl := range h.Slots {
					if e.UsesLanes() && sl.Position > e.LaneCount {
						return nil, invalid("已有道次超过所选道数")
					}
					if e.MultipleAttempts() && sl.Result != nil && len(sl.Result.Attempts) > e.AttemptCount {
						return nil, invalid("已有逐次成绩，不能减少次数")
					}
				}
			}
		}
		if results > 0 && e.Kind.Scalar() && (e.ScoreDecimals != old.ScoreDecimals || e.AttemptMode != old.AttemptMode || e.RankDirection != old.RankDirection) {
			return nil, invalid("已有成绩，不能改计分规则")
		}
		if results > 0 && e.Rules().StartLabel != old.Rules().StartLabel {
			return nil, invalid("已有成绩，不能改起跑方式")
		}
		stmts = append(stmts, store.Stmt(`UPDATE events SET name = ?, gender = ?, discipline = ?, group_label = ?,
			kind = ?, date = ?, lane_count = ?, attempt_count = ?, note = ?, start_mode = ?, distance_mode = ?, relay_leg_count = ?, config_json = ? WHERE id = ?`,
			e.Name, string(e.Gender), e.Discipline, e.GroupLabel, string(e.Kind), e.Date, e.LaneCount, e.AttemptCount, e.Note, string(e.StartMode), string(e.DistanceMode), e.RelayLegCount, store.EventConfigJSON(e), id))
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
	if e.CombinedID != "" {
		return invalid("请在全能中删除分项")
	}
	counts, err := s.store.Count(ctx, store.Scope{EventID: id})
	if err != nil {
		return err
	}
	if counts.Results > 0 && (!allowResults || src == store.SourceMCP) {
		return invalid("「%s」已有成绩，只能在界面上删除", e.Name)
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "event.deleted", MeetID: e.MeetID, EventID: id, Message: e.Name, Detail: counts},
		Change{Kind: "events", MeetID: e.MeetID, EventID: id},
		store.Stmt(`DELETE FROM events WHERE id = ?`, id))
}
