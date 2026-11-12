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
