package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Report kinds.
const (
	ReportStartList  = "startlist"   // 秩序册
	ReportEntries    = "entries"     // 报名表
	ReportTeamScores = "team_scores" // 团体总分表
	ReportResults    = "results"     // 成绩册
)

// RenderReport returns a printable HTML document for the meet. eventIDs
// limits it to some events (in the list's order, domain.ScheduleOrder);
// empty means all.
func (s *Service) RenderReport(ctx context.Context, kind, meetID string, eventIDs []string) (string, error) {
	if kind == ReportEntries {
		return s.renderEntries(ctx, meetID, eventIDs)
	}
	if kind == ReportResults || kind == ReportTeamScores {
		return s.RenderGroupedReport(ctx, GroupReportRequest{Kind: kind, MeetID: meetID, EventIDs: eventIDs})
	}
	if kind != ReportStartList {
		return "", invalid("报表类型不正确")
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	book, err := s.reportBook(ctx, meetID, eventIDs, false, false)
	if err != nil {
		return "", err
	}
	book.NewPage = prefs.PrintNewPage
	if !prefs.PrintSchedule {
		book.Schedule = nil
	}
	if !prefs.PrintRoster {
		book.Rosters = nil
	}
	return report.StartList(*book)
}

// reportBook gathers the printed content; resultsTime adds each heat's
// results time (成绩册, when the settings ask for it).
func (s *Service) reportBook(ctx context.Context, meetID string, eventIDs []string, resultsTime, withResults bool) (*report.Book, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	parents, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return nil, err
	}
	only := map[string]bool{}
	for _, id := range eventIDs {
		only[id] = true
	}
	for _, p := range parents {
		if only[p.Combined.ID] {
			for _, c := range p.Components {
				only[c.EventID] = true
			}
		}
	}
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	if len(only) > 0 {
		sc.EventIDs = []string{}
		for _, e := range events {
			if only[e.ID] {
				sc.EventIDs = append(sc.EventIDs, e.ID)
			}
		}
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}
	var athletes []domain.Athlete
	if sc.EventIDs != nil {
		athletes, err = s.store.ListParticipants(ctx, sc)
	} else {
		athletes, err = s.store.ListAthletes(ctx, meetID)
	}
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	var rounds map[string][]domain.Round
	if withResults {
		rounds, err = s.store.LoadRounds(ctx, sc)
	} else {
		rounds, err = s.store.LoadPlanRounds(ctx, sc)
	}
	if err != nil {
		return nil, err
	}
	teams, err := s.store.ListTeams(ctx, sc)
	if err != nil {
		return nil, err
	}
	st := &eventState{teams: map[string]domain.Team{}, athletes: map[string]domain.Athlete{}, units: map[string]domain.Unit{}}
	for _, a := range athletes {
		st.athletes[a.ID] = a
	}
	for _, u := range units {
		st.units[u.ID] = u
	}
	for _, t := range teams {
		st.teams[t.ID] = t
	}
	seeds := map[string]*int64{}
	for _, e := range entries {
		seeds[e.EventID+"|"+e.CompetitorID()] = e.SeedMark
	}
	combinedState := &combinedMeetState{parents: parents, events: map[string]domain.Event{}, rounds: rounds, athletes: athletes, units: map[string]string{}}
	for _, e := range events {
		combinedState.events[e.ID] = e
	}
	for _, u := range units {
		combinedState.units[u.ID] = u.Name
	}
	book := &report.Book{Meet: meet.Name, Dates: domain.DateRangeText(meet.StartDate, meet.EndDate), Location: meet.Location,
		PrintedAt: domain.RecordedText(s.store.Now(), s.zone)}
	var listed map[string]bool // nil = every athlete of the meet
	if len(only) > 0 {
		listed = map[string]bool{}
		for _, e := range entries {
			if !only[e.EventID] {
				continue
			}
			if e.TeamID == "" {
				listed[e.AthleteID] = true
			}
			for _, id := range st.teams[e.TeamID].MemberIDs {
				listed[id] = true
			}
		}
		for _, p := range parents {
			if only[p.Combined.ID] {
				for _, id := range p.AthleteIDs {
					listed[id] = true
				}
			}
		}
	}
	book.Rosters = rosterBook(units, athletes, listed)
	titles := map[string]string{}
	for _, p := range parents {
		titles[p.Combined.ID] = combinedTitle(p.Combined)
	}
	book.Schedule = scheduleBook(newMeetPlan(*meet, events, rounds, teams, athletes, entries, 0).timetable(only), combinedState.events, titles)
	for _, e := range domain.ScheduleOrder(events, rounds, meet.StartDate) {
		if len(only) > 0 && !only[e.ID] {
			continue
		}
		st.event = e
		date := domain.PlannedDay(e, rounds[e.ID], meet.StartDate)
		named := printedTeamNames(e, st)
		ev := reportEvent(e, domain.DayText(date), len(named) > 0, withResults)
		line := func(sl domain.Slot) report.Line {
			a, _ := st.competitor(sl.CompetitorID())
			l := report.Line{Position: sl.Position, Bib: a.Bib, Name: a.Name, Unit: st.unitName(a.UnitID), Best: -1}
			if sl.TeamID != "" {
				if !named[sl.TeamID] {
					l.Name = ""
				}
				l.Members = st.memberLabels(sl.TeamID)
			}
			if seed := seeds[e.ID+"|"+a.ID]; seed != nil {
				l.Seed = scoring.FormatEventMark(e, *seed)
				ev.Seeded = true
			}
			if r := sl.Result; r != nil && withResults {
				l.Mark, l.Qualify = r.Status.Label(), r.Qualify
				for _, a := range r.Attempts {
					l.Attempts = append(l.Attempts, scoring.FormatEventAttempt(e, a))
				}
				if r.Status == domain.StatusOK && r.Mark != nil {
					l.Mark = scoring.FormatEventMark(e, *r.Mark)
					l.Best = bestAttemptIndex(e, r.Attempts, *r.Mark)
				}
				for _, b := range r.Bars {
					l.Bars = append(l.Bars, scoring.FormatEventMark(e, b.Height)+" "+b.Trials)
				}
				if r.Rank != nil {
					l.Rank = strconv.Itoa(*r.Rank)
				}
			}
			return l
		}
		for _, r := range rounds[e.ID] {
			rr := report.Round{Label: r.Label(), ByHeat: r.Advance != nil}
			if r.Advance != nil {
				rr.Advance = advanceText(*r.Advance)
			}
			bySlot := map[string]report.Line{}
			for _, h := range r.Heats {
				at, _ := domain.EffectiveScheduled(r, h)
				heat := report.Heat{Label: domain.HeatLabel(h.Ordinal, h.Name), Time: domain.ScheduledText(at, date),
					Started: domain.ScheduledText(h.StartedAt, date)}
				if !withResults {
					heat.Count = countText(len(h.Slots), e.IsTeam())
				}
				if resultsTime {
					heat.ResultsTime = domain.RecordedText(h.ResultsUpdatedAt, s.zone)
				}
				for _, sl := range h.Slots {
					l := line(sl)
					heat.Lines = append(heat.Lines, l)
					l.Heat = heat.Label
					bySlot[sl.CompetitorID()] = l
				}
				rr.Heats = append(rr.Heats, heat)
			}
			if withResults {
				list, _ := standingsEvent(e, r)
				for _, row := range list {
					l := bySlot[row.CompetitorID]
					l.Rank = ""
					if row.Rank != nil {
						l.Rank = strconv.Itoa(*row.Rank)
					}
					if len(r.Heats) < 2 {
						l.Heat = ""
					}
					rr.Standings = append(rr.Standings, l)
				}
			}
			ev.Rounds = append(ev.Rounds, rr)
		}
		padAttempts(&ev)
		book.Events = append(book.Events, ev)
	}
	for _, parent := range parents {
		if len(only) > 0 && !only[parent.Combined.ID] {
			continue
		}
		c := report.Combined{Name: titles[parent.Combined.ID], State: "暂列排名", Tables: []report.CombinedTable{}, Roster: []report.Line{}}
		for _, id := range parent.AthleteIDs {
			a := st.athletes[id]
			c.Roster = append(c.Roster, report.Line{Name: a.Name, Bib: a.Bib, Unit: st.unitName(a.UnitID)})
		}
		if !withResults {
			for i, part := range parent.Components {
				e := combinedState.events[part.EventID]
				c.Components = append(c.Components, report.CombinedPart{No: i + 1, Name: e.Discipline,
					Time: domain.ScheduledText(domain.EarliestScheduled(rounds[e.ID]), "")})
			}
			book.Combineds = append(book.Combineds, c)
			continue
		}
		d, err := combinedState.detail(parent, nil)
		if err != nil {
			return nil, err
		}

		if d.Complete {
			c.State = "正式排名"
		}
		headers := []string{}
		rows := []report.CombinedRow{}
		for _, part := range d.Components {
			headers = append(headers, part.Event.Discipline)
		}

		for _, row := range d.Rows {
			r := report.CombinedRow{Name: row.Name, Bib: row.Bib, Unit: row.UnitName, Total: strconv.FormatInt(row.Total, 10), Cells: []report.CombinedCell{}}
			if row.Rank != nil {
				r.Rank = strconv.Itoa(*row.Rank)
			}
			for _, cell := range row.Cells {
				out := report.CombinedCell{Mark: "待录"}
				if !cell.Pending {
					out.Mark = cell.MarkText
					if cell.Excluded {
						out.Points = "不列名次"
					} else {
						out.Points = strconv.FormatInt(*cell.Points, 10)
					}
				}
				r.Cells = append(r.Cells, out)
			}
			rows = append(rows, r)
		}
		for start := 0; start < len(headers); start += 4 {
			end := min(start+4, len(headers))
			table := report.CombinedTable{Headers: headers[start:end], Rows: []report.CombinedRow{}}
			if len(headers) > 4 {
				if end == start+1 {
					table.Label = fmt.Sprintf("分项 %d", end)
				} else {
					table.Label = fmt.Sprintf("分项 %d–%d", start+1, end)
				}
			}
			for _, row := range rows {
				copy := row
				copy.Cells = row.Cells[start:end]
				table.Rows = append(table.Rows, copy)
			}
			c.Tables = append(c.Tables, table)
		}
		book.Combineds = append(book.Combineds, c)
	}
	return book, nil
}

