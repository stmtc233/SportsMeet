// Package store owns the SQLite file (docs/agents/storage.md): one write
// connection, one read-only connection, and numbered migrations.
package store

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// FileName is the database file inside the data directory.
const FileName = "sportsmeet.db"

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is the app's database. Writes go through Write only; the service
// layer serializes them, so reads it makes before a Write see the data the
// Write will change (mygo's Transaction takes a batch, not a callback).
type Store struct {
	write *sqlite.DB
	read  *sqlite.DB
	Now   func() int64 // ms; tests replace it
}

// Open opens (creating if needed) the database in dir and migrates it.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, FileName)
	w, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := w.Execute(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		w.Close()
		return nil, err
	}
	s := &Store{write: w, Now: func() int64 { return time.Now().UnixMilli() }}
	if err := s.migrate(ctx); err != nil {
		w.Close()
		return nil, fmt.Errorf("迁移数据库失败:%w", err)
	}
	r, err := sqlite.Open(ctx, path, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		w.Close()
		return nil, err
	}
	s.read = r
	return s, nil
}

// Close closes both connections.
func (s *Store) Close() error {
	var err error
	if s.read != nil {
		err = s.read.Close()
	}
	if e := s.write.Close(); e != nil {
		err = e
	}
	return err
}

// Write runs statements atomically on the write connection.
func (s *Store) Write(ctx context.Context, stmts ...sqlite.Statement) error {
	_, err := s.write.Transaction(ctx, stmts)
	return err
}

// Query reads on the read-only connection.
func (s *Store) Query(ctx context.Context, sql string, args ...any) (sqlite.Rows, error) {
	return s.read.Query(ctx, sql, args...)
}

// Stmt is shorthand for a statement.
func Stmt(sql string, args ...any) sqlite.Statement { return sqlite.Statement{SQL: sql, Args: args} }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.write.Execute(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	rows, err := s.write.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	done := map[int64]bool{}
	for _, r := range rows.Values {
		done[r[0].(int64)] = true
	}
	names, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name() < names[j].Name() })
	for _, n := range names {
		num, _, _ := strings.Cut(n.Name(), "_")
		v, err := strconv.ParseInt(num, 10, 64)
		if err != nil {
			return fmt.Errorf("迁移文件名应以序号开头:%s", n.Name())
		}
		if done[v] {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + n.Name())
		if err != nil {
			return err
		}
		stmts := splitSQL(string(body))
		stmts = append(stmts, Stmt("INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", v, s.Now()))
		if _, err := s.write.Transaction(ctx, stmts); err != nil {
			return fmt.Errorf("%s:%w", n.Name(), err)
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
