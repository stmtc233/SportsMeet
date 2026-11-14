package store

import (
	"context"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ListMeets returns meets, newest start date first.
func (s *Store) ListMeets(ctx context.Context) ([]domain.Meet, error) {
	rows, err := s.Query(ctx, `SELECT `+meetColumns+` FROM meets ORDER BY start_date DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Meet, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, scanMeet(r))
	}
	return out, nil
}

const meetColumns = `id, name, start_date, end_date, location, note, created_at,
	limit_athlete_events, limit_unit_athletes, limit_unit_teams`

func scanMeet(r []any) domain.Meet {
	return domain.Meet{
		ID: str(r[0]), Name: str(r[1]), StartDate: str(r[2]), EndDate: str(r[3]),
		Location: str(r[4]), Note: str(r[5]), CreatedAt: i64(r[6]),
		EntryLimits: domain.EntryLimits{AthleteEvents: int(i64(r[7])), UnitAthletes: int(i64(r[8])), UnitTeams: int(i64(r[9]))},
	}
}

// InsertMeet is the statement creating m.
func InsertMeet(m domain.Meet) sqlite.Statement {
	l := m.EntryLimits
	return Stmt(`INSERT INTO meets(id, name, start_date, end_date, location, note, created_at,
		limit_athlete_events, limit_unit_athletes, limit_unit_teams)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, m.ID, m.Name, m.StartDate, m.EndDate, m.Location, m.Note, m.CreatedAt,
		l.AthleteEvents, l.UnitAthletes, l.UnitTeams)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func i64(v any) int64 {
	n, _ := v.(int64)
	return n
}