// scheduleBook is the 竞赛日程表: the timetable's scheduled rounds by day
// and time, numbered through; rounds with no time are not printed. A
// component is named after its 全能 (titles), so it does not read as an
// event of its own.
func scheduleBook(t *Timetable, events map[string]domain.Event, titles map[string]string) []report.ScheduleDay {
	no := 0
	var out []report.ScheduleDay
	for _, d := range t.Days {
		day := report.ScheduleDay{Day: domain.DayText(d.Day), Rows: make([]report.ScheduleRow, 0, len(d.Rounds))}
		for _, r := range d.Rounds {
			no++
			e := events[r.EventID]
			name := r.EventName
			if title := titles[e.CombinedID]; title != "" {
				name = title + " · " + e.Discipline
			}
			day.Rows = append(day.Rows, report.ScheduleRow{No: no, Time: r.StartText, Kind: kindText(e.Kind), Event: name, Round: r.RoundLabel, Heats: r.HeatCount, People: r.People})
		}
		out = append(out, day)
	}
	return out
}

// kindText is an event's column in the 竞赛日程表.
func kindText(k domain.EventKind) string {
	switch k {
	case domain.KindTrack, domain.KindRelay:
		return "径赛"
	case domain.KindDistance, domain.KindHeight:
		return "田赛"
	}
	return k.Label()
}

