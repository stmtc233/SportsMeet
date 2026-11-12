package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Source of a write, recorded in the operation log.
type Source string

const (
	SourceUI     Source = "ui"
	SourceMCP    Source = "mcp"
	SourceSystem Source = "system"
)

// Audit is one operation-log row.
type Audit struct {
	Source  Source
	Action  string // e.g. meet.created
	MeetID  string
	EventID string
	HeatID  string
	Message string
	Detail  any // JSON-encodable, nil = {}
}

// InsertAudit is the statement logging a; it rides in the same
// transaction as the write it describes.
func InsertAudit(ts int64, a Audit) sqlite.Statement {
	detail := "{}"
	if a.Detail != nil {
		if b, err := json.Marshal(a.Detail); err == nil {
			detail = string(b)
		}
	}
	return Stmt(`INSERT INTO audit_log(ts, source, action, meet_id, event_id, heat_id, message, detail_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ts, string(a.Source), a.Action, a.MeetID, a.EventID, a.HeatID, a.Message, detail)
}

// AuditRow is one stored operation-log row, without its JSON detail.
type AuditRow struct {
	ID      int64
	At      int64 // ms
	Source  Source
	Action  string
	MeetID  string
	EventID string
	HeatID  string
	Message string
}

// AuditFilter selects log rows: "" fields match everything. Rows come
// newest first; BeforeAt / BeforeID (both 0 = from the newest) continue
// after the last row of a previous page.
type AuditFilter struct {
	MeetID   string
	Source   Source
	BeforeAt int64
	BeforeID int64
	Limit    int
}

// ListAudit reads one page of the log in a single query, newest first
// (time, then insertion order).
func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]AuditRow, error) {
	where, args := []string{}, []any{}
	if f.MeetID != "" {
		where, args = append(where, "meet_id = ?"), append(args, f.MeetID)
	}
	if f.Source != "" {
		where, args = append(where, "source = ?"), append(args, string(f.Source))
	}
	if f.BeforeAt != 0 || f.BeforeID != 0 {
		where, args = append(where, "(ts < ? OR (ts = ? AND id < ?))"), append(args, f.BeforeAt, f.BeforeAt, f.BeforeID)
	}
	q := `SELECT id, ts, source, action, meet_id, event_id, heat_id, message FROM audit_log`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := s.Query(ctx, q+" ORDER BY ts DESC, id DESC LIMIT ?", append(args, f.Limit)...)
	if err != nil {
		return nil, err
	}
	out := make([]AuditRow, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, AuditRow{ID: i64(r[0]), At: i64(r[1]), Source: Source(str(r[2])), Action: str(r[3]),
			MeetID: str(r[4]), EventID: str(r[5]), HeatID: str(r[6]), Message: str(r[7])})
	}
	return out, nil
}
