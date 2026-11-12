package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

// structure describes a database by what its tables and indexes are, not
// by how the SQL creating them was written: a column added by ALTER TABLE
// and one written in CREATE TABLE compare equal. CHECK constraints are not
// visible this way; keep those in step by reading.
func structure(t *testing.T, db *sqlite.DB) map[string]string {
	t.Helper()
	ctx := context.Background()
	query := func(sql string, args ...any) [][]any {
		rows, err := db.Query(ctx, sql, args...)
		if err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return rows.Values
	}
	lines := func(rows [][]any) string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = fmt.Sprintf("%v", r)
		}
		sort.Strings(out)
		return strings.Join(out, "\n")
	}
	out := map[string]string{}
	for _, r := range query(`SELECT type, name, coalesce(sql, '') FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'`) {
		kind, name, sql := r[0].(string), r[1].(string), r[2].(string)
		if kind != "table" {
			out[kind+" "+name] = strings.Join(strings.Fields(sql), " ")
			continue
		}
		out["table "+name] = lines(query(`SELECT name, type, "notnull", coalesce(dflt_value, ''), pk FROM pragma_table_info(?)`, name))
		out["foreign keys "+name] = lines(query(`SELECT "table", "from", coalesce("to", ''), on_update, on_delete FROM pragma_foreign_key_list(?)`, name))
	}
	return out
}

// schema.sql is the snapshot people read; it must describe what the
// migrations build (docs/agents/storage.md「迁移」).
func TestSchemaSnapshotMatchesMigrations(t *testing.T) {
	ctx := context.Background()
	migrated, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()

	body, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "snapshot.db"), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if _, err := snapshot.Transaction(ctx, splitSQL(string(body))); err != nil {
		t.Fatalf("schema.sql: %v", err)
	}

	got, want := structure(t, snapshot), structure(t, migrated.write)
	if len(want) == 0 {
		t.Fatal("no tables after migrating")
	}
	names := map[string]bool{}
	for k := range got {
		names[k] = true
	}
	for k := range want {
		names[k] = true
	}
	for k := range names {
		if !reflect.DeepEqual(got[k], want[k]) {
			t.Errorf("%s 不一致\nschema.sql:\n%s\n迁移结果:\n%s", k, got[k], want[k])
		}
	}
}

// A database with only migration 1 keeps its actual slots and scores during upgrade.
func TestEventConfigMigrationKeepsExistingData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(dir, FileName), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	init, err := migrationFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	stmts := splitSQL(string(init))
	stmts = append(stmts,
		Stmt(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`),
		Stmt(`INSERT INTO schema_migrations VALUES (1,0)`),
		Stmt(`INSERT INTO meets(id,name,start_date,created_at) VALUES ('m','赛事','2026-11-12',0)`),
		Stmt(`INSERT INTO athletes(id,meet_id,name,created_at) VALUES ('a','m','运动员',0)`),
		Stmt(`INSERT INTO events(id,meet_id,name,gender,discipline,kind,created_at) VALUES ('e','m','男子1500m','男','1500m','track',0)`),
		Stmt(`INSERT INTO rounds(id,event_id,stage,ordinal) VALUES ('r','e','final',1)`),
		Stmt(`INSERT INTO heats(id,round_id,ordinal) VALUES ('h','r',1)`),
		Stmt(`INSERT INTO slots(heat_id,position,athlete_id) VALUES ('h',7,'a')`),
		Stmt(`INSERT INTO results(heat_id,position,status,mark,updated_at) VALUES ('h',7,'OK',300000,0)`),
	)
	if _, err := db.Transaction(ctx, stmts); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, err := s.ListEvents(ctx, Scope{MeetID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].UsesLanes() || events[0].RelayLegCount != 0 {
		t.Fatalf("migration inferred config: %+v", events)
	}
	rounds, err := s.LoadRounds(ctx, Scope{MeetID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	sl := rounds["e"][0].Heats[0].Slots[0]
	if sl.Position != 7 || *sl.Result.Mark != 300000 {
		t.Fatal("migration changed existing result")
	}
}

// A database last opened before migration 6 keeps its heats; rounds start
// unscheduled and heats have no expected duration.
func TestScheduleMigrationKeepsExistingData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Write(ctx,
		Stmt(`INSERT INTO meets(id,name,start_date,created_at) VALUES ('m','赛事','2026-11-12',0)`),
		Stmt(`INSERT INTO events(id,meet_id,name,gender,discipline,kind,created_at) VALUES ('e','m','男子100m','男','100m','track',0)`),
		Stmt(`INSERT INTO rounds(id,event_id,stage,ordinal) VALUES ('r','e','final',1)`),
		Stmt(`INSERT INTO heats(id,round_id,ordinal,scheduled_at) VALUES ('h','r',1,'2026-11-12T09:30')`),
	)
	s.Close()
	if err != nil {
		t.Fatal(err)
	}
	unapply(t, dir)
	if s, err = Open(ctx, dir); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rounds, err := s.LoadRounds(ctx, Scope{MeetID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	r := rounds["e"][0]
	if r.ScheduledAt != "" || r.Heats[0].ScheduledAt != "2026-11-12T09:30" || r.Heats[0].DurationMinutes != nil {
		t.Fatalf("after migration: %+v", r)
	}
	rows, err := s.Query(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_audit_meet'`)
	if err != nil || rows.Values[0][0].(int64) != 1 {
		t.Fatalf("audit index: %v %v", rows, err)
	}
}
