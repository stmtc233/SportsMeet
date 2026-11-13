package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

func (s *Store) ListGroups(ctx context.Context, meetID string) ([]domain.CompetitionGroup, error) {
	rows, err := s.Query(ctx, `SELECT id,meet_id,name,ordinal FROM meet_groups WHERE meet_id=? ORDER BY ordinal,rowid`, meetID)
	if err != nil {
		return nil, err
	}
	out := []domain.CompetitionGroup{}
	for _, r := range rows.Values {
		out = append(out, domain.CompetitionGroup{ID: str(r[0]), MeetID: str(r[1]), Name: str(r[2]), Ordinal: int(i64(r[3]))})
	}
	return out, nil
}

// Backfill only explicit project labels. Athlete grades and entries are
// suggestions for later review, never automatic data reassignment.
func (s *Store) migrateGroups(ctx context.Context) ([]sqlite.Statement, error) {
	out := []sqlite.Statement{}
	ids := map[string]string{}
	ordinals := map[string]int{}
	for _, table := range []string{"combined_events", "events"} {
		rows, err := s.write.Query(ctx, `SELECT id,meet_id,group_label FROM `+table+` ORDER BY rowid`)
		if err != nil {
			return nil, err
		}
		for _, r := range rows.Values {
			name, meetID := strings.TrimSpace(str(r[2])), str(r[1])
			key := meetID + "\x00" + domain.NormalizeText(name)
			if name == "" {
				continue
			}
			id := ids[key]
			if id == "" {
				sum := sha256.Sum256([]byte(key))
				id = fmt.Sprintf("group-%x", sum[:16])
				ids[key] = id
				ordinals[meetID]++
				out = append(out, Stmt(`INSERT INTO meet_groups(id,meet_id,name,name_key,ordinal) VALUES (?,?,?,?,?)`, id, meetID, name, domain.NormalizeText(name), ordinals[meetID]))
			}
			out = append(out, Stmt(`UPDATE `+table+` SET group_id=? WHERE id=?`, id, str(r[0])))
		}
	}
	return out, nil
}
