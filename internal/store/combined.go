package store

import (
	"context"
	"encoding/json"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

type CombinedData struct {
	Combined   domain.Combined
	Components []domain.CombinedComponent
	AthleteIDs []string
}

// ListCombined reads all parents, component rules and entries in three queries.
func (s *Store) ListCombined(ctx context.Context, meetID string) ([]CombinedData, error) {
	rr, err := s.Query(ctx, `SELECT id,meet_id,name,gender,group_label,note,rule_version FROM combined_events WHERE meet_id=? ORDER BY rowid`, meetID)
	if err != nil {
		return nil, err
	}
	cr, err := s.Query(ctx, `SELECT combined_id,event_id,ordinal,rule_json FROM combined_components WHERE combined_id IN (SELECT id FROM combined_events WHERE meet_id=?) ORDER BY ordinal`, meetID)
	if err != nil {
		return nil, err
	}
	er, err := s.Query(ctx, `SELECT combined_id,athlete_id FROM combined_entries WHERE combined_id IN (SELECT id FROM combined_events WHERE meet_id=?) ORDER BY rowid`, meetID)
	if err != nil {
		return nil, err
	}
	components := map[string][]domain.CombinedComponent{}
	athletes := map[string][]string{}
	for _, r := range cr.Values {
		c := domain.CombinedComponent{CombinedID: str(r[0]), EventID: str(r[1]), Ordinal: int(i64(r[2]))}
		if err := json.Unmarshal([]byte(str(r[3])), &c.Rule); err != nil {
			return nil, err
		}
		components[c.CombinedID] = append(components[c.CombinedID], c)
	}
	for _, r := range er.Values {
		athletes[str(r[0])] = append(athletes[str(r[0])], str(r[1]))
	}
	out := make([]CombinedData, 0, len(rr.Values))
	for _, r := range rr.Values {
		id := str(r[0])
		out = append(out, CombinedData{Combined: domain.Combined{ID: id, MeetID: str(r[1]), Name: str(r[2]), Gender: domain.Gender(str(r[3])), GroupLabel: str(r[4]), Note: str(r[5]), RuleVersion: int(i64(r[6]))}, Components: components[id], AthleteIDs: athletes[id]})
	}
	return out, nil
}
func (s *Store) CombinedMeet(ctx context.Context, id string) (string, error) {
	rows, err := s.Query(ctx, `SELECT meet_id FROM combined_events WHERE id=?`, id)
	if err != nil || len(rows.Values) == 0 {
		return "", err
	}
	return str(rows.Values[0][0]), nil
}
