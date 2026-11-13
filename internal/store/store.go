// Package store owns the SQLite file (docs/agents/storage.md): one write
// connection, a bounded read-only pool, and numbered migrations.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

// FileName is the database file inside the data directory.
const FileName = "sportsmeet.db"

// BackupDir holds the copies made before migrating, inside the data
// directory; the newest backupKeep are kept.
const (
	BackupDir  = "backups"
	backupKeep = 5
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is the app's database. Writes go through Write only; the service
// layer serializes them, so reads it makes before a Write see the data the
// Write will change (mygo's Transaction takes a batch, not a callback).
type Store struct {
	write     *sqlite.DB
	readers   []*sqlite.DB
	available chan *sqlite.DB
	closed    chan struct{}
	closeOnce sync.Once
	closeErr  error
	Now       func() int64 // ms; tests replace it
}

// Open opens (creating if needed) the database in dir and migrates it.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, FileName)
	open := func() (*sqlite.DB, error) {
		w, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
		if err != nil {
			return nil, err
		}
		if _, err := w.Execute(ctx, "PRAGMA journal_mode = WAL"); err != nil {
			w.Close()
			return nil, err
		}
		return w, nil
	}
	w, err := open()
	if err != nil {
		return nil, err
	}
	s := &Store{write: w, Now: func() int64 { return time.Now().UnixMilli() }}
	names, newest, err := pending(ctx, w)
	if err != nil {
		w.Close()
		return nil, err
	}
	if len(names) > 0 && newest > 0 { // a new database has nothing to keep
		if err := w.Close(); err != nil {
			return nil, err
		}
		if err := backup(dir, s.Now(), newest); err != nil {
			return nil, fmt.Errorf("备份失败:%w", err)
		}
		if w, err = open(); err != nil {
			return nil, err
		}
		s.write = w
	}
	if err := s.migrate(ctx, names); err != nil {
		w.Close()
		return nil, fmt.Errorf("迁移数据库失败:%w", err)
	}
	// A bounded pool prevents a list query from holding every validation read
	// behind it. Writes remain serialized by service and one write connection.
	s.available, s.closed = make(chan *sqlite.DB, 4), make(chan struct{})
	for range cap(s.available) {
		r, err := sqlite.Open(ctx, path, sqlite.OpenOptions{ReadOnly: true})
		if err != nil {
			s.Close()
			return nil, err
		}
		s.readers = append(s.readers, r)
		s.available <- r
	}
	return s, nil
}

// Close interrupts active queries and releases every connection exactly once.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		for _, r := range s.readers {
			s.closeErr = errors.Join(s.closeErr, r.Close())
		}
		s.closeErr = errors.Join(s.closeErr, s.write.Close())
	})
	return s.closeErr
}

// Write runs statements atomically on the write connection.
func (s *Store) Write(ctx context.Context, stmts ...sqlite.Statement) error {
	_, err := s.write.Transaction(ctx, stmts)
	return err
}

// Query leases a read-only connection. Waiting callers can cancel or close.
func (s *Store) Query(ctx context.Context, sql string, args ...any) (sqlite.Rows, error) {
	select {
	case <-ctx.Done():
		return sqlite.Rows{}, ctx.Err()
	case <-s.closed:
		return sqlite.Rows{}, sqlite.ErrClosed
	case r := <-s.available:
		defer func() { s.available <- r }()
		return r.Query(ctx, sql, args...)
	}
}

// Stmt is shorthand for a statement.
func Stmt(sql string, args ...any) sqlite.Statement { return sqlite.Statement{SQL: sql, Args: args} }

// backup copies the database file as it is before migrations change
// it, so a migration that goes wrong on someone's data can be undone by
// putting the file back. version is the newest migration already applied.
// No connection may be open: closing the last one moves the write-ahead
// log into the file, which makes the file alone a complete copy (the
// plugin rejects VACUUM INTO).
func backup(dir string, now, version int64) error {
	path := filepath.Join(dir, FileName)
	if wal, err := os.Stat(path + "-wal"); err == nil && wal.Size() > 0 {
		return errors.New("数据库正被另一个程序使用")
	}
	backups := filepath.Join(dir, BackupDir)
	if err := os.MkdirAll(backups, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("sportsmeet-%s-v%04d.db", time.UnixMilli(now).Format("20060102-150405"), version)
	if err := os.WriteFile(filepath.Join(backups, name), data, 0o644); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(backups, "sportsmeet-*.db"))
	if err != nil {
		return err
	}
	sort.Strings(old) // names start with the time, so oldest first
	for len(old) > backupKeep {
		_ = os.Remove(old[0])
		old = old[1:]
	}
	return nil
}

// pending returns the migration files not yet applied, in order, and the
// newest version that is (0 for a new database).
func pending(ctx context.Context, db *sqlite.DB) (names []string, newest int64, err error) {
	if _, err := db.Execute(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, 0, err
	}
	done := map[int64]bool{}
	for _, r := range rows.Values {
		v := r[0].(int64)
		done[v] = true
		newest = max(newest, v)
	}
	files, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, 0, err
	}
	for _, f := range files {
		v, err := migrationVersion(f.Name())
		if err != nil {
			return nil, 0, err
		}
		if !done[v] {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	return names, newest, nil
}

func migrationVersion(name string) (int64, error) {
	num, _, _ := strings.Cut(name, "_")
	v, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("迁移文件名应以序号开头:%s", name)
	}
	return v, nil
}

// migrate applies the named migration files, each in its own transaction.
func (s *Store) migrate(ctx context.Context, names []string) error {
	for _, name := range names {
		v, err := migrationVersion(name)
		if err != nil {
			return err
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		stmts := splitSQL(string(body))
		if v == 10 {
			backfill, err := s.migrateGroups(ctx)
			if err != nil {
				return err
			}
			stmts = append(stmts, backfill...)
		}
		if v == 8 {
			// SQLite lower/trim does not implement domain's Unicode/whitespace
			// normalization. Backfill in Go inside this migration's one batch.
			rows, err := s.write.Query(ctx, `SELECT id, bib, student_number, name FROM athletes`)
			if err != nil {
				return err
			}
			for _, r := range rows.Values {
				stmts = append(stmts, Stmt(`UPDATE athletes SET bib_key=?, student_number_key=?, name_key=? WHERE id=?`,
					domain.NormalizeText(str(r[1])), domain.NormalizeText(str(r[2])), domain.NormalizeText(str(r[3])), str(r[0])))
			}
		}
		stmts = append(stmts, Stmt("INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", v, s.Now()))
		if _, err := s.write.Transaction(ctx, stmts); err != nil {
			return fmt.Errorf("%s:%w", name, err)
		}
	}
	return nil
}

// splitSQL splits a migration into statements at semicolons ending a line,
// dropping comment lines. Migrations must not use triggers or multi-line
// string literals containing such semicolons.
func splitSQL(body string) []sqlite.Statement {
	var out []sqlite.Statement
	var cur strings.Builder
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
		if strings.HasSuffix(t, ";") {
			out = append(out, Stmt(strings.TrimSpace(cur.String())))
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, Stmt(s))
	}
	return out
}
