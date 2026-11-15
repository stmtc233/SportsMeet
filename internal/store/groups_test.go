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
	rollback(t, dir, identityRollback()[:len(limitsRollback())+9])
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

func TestAthleteUnitGroupMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	// u has one group and athletes of none; v lacks the group of one of its
	// athletes; w has two groups; x none; the athlete without a unit stays.
	if err := s.Write(ctx,
		Stmt(`INSERT INTO meets(id,name,start_date,created_at) VALUES ('m','校运会','2026-11-12',1)`),
		Stmt(`INSERT INTO meet_groups(id,meet_id,name,name_key,ordinal) VALUES ('g1','m','初一','初一',1),('g2','m','初二','初二',2)`),
		Stmt(`INSERT INTO units(id,meet_id,name) VALUES ('u','m','一中'),('v','m','二中'),('w','m','三中'),('x','m','四中')`),
		Stmt(`INSERT INTO unit_groups(unit_id,group_id) VALUES ('u','g1'),('v','g1'),('w','g1'),('w','g2')`),
		Stmt(`INSERT INTO athletes(id,meet_id,name,unit_id,group_id,created_at) VALUES ('a1','m','甲','u',NULL,1),('a2','m','乙','v','g2',2),('a3','m','丙','v',NULL,3),('a4','m','丁','w',NULL,4),('a5','m','戊','x',NULL,5),('a6','m','己',NULL,'g2',6)`),
	); err != nil {
		t.Fatal(err)
	}
	s.Close()
	rollback(t, dir, followRollback())
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Query(ctx, `SELECT id, group_id FROM athletes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{"g1", "g2", nil, nil, nil, "g2"}
	for i, r := range rows.Values {
		if r[1] != want[i] {
			t.Fatalf("athlete %v group = %v, want %v", r[0], r[1], want[i])
		}
	}
	rows, err = s.Query(ctx, `SELECT unit_id || ':' || group_id FROM unit_groups ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	got := []any{}
	for _, r := range rows.Values {
		got = append(got, r[0])
	}
	if len(got) != 5 || got[1] != "v:g1" || got[2] != "v:g2" {
		t.Fatal("unit groups", got)
	}
}
