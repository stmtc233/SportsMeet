package store

import (
	"encoding/json"

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
