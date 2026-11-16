package service

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func genderValue(value string) *string { return &value }

func TestGenderIdentityDoesNotChangeCompetition(t *testing.T) {
	f := newFixture(t, 1, track())
	female, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "100m", Kind: "track"})
	must(t, err)
	f.addHeat(0, f.athletes[0])
	before := f.detail()
	heat := before.Rounds[0].Heats[0]
	must(t, f.svc.SaveResults(f.ctx, ui, heat.ID, []ResultInput{{Position: 1, Mark: "12.34"}}))
	before = f.detail()
	options, err := f.svc.AthleteEntryOptions(f.ctx, f.meet, f.athletes[0], "男", "")
	must(t, err)
	for _, identity := range []string{"非二元", "不愿透露", "女", "其他", ""} {
		athlete, err := f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "选手01", Bib: "001", UnitID: f.units[0], GenderIdentity: genderValue(identity)})
		must(t, err)
		if athlete.CompetitionGender != domain.GenderMale || string(athlete.GenderIdentity) != identity {
			t.Fatalf("independence lost: %+v", athlete)
		}
		after := f.detail()
		if !reflect.DeepEqual(before.Rounds, after.Rounds) || !reflect.DeepEqual(before.Entries, after.Entries) {
			t.Fatal("identity changed competition data")
		}
		got, err := f.svc.AthleteEntryOptions(f.ctx, f.meet, athlete.ID, athlete.CompetitionGender, "")
		must(t, err)
		if !reflect.DeepEqual(options, got) {
			t.Fatal("identity changed entry choices")
		}
	}
	// Category edits preserve identity and existing placements/results/entries.
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "选手01", Bib: "001", UnitID: f.units[0], CompetitionGender: genderValue("女子"), GenderIdentity: genderValue("非二元")})
	must(t, err)
	after := f.detail()
	if !reflect.DeepEqual(before.Rounds, after.Rounds) || !after.Entries[0].GenderMismatch {
		t.Fatal("category edit moved competition data or lost warning")
	}
	all, err := f.svc.AthleteEntryOptions(f.ctx, f.meet, f.athletes[0], "男", "", true)
	must(t, err)
	found := false
	for _, option := range all {
		if option.EventID == female.ID {
			found = !option.Matches
		}
	}
	if !found {
		t.Fatal("view-all omitted a nonmatching event")
	}
	// The legacy MCP parameter updates only category and preserves identity.
	athlete, err := f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "选手01", Gender: "男", Bib: "001", UnitID: f.units[0]})
	must(t, err)
	if athlete.GenderIdentity != domain.GenderIdentityNonbinary || athlete.CompetitionGender != domain.GenderMale {
		t.Fatal(athlete)
	}
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, athlete.ID, AthleteInput{Name: athlete.Name, CompetitionGender: genderValue(""), GenderIdentity: genderValue("")})
	must(t, err)
	rows, err := f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if rows[0].Athlete.CompetitionGender != "" || rows[0].Athlete.GenderIdentity != "" {
		t.Fatal("explicit clearing was ignored")
	}
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: "冲突", Gender: "男", CompetitionGender: genderValue("女")})
	wantInvalid(t, err, "不一致")
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, "", AthleteInput{Name: "混合", CompetitionGender: genderValue("混合")})
	wantInvalid(t, err, "参赛类别")
}

