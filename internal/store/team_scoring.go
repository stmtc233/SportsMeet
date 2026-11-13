package store

import (
	"context"
	"encoding/json"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func (s *Store) TeamScoringRules(ctx context.Context, meetID string) (domain.TeamScoringRules, error) {
	r := domain.DefaultTeamScoringRules()
	rows, err := s.Query(ctx, `SELECT rules_json FROM meet_team_scoring WHERE meet_id=?`, meetID)
	if err != nil {
		return r, err
	}
	if len(rows.Values) > 0 {
		err = json.Unmarshal([]byte(str(rows.Values[0][0])), &r)
	}
	return r, err
}
