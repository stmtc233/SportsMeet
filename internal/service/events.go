package service

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func itoa(n int) string { return strconv.Itoa(n) }

// RoundInput is a round held before the final (预赛 heat, 次赛
// quarterfinal, 半决赛 semifinal) and who advances from it: the first
// AdvanceByPlace of each heat plus the AdvanceByMark best of the rest.
type RoundInput struct {
	Stage          string `json:"stage"`
	AdvanceByPlace int    `json:"advanceByPlace"`
	AdvanceByMark  int    `json:"advanceByMark"`
}

// EventInput creates or updates an event. Rounds lists the rounds before
// the final, which every event has; empty means the final alone.
type EventInput struct {
	ParticipantMode string       `json:"participantMode,omitempty"`
	TeamSize        int          `json:"teamSize,omitempty"`
	ScoreDecimals   int          `json:"scoreDecimals,omitempty"`
	RankDirection   string       `json:"rankDirection,omitempty"`
	AttemptMode     string       `json:"attemptMode,omitempty"`
	StartMode       string       `json:"startMode,omitempty"`
	DistanceMode    string       `json:"distanceMode,omitempty"`
	RelayLegCount   int          `json:"relayLegCount,omitempty"`
	Name            string       `json:"name,omitempty"` // "" = composed from the fields below
	Gender          string       `json:"gender"`
	Discipline      string       `json:"discipline"`
	GroupLabel      string       `json:"groupLabel,omitempty"`
	Kind            string       `json:"kind"`
	Date            string       `json:"date,omitempty"`
	LaneCount       int          `json:"laneCount,omitempty"`
	AttemptCount    int          `json:"attemptCount,omitempty"`
	Note            string       `json:"note,omitempty"`
	Rounds          []RoundInput `json:"rounds,omitempty"`
}

// EventRow is an event in the meet's list, with its progress.
type EventRow struct {
	Event    domain.Event `json:"event"`
	AutoName bool         `json:"autoName"` // the name is the composed one
	// Rounds are the rounds before the final, in order; FormatText names
	// the whole sequence (「预赛 + 决赛」, 「直接决赛」).
	Rounds     []RoundInput `json:"rounds"`
	FormatText string       `json:"formatText"`
	Entries    int          `json:"entries"`
	Heats      int          `json:"heats"`
	Placed     int          `json:"placed"`
	Results    int          `json:"results"`
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
		row := EventRow{Event: e, Entries: entered[e.ID], Rounds: []RoundInput{}, FormatText: formatText(rounds[e.ID]),
			AutoName: e.Name == domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)}
		if first := domain.EarliestScheduled(rounds[e.ID]); first != "" {
			row.TimeText = domain.ScheduledText(first, start)
		} else if day := domain.EventDay(e, start); day != start {
			row.TimeText = day
		}
		for _, r := range rounds[e.ID] {
			if r.Stage != domain.StageFinal {
				in := RoundInput{Stage: string(r.Stage)}
				if r.Advance != nil {
					in.AdvanceByPlace, in.AdvanceByMark = r.Advance.ByPlace, r.Advance.ByMark
				}
				row.Rounds = append(row.Rounds, in)
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

// formatText names an event's rounds in order; the final alone is 「直接决赛」.
func formatText(rounds []domain.Round) string {
	if len(rounds) <= 1 {
		return "直接决赛"
	}
	labels := make([]string, len(rounds))
	for i, r := range rounds {
		labels[i] = r.Label()
	}
	return strings.Join(labels, " + ")
}

// qualifying validates the rounds before the final and returns them in
// stage order.
func (in EventInput) qualifying() ([]domain.Round, error) {
	out := make([]domain.Round, 0, len(in.Rounds))
	seen := map[domain.Stage]bool{}
	for _, r := range in.Rounds {
		stage := domain.Stage(strings.TrimSpace(r.Stage))
		switch {
		case !stage.Valid() || stage == domain.StageFinal:
			return nil, invalid("轮次只能是预赛、次赛或半决赛")
		case seen[stage]:
			return nil, invalid("%s重复", stage.Label())
		case r.AdvanceByPlace < 0 || r.AdvanceByMark < 0 || r.AdvanceByPlace+r.AdvanceByMark == 0:
			return nil, invalid("请填写%s晋级人数", stage.Label())
		}
		seen[stage] = true
		out = append(out, domain.Round{Stage: stage, Advance: &domain.Advance{ByPlace: r.AdvanceByPlace, ByMark: r.AdvanceByMark}})
	}
	sort.SliceStable(out, func(i, j int) bool { return domain.StageOrder(out[i].Stage) < domain.StageOrder(out[j].Stage) })
	return out, nil
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
// final round and the rounds asked for before it. An update adds and
// removes rounds to match; a round that has heats is not removed.
func (s *Service) SaveEvent(ctx context.Context, src store.Source, meetID, id string, in EventInput) (*domain.Event, error) {
	e, err := in.event()
	if err != nil {
		return nil, err
	}
	before, err := in.qualifying()
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
	audit := store.Audit{Source: src, Action: "event.updated", MeetID: meetID, EventID: id, Message: e.Name}
	var stmts []sqlite.Statement

	if id == "" {
		e.ID = newID()
		audit.Action, audit.EventID = "event.created", e.ID
		stmts = append(stmts, insertEvent(e, s.store.Now()))
		for i, r := range before {
			r.ID, r.EventID, r.Ordinal = newID(), e.ID, i+1
			stmts = append(stmts, insertRound(r))
		}
		stmts = append(stmts, insertRound(domain.Round{ID: newID(), EventID: e.ID, Stage: domain.StageFinal, Ordinal: len(before) + 1}))
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
		have := map[domain.Stage]domain.Round{}
		for _, r := range rounds[id] {
			have[r.Stage] = r
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
		// Rounds match by stage and are numbered in stage order, the final last.
		want := map[domain.Stage]bool{}
		for i, r := range before {
			want[r.Stage] = true
			if old, ok := have[r.Stage]; ok {
				stmts = append(stmts, store.Stmt(`UPDATE rounds SET ordinal = ?, advance_by_place = ?, advance_by_mark = ? WHERE id = ?`,
					i+1, r.Advance.ByPlace, r.Advance.ByMark, old.ID))
				continue
			}
			r.ID, r.EventID, r.Ordinal = newID(), id, i+1
			stmts = append(stmts, insertRound(r))
		}
		for _, r := range rounds[id] {
			switch {
			case r.Stage == domain.StageFinal:
				stmts = append(stmts, store.Stmt(`UPDATE rounds SET ordinal = ? WHERE id = ?`, len(before)+1, r.ID))
			case want[r.Stage]:
			case len(r.Heats) > 0:
				return nil, invalid("%s已有组，请先删除这些组", r.Label())
			default:
				stmts = append(stmts, store.Stmt(`DELETE FROM rounds WHERE id = ?`, r.ID))
			}
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
