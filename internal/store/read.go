package store

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Scope selects one event, an event set, or a whole meet.
// Queries filter by it instead of running once per event or heat
// (docs/agents/storage.md「查询」).
type Scope struct {
	MeetID  string
	EventID string
	// EventIDs restricts a selection to these events; nil means no restriction.
	EventIDs []string
}

// events returns a condition on an event-id column and its argument.
func (sc Scope) events(col string) (string, any) {
	if sc.EventID != "" {
		return col + " = ?", sc.EventID
	}
	if sc.EventIDs != nil {
		b, _ := json.Marshal(sc.EventIDs)
		return col + " IN (SELECT value FROM json_each(?))", string(b)
	}
	return col + " IN (SELECT id FROM events WHERE meet_id = ?)", sc.MeetID
}

// GetMeet returns the meet, or nil when it does not exist.
func (s *Store) GetMeet(ctx context.Context, id string) (*domain.Meet, error) {
	rows, err := s.Query(ctx, `SELECT `+meetColumns+` FROM meets WHERE id = ?`, id)
	if err != nil || len(rows.Values) == 0 {
		return nil, err
	}
	m := scanMeet(rows.Values[0])
	return &m, nil
}

// ListUnits returns a meet's units by name.
func (s *Store) ListUnits(ctx context.Context, meetID string) ([]domain.Unit, error) {
	rows, err := s.Query(ctx, `SELECT id, meet_id, name, short_name, COALESCE((SELECT json_group_array(group_id) FROM unit_groups WHERE unit_id=units.id),'[]') FROM units WHERE meet_id = ? ORDER BY name`, meetID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Unit, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, domain.Unit{ID: str(r[0]), MeetID: str(r[1]), Name: str(r[2]), ShortName: str(r[3]), GroupIDs: stringIDs(r[4])})
	}
	return out, nil
}

// ListAthletes returns a meet's athletes in creation order.
func (s *Store) ListAthletes(ctx context.Context, meetID string) ([]domain.Athlete, error) {
	return s.readAthletes(ctx, "meet_id = ?", meetID)
}

// ListParticipants includes entered athletes, placed athletes and every team
// member in the scope. Placing someone does not require an existing entry.
func (s *Store) ListParticipants(ctx context.Context, sc Scope) ([]domain.Athlete, error) {
	entry, arg := sc.events("event_id")
	return s.readAthletes(ctx, `id IN (
 SELECT athlete_id FROM entries WHERE `+entry+`
 UNION SELECT athlete_id FROM slots WHERE heat_id IN (SELECT id FROM heats WHERE round_id IN (SELECT id FROM rounds WHERE `+entry+`))
 UNION SELECT athlete_id FROM team_members WHERE team_id IN (SELECT id FROM teams WHERE `+entry+`))`, arg, arg, arg)
}

func (s *Store) readAthletes(ctx context.Context, where string, args ...any) ([]domain.Athlete, error) {
	rows, err := s.Query(ctx, `SELECT id, meet_id, name, gender, bib, student_number, unit_id, class_name,
		grade, birth_date, note, group_id FROM athletes WHERE `+where+` ORDER BY created_at, rowid`, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Athlete, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, domain.Athlete{ID: str(r[0]), MeetID: str(r[1]), Name: str(r[2]), Gender: domain.Gender(str(r[3])),
			Bib: str(r[4]), StudentNumber: str(r[5]), UnitID: str(r[6]), ClassName: str(r[7]), Grade: str(r[8]),
			BirthDate: str(r[9]), Note: str(r[10]), GroupID: str(r[11])})
	}
	return out, nil
}

// ListEvents returns the scope's events in creation order.
func (s *Store) ListEvents(ctx context.Context, sc Scope) ([]domain.Event, error) {
	cond, arg := "meet_id = ?", any(sc.MeetID)
	if sc.EventID != "" {
		cond, arg = "id = ?", sc.EventID
	} else if sc.EventIDs != nil {
		cond, arg = sc.events("id")
	}
	rows, err := s.Query(ctx, `SELECT id, meet_id, name, gender, discipline, COALESCE((SELECT name FROM meet_groups WHERE id=events.group_id),''), kind, date, lane_count,
		attempt_count, note, start_mode, distance_mode, relay_leg_count, config_json, combined_id, group_id FROM events WHERE `+cond+` ORDER BY created_at, rowid`, arg)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Event, 0, len(rows.Values))
	for _, r := range rows.Values {
		e := domain.Event{ID: str(r[0]), MeetID: str(r[1]), Name: str(r[2]), Gender: domain.Gender(str(r[3])),
			Discipline: str(r[4]), GroupLabel: str(r[5]), Kind: domain.EventKind(str(r[6])), Date: str(r[7]),
			LaneCount: int(i64(r[8])), AttemptCount: int(i64(r[9])), Note: str(r[10]), StartMode: domain.StartMode(str(r[11])), DistanceMode: domain.DistanceMode(str(r[12])), RelayLegCount: int(i64(r[13])), CombinedID: str(r[15]), GroupID: str(r[16])}
		if err := json.Unmarshal([]byte(str(r[14])), &e.CompetitionConfig); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// ListEntries returns the scope's athlete and relay team entries in entry order.
func (s *Store) ListEntries(ctx context.Context, sc Scope) ([]domain.Entry, error) {
	cond, arg := sc.events("event_id")
	rows, err := s.Query(ctx, `SELECT event_id, athlete_id, seed_mark, team_id FROM entries
		WHERE `+cond+` ORDER BY rowid`, arg)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Entry, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, domain.Entry{EventID: str(r[0]), AthleteID: str(r[1]), SeedMark: optI64(r[2]), TeamID: str(r[3])})
	}
	return out, nil
}

