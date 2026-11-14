package store

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// EventOverview contains counts and only the times/round headers needed for
// list ordering. TimingRounds is not a complete tree of heats or results.
type EventOverview struct {
	Entries, Heats, Placed, Results int
	TimingRounds                    []domain.Round
}

// EventOverviews aggregates in SQLite: two queries return at most one row per
// round and one per event, irrespective of the number of slots/results. Round
// subqueries use FK indexes, avoiding a large joined row set and DISTINCT sort.
func (s *Store) EventOverviews(ctx context.Context, meetID string) (map[string]EventOverview, error) {
	rows, err := s.Query(ctx, `SELECT r.id, r.event_id, r.stage, r.ordinal, r.name,
 r.advance_by_place, r.advance_by_mark, r.scheduled_at,
 (SELECT count(*) FROM heats WHERE round_id=r.id),
 (SELECT count(*) FROM slots WHERE heat_id IN (SELECT id FROM heats WHERE round_id=r.id)),
 (SELECT count(*) FROM results WHERE heat_id IN (SELECT id FROM heats WHERE round_id=r.id)),
 (SELECT min(nullif(scheduled_at,'')) FROM heats WHERE round_id=r.id)
 FROM rounds r
 WHERE r.event_id IN (SELECT id FROM events WHERE meet_id=?)
 ORDER BY r.ordinal`, meetID)
	if err != nil {
		return nil, err
	}
	out := map[string]EventOverview{}
	for _, v := range rows.Values {
		r := domain.Round{ID: str(v[0]), EventID: str(v[1]), Stage: domain.Stage(str(v[2])), Ordinal: int(i64(v[3])), Name: str(v[4]), ScheduledAt: str(v[7])}
		if p, m := optInt(v[5]), optInt(v[6]); p != nil || m != nil {
			r.Advance = &domain.Advance{}
			if p != nil {
				r.Advance.ByPlace = *p
			}
			if m != nil {
				r.Advance.ByMark = *m
			}
		}
		if at := str(v[11]); at != "" {
			r.Heats = []domain.Heat{{ScheduledAt: at}}
		}
		e := out[r.EventID]
		e.Heats += int(i64(v[8]))
		e.Placed += int(i64(v[9]))
		e.Results += int(i64(v[10]))
		e.TimingRounds = append(e.TimingRounds, r)
		out[r.EventID] = e
	}
	rows, err = s.Query(ctx, `SELECT event_id, count(*) FROM entries
 WHERE event_id IN (SELECT id FROM events WHERE meet_id=?) GROUP BY event_id`, meetID)
	if err != nil {
		return nil, err
	}
	for _, v := range rows.Values {
		e := out[str(v[0])]
		e.Entries = int(i64(v[1]))
		out[str(v[0])] = e
	}
	return out, nil
}

// EntryPeople counts people per event, expanding teams without materializing
// every entry or seed mark. Empty teams contribute zero people.
func (s *Store) EntryPeople(ctx context.Context, sc Scope) (map[string]int, error) {
	cond, arg := sc.events("event_id")
	rows, err := s.Query(ctx, `SELECT e.event_id,
 sum(CASE WHEN e.team_id IS NULL THEN 1 ELSE coalesce(m.people,0) END)
 FROM entries e LEFT JOIN (SELECT team_id, count(*) people FROM team_members
 WHERE team_id IN (SELECT team_id FROM entries WHERE `+cond+`) GROUP BY team_id) m ON m.team_id=e.team_id
 WHERE e.`+cond+` GROUP BY e.event_id`, arg, arg)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, v := range rows.Values {
		out[str(v[0])] = int(i64(v[1]))
	}
	return out, nil
}

// AthleteIdentityMatches uses canonical nonunique indexes; service still owns
// collision validation and permits atomic bib swaps during table imports.
func (s *Store) AthleteIdentityMatches(ctx context.Context, a domain.Athlete) ([]domain.Athlete, error) {
	return s.readAthletes(ctx, `id IN (
 SELECT id FROM athletes WHERE meet_id=? AND bib_key=? AND bib_key<>''
 UNION SELECT id FROM athletes WHERE meet_id=? AND student_number_key=? AND student_number_key<>'') AND id<>?`,
		a.MeetID, domain.NormalizeText(a.Bib), a.MeetID, domain.NormalizeText(a.StudentNumber), a.ID)
}

func (s *Store) AthleteExists(ctx context.Context, meetID, id string) (bool, error) {
	rows, err := s.Query(ctx, `SELECT EXISTS(SELECT 1 FROM athletes WHERE id=? AND meet_id=?)`, id, meetID)
	if err != nil {
		return false, err
	}
	return i64(rows.Values[0][0]) != 0, nil
}

func (s *Store) AthleteNameExists(ctx context.Context, meetID, exceptID, name string) (bool, error) {
	rows, err := s.Query(ctx, `SELECT EXISTS(SELECT 1 FROM athletes WHERE meet_id=? AND name_key=? AND id<>?)`, meetID, domain.NormalizeText(name), exceptID)
	if err != nil {
		return false, err
	}
	return i64(rows.Values[0][0]) != 0, nil
}

// LimitEntries returns a meet's entries as the entry limits count them,
// in one query: entries of events outside combineds, then combined entries;
// each with the athlete's or the team's unit.
func (s *Store) LimitEntries(ctx context.Context, meetID string) ([]domain.LimitEntry, error) {
	rows, err := s.Query(ctx, `SELECT e.event_id, COALESCE(e.athlete_id,''), COALESCE(e.team_id,''), COALESCE(a.unit_id, t.unit_id, '')
 FROM entries e JOIN events v ON v.id = e.event_id
 LEFT JOIN athletes a ON a.id = e.athlete_id LEFT JOIN teams t ON t.id = e.team_id
 WHERE v.meet_id = ? AND COALESCE(v.combined_id,'') = ''
 UNION ALL SELECT c.combined_id, c.athlete_id, '', COALESCE(a.unit_id,'')
 FROM combined_entries c JOIN combined_events v ON v.id = c.combined_id JOIN athletes a ON a.id = c.athlete_id
 WHERE v.meet_id = ?`, meetID, meetID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LimitEntry, len(rows.Values))
	for i, r := range rows.Values {
		out[i] = domain.LimitEntry{Item: str(r[0]), AthleteID: str(r[1]), TeamID: str(r[2]), UnitID: str(r[3])}
	}
	return out, nil
}
