package service

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func TestParseTable(t *testing.T) {
	tab, err := ParseTable("\xef\xbb\xbf姓名\t性别\t学号\t班级\t报名项目\n张三\t男\t1001\t一班\t100m、跳远\n\n李四\t女\t1002\t二班\t\n")
	must(t, err)
	if !tab.HasHeader || len(tab.Rows) != 3 || len(tab.Rows[2]) != 5 {
		t.Fatalf("table = %+v", tab)
	}
	if want := []string{ColName, ColGender, ColStudentNumber, ColClassName, ColEvents}; !reflect.DeepEqual(tab.Mapping, want) {
		t.Fatalf("mapping = %v", tab.Mapping)
	}
	// Empty cells copied from a spreadsheet keep their columns.
	tab, err = ParseTable("姓名\t学号\t参赛单位\t参赛组别\n张三\t\t高一(2)班\t高一\n李四\t\t\t高二\n")
	must(t, err)
	if want := [][]string{{"张三", "", "高一(2)班", "高一"}, {"李四", "", "", "高二"}}; !reflect.DeepEqual(tab.Rows[1:], want) {
		t.Fatalf("empty cells = %q", tab.Rows[1:])
	}
	tab, err = ParseTable("王五, 1002\n赵六,1003,多一列")
	must(t, err)
	if tab.HasHeader || len(tab.Rows) != 2 || len(tab.Rows[0]) != 3 || tab.Mapping[0] != "" {
		t.Fatalf("csv without header = %+v", tab)
	}
}

func TestImportAthletes(t *testing.T) {
	f := newFixture(t, 2, track()) // 选手01 bib 001 一班, 选手02 bib 002 二班
	_, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", EventInput{Gender: "女", Discipline: "100m", Kind: "track"})
	must(t, err)
	mapping := []string{ColName, ColGender, ColBib, ColUnit, ColEvents}
	in := AthleteImport{Mapping: mapping, Rows: [][]string{
		{"张三", "男", "101", "三班", "100 米"},
		{"李四", "女", "102", "一班", "女子100m"},
		{"选手一", "", "001", "", "男子100m"}, // bib 001 exists: update
		{"", "男", "103", "", ""},
		{"王五", "男", "101", "", ""},
		{"赵六", "男", "104", "", "铅球"},
		{"选手01", "男", "", "", ""},
	}}
	p, err := f.svc.PreviewAthleteImport(f.ctx, f.meet, in)
	must(t, err)
	if p.Creates != 3 || p.Updates != 1 || p.Errors != 3 || p.Entries != 3 || !reflect.DeepEqual(p.NewUnits, []string{"三班"}) {
		t.Fatalf("preview = %+v", p)
	}
	msgs := []string{}
	for _, r := range p.Rows {
		msgs = append(msgs, r.Action+":"+r.Message)
	}
	want := []string{"create:", "create:", "update:", "error:请填写姓名", "error:号码与第 1 行重复", "error:本赛事没有项目「铅球」", "create:已有同名运动员"}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("rows = %q", msgs)
	}
	_, err = f.svc.ImportAthletes(f.ctx, ui, f.meet, in)
	wantInvalid(t, err, "3 行有误")
	if list, _ := f.svc.ListAthletes(f.ctx, f.meet); len(list) != 2 {
		t.Fatal("a refused import wrote athletes")
	}

	in.Rows = append(in.Rows[:3], [][]string{{"选手02", "男", "", "", ""}}...) // same name, no id: a new athlete, with a warning
	p, err = f.svc.ImportAthletes(f.ctx, ui, f.meet, in)
	must(t, err)
	if p.Creates != 3 || p.Updates != 1 || p.Rows[3].Message != "已有同名运动员" {
		t.Fatalf("import = %+v", p)
	}
	list, _ := f.svc.ListAthletes(f.ctx, f.meet)
	if len(list) != 5 || list[0].Athlete.Name != "选手一" || list[0].Athlete.Gender != "男" || list[0].UnitName != "一班" || list[0].Entries != 1 {
		t.Fatalf("after import: %+v", list)
	}
	if list[2].Athlete.Name != "张三" || list[2].UnitName != "三班" || list[2].Entries != 1 || list[3].UnitName != "一班" {
		t.Fatalf("new athletes: %+v", list[2:])
	}
	// Importing the same table again creates nobody with a bib.
	p, err = f.svc.PreviewAthleteImport(f.ctx, f.meet, AthleteImport{Mapping: mapping, Rows: in.Rows[:3]})
	must(t, err)
	if p.Creates != 0 || p.Updates != 3 || p.Entries != 0 || len(p.NewUnits) != 0 {
		t.Fatalf("second preview = %+v", p)
	}
	_, err = f.svc.PreviewAthleteImport(f.ctx, f.meet, AthleteImport{Mapping: []string{ColBib}, Rows: in.Rows})
	wantInvalid(t, err, "姓名")
}

