package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

func backups(t *testing.T, dir string) []string {
	t.Helper()
	list, err := filepath.Glob(filepath.Join(dir, BackupDir, "*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// unapply makes the database look like one last opened before migration
// 0006: the rows of 0006 and 0007 are gone and so are the columns and index
// they add.
func unapply(t *testing.T, dir string) {
	t.Helper()
	rollback(t, dir, append(startedRollback(),
		Stmt("DELETE FROM schema_migrations WHERE version = 6"),
		Stmt("DROP INDEX idx_audit_meet"),
		Stmt("ALTER TABLE rounds DROP COLUMN scheduled_at"), Stmt("ALTER TABLE heats DROP COLUMN duration_minutes"),
	))
}

// unapplyStarted makes the database look like one last opened before
// migration 0007.
func unapplyStarted(t *testing.T, dir string) {
	t.Helper()
	rollback(t, dir, startedRollback())
}

func startedRollback() []sqlite.Statement {
	return append(identityRollback(), Stmt("DELETE FROM schema_migrations WHERE version = 7"), Stmt("ALTER TABLE heats DROP COLUMN started_at"))
}

func identityRollback() []sqlite.Statement {
	return []sqlite.Statement{
		Stmt("DELETE FROM schema_migrations WHERE version = 8"),
		Stmt("DROP INDEX idx_athletes_bib_key"), Stmt("DROP INDEX idx_athletes_student_key"), Stmt("DROP INDEX idx_athletes_name_key"),
		Stmt("DROP INDEX idx_entries_event"),
		Stmt("ALTER TABLE athletes DROP COLUMN bib_key"), Stmt("ALTER TABLE athletes DROP COLUMN student_number_key"), Stmt("ALTER TABLE athletes DROP COLUMN name_key"),
	}
}

func rollback(t *testing.T, dir string, stmts []sqlite.Statement) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(dir, FileName), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Transaction(ctx, stmts); err != nil {
		t.Fatal(err)
	}
}

func TestBackupBeforeMigrating(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Write(ctx, InsertMeet(domain.Meet{ID: "m1", Name: "秋季运动会", StartDate: "2026-11-12"})); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("a new database was backed up: %v", got)
	}

	s, err = Open(ctx, dir) // nothing to migrate
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("backed up with no migration pending: %v", got)
	}

	unapply(t, dir)
	// Older copies: only the newest backupKeep files stay.
	if err := os.MkdirAll(filepath.Join(dir, BackupDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < backupKeep+2; i++ {
		name := filepath.Join(dir, BackupDir, fmt.Sprintf("sportsmeet-2020010%d-000000-v0001.db", i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got := backups(t, dir)
	if len(got) != backupKeep {
		t.Fatalf("kept %d backups, want %d: %v", len(got), backupKeep, got)
	}
	newest := got[len(got)-1]
	if filepath.Base(newest)[len("sportsmeet-20060102-150405"):] != "-v0005.db" {
		t.Fatalf("backup name = %s, want the version before migrating (v0005)", filepath.Base(newest))
	}
	copyDB, err := sqlite.Open(ctx, newest, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	rows, err := copyDB.Query(ctx, "SELECT (SELECT name FROM meets WHERE id = 'm1'), (SELECT count(*) FROM schema_migrations WHERE version = 6)")
	if err != nil || rows.Values[0][0] != "秋季运动会" || rows.Values[0][1].(int64) != 0 {
		t.Fatalf("backup content = %v, %v; want the data as it was before migration 6", rows.Values, err)
	}
	rows, err = s.Query(ctx, "SELECT count(*) FROM schema_migrations WHERE version = 6")
	if err != nil || rows.Values[0][0].(int64) != 1 {
		t.Fatalf("migration 6 not applied after the backup: %v, %v", rows.Values, err)
	}
}

func TestMigrationStopsWhenBackupFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	unapply(t, dir)
	// A file where the backup directory should be.
	if err := os.WriteFile(filepath.Join(dir, BackupDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err = Open(ctx, dir); err == nil {
		s.Close()
		t.Fatal("opened without a backup")
	}
	db, err := sqlite.Open(ctx, filepath.Join(dir, FileName), sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(ctx, "SELECT count(*) FROM schema_migrations WHERE version = 6")
	if err != nil || rows.Values[0][0].(int64) != 0 {
		t.Fatalf("migrated without a backup: %v, %v", rows.Values, err)
	}
}
