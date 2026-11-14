package service

import (
	"reflect"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestEntrySummary(t *testing.T) {
	f := newFixture(t, 4, track()) // 一班: 选手01 选手03; 二班: 选手02 选手04
	s, err := f.svc.EntrySummary(f.ctx, f.meet)
	must(t, err)
	if len(s.Units) != 2 || s.Units[0].Athletes != 2 || s.Units[0].Entries != 0 || len(s.Groups) != 0 || s.Limited {
		t.Fatalf("blank = %+v", s)
	}

	g, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高一"})
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[0], UnitInput{Name: "一班", GroupIDs: []string{g.ID}})
	must(t, err)
	_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, f.athletes[0], AthleteInput{Name: "选手01", Gender: "男", Bib: "001", UnitID: f.units[0], GroupID: g.ID})
	must(t, err)
	limit := domain.EntryLimits{UnitAthletes: 1}
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "秋季运动会", StartDate: "2026-11-12", EntryLimits: &limit}))
	must(t, f.svc.AddEntries(f.ctx, ui, f.event, []EntryInput{{AthleteID: f.athletes[0]}, {AthleteID: f.athletes[2]}, {AthleteID: f.athletes[1]}}))
	r, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", relay())
	must(t, err)
	tm, err := f.svc.SaveTeam(f.ctx, ui, r.ID, "", TeamInput{Name: "一班", UnitID: f.units[0], MemberIDs: []string{f.athletes[0], f.athletes[2]}})
	must(t, err)
	must(t, f.svc.AddEntries(f.ctx, ui, r.ID, []EntryInput{{TeamID: tm.ID}}))

	s, err = f.svc.EntrySummary(f.ctx, f.meet)
	must(t, err)
	type row struct{ Athletes, Entries, Teams, Over int }
	var units []row
	for _, u := range s.Units {
		units = append(units, row{u.Athletes, u.Entries, u.Teams, u.Over})
	}
	if want := []row{{2, 2, 1, 1}, {2, 1, 0, 0}}; !reflect.DeepEqual(units, want) || !s.Limited {
		t.Fatalf("units = %+v", units)
	}
	// 一班's 选手03 has no group: 一班 shows under 未分组 too, before 二班.
	want := []GroupEntries{
		{ID: g.ID, Name: "高一", Units: 1, Athletes: 1, Entries: 1, Members: []GroupMember{{f.units[0], 1}}},
		{Name: "未分组", Units: 1, Athletes: 3, Entries: 2, Members: []GroupMember{{f.units[0], 1}, {f.units[1], 2}}},
	}
	if !reflect.DeepEqual(s.Groups, want) {
		t.Fatalf("groups = %+v", s.Groups)
	}

	// Every unit in a group: 未分组 stays for the athletes of no group alone.
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[1], UnitInput{Name: "二班", GroupIDs: []string{g.ID}})
	must(t, err)
	s, err = f.svc.EntrySummary(f.ctx, f.meet)
	must(t, err)
	if none := s.Groups[1]; len(s.Groups) != 2 || none.Units != 0 || none.Athletes != 3 || len(none.Members) != 2 {
		t.Fatalf("loose athletes = %+v", s.Groups)
	}
	for _, id := range f.athletes[1:] {
		a := map[string]AthleteInput{f.athletes[1]: {Name: "选手02", Gender: "男", Bib: "002", UnitID: f.units[1]}, f.athletes[2]: {Name: "选手03", Gender: "男", Bib: "003", UnitID: f.units[0]}, f.athletes[3]: {Name: "选手04", Gender: "男", Bib: "004", UnitID: f.units[1]}}[id]
		a.GroupID = g.ID
		_, err = f.svc.SaveAthlete(f.ctx, ui, f.meet, id, a)
		must(t, err)
	}
	s, err = f.svc.EntrySummary(f.ctx, f.meet)
	must(t, err)
	if len(s.Groups) != 1 || !reflect.DeepEqual(s.Groups[0].Members, []GroupMember{{f.units[0], 2}, {f.units[1], 2}}) {
		t.Fatalf("all grouped = %+v", s.Groups)
	}
}