func sample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/interchange/sample-meet.json")
	must(t, err)
	return data
}

// strip removes what differs between two exports of the same content.
func strip(t *testing.T, f *interchange.File) string {
	t.Helper()
	ids := map[string]string{}
	n := 0
	id := func(p *string) {
		if *p == "" {
			return
		}
		if _, ok := ids[*p]; !ok {
			n++
			ids[*p] = "id" + itoa(n)
		}
		*p = ids[*p]
	}
	f.ExportedAt = ""
	id(&f.Meet.ID)
	for i := range f.Groups {
		id(&f.Groups[i].ID)
	}
	for i := range f.Units {
		id(&f.Units[i].ID)
		for j := range f.Units[i].GroupIDs {
			id(&f.Units[i].GroupIDs[j])
		}
	}
	for i := range f.Athletes {
		id(&f.Athletes[i].ID)
		id(&f.Athletes[i].GroupID)
		if f.Athletes[i].UnitID != nil {
			id(f.Athletes[i].UnitID)
		}
	}
	for i := range f.Events {
		e := &f.Events[i]
		id(&e.ID)
		id(&e.GroupID)
		for j := range e.Entries {
			id(&e.Entries[j].AthleteID)
		}
		for j := range e.Rounds {
			id(&e.Rounds[j].ID)
			for k := range e.Rounds[j].Heats {
				h := &e.Rounds[j].Heats[k]
				id(&h.ID)
				for l := range h.Slots {
					id(&h.Slots[l].AthleteID)
				}
			}
		}
	}
	out, err := interchange.Encode(f)
	must(t, err)
	return string(out)
}

func TestImportSampleAndRoundTrip(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()
	data := sample(t)
	p, err := svc.PreviewImport(ctx, data, "")
	must(t, err)
	if len(p.Errors) != 0 || !p.NewMeet || p.Athletes.New == 0 || p.Events.New != 3 || len(p.Warnings) != 0 {
		t.Fatalf("preview = %+v", p)
	}
	if _, err := svc.ImportMeet(ctx, ui, data, "", "wrong"); !errors.Is(err, ErrStale) {
		t.Fatalf("wrong hash: %v", err)
	}
	done, err := svc.ImportMeet(ctx, ui, data, "", p.Hash)
	must(t, err)
	meets, _ := svc.ListMeets(ctx)
	if len(meets) != 1 || meets[0].ID != done.MeetID {
		t.Fatalf("meets = %+v", meets)
	}

	// Exported, imported as another meet and exported again: same content.
	first, err := svc.ExportMeet(ctx, done.MeetID)
	must(t, err)
	encoded, err := interchange.Encode(first)
	must(t, err)
	if _, err := interchange.Decode(encoded); err != nil {
		t.Fatalf("the export does not pass its own validation: %v", err)
	}
	p2, err := svc.PreviewImport(ctx, encoded, "")
	must(t, err)
	copyMeet, err := svc.ImportMeet(ctx, ui, encoded, "", p2.Hash)
	must(t, err)
	second, err := svc.ExportMeet(ctx, copyMeet.MeetID)
	must(t, err)
	if a, b := strip(t, first), strip(t, second); a != b {
		t.Fatalf("round trip changed the meet:\n%s\n---\n%s", a, b)
	}
	if p2.ResultsNew != p.ResultsNew || p.ResultsNew == 0 || p2.Heats != p.Heats {
		t.Fatalf("results %d → %d, heats %d → %d", p.ResultsNew, p2.ResultsNew, p.Heats, p2.Heats)
	}

	// Into the meet it came from: everything is reused, nothing is written.
	again, err := svc.PreviewImport(ctx, encoded, done.MeetID)
	must(t, err)
	if len(again.Errors) != 0 || again.Athletes.New != 0 || again.Units.New != 0 || again.Events.New != 0 ||
		again.Entries != 0 || again.Heats != 0 || again.ResultsNew != 0 || again.ResultsOverwritten != 0 ||
		again.Athletes.Reused != p.Athletes.New {
		t.Fatalf("re-import preview = %+v", again)
	}
}

