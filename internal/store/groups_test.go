package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

func TestGroupMigrationPreservesRecordsAndBacksUpV9(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	rollback(t, dir, identityRollback()[:9])
	db, err := sqlite.Open(ctx, filepath.Join(dir, FileName), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stmts := []sqlite.Statement{
		Stmt(`INSERT INTO meets(id,name,start_date,created_at) VALUES ('m','校运会','2026-11-12',1),('m2','第二场','2026-11-12',2)`),
		Stmt(`INSERT INTO units(id,meet_id,name) VALUES ('u','m','初一1班')`),
		Stmt(`INSERT INTO athletes(id,meet_id,name,unit_id,grade,created_at) VALUES ('a','m','小明','u','初一',1)`),
		Stmt(`INSERT INTO events(id,meet_id,name,gender,discipline,group_label,kind,created_at) VALUES ('e','m','100m','男','100m','初一','track',1),('e2','m','跳远','男','跳远',' 初一 ','distance',2),('e3','m','200m','男','200m','','track',3),('e4','m2','100m','男','100m','初一','track',1)`),
		Stmt(`INSERT INTO entries(event_id,athlete_id) VALUES ('e','a')`),
		Stmt(`INSERT INTO rounds(id,event_id,stage,ordinal) VALUES ('r','e','final',1)`),
		Stmt(`INSERT INTO heats(id,round_id,ordinal,scheduled_at,started_at) VALUES ('h','r',1,'2026-11-12T09:00','2026-11-12T09:02')`),
		Stmt(`INSERT INTO slots(heat_id,position,athlete_id) VALUES ('h',1,'a')`),
		Stmt(`INSERT INTO results(heat_id,position,status,mark,rank,updated_at) VALUES ('h',1,'OK',12000,1,1)`),
	}
	if _, err = db.Transaction(ctx, stmts); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	groups, err := s.ListGroups(ctx, "m")
	if err != nil || len(groups) != 1 || groups[0].Name != "初一" {
		t.Fatal(groups, err)
	}
	other, err := s.ListGroups(ctx, "m2")
	if err != nil || len(other) != 1 || other[0].ID == groups[0].ID {
		t.Fatal("cross-meet group collision", other, err)
	}
	rows, err := s.Query(ctx, `SELECT (SELECT group_id FROM events WHERE id='e'),(SELECT group_id FROM events WHERE id='e2'),(SELECT group_id FROM events WHERE id='e3'),(SELECT group_id FROM athletes WHERE id='a'),(SELECT count(*) FROM unit_groups),(SELECT mark FROM results WHERE heat_id='h'),(SELECT started_at FROM heats WHERE id='h')`)
	if err != nil {
		t.Fatal(err)
	}
	r := rows.Values[0]
	if r[0] != groups[0].ID || r[1] != r[0] || r[2] != nil || r[3] != nil || i64(r[4]) != 0 || i64(r[5]) != 12000 || r[6] != "2026-11-12T09:02" {
		t.Fatal("migration changed data", r)
	}
	copies := backups(t, dir)
	if len(copies) != 1 || filepath.Base(copies[0])[len("sportsmeet-20060102-150405"):] != "-v0009.db" {
		t.Fatal("missing v9 backup", copies)
	}
	rows, err = s.Query(ctx, `PRAGMA foreign_key_check`)
	if err != nil || len(rows.Values) > 0 {
		t.Fatal(rows, err)
	}
}