// combinedTitle names a 全能 with its gender and group where its name does
// not already say them: 男子三项全能 初一组.
func combinedTitle(c domain.Combined) string {
	name := c.Name
	if prefix := domain.ComposeEventName(c.Gender, "", ""); prefix != "" && !strings.Contains(name, prefix) {
		name = prefix + name
	}
	if g := strings.TrimSpace(c.GroupLabel); g != "" && !strings.Contains(name, g) {
		name += " " + g
	}
	return name
}

// reportEvent is an event's heading and columns; its rounds are added after.
// named says some team's name is printed.
func reportEvent(e domain.Event, day string, named, withResults bool) report.Event {
	rules := e.Rules()
	config := rules.StartLabel
	if rules.AttemptLabel != "" {
		config = fmt.Sprintf("%s %d 次", rules.AttemptLabel, e.AttemptCount)
	}
	if e.Kind == domain.KindRelay && e.RelayLegCount > 0 {
		config += fmt.Sprintf(" · 建议 %d 棒", e.RelayLegCount)
	}
	if e.IsTeam() && e.Kind != domain.KindRelay && e.TeamSize > 0 {
		config += fmt.Sprintf(" · 建议 %d 人", e.TeamSize)
	}
	if e.Kind.Scalar() {
		method := map[string]string{"single": "一次", "best": "取最好", "sum": "求和"}[e.AttemptMode]
		config = e.Kind.Label() + " · " + method
		if e.MultipleAttempts() {
			config += fmt.Sprintf(" · %d 次", e.AttemptCount)
		}
	}
	var meta []string
	for _, part := range []string{day, strings.TrimPrefix(config, " · ")} {
		if part != "" {
			meta = append(meta, part)
		}
	}
	ev := report.Event{Name: e.Name, Meta: strings.Join(meta, " · "), Team: e.IsTeam(), NameLabel: competitorHeading(e, named), PositionLabel: rules.PositionLabel}
	if withResults && e.MultipleAttempts() {
		for i := 1; i <= e.AttemptCount; i++ {
			ev.TryHeads = append(ev.TryHeads, fmt.Sprintf("第%d次", i))
		}
	}
	ev.Bars = withResults && e.Kind == domain.KindHeight
	ev.Wide = ev.Team || len(ev.TryHeads) > 0 || ev.Bars
	return ev
}