type resultDetail struct {
	HeightPending bool             `json:"heightPending,omitempty"`
	Attempts      []domain.Attempt `json:"attempts,omitempty"`
	Bars          []domain.Bar     `json:"bars,omitempty"`
}

// ResultDetailJSON encodes the per-attempt and per-height records.
func ResultDetailJSON(r domain.Result) string {
	b, err := json.Marshal(resultDetail{Attempts: r.Attempts, Bars: r.Bars, HeightPending: r.HeightPending})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// LoadRounds reads the scope's rounds with heats, slots and results in
// four queries, and returns them by event id, ordered by ordinal.
func (s *Store) LoadRounds(ctx context.Context, sc Scope) (map[string][]domain.Round, error) {
	return s.loadRounds(ctx, sc, true)
}

// LoadPlanRounds reads positions and result presence, without marks or attempt
// records. Consumers may count Results, but must not score or write these values.
func (s *Store) LoadPlanRounds(ctx context.Context, sc Scope) (map[string][]domain.Round, error) {
	return s.loadRounds(ctx, sc, false)
}

func (s *Store) loadRounds(ctx context.Context, sc Scope, details bool) (map[string][]domain.Round, error) {
	cond, arg := sc.events("event_id")
	rr, err := s.Query(ctx, `SELECT id, event_id, stage, ordinal, name, advance_by_place, advance_by_mark, scheduled_at
		FROM rounds WHERE `+cond+` ORDER BY ordinal`, arg)
	if err != nil {
		return nil, err
	}
	inRounds := "round_id IN (SELECT id FROM rounds WHERE " + cond + ")"
	hr, err := s.Query(ctx, `SELECT id, round_id, ordinal, name, scheduled_at, wind, duration_minutes, started_at FROM heats
		WHERE `+inRounds+` ORDER BY ordinal`, arg)
	if err != nil {
		return nil, err
	}
	inHeats := "heat_id IN (SELECT id FROM heats WHERE " + inRounds + ")"
	slotSQL := `SELECT heat_id, position, athlete_id, team_id FROM slots WHERE ` + inHeats + ` ORDER BY position`
	if !details {
		slotSQL = `SELECT sl.heat_id, sl.position, sl.athlete_id, sl.team_id, res.updated_at
 FROM slots sl LEFT JOIN results res ON res.heat_id=sl.heat_id AND res.position=sl.position
 WHERE sl.` + inHeats + ` ORDER BY sl.position`
	}
	sr, err := s.Query(ctx, slotSQL, arg)
	if err != nil {
		return nil, err
	}
	var resultRows [][]any
	if details {
		res, err := s.Query(ctx, `SELECT heat_id, position, status, mark, rank, qualify, wind, detail_json, note, updated_at FROM results WHERE `+inHeats, arg)
		if err != nil {
			return nil, err
		}
		resultRows = res.Values
	}

	type slotKey struct {
		heat string
		pos  int
	}
	results := make(map[slotKey]*domain.Result, len(resultRows))
	updated := map[string]int64{}
	present := &domain.Result{}
	for _, r := range resultRows {
		updated[str(r[0])] = max(updated[str(r[0])], i64(r[9]))
		v := &domain.Result{Status: domain.Status(str(r[2])), Mark: optI64(r[3]), Rank: optInt(r[4]),
			Qualify: str(r[5]), Wind: optInt(r[6]), Note: str(r[8])}
		var d resultDetail
		if json.Unmarshal([]byte(str(r[7])), &d) == nil {
			v.Attempts, v.Bars = d.Attempts, d.Bars
			v.HeightPending = d.HeightPending
		}
		results[slotKey{str(r[0]), int(i64(r[1]))}] = v
	}
	slots := map[string][]domain.Slot{}
	for _, r := range sr.Values {
		h, pos := str(r[0]), int(i64(r[1]))
		result := results[slotKey{h, pos}]
		if !details && r[4] != nil {
			result = present
			updated[h] = max(updated[h], i64(r[4]))
		}
		slots[h] = append(slots[h], domain.Slot{Position: pos, AthleteID: str(r[2]), TeamID: str(r[3]), Result: result})
	}
	heats := map[string][]domain.Heat{}
	for _, r := range hr.Values {
		h := domain.Heat{ID: str(r[0]), RoundID: str(r[1]), Ordinal: int(i64(r[2])), Name: str(r[3]),
			ScheduledAt: str(r[4]), Wind: optInt(r[5]), DurationMinutes: optInt(r[6]), StartedAt: str(r[7]), Slots: slots[str(r[0])], ResultsUpdatedAt: updated[str(r[0])]}
		if h.Slots == nil {
			h.Slots = []domain.Slot{}
		}
		heats[h.RoundID] = append(heats[h.RoundID], h)
	}
	out := map[string][]domain.Round{}
	for _, r := range rr.Values {
		round := domain.Round{ID: str(r[0]), EventID: str(r[1]), Stage: domain.Stage(str(r[2])),
			Ordinal: int(i64(r[3])), Name: str(r[4]), ScheduledAt: str(r[7]), Heats: heats[str(r[0])]}
		if round.Heats == nil {
			round.Heats = []domain.Heat{}
		}
		if p, m := optInt(r[5]), optInt(r[6]); p != nil || m != nil {
			round.Advance = &domain.Advance{}
			if p != nil {
				round.Advance.ByPlace = *p
			}
			if m != nil {
				round.Advance.ByMark = *m
			}
		}
		out[round.EventID] = append(out[round.EventID], round)
	}
	for _, list := range out {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Ordinal < list[j].Ordinal })
	}
	return out, nil
}

