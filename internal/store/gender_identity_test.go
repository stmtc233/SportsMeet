package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestAthleteGenderMigrationPreservesCompetitionAndBackup(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	rollback(t, dir, genderRollback())
	legacy, err := sqlite.Open(ctx, filepath.Join(dir, FileName), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Transaction(ctx, []sqlite.Statement{
		Stmt(`INSERT INTO meets(id,name,start_date,created_at) VALUES ('m','校运会','2026-11-16',1)`),
		Stmt(`INSERT INTO athletes(id,meet_id,name,gender,created_at) VALUES ('a','m','甲','女',1),('b','m','乙','',2)`),
		Stmt(`INSERT INTO events(id,meet_id,name,gender,discipline,kind,created_at) VALUES ('e','m','女子100m','女','100m','track',1)`),
		Stmt(`INSERT INTO entries(event_id,athlete_id) VALUES ('e','a')`),
		Stmt(`INSERT INTO rounds(id,event_id,stage,ordinal) VALUES ('r','e','final',1)`),
		Stmt(`INSERT INTO heats(id,round_id,ordinal,scheduled_at,started_at) VALUES ('h','r',1,'2026-11-16T09:00','2026-11-16T09:01')`),
		Stmt(`INSERT INTO slots(heat_id,position,athlete_id) VALUES ('h',1,'a')`),
		Stmt(`INSERT INTO results(heat_id,position,status,mark,rank,updated_at) VALUES ('h',1,'OK',12340,1,1)`),
	})
	legacy.Close()
	if err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	athletes, err := db.ListAthletes(ctx, "m")
	if err != nil || len(athletes) != 2 || athletes[0].CompetitionGender != domain.GenderFemale || athletes[1].CompetitionGender != "" || athletes[0].GenderIdentity != "" || athletes[1].GenderIdentity != "" {
		t.Fatal(athletes, err)
	}
	rounds, err := db.LoadRounds(ctx, Scope{MeetID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	heat := rounds["e"][0].Heats[0]
	if heat.ScheduledAt != "2026-11-16T09:00" || heat.StartedAt != "2026-11-16T09:01" || heat.Slots[0].AthleteID != "a" || *heat.Slots[0].Result.Mark != 12340 || *heat.Slots[0].Result.Rank != 1 {
		t.Fatal("competition changed", heat)
	}
	entries, err := db.ListEntries(ctx, Scope{MeetID: "m"})
	if err != nil || len(entries) != 1 || entries[0].AthleteID != "a" {
		t.Fatal(entries, err)
	}
	copies := backups(t, dir)
	if len(copies) != 1 || !strings.HasSuffix(copies[0], "-v0014.db") {
		t.Fatal("missing pre-migration backup", copies)
	}
	backup, err := sqlite.Open(ctx, copies[0], sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	rows, err := backup.Query(ctx, `SELECT gender FROM athletes WHERE id='a'`)
	if err != nil || rows.Values[0][0] != "女" {
		t.Fatal("backup lost original data", rows, err)
	}
}
