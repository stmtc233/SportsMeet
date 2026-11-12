package store

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ListTeams loads teams and their ordered members in two queries for the whole scope.
func (s *Store) ListTeams(ctx context.Context, sc Scope) ([]domain.Team, error) {
	cond, arg := sc.events("event_id")
	rows, err := s.Query(ctx, `SELECT id, event_id, name, unit_id FROM teams WHERE `+cond+` ORDER BY rowid`, arg)
	if err != nil {
		return nil, err
	}
	members, err := s.Query(ctx, `SELECT team_id, athlete_id FROM team_members WHERE team_id IN (SELECT id FROM teams WHERE `+cond+`) ORDER BY leg`, arg)
	if err != nil {
		return nil, err
	}
	byTeam := map[string][]string{}
	for _, r := range members.Values {
		byTeam[str(r[0])] = append(byTeam[str(r[0])], str(r[1]))
	}
	out := make([]domain.Team, 0, len(rows.Values))
	for _, r := range rows.Values {
		ids := byTeam[str(r[0])]
		if ids == nil {
			ids = []string{}
		}
		out = append(out, domain.Team{ID: str(r[0]), EventID: str(r[1]), Name: str(r[2]), UnitID: str(r[3]), MemberIDs: ids})
	}
	return out, nil
}
