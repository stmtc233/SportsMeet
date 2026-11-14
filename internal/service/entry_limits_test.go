package service

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func setLimits(t *testing.T, f *fixture, l domain.EntryLimits) {
	t.Helper()
	m, err := f.svc.GetMeet(f.ctx, f.meet)
	must(t, err)
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: m.Name, StartDate: m.StartDate, EntryLimits: &l}))
}

func saveEvent(t *testing.T, f *fixture, in EventInput) string {
	t.Helper()
	e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", in)
	must(t, err)
	return e.ID
}

func entryOf(d *EventDetail, id string) EntryView {
	for _, e := range d.Entries {
		if e.CompetitorID == id {
			return e
		}
	}
	return EntryView{}
}

// Limits are only pointed out: every entry below is saved.
func TestEntryLimitsWarnWithoutRefusing(t *testing.T) {
	f := newFixture(t, 6, track()) // 一班: 选手01/03/05, 二班: 选手02/04/06
	limits := domain.EntryLimits{AthleteEvents: 2, UnitAthletes: 2, UnitTeams: 1}
	setLimits(t, f, limits)
	m, err := f.svc.GetMeet(f.ctx, f.meet)
	must(t, err)
	if m.EntryLimits != limits {
		t.Fatal("limits not saved", m.EntryLimits)
	}
	// Updating without limits keeps them; out-of-range limits are refused.
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "改名", StartDate: m.StartDate}))
	if m, _ = f.svc.GetMeet(f.ctx, f.meet); m.EntryLimits != limits {
		t.Fatal("limits lost on update")
	}
	wantInvalid(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "改名", StartDate: m.StartDate, EntryLimits: &domain.EntryLimits{UnitTeams: 100}}), "0–99")

	a := f.athletes
	e100 := f.event
	e200 := saveEvent(t, f, EventInput{Gender: "男", Discipline: "200m", Kind: "track", LaneCount: 8})
	long := saveEvent(t, f, EventInput{Gender: "男", Discipline: "跳远", Kind: "distance"})
	f.enter(a[0], a[2])
	f.event = e200
	f.enter(a[0])

	got, err := f.svc.EntryLimitWarnings(f.ctx, long, []string{a[0], a[1]})
	must(t, err)
	if !reflect.DeepEqual(got, []string{"「选手01」报 3 项，限报 2 项"}) {
		t.Fatalf("athlete warnings = %q", got)
	}
	got, err = f.svc.EntryLimitWarnings(f.ctx, e100, []string{a[4], a[0]})
	must(t, err)
	if !reflect.DeepEqual(got, []string{"「一班」报 3 人，限报 2 人"}) {
		t.Fatalf("unit warnings = %q", got)
	}

	f.event = long
	f.enter(a[0])
	if e := entryOf(f.detail(), a[0]); !e.OverEvents || e.OverUnit {
		t.Fatalf("entry flags = %+v", e)
	}
	f.event = e100
	f.enter(a[4])
	if e := entryOf(f.detail(), a[2]); e.OverEvents || !e.OverUnit {
		t.Fatalf("unit flags = %+v", e)
	}
	if e := entryOf(f.detail(), a[1]); e.OverUnit {
		t.Fatal("unentered unit flagged")
	}
	rows, err := f.svc.ListAthletes(f.ctx, f.meet)
	must(t, err)
	for _, r := range rows {
		if r.OverLimit != (r.Athlete.ID == a[0]) {
			t.Fatalf("%s overLimit = %v", r.Athlete.Name, r.OverLimit)
		}
	}

	// Team events count teams per unit and never count for athletes.
	relayID := saveEvent(t, f, relay())
	var teams []string
	for i, members := range [][]string{{a[0], a[2]}, {a[4]}} {
		team, err := f.svc.SaveTeam(f.ctx, ui, relayID, "", TeamInput{Name: []string{"甲队", "乙队"}[i], UnitID: f.units[0], MemberIDs: members})
		must(t, err)
		teams = append(teams, team.ID)
	}
	must(t, f.svc.AddEntries(f.ctx, ui, relayID, []EntryInput{{TeamID: teams[0]}}))
	got, err = f.svc.EntryLimitWarnings(f.ctx, relayID, []string{teams[1]})
	must(t, err)
	if !reflect.DeepEqual(got, []string{"「一班」报 2 队，限报 1 队"}) {
		t.Fatalf("team warnings = %q", got)
	}

	// A combined is one event, whatever its components.
	c, err := f.svc.SaveCombined(f.ctx, ui, f.meet, "", combinedInput())
	must(t, err)
	got, err = f.svc.CombinedLimitWarnings(f.ctx, c.ID, []string{a[1]})
	must(t, err)
	if len(got) != 0 {
		t.Fatalf("combined counted per component: %q", got)
	}
	must(t, f.svc.SetCombinedEntries(f.ctx, ui, c.ID, []string{a[1]}))
	f.event = e100
	f.enter(a[1])
	rows, _ = f.svc.ListAthletes(f.ctx, f.meet)
	for _, r := range rows {
		if r.Athlete.ID == a[1] && r.OverLimit {
			t.Fatal("two events flagged")
		}
	}

	// MCP previews carry the same warnings.
	p, err := f.svc.PreviewMutation(f.ctx, Mutation{Action: "add_entries", EventID: e200, Entries: []EntryInput{{AthleteID: a[1]}}})
	must(t, err)
	if plan, ok := p.Result.(EntriesPlan); !ok || !reflect.DeepEqual(plan.Warnings, []string{"「选手02」报 3 项，限报 2 项"}) {
		t.Fatalf("mcp preview = %+v", p.Result)
	}

	// The table import marks the rows that go over.
	preview, err := f.svc.PreviewAthleteImport(f.ctx, f.meet, AthleteImport{Mapping: []string{ColName, ColBib, ColEvents},
		Rows: [][]string{{"选手04", "004", "男子100m、男子200m、男子跳远"}, {"选手06", "006", "男子100m"}, {"新人", "201", "男子200m"}}})
	must(t, err)
	if msg := preview.Rows[0].Message; !strings.Contains(msg, "报 3 项，限报 2 项") || !strings.Contains(msg, "「男子100m」本单位报 3 人，限报 2 人") {
		t.Fatalf("import row 1 = %q", msg)
	}
	if msg := preview.Rows[1].Message; msg != "「男子100m」本单位报 3 人，限报 2 人" {
		t.Fatalf("import row 2 = %q", msg)
	}
	if preview.Errors != 0 || preview.Rows[2].Message != "" {
		t.Fatalf("import row 3 = %+v", preview.Rows[2])
	}

	// The limits travel with the meet's file.
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	if file.Meet.EntryLimits == nil || *file.Meet.EntryLimits != limits {
		t.Fatal("limits not exported")
	}
	data, err := interchange.Encode(file)
	must(t, err)
	ip, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	imported, err := f.svc.ImportMeet(f.ctx, ui, data, "", ip.Hash)
	must(t, err)
	meets, err := f.svc.ListMeets(f.ctx)
	must(t, err)
	found := false
	for _, mv := range meets {
		if mv.ID != f.meet && mv.EntryLimits == limits {
			found = true
		}
	}
	if !found || imported == nil {
		t.Fatal("imported meet lost its limits")
	}
}

func TestNoLimitsNoWarnings(t *testing.T) {
	f := newFixture(t, 3, track())
	f.enter(f.athletes...)
	got, err := f.svc.EntryLimitWarnings(f.ctx, f.event, f.athletes)
	must(t, err)
	if len(got) != 0 {
		t.Fatal(got)
	}
	for _, e := range f.detail().Entries {
		if e.OverEvents || e.OverUnit {
			t.Fatal("flagged without limits")
		}
	}
}