// MeetCounts is what deleting a meet or an event would remove.
type MeetCounts struct {
	Events   int `json:"events"`
	Athletes int `json:"athletes"`
	Heats    int `json:"heats"`
	Results  int `json:"results"`
}

// Count returns the scope's totals.
func (s *Store) Count(ctx context.Context, sc Scope) (MeetCounts, error) {
	cond, arg := sc.events("event_id")
	inRounds := "round_id IN (SELECT id FROM rounds WHERE " + cond + ")"
	q := `SELECT
		(SELECT count(*) FROM heats WHERE ` + inRounds + `),
		(SELECT count(*) FROM results WHERE heat_id IN (SELECT id FROM heats WHERE ` + inRounds + `))`
	rows, err := s.Query(ctx, q, arg, arg)
	if err != nil {
		return MeetCounts{}, err
	}
	c := MeetCounts{Heats: int(i64(rows.Values[0][0])), Results: int(i64(rows.Values[0][1]))}
	if sc.EventID == "" {
		rows, err = s.Query(ctx, `SELECT (SELECT count(*) FROM events WHERE meet_id = ?),
			(SELECT count(*) FROM athletes WHERE meet_id = ?)`, sc.MeetID, sc.MeetID)
		if err != nil {
			return MeetCounts{}, err
		}
		c.Events, c.Athletes = int(i64(rows.Values[0][0])), int(i64(rows.Values[0][1]))
	}
	return c, nil
}

func optI64(v any) *int64 {
	n, ok := v.(int64)
	if !ok {
		return nil
	}
	return &n
}

func optInt(v any) *int {
	n, ok := v.(int64)
	if !ok {
		return nil
	}
	i := int(n)
	return &i
}

// AthleteUsage returns, for a meet's athletes, how many events each has
// entered and how many slots each occupies.
func (s *Store) AthleteUsage(ctx context.Context, meetID string) (entries, slots map[string]int, err error) {
	entries, slots = map[string]int{}, map[string]int{}
	rows, err := s.Query(ctx, `SELECT athlete_id,count(DISTINCT event_id) FROM (
 SELECT athlete_id,event_id FROM entries WHERE athlete_id IN (SELECT id FROM athletes WHERE meet_id=?)
 UNION ALL SELECT m.athlete_id,e.event_id FROM team_members m JOIN entries e ON e.team_id=m.team_id JOIN athletes a ON a.id=m.athlete_id WHERE a.meet_id=?
 ) GROUP BY athlete_id`, meetID, meetID)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rows.Values {
		entries[str(r[0])] = int(i64(r[1]))
	}
	rows, err = s.Query(ctx, `SELECT athlete_id,count(*) FROM (
 SELECT athlete_id FROM slots WHERE athlete_id IN (SELECT id FROM athletes WHERE meet_id=?)
 UNION ALL SELECT m.athlete_id FROM team_members m JOIN slots s ON s.team_id=m.team_id JOIN athletes a ON a.id=m.athlete_id WHERE a.meet_id=?
 ) GROUP BY athlete_id`, meetID, meetID)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rows.Values {
		slots[str(r[0])] = int(i64(r[1]))
	}

	return entries, slots, nil
}

// GetSetting returns a stored setting, or "" when unset.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	rows, err := s.Query(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	if err != nil || len(rows.Values) == 0 {
		return "", err
	}
	return str(rows.Values[0][0]), nil
}

// EventConfigJSON encodes the event-local scalar and participant configuration.
func EventConfigJSON(e domain.Event) string {
	b, _ := json.Marshal(e.CompetitionConfig)
	return string(b)
}

func stringIDs(v any) []string {
	ids := []string{}
	_ = json.Unmarshal([]byte(str(v)), &ids)
	return ids
}
