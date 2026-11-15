package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestOverviewTimeline(t *testing.T) {
	five, ten := 5, 10
	scored := func(ids ...string) []domain.Slot {
		out := make([]domain.Slot, len(ids))
		for i, id := range ids {
			out[i] = domain.Slot{Position: i + 1, AthleteID: id, Result: &domain.Result{}}
		}
		return out
	}
	events := []domain.Event{
		{ID: "100", Name: "男子100m", Kind: domain.KindTrack},
		{ID: "lj", Name: "男子跳远", Kind: domain.KindDistance},
		{ID: "sp", Name: "女子铅球", Kind: domain.KindDistance},
		{ID: "rope", Name: "跳绳", Kind: domain.KindCount},
		{ID: "hj", Name: "男子跳高", Kind: domain.KindHeight},
	}
	rounds := map[string][]domain.Round{
		// 预赛 finished 08:00–08:10, 决赛 08:30 not begun, its heat without a duration.
		"100": {
			{ID: "100h", Stage: domain.StageHeat, Ordinal: 1, ScheduledAt: "2026-11-12T08:00", Heats: []domain.Heat{
				{ID: "h1", DurationMinutes: &five, Slots: scored("a")},
				{ID: "h2", ScheduledAt: "2026-11-12T08:05", DurationMinutes: &five, Slots: scored("b")},
			}},
			{ID: "100f", Stage: domain.StageFinal, Ordinal: 1, ScheduledAt: "2026-11-12T08:30", Heats: []domain.Heat{{ID: "f1", Slots: []domain.Slot{{Position: 1, AthleteID: "a"}}}}},
		},
		// Two field rounds at once: the second takes a row of its own.
		"lj": {{ID: "ljf", Stage: domain.StageFinal, Ordinal: 1, ScheduledAt: "2026-11-12T08:00",
			Heats: []domain.Heat{{ID: "l1", DurationMinutes: &ten, Slots: []domain.Slot{{Position: 1, AthleteID: "c", Result: &domain.Result{}}, {Position: 2, AthleteID: "d"}}}}}},
		"sp":   {{ID: "spf", Stage: domain.StageFinal, Ordinal: 1, ScheduledAt: "2026-11-12T08:05"}},
		"rope": {{ID: "ropef", Stage: domain.StageFinal, Ordinal: 1}},
		"hj":   {{ID: "hjf", Stage: domain.StageFinal, Ordinal: 1, ScheduledAt: "2026-11-13T09:00"}},
	}
	p := newMeetPlan(domain.Meet{StartDate: "2026-11-12", EndDate: "2026-11-13"}, events, rounds, nil, nil,
		[]domain.Entry{{EventID: "sp", AthleteID: "e"}, {EventID: "sp", AthleteID: "f"}}, 0)

	tl := p.timeline("2026-11-12T08:20")
	if tl.Day != "2026-11-12" || !tl.Live || tl.Unscheduled != 1 || len(tl.Days) != 2 {
		t.Fatalf("timeline = %+v", tl)
	}
	d := tl.Days[0]
	if d.Text != "11-12 周四" || strings.Join(d.Lanes, " ") != "track field" {
		t.Fatalf("day = %+v", d)
	}
	// 08:00 splits at now: what started before it, the now row, the rest.
	if got := rowsText(d.Rows); got != "08:00 [男子100m 预赛] [男子跳远 女子铅球] | now 08:20 | [男子100m 决赛] []" {
		t.Fatalf("rows = %s", got)
	}
	heat, lj, sp, final := d.Rows[0].Cells[0][0], d.Rows[0].Cells[1][0], d.Rows[0].Cells[1][1], d.Rows[2].Cells[0][0]
	if heat.StartText != "08:00" || heat.TimeText != "08:00–08:10" || heat.State != StepDone || heat.People != 2 {
		t.Fatalf("heat = %+v", heat)
	}
	// Not yet due at 08:20: the next round; a heat without a duration leaves no end.
	if final.State != EventArranged || final.TimeText != "08:30" || !final.Next || final.Late {
		t.Fatalf("final = %+v", final)
	}
	if lj.State != StepDoing || lj.Late || sp.State != StepTodo || !sp.Late || sp.People != 2 || !sp.PeopleEntered {
		t.Fatalf("field = %+v %+v", lj, sp)
	}
	// Another day: no now row.
	if d2 := tl.Days[1]; rowsText(d2.Rows) != "09:00 [男子跳高]" || d2.Rows[0].Cells[0][0].Late || d2.Rows[0].Cells[0][0].Next {
		t.Fatalf("second day = %+v", d2)
	}

	// After the meet's first day: the first day with a round not finished.
	if tl := p.timeline("2026-12-01T10:00"); tl.Day != "2026-11-12" || tl.Live || len(tl.Days[0].Rows) != 1 || !tl.Days[1].Rows[0].Cells[0][0].Late {
		t.Fatalf("later = %+v", tl)
	}
	// Now before the day's hours, and after them.
	if tl := p.timeline("2026-11-13T08:00"); tl.Day != "2026-11-13" || rowsText(tl.Days[1].Rows) != "now 08:00 | 09:00 [男子跳高]" || !tl.Days[1].Rows[1].Cells[0][0].Next {
		t.Fatalf("second = %s", rowsText(tl.Days[1].Rows))
	}
	if got := rowsText(p.timeline("2026-11-12T12:00").Days[0].Rows); got != "08:00 [男子100m 预赛 男子100m 决赛] [男子跳远 女子铅球] | now 12:00" {
		t.Fatalf("after = %s", got)
	}
	// Now on the hour, nothing before it in that hour: the now row, then the hour.
	if got := rowsText(p.timeline("2026-11-12T08:00").Days[0].Rows); got != "now 08:00 | 08:00 [男子100m 预赛 男子100m 决赛] [男子跳远 女子铅球]" {
		t.Fatalf("on the hour = %s", got)
	}
}

// rowsText sums rows up: an hour's text and its cells' rounds, or now.
func rowsText(rows []TimelineRow) string {
	var parts []string
	for _, r := range rows {
		if r.Now {
			parts = append(parts, "now "+r.Text)
			continue
		}
		line := r.Text
		for _, c := range r.Cells {
			var names []string
			for _, x := range c {
				names = append(names, x.Text)
			}
			line += " [" + strings.Join(names, " ") + "]"
		}
		parts = append(parts, strings.TrimSpace(line))
	}
	return strings.Join(parts, " | ")
}
