package store

import (
	"context"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestOpenMigratesAndReopens(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	m := domain.Meet{ID: "m1", Name: "秋季运动会", StartDate: "2026-11-12", CreatedAt: 1}
	if err := s.Write(ctx, InsertMeet(m), InsertAudit(1, Audit{Source: SourceUI, Action: "meet.created", MeetID: "m1"})); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(ctx, dir) // second open must skip applied migrations
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	meets, err := s.ListMeets(ctx)
	if err != nil || len(meets) != 1 || meets[0].Name != "秋季运动会" {
		t.Fatalf("ListMeets = %+v, %v", meets, err)
	}
}

func TestForeignKeysCascade(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.Write(ctx,
		InsertMeet(domain.Meet{ID: "m1", Name: "x", StartDate: "2026-11-12"}),
		Stmt(`INSERT INTO events(id, meet_id, name, gender, discipline, kind, created_at) VALUES ('e1','m1','男子100m','男','100m','track',0)`),
		Stmt(`DELETE FROM meets WHERE id = 'm1'`),
	)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.Query(ctx, "SELECT count(*) FROM events")
	if err != nil || rows.Values[0][0].(int64) != 0 {
		t.Fatalf("events left after meet delete: %v, %v", rows.Values, err)
	}
	if err := s.Write(ctx, Stmt(`INSERT INTO units(id, meet_id, name) VALUES ('u1', 'missing', 'x')`)); err == nil {
		t.Fatal("insert with dangling meet_id succeeded; foreign keys are off")
	}
}
