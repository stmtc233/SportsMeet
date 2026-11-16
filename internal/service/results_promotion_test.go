package service

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func promotionFixture(t *testing.T) *fixture {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByMark: 2}}
	f := newFixture(t, 3, in)
	f.enter(f.athletes...)
	f.addHeat(0, f.athletes...)
	return f
}
func promotionData(t *testing.T, f *fixture) []byte {
	t.Helper()
	d := f.detail()
	b := resultsFile(t, f.meet, d.Rounds[0].Heats[0].ID,
		interchange.HeatResult{Position: 1, AthleteID: f.athletes[0], Status: "OK", Mark: ms(11000)},
		interchange.HeatResult{Position: 2, AthleteID: f.athletes[1], Status: "OK", Mark: ms(12000)},
		interchange.HeatResult{Position: 3, AthleteID: f.athletes[2], Status: "OK", Mark: ms(12000)})
	var file interchange.ResultsFile
	must(t, json.Unmarshal(b, &file))
	file.Version = 2
	file.Promotions = []interchange.ResultsPromotion{{EventID: f.event, SourceRoundID: d.Rounds[0].ID, TargetRoundID: d.Rounds[1].ID, Count: 2,
		Heats: []interchange.PromotionHeat{{HeatID: "external-final", Ordinal: 1, Slots: []interchange.PromotionSlot{{Position: 4, AthleteID: f.athletes[0]}, {Position: 5, AthleteID: f.athletes[1]}, {Position: 3, AthleteID: f.athletes[2]}}}}}}
	b, err := json.Marshal(file)
	must(t, err)
	return b
}
func TestImportPromotionExistingNextRound(t *testing.T) {
	f := promotionFixture(t)
	data := promotionData(t, f)
	d := f.detail()
	target := d.Rounds[1].ID
	for i := 0; i < 2; i++ {
		p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
		must(t, err)
		if len(p.Errors) > 0 {
			t.Fatal(p.Errors)
		}
		_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
		must(t, err)
	}
	d = f.detail()
	if len(d.Rounds) != 2 || d.Rounds[1].ID != target || len(d.Rounds[1].Heats) != 1 || len(d.Rounds[1].Heats[0].Slots) != 3 {
		t.Fatal(d)
	}
	for _, sl := range d.Rounds[0].Heats[0].Slots {
		if sl.Result == nil || sl.Result.Qualify != "q" {
			t.Fatal(sl)
		}
	}
	// Reusing the same target with a different roster is rejected, even without final results.
	var changed interchange.ResultsFile
	must(t, json.Unmarshal(data, &changed))
	changed.Promotions[0].Heats[0].Slots[0].Position = 6
	data, err := json.Marshal(changed)
	must(t, err)
	p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
	must(t, err)
	if !strings.Contains(strings.Join(p.Errors, ""), "已有不同编排") {
		t.Fatal(p)
	}
}
func TestImportPromotionRejectsNonQualifier(t *testing.T) {
	f := promotionFixture(t)
	data := promotionData(t, f)
	var changed interchange.ResultsFile
	must(t, json.Unmarshal(data, &changed))
	changed.Promotions[0].Count = 1
	data, err := json.Marshal(changed)
	must(t, err)
	p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) == 0 {
		t.Fatal(p)
	}
	_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
	wantInvalid(t, err, "文件有误")
	if len(f.detail().Rounds[1].Heats) != 0 {
		t.Fatal("partial import")
	}
}

// Run against both actual implementations, not re-created JSON fixtures.
// SYNCTRACK_SERVER_SOURCE=/path/to/SyncTrack/tools/synctrack_server go test ./internal/service -run TestSyncTrackPromotionRoundTrip -count=1
func TestSyncTrackPromotionRoundTrip(t *testing.T) {
	server := os.Getenv("SYNCTRACK_SERVER_SOURCE")
	if server == "" {
		t.Skip("set SYNCTRACK_SERVER_SOURCE for cross-repository verification")
	}
	f := promotionFixture(t)
	d := f.detail()
	targetID := d.Rounds[1].ID
	_, err := f.svc.SetRoundStart(f.ctx, ui, RoundStart{EventID: f.event, RoundID: targetID, ScheduledAt: "2026-11-12T14:00"})
	must(t, err)
	snapshot, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	raw, err := interchange.Encode(snapshot)
	must(t, err)
	dir := t.TempDir()
	input, output, returned := filepath.Join(dir, "meet.json"), filepath.Join(dir, "results.json"), filepath.Join(dir, "return.json")
	must(t, os.WriteFile(input, raw, 0600))
	run := func(back bool) {
		cmd := exec.Command("go", "test", "./internal/server", "-run", "^TestSportsMeetExternalPromotionContract$", "-count=1")
		cmd.Dir = server
		cmd.Env = append(os.Environ(), "SPORTSMEET_CONTRACT_INPUT="+input, "SPORTSMEET_CONTRACT_OUTPUT="+output)
		if back {
			cmd.Env = append(cmd.Env, "SPORTSMEET_CONTRACT_RETURN="+returned)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("SyncTrack: %v\n%s", err, out)
		}
	}
	run(false)
	data, err := os.ReadFile(output)
	must(t, err)
	for i := 0; i < 2; i++ {
		p, err := f.svc.PreviewResultsImport(f.ctx, data, f.meet)
		must(t, err)
		if len(p.Errors) > 0 {
			t.Fatal(p.Errors)
		}
		_, err = f.svc.ImportResults(f.ctx, ui, data, f.meet, p.Hash)
		must(t, err)
	}
	d = f.detail()
	if len(d.Rounds) != 2 || d.Rounds[1].ID != targetID || len(d.Rounds[1].Heats) != 1 {
		t.Fatal(d)
	}
	if d.Rounds[1].ScheduledAt != "2026-11-12T14:00" || d.Rounds[1].Heats[0].ScheduledAt != "" {
		t.Fatal("next round schedule was not inherited")
	}
	for _, sl := range d.Rounds[1].Heats[0].Slots {
		if sl.Result == nil || sl.Result.Mark == nil {
			t.Fatal("final result missing")
		}
	}
	snapshot, err = f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	raw, err = interchange.Encode(snapshot)
	must(t, err)
	must(t, os.WriteFile(returned, raw, 0600))
	run(true)
}
