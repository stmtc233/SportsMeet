package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestReadPoolCancellationAndClose(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	// Exhaust every lease so Query must wait outside the native connection.
	var held []*sqlite.DB
	for range cap(s.available) {
		held = append(held, <-s.available)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Query(canceled, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	for _, db := range held {
		if _, err := db.Execute(ctx, "CREATE TABLE forbidden(id INTEGER)"); err == nil {
			t.Fatal("pool connection is writable")
		}
	}
	result := make(chan error, 1)
	go func() { _, err := s.Query(ctx, "SELECT 1"); result <- err }()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, sqlite.ErrClosed) {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("closing did not wake a waiting query")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEventEntryLookupUsesCompleteIndex(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Query(ctx, `EXPLAIN QUERY PLAN SELECT athlete_id FROM entries WHERE event_id=? ORDER BY rowid`, "event")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows.Values {
		if strings.Contains(str(row[len(row)-1]), "idx_entries_event") {
			return
		}
	}
	t.Fatalf("event entry lookup does not use its full index: %v", rows.Values)
}

func TestIdentityKeyMigrationUsesDomainNormalization(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Write(ctx, InsertMeet(domain.Meet{ID: "m", Name: "赛事", StartDate: "2026-11-12"})); err != nil {
		t.Fatal(err)
	}
	s.Close()
	rollback(t, dir, append(identityRollback(),
		Stmt(`INSERT INTO athletes(id,meet_id,name,bib,student_number,created_at) VALUES ('a','m',?,?,?,0)`, "名\t 字", " ÄB  12 ", " S\t  01 "),
	))
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	matches, err := s.AthleteIdentityMatches(ctx, domain.Athlete{MeetID: "m", Bib: "äb 12"})
	if err != nil || len(matches) != 1 || matches[0].Bib != " ÄB  12 " {
		t.Fatalf("bib normalization or source preservation: %+v %v", matches, err)
	}
	matches, err = s.AthleteIdentityMatches(ctx, domain.Athlete{MeetID: "m", StudentNumber: "s 01"})
	if err != nil || len(matches) != 1 {
		t.Fatalf("student normalization: %+v %v", matches, err)
	}
	exists, err := s.AthleteNameExists(ctx, "m", "", "名 字")
	if err != nil || !exists {
		t.Fatalf("name normalization: %t %v", exists, err)
	}
	// The indexes must permit final-state-valid identity swaps in one batch.
	if err := s.Write(ctx,
		Stmt(`INSERT INTO athletes(id,meet_id,name,bib_key,created_at) VALUES ('b','m','乙','another',1)`),
		Stmt(`UPDATE athletes SET bib_key='another' WHERE id='a'`),
		Stmt(`UPDATE athletes SET bib_key='äb 12' WHERE id='b'`),
	); err != nil {
		t.Fatalf("identity indexes must be nonunique: %v", err)
	}
}
