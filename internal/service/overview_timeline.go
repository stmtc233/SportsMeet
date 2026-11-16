package service

import (
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// OverviewTimeline is the 赛程 on 总览: each day's rounds hour by hour, in
// lanes by event kind, with where each stands and where now is.
type OverviewTimeline struct {
	Days []TimelineDay `json:"days"`
	// Day is the day to open: today when the meet has rounds then, else the
	// first day with a round not finished, else the first; "" without days.
	Day string `json:"day"`
	// Live: today is one of the days, so the now row moves.
	Live bool `json:"live"`
	// Unscheduled counts rounds whose effective times are not all set.
	Unscheduled int `json:"unscheduled"`
}

// TimelineDay is one day of the 赛程: a row per hour that has rounds, a
// cell per lane in each.
type TimelineDay struct {
	Day  string `json:"day"`  // YYYY-MM-DD
	Text string `json:"text"` // 11-12 周四
	// Lanes are the day's lanes in order; each row has a cell for each.
	Lanes []string      `json:"lanes"`
	Rows  []TimelineRow `json:"rows"`
}

// TimelineRow is an hour of the day, or (Now) where now falls among them.
// Today, the hour now is in splits at it: rounds starting before now
// above the now row, the rest below, the hour named on the first part.
type TimelineRow struct {
	Now bool `json:"now"`
	// Text is the hour (08:00), "" for the part of an hour after now, or
	// for the now row the time it is (10:42).
	Text string `json:"text"`
	// Cells hold each lane's rounds by start; none for the now row.
	Cells [][]TimelineRound `json:"cells"`
}

// Timeline lanes: 径赛 (track, relay), 田赛 (distance, height) and 其他
// (count, score). A lane without rounds on the day is left out.
const (
	LaneTrack = "track"
	LaneField = "field"
	LaneOther = "other"
)

// TimelineRound is a round of the 赛程.
type TimelineRound struct {
	EventID   string `json:"eventId"`
	RoundID   string `json:"roundId"`
	Text      string `json:"text"`      // 男子100m 预赛, the event alone with one round
	StartText string `json:"startText"` // 09:00, its earliest planned time
	// TimeText runs to the last heat's planned end when every heat has a
	// duration: 09:00–09:40; else as StartText.
	TimeText string `json:"timeText"`
	Heats    int    `json:"heats"`
	// People as on the 日程 page: placed in heats, or entered for a first
	// round without heats (PeopleEntered).
	People        int  `json:"people"`
	PeopleEntered bool `json:"peopleEntered"`
	// State: todo (no heats), arranged, doing (some result is in), done.
	State string `json:"state"`
	// Late: not finished, no result in, and its start has passed.
	Late bool `json:"late"`
	// Next: the first round, by time, not begun and not yet due.
	Next bool `json:"next"`
}

func laneOf(k domain.EventKind) string {
	switch k {
	case domain.KindTrack, domain.KindRelay:
		return LaneTrack
	case domain.KindDistance, domain.KindHeight:
		return LaneField
	}
	return LaneOther
}

// timelineRound is a round with where it goes.
type timelineRound struct {
	at    string
	day   string
	start int // minutes after the day's 00:00
	lane  string
	round TimelineRound
}

// timeline lays the meet's planned rounds out by day and hour; now is the
// local wall-clock minute (YYYY-MM-DDTHH:mm) to judge late and next by.
func (p *meetPlan) timeline(now string) OverviewTimeline {
	out := OverviewTimeline{Days: []TimelineDay{}}
	var list []timelineRound
	for _, e := range p.events {
		rs := p.rounds[e.ID]
		for i, r := range rs {
			if roundTimeState(r) != StepDone {
				out.Unscheduled++
			}
			at := domain.EarliestScheduled([]domain.Round{r})
			if at == "" {
				continue
			}
			day := domain.ScheduledDay(at)
			start, _ := domain.MinutesInto(at, day)
			tr := TimelineRound{EventID: e.ID, RoundID: r.ID, Text: roundText(e, len(rs), r), StartText: domain.ScheduledText(at, day),
				Heats: len(r.Heats), State: StepTodo}
			// The end: the latest heat end, when every heat has one.
			endAt, known := "", len(r.Heats) > 0
			for _, h := range r.Heats {
				tr.People += len(p.people(h))
				hAt, ok := domain.HeatEnd(r, h)
				known = known && ok
				if ok && hAt > endAt {
					endAt = hAt
				}
			}
			if len(r.Heats) == 0 && i == 0 {
				tr.People, tr.PeopleEntered = p.entered[e.ID], true
			}
			tr.TimeText = tr.StartText
			if known {
				tr.TimeText += "–" + domain.ScheduledText(endAt, day)
			}
			done, began := roundFinished(r)
			switch {
			case done:
				tr.State = StepDone
			case began:
				tr.State = StepDoing
			case len(r.Heats) > 0:
				tr.State = EventArranged
			}
			tr.Late = !done && !began && now != "" && at < now
			list = append(list, timelineRound{at, day, start, laneOf(e.Kind), tr})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].at < list[j].at })
	for i := range list {
		r := &list[i].round
		if r.State != StepDone && r.State != StepDoing && (now == "" || list[i].at >= now) {
			r.Next = true
			break
		}
	}

	today := domain.ScheduledDay(now)
	for start := 0; start < len(list); {
		stop := start
		for stop < len(list) && list[stop].day == list[start].day {
			stop++
		}
		nowAt := ""
		if list[start].day == today {
			nowAt = now
		}
		out.Days = append(out.Days, timelineDay(list[start:stop], nowAt))
		start = stop
	}

	for _, d := range out.Days {
		if d.Day == today {
			out.Day, out.Live = today, true
		}
	}
	for _, pl := range list {
		if out.Day == "" && pl.round.State != StepDone {
			out.Day = pl.day
		}
	}
	if out.Day == "" && len(out.Days) > 0 {
		out.Day = out.Days[0].Day
	}
	return out
}