// padAttempts gives every line one cell per attempt column, adding columns
// when a line has more attempts than the event's count.
func padAttempts(ev *report.Event) {
	if len(ev.TryHeads) == 0 {
		return
	}
	n := len(ev.TryHeads)
	each := func(f func(l *report.Line)) {
		for ri := range ev.Rounds {
			r := &ev.Rounds[ri]
			for hi := range r.Heats {
				for li := range r.Heats[hi].Lines {
					f(&r.Heats[hi].Lines[li])
				}
			}
			for li := range r.Standings {
				f(&r.Standings[li])
			}
		}
	}
	each(func(l *report.Line) { n = max(n, len(l.Attempts)) })
	for i := len(ev.TryHeads) + 1; i <= n; i++ {
		ev.TryHeads = append(ev.TryHeads, fmt.Sprintf("第%d次", i))
	}
	each(func(l *report.Line) {
		for len(l.Attempts) < n {
			l.Attempts = append(l.Attempts, "")
		}
	})
}

// bestAttemptIndex is the attempt that made the mark, -1 when the mark is
// not one attempt's (a sum) or none matches.
func bestAttemptIndex(e domain.Event, attempts []domain.Attempt, mark int64) int {
	if e.Kind != domain.KindDistance && e.AttemptMode != "best" {
		return -1
	}
	for i, a := range attempts {
		if a.Outcome == "valid" && a.Mark != nil && *a.Mark == mark {
			return i
		}
	}
	return -1
}

// printedTeamNames are the event's teams whose name is printed. A name
// every team shares, or the unit's own, says nothing the unit column does
// not.
func printedTeamNames(e domain.Event, st *eventState) map[string]bool {
	out := map[string]bool{}
	if !e.IsTeam() {
		return out
	}
	var list []domain.Team
	names := map[string]bool{}
	for _, t := range st.teams {
		if t.EventID == e.ID {
			list = append(list, t)
			names[strings.TrimSpace(t.Name)] = true
		}
	}
	if len(list) > 1 && len(names) == 1 {
		return out
	}
	for _, t := range list {
		name, u := strings.TrimSpace(t.Name), st.units[t.UnitID]
		if name != "" && name != u.Name && name != u.ShortName {
			out[t.ID] = true
		}
	}
	return out
}

// memberLabels are a team's members in order: 1棒 周书航 12101.
func (st *eventState) memberLabels(teamID string) []string {
	var out []string
	for i, id := range st.teams[teamID].MemberIDs {
		a := st.athletes[id]
		label := teamMemberLabel(st.event, i, a.Name)
		if a.Bib != "" {
			label += " " + a.Bib
		}
		out = append(out, label)
	}
	return out
}

// countText is how many competitors a heat has: 6 人 / 3 队.
func countText(n int, team bool) string {
	if team {
		return fmt.Sprintf("%d 队", n)
	}
	return fmt.Sprintf("%d 人", n)
}

