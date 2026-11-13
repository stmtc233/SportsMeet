// Package service is the only write entry (AGENTS.md §5.1): the frontend
// bindings and MCP call the same methods, with the same validation, and
// every successful write logs an audit row and notifies listeners.
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Change tells listeners which data moved, so pages reload only that.
type Change struct {
	Kind    string `json:"kind"` // meets | units | athletes | events | entries | heats | results | combined | team_scoring
	MeetID  string `json:"meetId"`
	EventID string `json:"eventId"`
}

// Service wraps the store.
type Service struct {
	store  *store.Store
	notify func(Change)
	// mu serializes writes: a method reads, validates and writes while
	// holding it, so its reads stay current until its batch commits.
	mu sync.Mutex
	// pending is set only on a request-local planner while the real service holds mu.
	pending *preparedWrite
	// zone shows recorded times; nil is the computer's local zone. Tests set it.
	zone *time.Location
}

// New returns a service; notify is called after each committed write.
func New(s *store.Store, notify func(Change)) *Service {
	if notify == nil {
		notify = func(Change) {}
	}
	return &Service{store: s, notify: notify}
}

// write commits stmts plus the audit row, then notifies. A batch that
// fails only because of the audit row is retried without it: the log must
// not block the work it describes (docs/agents/domain.md「操作日志」).
func (s *Service) write(ctx context.Context, a store.Audit, c Change, stmts ...sqlite.Statement) error {
	if s.pending != nil {
		if s.pending.audit.Action != "" {
			return errors.New("service: an operation must prepare one batch")
		}
		s.pending.audit, s.pending.change, s.pending.stmts = a, c, stmts
		return nil
	}
	withLog := append(append([]sqlite.Statement(nil), stmts...), store.InsertAudit(s.store.Now(), a))
	if err := s.store.Write(ctx, withLog...); err != nil {
		if len(stmts) == 0 {
			return err
		}
		if err2 := s.store.Write(ctx, stmts...); err2 != nil {
			return err
		}
		log.Printf("操作日志写入失败(%s):%v", a.Action, err)
	}
	s.notify(c)
	return nil
}

// InputError is a validation failure shown to the user as is.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func invalid(format string, a ...any) error { return &InputError{Msg: fmt.Sprintf(format, a...)} }

// ErrStale means the data changed after the caller loaded it; the page
// reloads and merges instead of overwriting. The text is a code the
// frontend maps to a message (src/lib/errors.ts).
var ErrStale = errors.New("E_STALE")

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// nullable stores "" as NULL, for optional foreign keys.
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
