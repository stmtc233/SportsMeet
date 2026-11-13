package service

import (
	"context"
	"fmt"
	"slices"
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
	ReportTeamScores = "team_scores" // 团体总分表
	ReportResults    = "results"     // 成绩册
)

// RenderReport returns a printable HTML document for the meet. eventIDs
// limits it to some events (in the list's order, domain.ScheduleOrder);
// empty means all.
func (s *Service) RenderReport(ctx context.Context, kind, meetID string, eventIDs []string) (string, error) {
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	if kind == ReportTeamScores {
		meet, err := s.GetMeet(ctx, meetID)
		if err != nil {
			return "", err
		}
		table, err := s.teamScoreTable(ctx, meetID, eventIDs)
		if err != nil {
			return "", err
		}
		return report.TeamScores(report.Book{Meet: meet.Name, Dates: domain.DateRangeText(meet.StartDate, meet.EndDate), Location: meet.Location, PrintedAt: domain.RecordedText(s.store.Now(), s.zone), TeamScores: table})
	}
	book, err := s.reportBook(ctx, meetID, eventIDs, kind == ReportResults && prefs.PrintResultsTime, kind == ReportResults)
	if err != nil {
		return "", err
	}
	if kind == ReportResults && prefs.PrintTeamScores {
		book.TeamScores, err = s.teamScoreTable(ctx, meetID, eventIDs)
		if err != nil {
			return "", err
		}
	}
	book.NewPage = prefs.PrintNewPage
	if kind != ReportStartList || !prefs.PrintSchedule {
		book.Schedule = nil
	}
	if kind != ReportStartList || !prefs.PrintRoster {
		book.Rosters = nil
	}
	switch kind {
	case ReportStartList:
		return report.StartList(*book)
	case ReportResults:
		return report.Results(*book)
	}
	return "", invalid("报表类型不正确")
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
	book.Schedule = scheduleBook(newMeetPlan(*meet, events, rounds, teams, athletes, entries, 0).timetable(only))
	for _, e := range domain.ScheduleOrder(events, rounds, meet.StartDate) {
		if len(only) > 0 && !only[e.ID] {
			continue
		}
		date := domain.PlannedDay(e, rounds[e.ID], meet.StartDate)
		st.event = e
		rules := e.Rules()
		ev := report.Event{Name: e.Name, Date: date, PositionLabel: rules.PositionLabel, AttemptLabel: rules.AttemptLabel, Configuration: rules.StartLabel, Relay: e.IsTeam(), RosterLabel: "成员"}
		if e.Kind == domain.KindRelay {
			ev.RosterLabel = "棒次"
		}
		if rules.AttemptLabel != "" {
			ev.Configuration = fmt.Sprintf("%s %d 次", rules.AttemptLabel, e.AttemptCount)
		}
		if e.Kind == domain.KindRelay && e.RelayLegCount > 0 {
			ev.Configuration += fmt.Sprintf(" · 建议 %d 棒", e.RelayLegCount)
		}
		if e.IsTeam() && e.Kind != domain.KindRelay && e.TeamSize > 0 {
			ev.Configuration += fmt.Sprintf(" · 建议 %d 人", e.TeamSize)
		}
		if e.Kind.Scalar() {
			method := map[string]string{"single": "一次", "best": "取最好", "sum": "求和"}[e.AttemptMode]
			ev.Configuration = e.Kind.Label() + " · " + method
			if e.MultipleAttempts() {
				ev.Configuration += fmt.Sprintf(" · %d 次", e.AttemptCount)
			}
		}
		line := func(sl domain.Slot) report.Line {
			a, _ := st.competitor(sl.CompetitorID())
			l := report.Line{Position: sl.Position, Bib: a.Bib, Name: a.Name, Unit: st.unitName(a.UnitID)}
			if sl.TeamID != "" {
				for i, id := range st.teams[sl.TeamID].MemberIDs {
					l.Members = append(l.Members, teamMemberLabel(e, i, st.athletes[id].Name))
				}
			}
			if seed := seeds[e.ID+"|"+a.ID]; seed != nil {
				l.Seed = scoring.FormatEventMark(e, *seed)
			}
			if r := sl.Result; r != nil && withResults {
				l.Mark, l.Qualify = r.Status.Label(), r.Qualify
				for _, a := range r.Attempts {
					l.Attempts = append(l.Attempts, scoring.FormatEventAttempt(e, a))
				}
				if r.Status == domain.StatusOK && r.Mark != nil {
					l.Mark = scoring.FormatEventMark(e, *r.Mark)
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
		book.Events = append(book.Events, ev)
	}
	for _, parent := range parents {
		if len(only) > 0 && !only[parent.Combined.ID] {
			continue
		}
		c := report.Combined{Name: parent.Combined.Name, State: "暂列排名", Tables: []report.CombinedTable{}, Roster: []report.Line{}}
		for _, id := range parent.AthleteIDs {
			a := st.athletes[id]
			c.Roster = append(c.Roster, report.Line{Name: a.Name, Bib: a.Bib, Unit: st.unitName(a.UnitID)})
		}
		if !withResults {
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
			headers = append(headers, part.Event.Discipline+" / 积分")
		}

		for _, row := range d.Rows {
			r := report.CombinedRow{Name: row.Name, Bib: row.Bib, Unit: row.UnitName, Total: strconv.FormatInt(row.Total, 10), Cells: []string{}}
			if row.Rank != nil {
				r.Rank = strconv.Itoa(*row.Rank)
			}
			for _, cell := range row.Cells {
				text := "待录"
				if !cell.Pending {
					if cell.Excluded {
						text = cell.MarkText + " / 不列名次"
					} else {
						text = cell.MarkText + " / " + strconv.FormatInt(*cell.Points, 10)
					}
				}
				r.Cells = append(r.Cells, text)
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
// and time; rounds with no time are not printed.
func scheduleBook(t *Timetable) []report.ScheduleDay {
	rows := func(list []TimetableRound) []report.ScheduleRow {
		out := make([]report.ScheduleRow, 0, len(list))
		for _, r := range list {
			out = append(out, report.ScheduleRow{Time: r.StartText, Event: r.EventName, Round: r.RoundLabel, Heats: r.HeatCount, People: r.People})
		}
		return out
	}
	var out []report.ScheduleDay
	for _, d := range t.Days {
		out = append(out, report.ScheduleDay{Day: d.Day, Rows: rows(d.Rounds)})
	}
	return out
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
		u := report.UnitRoster{Name: name, Count: len(list)}
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
	ID   string `json:"id"`
	Name string `json:"name"`
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
			out = append(out, ReportTarget{ID: e.ID, Name: e.Name})
		}
	}
	for _, p := range parents {
		out = append(out, ReportTarget{ID: p.Combined.ID, Name: p.Combined.Name})
	}
	return out, nil
}

func (s *Service) teamScoreTable(ctx context.Context, meetID string, ids []string) (*report.TeamScoreTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Report selections may contain owned component IDs; only their parent counts.
	st, err := s.loadTeamScoreState(ctx, meetID)
	if err != nil {
		return nil, err
	}
	targets := []string{}
	for _, t := range st.targets {
		if len(ids) == 0 || slices.Contains(ids, t.id()) {
			targets = append(targets, t.id())
		}
	}
	// An explicit selection without a scoring target must not expand to the full meet.
	if !st.rules.Enabled || (len(ids) > 0 && len(targets) == 0) {
		return nil, nil
	}
	d, err := st.detail(TeamScoreFilter{MeetID: meetID, TargetIDs: targets}, st.rules)
	if err != nil {
		return nil, err
	}
	table := &report.TeamScoreTable{State: d.State, Counted: d.Counted, Total: d.Total, Scope: "全部项目", Rows: []report.TeamScoreRow{}}
	if len(ids) > 0 {
		table.Scope = "所选项目"
		for _, t := range d.Sources {
			if t.Multiplier > 0 {
				table.Targets = append(table.Targets, t.Name)
			}
		}
	}
	for _, r := range d.Rows {
		row := report.TeamScoreRow{Unit: r.UnitName, Male: r.Male.Text, Female: r.Female.Text, Mixed: r.Mixed.Text, Total: r.Total.Text}
		if r.Rank != nil {
			row.Rank = strconv.Itoa(*r.Rank)
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}