// timelineDay lays out one day's rounds, sorted by time, an hour a row;
// now, when it is that day, splits its hour with a now row, or goes
// before or after the hours when it falls outside them.
func timelineDay(list []timelineRound, now string) TimelineDay {
	day := list[0].day
	d := TimelineDay{Day: day, Text: domain.ShortDayText(day), Lanes: []string{}, Rows: []TimelineRow{}}
	used := map[string]bool{}
	for _, pl := range list {
		used[pl.lane] = true
	}
	col := map[string]int{}
	for _, key := range []string{LaneTrack, LaneField, LaneOther} {
		if used[key] {
			col[key] = len(d.Lanes)
			d.Lanes = append(d.Lanes, key)
		}
	}
	nowMin, hasNow := domain.MinutesInto(now, day)
	nowRow := TimelineRow{Now: true, Text: domain.ScheduledText(now, day), Cells: [][]TimelineRound{}}
	row := func(text string, rounds []timelineRound) TimelineRow {
		r := TimelineRow{Text: text, Cells: make([][]TimelineRound, len(d.Lanes))}
		for i := range r.Cells {
			r.Cells[i] = []TimelineRound{}
		}
		for _, pl := range rounds {
			r.Cells[col[pl.lane]] = append(r.Cells[col[pl.lane]], pl.round)
		}
		return r
	}
	for i := 0; i < len(list); {
		hour := list[i].start / 60
		j := i
		for j < len(list) && list[j].start/60 == hour {
			j++
		}
		text := domain.ScheduledText(domain.MinuteOf(day, hour*60), day)
		if hasNow && nowMin < hour*60 {
			d.Rows, hasNow = append(d.Rows, nowRow), false
		}
		if !hasNow || nowMin/60 != hour {
			d.Rows = append(d.Rows, row(text, list[i:j]))
			i = j
			continue
		}
		// The hour now is in: before now, the now row, then the rest.
		k := i
		for k < j && list[k].start < nowMin {
			k++
		}
		if k > i {
			d.Rows = append(d.Rows, row(text, list[i:k]))
			text = ""
		}
		d.Rows, hasNow = append(d.Rows, nowRow), false
		if k < j {
			d.Rows = append(d.Rows, row(text, list[k:j]))
		}
		i = j
	}
	if hasNow {
		d.Rows = append(d.Rows, nowRow)
	}
	return d
}