func TestImportIntoExistingMeet(t *testing.T) {
	f := newFixture(t, 3, track())
	a := f.athletes
	f.addHeat(0, a[0], a[1], a[2])
	heat := f.detail().Rounds[0].Heats[0].ID
	must(t, f.svc.SaveResults(f.ctx, ui, heat, []ResultInput{{Position: 1, Mark: "12.34"}}))
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)

	// The file changes a result, adds one, a new athlete, and a new event.
	slots := file.Events[0].Rounds[0].Heats[0].Slots
	slots[0].Result.Mark = ptr(int64(12000))
	slots[1].Result = &interchange.Result{Status: "DNF"}
	file.Athletes = append(file.Athletes, interchange.Athlete{ID: "n1", Name: "新人", Gender: "女", Affiliation: "三班"})
	file.Events = append(file.Events, interchange.Event{ID: "e2", Gender: "女", Discipline: "跳远", Kind: "distance",
		Entries: []interchange.Entry{{AthleteID: "n1"}, {AthleteID: a[0]}},
		Rounds: []interchange.Round{{ID: "r", Stage: "final", Ordinal: 1, Heats: []interchange.Heat{{ID: "h", Ordinal: 1,
			Slots: []interchange.Slot{{Position: 1, AthleteID: "n1", Result: &interchange.Result{Status: "OK", Mark: ptr(int64(4200)), Rank: ptr(1)}}}}}}}})
	data, err := interchange.Encode(file)
	must(t, err)
	p, err := f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if len(p.Errors) != 0 || p.Athletes.New != 1 || p.Athletes.Reused != 3 || p.Units.New != 1 || p.Units.Reused != 2 ||
		p.Events.New != 1 || p.Events.Reused != 1 || p.Entries != 2 || p.Heats != 1 || p.ResultsNew != 2 || p.ResultsOverwritten != 1 {
		t.Fatalf("preview = %+v", p)
	}
	if len(p.Overwrites) != 1 || p.Overwrites[0].Before != "12.34" || p.Overwrites[0].After != "12.00" || !strings.Contains(p.Overwrites[0].Athlete, "选手01") {
		t.Fatalf("overwrites = %+v", p.Overwrites)
	}
	if len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "性别") {
		t.Fatalf("warnings = %v", p.Warnings)
	}
	_, err = f.svc.ImportMeet(f.ctx, ui, data, f.meet, p.Hash)
	must(t, err)
	d := f.detail()
	s := d.Rounds[0].Heats[0].Slots
	if s[0].Result.MarkText != "12.00" || s[1].Result.Status != "DNF" {
		t.Fatalf("after import = %+v %+v", s[0].Result, s[1].Result)
	}
	events, _ := f.svc.ListEvents(f.ctx, f.meet)
	if len(events) != 2 || events[1].Event.Name != "女子跳远" || events[1].Results != 1 || events[1].Entries != 2 {
		t.Fatalf("events = %+v", events)
	}

	// A heat arranged differently here is left alone, with a warning.
	slots[0].AthleteID, slots[1].AthleteID = slots[1].AthleteID, slots[0].AthleteID
	data, _ = interchange.Encode(file)
	p, err = f.svc.PreviewImport(f.ctx, data, f.meet)
	must(t, err)
	if p.ResultsNew+p.ResultsOverwritten != 0 || len(p.Warnings) != 2 || !strings.Contains(strings.Join(p.Warnings, " "), "编排与文件不同") {
		t.Fatalf("different arrangement = %+v", p)
	}

	bad, err := f.svc.PreviewImport(f.ctx, []byte(`{"format":"sportsmeet-interchange","version":1,"meet":{"name":""}}`), "")
	must(t, err)
	if len(bad.Errors) == 0 {
		t.Fatal("an invalid file previewed without errors")
	}
	if _, err := f.svc.ImportMeet(f.ctx, ui, []byte(`{}`), "", bad.Hash); err == nil {
		t.Fatal("an invalid file was imported")
	}
}

// A v4 file brings its round starts and heat durations, and exports them again.
func TestImportV4RoundStartAndDuration(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := t.Context()
	data, err := os.ReadFile("../../testdata/interchange/sample-meet-v4.json")
	must(t, err)
	p, err := svc.PreviewImport(ctx, data, "")
	must(t, err)
	if len(p.Errors) != 0 {
		t.Fatalf("preview = %+v", p.Errors)
	}
	done, err := svc.ImportMeet(ctx, ui, data, "", p.Hash)
	must(t, err)
	out, err := svc.ExportMeet(ctx, done.MeetID)
	must(t, err)
	if out.Version != interchange.Version {
		t.Fatalf("export version %d", out.Version)
	}
	var start, duration bool
	for _, e := range out.Events {
		for _, r := range e.Rounds {
			start = start || (r.ScheduledAt != nil && *r.ScheduledAt == "2026-11-12T14:00")
			for _, h := range r.Heats {
				duration = duration || (h.DurationMinutes != nil && *h.DurationMinutes == 10)
			}
		}
	}
	if !start || !duration {
		t.Fatal("round start or heat duration lost on import")
	}
}