// rosterBook is the 代表队名单: each unit's athletes by gender and bib, the
// units in the list's order and the athletes with no unit last. listed
// limits it to some athletes; nil lists them all. Units with nobody to
// list are left out.
func rosterBook(units []domain.Unit, athletes []domain.Athlete, listed map[string]bool) []report.UnitRoster {
	byUnit := map[string][]domain.Athlete{}
	for _, a := range athletes {
		if listed == nil || listed[a.ID] {
			byUnit[a.UnitID] = append(byUnit[a.UnitID], a)
		}
	}
	genders := []struct {
		gender domain.Gender
		label  string
	}{{domain.GenderMale, "男子"}, {domain.GenderFemale, "女子"}, {domain.GenderUnknown, "其他"}}
	var out []report.UnitRoster
	add := func(name string, list []domain.Athlete) {
		if len(list) == 0 {
			return
		}
		sort.SliceStable(list, func(i, j int) bool { return bibLess(list[i].Bib, list[j].Bib) })
		u := report.UnitRoster{Name: name, Count: len(list), Bibs: list[0].Bib}
		for i := len(list) - 1; i > 0; i-- {
			if b := list[i].Bib; b != "" {
				if b != u.Bibs {
					u.Bibs += "–" + b
				}
				break
			}
		}
		for _, g := range genders {
			group := report.RosterGroup{Label: g.label}
			for _, a := range list {
				known := a.Gender == domain.GenderMale || a.Gender == domain.GenderFemale
				if a.Gender == g.gender || !known && g.gender == domain.GenderUnknown {
					group.Athletes = append(group.Athletes, report.Line{Bib: a.Bib, Name: a.Name})
				}
			}
			if len(group.Athletes) > 0 {
				u.Groups = append(u.Groups, group)
			}
		}
		// One group of unknown gender needs no heading.
		if len(u.Groups) == 1 && u.Groups[0].Label == "其他" {
			u.Groups[0].Label = ""
		}
		out = append(out, u)
	}
	for _, u := range units {
		add(u.Name, byUnit[u.ID])
		delete(byUnit, u.ID)
	}
	var rest []domain.Athlete
	for _, a := range athletes { // athletes' own order, not the map's
		if _, ok := byUnit[a.UnitID]; ok && (listed == nil || listed[a.ID]) {
			rest = append(rest, a)
		}
	}
	add("无参赛单位", rest)
	return out
}

// bibLess orders bibs for printing: numbers by value before other text,
// athletes with no bib last.
func bibLess(a, b string) bool {
	if a == "" || b == "" {
		return a != "" && b == ""
	}
	x, errX := strconv.Atoi(a)
	y, errY := strconv.Atoi(b)
	switch {
	case errX == nil && errY == nil:
		return x < y
	case errX == nil || errY == nil:
		return errX == nil
	}
	return a < b
}

func advanceText(a domain.Advance) string {
	var parts []string
	if a.ByPlace > 0 {
		parts = append(parts, fmt.Sprintf("各组前 %d 名", a.ByPlace))
	}
	if a.ByMark > 0 {
		parts = append(parts, fmt.Sprintf("其余成绩最好的 %d 名", a.ByMark))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "与") + "晋级"
}

func teamMemberLabel(e domain.Event, index int, name string) string {
	if e.Kind == domain.KindRelay {
		return fmt.Sprintf("%d棒 %s", index+1, name)
	}
	return name
}

type ReportTarget struct {
	GroupID string `json:"groupId"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

func (s *Service) ReportTargets(ctx context.Context, meetID string) ([]ReportTarget, error) {
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	parents, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return nil, err
	}
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	overviews, err := s.store.EventOverviews(ctx, meetID)
	if err != nil {
		return nil, err
	}
	rounds := make(map[string][]domain.Round, len(overviews))
	for id, overview := range overviews {
		rounds[id] = overview.TimingRounds
	}
	out := []ReportTarget{}
	for _, e := range domain.ScheduleOrder(events, rounds, meet.StartDate) {
		if e.CombinedID == "" {
			out = append(out, ReportTarget{GroupID: e.GroupID, ID: e.ID, Name: e.Name})
		}
	}
	for _, p := range parents {
		out = append(out, ReportTarget{GroupID: p.Combined.GroupID, ID: p.Combined.ID, Name: p.Combined.Name})
	}
	return out, nil
}
