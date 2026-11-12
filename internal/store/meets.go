package store

import (
	"context"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ListMeets returns meets, newest start date first.
func (s *Store) ListMeets(ctx context.Context) ([]domain.Meet, error) {
	rows, err := s.Query(ctx, `SELECT id, name, start_date, end_date, location, note, created_at
		FROM meets ORDER BY start_date DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Meet, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, domain.Meet{
			ID: str(r[0]), Name: str(r[1]), StartDate: str(r[2]), EndDate: str(r[3]),
			Location: str(r[4]), Note: str(r[5]), CreatedAt: i64(r[6]),
		})
	}
	return out, nil
}

// InsertMeet is the statement creating m.
func InsertMeet(m domain.Meet) sqlite.Statement {
	return Stmt(`INSERT INTO meets(id, name, start_date, end_date, location, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, m.ID, m.Name, m.StartDate, m.EndDate, m.Location, m.Note, m.CreatedAt)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func i64(v any) int64 {
	n, _ := v.(int64)
	return n
}