func TestGenderIdentityTableCompatibility(t *testing.T) {
	f := newFixture(t, 0, track())
	female, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "100m", Kind: "track"})
	must(t, err)
	tab, err := ParseTable("姓名\t参赛类别\t性别认同\t学号\t报名项目\n甲\t女子\t非二元\tS1\t100m")
	must(t, err)
	p, err := f.svc.ImportAthletes(f.ctx, ui, f.meet, AthleteImport{Mapping: tab.Mapping, Rows: tab.Rows[1:]})
	must(t, err)
	if p.Errors != 0 || p.Entries != 1 || p.Rows[0].CompetitionGenderText != "女子" || p.Rows[0].GenderIdentity != domain.GenderIdentityNonbinary {
		t.Fatal(p)
	}
	rows, err := f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	id := rows[0].Athlete.ID
	detail, err := f.svc.GetEvent(f.ctx, female.ID)
	must(t, err)
	if len(detail.Entries) != 1 || detail.Entries[0].AthleteID != id || detail.Entries[0].GenderMismatch {
		t.Fatal("identity affected event matching")
	}
	// Published old table header remains a competition category, not an identity.
	tab, err = ParseTable("姓名\t性别\t学号\n甲\t男\tS1")
	must(t, err)
	_, err = f.svc.ImportAthletes(f.ctx, ui, f.meet, AthleteImport{Mapping: tab.Mapping, Rows: tab.Rows[1:]})
	must(t, err)
	rows, err = f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if len(rows) != 1 || rows[0].Athlete.CompetitionGender != domain.GenderMale || rows[0].Athlete.GenderIdentity != domain.GenderIdentityNonbinary {
		t.Fatal(rows)
	}
	// Blank and missing identity cells never erase existing personal information.
	for _, input := range []AthleteImport{
		{Mapping: []string{ColName, ColStudentNumber}, Rows: [][]string{{"甲", "S1"}}},
		{Mapping: []string{ColName, ColStudentNumber, ColCompetitionGender, ColGenderIdentity}, Rows: [][]string{{"甲", "S1", "", ""}}},
	} {
		_, err = f.svc.ImportAthletes(f.ctx, ui, f.meet, input)
		must(t, err)
	}
	rows, err = f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if rows[0].Athlete.GenderIdentity != domain.GenderIdentityNonbinary {
		t.Fatal("blank import erased identity")
	}
	p, err = f.svc.PreviewAthleteImport(f.ctx, f.meet, AthleteImport{Mapping: []string{ColName, ColGenderIdentity, ColEvents}, Rows: [][]string{{"乙", "非二元", "100m"}}})
	must(t, err)
	if p.Errors != 1 || !strings.Contains(p.Rows[0].Message, "参赛类别") {
		t.Fatal("identity was used to infer a category", p)
	}
}

func TestGenderIdentityExchangeRoundTripAndReports(t *testing.T) {
	f := newFixture(t, 1, track())
	f.addHeat(0, f.athletes[0])
	before, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "选手01", Bib: "001", UnitID: f.units[0], GenderIdentity: genderValue("不愿透露")})
	must(t, err)
	after, err := f.svc.RenderReport(f.ctx, ReportStartList, f.meet, nil)
	must(t, err)
	if before != after || strings.Contains(after, "不愿透露") {
		t.Fatal("personal identity leaked into print")
	}
	for _, kind := range []string{ReportEntries, ReportStartList, ReportResults, ReportTeamScores, "checkin", "recordsheet"} {
		data, err := f.svc.GetReportData(f.ctx, ReportRequest{Kind: kind, MeetID: f.meet})
		must(t, err)
		if bytes.Contains(data.Data, []byte("genderIdentity")) || bytes.Contains(data.Data, []byte("不愿透露")) {
			t.Fatalf("identity leaked into report data %s", kind)
		}
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	encoded, err := interchange.Encode(file)
	must(t, err)
	var doc struct {
		Version  int              `json:"version"`
		Athletes []map[string]any `json:"athletes"`
	}
	must(t, json.Unmarshal(encoded, &doc))
	if doc.Version != 12 || doc.Athletes[0]["competitionGender"] != "男" || doc.Athletes[0]["genderIdentity"] != "不愿透露" {
		t.Fatal(doc)
	}
	if _, old := doc.Athletes[0]["gender"]; old {
		t.Fatal("new export contains the legacy ambiguous field")
	}
	preview, err := f.svc.PreviewImport(f.ctx, encoded, "")
	must(t, err)
	if len(preview.Errors) > 0 {
		t.Fatal(preview.Errors)
	}
	copyMeet, err := f.svc.ImportMeet(f.ctx, ui, encoded, "", preview.Hash)
	must(t, err)
	rows, err := f.svc.ListAthletes(f.ctx, copyMeet.MeetID)
	must(t, err)
	if len(rows) != 1 || rows[0].Athlete.GenderIdentity != domain.GenderIdentityUndisclosed || rows[0].Athlete.CompetitionGender != domain.GenderMale {
		t.Fatal(rows)
	}
	// Old-file reimport into an existing meet reuses athletes without erasing identity.
	file.Version = 11
	for i := range file.Athletes {
		file.Athletes[i].Gender = file.Athletes[i].CompetitionGender
	}
	legacy, err := interchange.Encode(file)
	must(t, err)
	preview, err = f.svc.PreviewImport(f.ctx, legacy, f.meet)
	must(t, err)
	_, err = f.svc.ImportMeet(f.ctx, ui, legacy, f.meet, preview.Hash)
	must(t, err)
	rows, err = f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	if rows[0].Athlete.GenderIdentity != domain.GenderIdentityUndisclosed {
		t.Fatal("legacy import erased identity")
	}
}
