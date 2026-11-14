package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Columns an athlete table can map to.
const (
	ColName          = "name"
	ColGender        = "gender"
	ColBib           = "bib"
	ColStudentNumber = "studentNumber"
	ColUnit          = "unit"
	ColClassName     = "className"
	ColGrade         = "grade"
	ColGroup         = "group"
	ColBirthDate     = "birthDate"
	ColNote          = "note"
	ColEvents        = "events" // event names, separated by 、 , ; or /
)

var headerColumns = map[string]string{
	"姓名": ColName, "名字": ColName, "运动员": ColName, "name": ColName,
	"性别": ColGender, "gender": ColGender, "sex": ColGender,
	"号码": ColBib, "号码布": ColBib, "号码簿": ColBib, "参赛号": ColBib, "bib": ColBib,
	"学号": ColStudentNumber, "证件号": ColStudentNumber, "学籍号": ColStudentNumber, "编号": ColStudentNumber,
	"单位": ColUnit, "参赛单位": ColUnit, "代表队": ColUnit, "学校": ColUnit, "学院": ColUnit, "队伍": ColUnit, "unit": ColUnit,
	"班级": ColClassName, "班": ColClassName, "class": ColClassName,
	"年级": ColGrade, "grade": ColGrade,
	"参赛组别": ColGroup, "组别": ColGroup, "group": ColGroup,
	"出生日期": ColBirthDate, "生日": ColBirthDate,
	"备注": ColNote, "note": ColNote,
	"项目": ColEvents, "报名项目": ColEvents, "参赛项目": ColEvents,
}

// Table is pasted or loaded text split into cells.
type Table struct {
	Rows [][]string `json:"rows"`
	// HasHeader is true when the first row names columns.
	HasHeader bool `json:"hasHeader"`
	// Mapping gives each column's field ("" = ignored), guessed from the
	// header row.
	Mapping []string `json:"mapping"`
}

// ParseTable splits text copied from a spreadsheet (tabs) or a CSV file
// (commas) into rows, and guesses the columns from a header row.
func ParseTable(text string) (Table, error) {
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\xef\xbb\xbf")
	first, _, _ := strings.Cut(strings.TrimLeft(text, "\n"), "\n")
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	switch {
	case strings.Contains(first, "\t"):
		// Leading-space trimming would also eat the tabs of empty cells and
		// shift the rest of the row; cells are trimmed below instead.
		r.Comma = '\t'
	case !strings.Contains(first, ",") && strings.Contains(first, "，"):
		r.Comma = '，'
		r.TrimLeadingSpace = true
	default:
		r.TrimLeadingSpace = true
	}
	records, err := r.ReadAll()
	if err != nil {
		return Table{}, invalid("表格内容无法识别")
	}
	t := Table{Rows: [][]string{}}
	width := 0
	for _, rec := range records {
		blank := true
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
			blank = blank && rec[i] == ""
		}
		if !blank {
			t.Rows = append(t.Rows, rec)
			width = max(width, len(rec))
		}
	}
	for i, rec := range t.Rows {
		for len(rec) < width {
			rec = append(rec, "")
		}
		t.Rows[i] = rec
	}
	t.Mapping = make([]string, width)
	if len(t.Rows) > 0 {
		used := map[string]bool{}
		for i, cell := range t.Rows[0] {
			if col := headerColumns[domain.NormalizeText(cell)]; col != "" && !used[col] {
				t.Mapping[i], used[col], t.HasHeader = col, true, true
			}
		}
	}
	return t, nil
}

// AthleteImport is a table with its column mapping; Rows are data rows.
type AthleteImport struct {
	Rows    [][]string `json:"rows"`
	Mapping []string   `json:"mapping"`
}

// ImportRow is one row of the preview.
type ImportRow struct {
	Line    int    `json:"line"`   // 1-based, among the data rows
	Action  string `json:"action"` // create | update | keep | error
	Name    string `json:"name"`
	Message string `json:"message"`        // the error, or a warning
	Unit    string `json:"unit,omitempty"` // a group's entry form names each row's unit
}

// AthleteImportPreview says what an import would do.
type AthleteImportPreview struct {
	Rows     []ImportRow `json:"rows"`
	Creates  int         `json:"creates"`
	Updates  int         `json:"updates"`
	Errors   int         `json:"errors"`
	NewUnits []string    `json:"newUnits"`
	Entries  int         `json:"entries"` // new entries
	Teams    int         `json:"teams"`   // new teams, each entered
	// TeamRows are the rows of an entry form's team sheet.
	TeamRows []ImportRow `json:"teamRows,omitempty"`
}

// formRows is what an entry form adds to a table: the sheet's line of each
// row, problems found while reading a row, the full names of the events a
// row ticks, and the teams of the team sheet.
type formRows struct {
	lines    []int
	problems []string
	events   [][]string
	units    []string // each row's unit, as the form names it
	teams    []formTeam
}

// formTeam is one row of the team sheet: an event, the unit's name as the
// table gives it, and members by name or student number in roster order.
type formTeam struct {
	line    int
	unit    string
	event   string
	members []string
	problem string // why the row cannot be imported; "" = none
}

type athleteImportPlan struct {
	preview AthleteImportPreview
	stmts   []sqlite.Statement
}

var eventSeparators = strings.NewReplacer("，", "\n", ",", "\n", "、", "\n", ";", "\n", "；", "\n", "/", "\n", "|", "\n")

// planAthleteImport validates every row and builds the statements of the
// import; the preview and the import share it, so they cannot disagree.
// form is nil for a pasted table.
func (s *Service) planAthleteImport(ctx context.Context, meetID string, in AthleteImport, form *formRows) (*athleteImportPlan, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, m := range in.Mapping {
		if m == "" {
			continue
		}
		if _, dup := col[m]; dup {
			return nil, invalid("有两列对应同一个字段")
		}
		col[m] = i
	}
	if _, ok := col[ColName]; !ok {
		return nil, invalid("请指定「姓名」列")
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	sc := store.Scope{MeetID: meetID}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}

	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, err
	}
	groupByName, groupNames := map[string]string{}, map[string]string{}
	for _, g := range groups {
		groupByName[domain.NormalizeText(g.Name)] = g.ID
		groupNames[g.ID] = g.Name
	}
	byNumber, byBib, byName := map[string]*domain.Athlete{}, map[string]*domain.Athlete{}, map[string]bool{}
	for i := range athletes {
		a := &athletes[i]
		if k := domain.NormalizeText(a.StudentNumber); k != "" {
			byNumber[k] = a
		}
		if k := domain.NormalizeText(a.Bib); k != "" {
			byBib[k] = a
		}
		byName[domain.NormalizeText(a.Name)] = true
	}
	unitIDs := map[string]string{}
	unitGroups := map[string][]string{}
	ambiguousUnits := map[string]bool{}
	for _, u := range units {
		unitGroups[u.ID] = u.GroupIDs
		unitIDs[domain.NormalizeText(u.Name)] = u.ID
		if u.ShortName != "" {
			key := domain.NormalizeText(u.ShortName)
			if prev := unitIDs[key]; prev != "" && prev != u.ID {
				ambiguousUnits[key] = true
			}
			unitIDs[key] = u.ID
		}
	}
	entered := map[string]bool{}
	for _, e := range entries {
		entered[e.EventID+"|"+e.AthleteID] = true
	}

	plan := &athleteImportPlan{preview: AthleteImportPreview{Rows: []ImportRow{}, NewUnits: []string{}}}
	p := &plan.preview
	now := s.store.Now()
	seenNumber, seenBib := map[string]int{}, map[string]int{}
	seenName := map[string]bool{}        // unit|name of form rows with neither id
	touched := map[string]int{}          // existing athlete id → line that updates it
	saved := map[string]domain.Athlete{} // athletes as the import leaves them
	var unitStmts, stmts []sqlite.Statement
	var limited []importLimitRow

	for i, row := range in.Rows {
		line := i + 1
		if form != nil {
			line = form.lines[i]
		}
		cell := func(name string) string {
			if c, ok := col[name]; ok && c < len(row) {
				return strings.TrimSpace(row[c])
			}
			return ""
		}
		out := ImportRow{Line: line, Name: cell(ColName)}
		fail := func(format string, a ...any) {
			out.Action, out.Message = "error", fmt.Sprintf(format, a...)
			p.Errors++
			p.Rows = append(p.Rows, out)
		}
		if form != nil && form.problems[i] != "" {
			fail("%s", form.problems[i])
			continue
		}
		input := AthleteInput{Name: cell(ColName), Gender: cell(ColGender), Bib: cell(ColBib),
			StudentNumber: cell(ColStudentNumber), ClassName: cell(ColClassName), Grade: cell(ColGrade),
			BirthDate: strings.ReplaceAll(cell(ColBirthDate), "/", "-"), Note: cell(ColNote)}
		a, err := input.athlete()
		if err != nil {
			fail("%s", err)
			continue
		}
		num, bib := domain.NormalizeText(a.StudentNumber), domain.NormalizeText(a.Bib)
		if n := seenNumber[num]; num != "" && n > 0 {
			fail("学号与第 %d 行重复", n)
			continue
		}
		if n := seenBib[bib]; bib != "" && n > 0 {
			fail("号码与第 %d 行重复", n)
			continue
		}

		// An athlete already in the meet is found by student number, else
		// by bib; the row then updates it instead of creating a second one.
		var existing *domain.Athlete
		switch {
		case num != "":
			existing = byNumber[num]
		case bib != "":
			existing = byBib[bib]
		}
		// A form names its unit, so a row without either id is the unit's
		// athlete of that name: the same form imported again adds no one.
		if form != nil && num == "" && bib == "" {
			k := domain.NormalizeText(cell(ColUnit)) + "|" + domain.NormalizeText(a.Name)
			if seenName[k] {
				fail("同一单位有两行同名，请填写学号")
				continue
			}
			seenName[k] = true
		}
		if existing == nil && form != nil && num == "" && bib == "" {
			unit, name := unitIDs[domain.NormalizeText(cell(ColUnit))], domain.NormalizeText(a.Name)
			var same []*domain.Athlete
			for i := range athletes {
				if athletes[i].UnitID == unit && domain.NormalizeText(athletes[i].Name) == name {
					same = append(same, &athletes[i])
				}
			}
			if len(same) > 1 {
				fail("本单位有同名运动员，请填写学号")
				continue
			}
			if len(same) == 1 {
				existing = same[0]
			}
		}
		if existing != nil {
			if n := touched[existing.ID]; n > 0 {
				fail("与第 %d 行是同一名运动员", n)
				continue
			}
			if other := byBib[bib]; bib != "" && other != nil && other.ID != existing.ID {
				fail("号码 %s 已属于「%s」", a.Bib, other.Name)
				continue
			}
		} else if other := byBib[bib]; bib != "" && other != nil {
			fail("号码 %s 已属于「%s」", a.Bib, other.Name)
			continue
		}

		if name := cell(ColGroup); name != "" {
			a.GroupID = groupByName[domain.NormalizeText(name)]
			if a.GroupID == "" {
				fail("组别「%s」不存在", name)
				continue
			}
		} else if existing != nil {
			a.GroupID = existing.GroupID
		}
		unitKey := domain.NormalizeText(cell(ColUnit))
		if ambiguousUnits[unitKey] {
			exact := ""
			for _, u := range units {
				if domain.NormalizeText(u.Name) == unitKey {
					exact = u.ID
				}
			}
			if exact == "" {
				fail("单位简称重复，请填写全称")
				continue
			}
			unitIDs[unitKey] = exact
		}
		unitID := unitIDs[unitKey]
		if unitID == "" && existing != nil {
			unitID = existing.UnitID
		}
		if a.GroupID == "" && len(unitGroups[unitID]) == 1 {
			a.GroupID = unitGroups[unitID][0]
		}
		gender := a.Gender
		if gender == "" && existing != nil {
			gender = existing.Gender
		}

		tokens := strings.Split(eventSeparators.Replace(cell(ColEvents)), "\n")
		if form != nil {
			tokens = form.events[i]
		}
		var eventIDs []string
		problem := ""
		for _, token := range tokens {
			if token = strings.TrimSpace(token); token == "" {
				continue
			}
			e, n := findEvent(events, token, gender, a.GroupID, false)
			if e == nil {
				problem = eventProblem(events, token, n, gender, a.GroupID, groupNames[a.GroupID], len(groups) > 0, false)
				break
			}
			if !slices.Contains(eventIDs, e.ID) {
				eventIDs = append(eventIDs, e.ID)
			}
		}
		if problem != "" {
			fail("%s", problem)
			continue
		}

		if name := cell(ColUnit); name != "" {
			key := domain.NormalizeText(name)
			if unitIDs[key] == "" {
				unitIDs[key] = newID()
				p.NewUnits = append(p.NewUnits, name)
				unitStmts = append(unitStmts, store.Stmt(`INSERT INTO units(id, meet_id, name) VALUES (?, ?, ?)`, unitIDs[key], meetID, name))
			}
			a.UnitID = unitIDs[key]
		}

		if num != "" {
			seenNumber[num] = line
		}
		if bib != "" {
			seenBib[bib] = line
		}
		if existing != nil {
			touched[existing.ID] = line
			merged := *existing // blank cells keep what the athlete has
			merged.Name = a.Name
			for _, f := range []struct{ dst, src *string }{
				{&merged.Bib, &a.Bib}, {&merged.StudentNumber, &a.StudentNumber}, {&merged.UnitID, &a.UnitID},
				{&merged.GroupID, &a.GroupID}, {&merged.ClassName, &a.ClassName}, {&merged.Grade, &a.Grade}, {&merged.BirthDate, &a.BirthDate}, {&merged.Note, &a.Note},
			} {
				if *f.src != "" {
					*f.dst = *f.src
				}
			}
			if a.Gender != "" {
				merged.Gender = a.Gender
			}
			a = merged
			out.Action = "update"
			p.Updates++
			stmts = append(stmts, updateAthlete(a))
		} else {
			a.ID, a.MeetID = newID(), meetID
			out.Action = "create"
			if byName[domain.NormalizeText(a.Name)] {
				out.Message = "已有同名运动员"
			}
			p.Creates++
			stmts = append(stmts, insertAthlete(a, now))
		}
		saved[a.ID] = a
		for _, id := range eventIDs {
			if k := id + "|" + a.ID; !entered[k] {
				entered[k] = true
				p.Entries++
				stmts = append(stmts, store.Stmt(`INSERT INTO entries(event_id, athlete_id) VALUES (?, ?)`, id, a.ID))
			}
			for _, e := range events {
				if e.ID == id && e.GroupID != "" && a.GroupID != "" && e.GroupID != a.GroupID {
					out.Message = "跨组报名"
				}
			}
		}
		if a.GroupID == "" && len(groups) > 0 && out.Message == "" {
			out.Message = "未指定参赛组别"
		}
		limited = append(limited, importLimitRow{row: len(p.Rows), athleteID: a.ID, unitID: a.UnitID, eventIDs: eventIDs})
		p.Rows = append(p.Rows, out)
	}

	var teamStmts []sqlite.Statement
	var newTeams []importLimitTeam
	if form != nil && len(form.teams) > 0 {
		for _, a := range athletes {
			if _, ok := saved[a.ID]; !ok {
				saved[a.ID] = a
			}
		}
		teams, err := s.store.ListTeams(ctx, sc)
		if err != nil {
			return nil, err
		}
		unitByID := map[string]domain.Unit{}
		for _, u := range units {
			unitByID[u.ID] = u
		}
		teamStmts, newTeams = planFormTeams(form.teams, events, teams, unitIDs, unitByID, saved, p)
	}
	if err := s.importLimitWarnings(ctx, meet, events, limited, newTeams, p); err != nil {
		return nil, err
	}
	plan.stmts = append(append(unitStmts, stmts...), teamStmts...)
	return plan, nil
}

// planFormTeams turns the team sheet into new teams, each entered. A unit
// that already has a team in an event keeps its teams as they are, so a
// form imported twice adds nothing; the rows that would add more say so.
func planFormTeams(rows []formTeam, events []domain.Event, teams []domain.Team, unitIDs map[string]string, unitByID map[string]domain.Unit, saved map[string]domain.Athlete, p *AthleteImportPreview) ([]sqlite.Statement, []importLimitTeam) {
	hasTeam, teamOf := map[string]bool{}, map[string]domain.Team{} // event|unit, event|athlete
	for _, t := range teams {
		hasTeam[t.EventID+"|"+t.UnitID] = true
		for _, m := range t.MemberIDs {
			teamOf[t.EventID+"|"+m] = t
		}
	}
	type draft struct {
		row  int // index into p.TeamRows
		team domain.Team
	}
	var drafts []draft
	count := map[string]int{} // event|unit → new teams
	for _, r := range rows {
		out := ImportRow{Line: r.line, Name: r.event}
		fail := func(format string, a ...any) {
			out.Action, out.Message = "error", fmt.Sprintf(format, a...)
			p.Errors++
			p.TeamRows = append(p.TeamRows, out)
		}
		if r.problem != "" {
			fail("%s", r.problem)
			continue
		}
		if r.event == "" {
			fail("请填写项目")
			continue
		}
		e, _ := findEvent(events, r.event, "", "", true)
		if e == nil || !e.IsTeam() || domain.NormalizeText(e.Name) != domain.NormalizeText(r.event) {
			fail("本赛事没有队伍项目「%s」", r.event)
			continue
		}
		unitID := unitIDs[domain.NormalizeText(r.unit)]
		key := e.ID + "|" + unitID
		if hasTeam[key] {
			out.Action, out.Message = "keep", "本单位已有队伍，未改动"
			for _, t := range teams {
				if t.EventID == e.ID && t.UnitID == unitID && sameMembers(t.MemberIDs, r.members, saved) {
					out.Message = ""
				}
			}
			p.TeamRows = append(p.TeamRows, out)
			continue
		}
		var ids, names []string
		problem := ""
		for _, m := range r.members {
			id, n := findMember(saved, unitID, m)
			switch {
			case n == 0:
				problem = fmt.Sprintf("本单位没有「%s」", m)
			case n > 1:
				problem = fmt.Sprintf("「%s」有同名，请写学号", m)
			case slices.Contains(ids, id):
				problem = fmt.Sprintf("「%s」重复", m)
			}
			if problem == "" {
				if t, ok := teamOf[e.ID+"|"+id]; ok {
					problem = fmt.Sprintf("「%s」已在「%s」", m, t.Name)
				}
			}
			if problem != "" {
				break
			}
			ids = append(ids, id)
			names = append(names, saved[id].Name)
			teamOf[e.ID+"|"+id] = domain.Team{Name: e.Name}
		}
		if problem != "" {
			fail("%s", problem)
			continue
		}
		count[key]++
		out.Action, out.Message = "create", strings.Join(names, "、")
		drafts = append(drafts, draft{row: len(p.TeamRows), team: domain.Team{ID: newID(), EventID: e.ID, UnitID: unitID, MemberIDs: ids}})
		p.TeamRows = append(p.TeamRows, out)
	}
	// One team takes the unit's name; several are numbered.
	var stmts []sqlite.Statement
	var created []importLimitTeam
	seen := map[string]int{}
	for _, d := range drafts {
		t := d.team
		key := t.EventID + "|" + t.UnitID
		seen[key]++
		t.Name = unitByID[t.UnitID].Name
		if count[key] > 1 {
			t.Name = fmt.Sprintf("%s %d队", t.Name, seen[key])
		}
		stmts = append(stmts, insertTeam(t)...)
		stmts = append(stmts, insertEntry(t.EventID, "", t.ID, nil))
		p.Teams++
		created = append(created, importLimitTeam{row: d.row, eventID: t.EventID, teamID: t.ID, unitID: t.UnitID})
	}
	return stmts, created
}

// findMember finds a unit's athlete by student number, else by name; it
// returns how many fit when not exactly one.
func findMember(athletes map[string]domain.Athlete, unitID, text string) (string, int) {
	key := domain.NormalizeText(text)
	var byNumber, byName []string
	for id, a := range athletes {
		if a.UnitID != unitID {
			continue
		}
		if key != "" && domain.NormalizeText(a.StudentNumber) == key {
			byNumber = append(byNumber, id)
		}
		if domain.NormalizeText(a.Name) == key {
			byName = append(byName, id)
		}
	}
	if len(byNumber) == 1 {
		return byNumber[0], 1
	}
	if len(byName) == 1 {
		return byName[0], 1
	}
	return "", len(byName)
}

// sameMembers reports whether a team's roster is the one a row writes.
func sameMembers(ids, members []string, athletes map[string]domain.Athlete) bool {
	if len(ids) != len(members) {
		return false
	}
	for i, id := range ids {
		a := athletes[id]
		m := domain.NormalizeText(members[i])
		if m != domain.NormalizeText(a.Name) && m != domain.NormalizeText(a.StudentNumber) {
			return false
		}
	}
	return true
}

// eventProblem says why a cell names no single event: the meet has none
// of that name, none for the athlete's gender or group, or several.
func eventProblem(events []domain.Event, token string, n int, gender domain.Gender, groupID, groupName string, grouped, teams bool) string {
	if n == 0 {
		if _, all := findEvent(events, token, "", "", teams); all == 0 {
			return fmt.Sprintf("本赛事没有项目「%s」", token)
		}
		if _, any := findEvent(events, token, gender, "", teams); any == 0 {
			return fmt.Sprintf("「%s」没有%s子项目", token, gender)
		}
		return fmt.Sprintf("「%s」没有%s的项目", token, groupName)
	}
	switch {
	case gender == "":
		return fmt.Sprintf("「%s」有多个项目，请填写性别", token)
	case groupID == "" && grouped:
		return fmt.Sprintf("「%s」有多个项目，请填写组别", token)
	}
	return fmt.Sprintf("「%s」有多个项目，请填写项目全名", token)
}

// importLimitRow is a row that will be saved, as the entry limits see it.
type importLimitRow struct {
	row       int
	athleteID string
	unitID    string
	eventIDs  []string
}

// importLimitTeam is a team an entry form creates; row indexes TeamRows.
type importLimitTeam struct {
	row                     int
	eventID, teamID, unitID string
}

// importLimitWarnings adds to the rows that enter events the limits the
// import would exceed; they never stop it.
func (s *Service) importLimitWarnings(ctx context.Context, meet *domain.Meet, events []domain.Event, rows []importLimitRow, teams []importLimitTeam, p *AthleteImportPreview) error {
	limits, err := s.loadEntryLimits(ctx, meet)
	if err != nil || !limits.limits.Any() {
		return err
	}
	unitOf := map[string]string{}
	for _, r := range rows {
		unitOf[r.athleteID] = r.unitID
	}
	for i, e := range limits.entries {
		if u, ok := unitOf[e.AthleteID]; ok && e.TeamID == "" {
			limits.entries[i].UnitID = u
		}
	}
	for _, r := range rows {
		for _, id := range r.eventIDs {
			limits.entries = append(limits.entries, domain.LimitEntry{Item: id, AthleteID: r.athleteID, UnitID: r.unitID})
		}
	}
	for _, t := range teams {
		limits.entries = append(limits.entries, domain.LimitEntry{Item: t.eventID, TeamID: t.teamID, UnitID: t.unitID})
	}
	report := limits.check()
	names := map[string]string{}
	for _, e := range events {
		names[e.ID] = e.Name
	}
	note := func(r *ImportRow, notes []string) {
		if len(notes) == 0 {
			return
		}
		if r.Message != "" {
			notes = append([]string{r.Message}, notes...)
		}
		r.Message = strings.Join(notes, "；")
	}
	for _, r := range rows {
		if len(r.eventIDs) == 0 {
			continue
		}
		var notes []string
		if n := report.Athletes[r.athleteID]; n > 0 {
			notes = append(notes, limits.athleteOverText(n))
		}
		for _, id := range r.eventIDs {
			if n := report.Units[domain.UnitItem{Item: id, UnitID: r.unitID}]; n > 0 {
				notes = append(notes, fmt.Sprintf("「%s」本单位%s", names[id], limits.unitOverText(n, false)))
			}
		}
		note(&p.Rows[r.row], notes)
	}
	for _, t := range teams {
		if n := report.Units[domain.UnitItem{Item: t.eventID, UnitID: t.unitID}]; n > 0 {
			note(&p.TeamRows[t.row], []string{"本单位" + limits.unitOverText(n, true)})
		}
	}
	return nil
}

// findEvent finds the event a cell names: its full name, or its
// discipline among the events of the athlete's gender (and mixed ones)
// and group, else among those with no group. Team events count only when teams is set. When no single
// event fits it returns nil and how many did.
func findEvent(events []domain.Event, token string, gender domain.Gender, groupID string, teams bool) (*domain.Event, int) {
	name, disc := domain.NormalizeText(token), domain.DisciplineKey(token)
	var exact, candidates []domain.Event
	for _, e := range events {
		if e.CombinedID != "" || (e.IsTeam() && !teams) {
			continue
		}
		if domain.NormalizeText(e.Name) == name {
			exact = append(exact, e)
		}
		if domain.DisciplineKey(e.Discipline) == disc {
			candidates = append(candidates, e)
		}
	}
	if len(exact) == 1 {
		return &exact[0], 1
	}
	if len(exact) > 1 {
		candidates = exact
	}
	// The athlete's own group first, else events with no group, which are
	// open to every group.
	var found, open []domain.Event
	for _, e := range candidates {
		if gender != "" && e.Gender != gender && e.Gender != domain.GenderMixed {
			continue
		}
		switch {
		case groupID == "" || e.GroupID == groupID:
			found = append(found, e)
		case e.GroupID == "":
			open = append(open, e)
		}
	}
	if len(found) == 0 {
		found = open
	}
	if len(found) == 1 {
		return &found[0], 1
	}
	return nil, len(found)
}

// matchEvent is findEvent among individual events, by id; "" = none.
func matchEvent(events []domain.Event, token string, gender domain.Gender, groupID ...string) string {
	g := ""
	if len(groupID) > 0 {
		g = groupID[0]
	}
	if e, _ := findEvent(events, token, gender, g, false); e != nil {
		return e.ID
	}
	return ""
}

// PreviewAthleteImport reports what importing the table would do.
func (s *Service) PreviewAthleteImport(ctx context.Context, meetID string, in AthleteImport) (*AthleteImportPreview, error) {
	plan, err := s.planAthleteImport(ctx, meetID, in, nil)
	if err != nil {
		return nil, err
	}
	return &plan.preview, nil
}

// ImportAthletes creates and updates athletes from a table, with their
// units and entries, in one transaction. Any row in error stops it.
func (s *Service) ImportAthletes(ctx context.Context, src store.Source, meetID string, in AthleteImport) (*AthleteImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planAthleteImport(ctx, meetID, in, nil)
	if err != nil {
		return nil, err
	}
	p := &plan.preview
	if p.Errors > 0 {
		return nil, invalid("有 %d 行有误，请修正后再导入", p.Errors)
	}
	if len(plan.stmts) == 0 {
		return p, nil
	}
	err = s.write(ctx,
		store.Audit{Source: src, Action: "athlete.imported", MeetID: meetID,
			Message: fmt.Sprintf("新建 %d 人，更新 %d 人", p.Creates, p.Updates),
			Detail:  map[string]int{"creates": p.Creates, "updates": p.Updates, "units": len(p.NewUnits), "entries": p.Entries, "teams": p.Teams}},
		Change{Kind: "athletes", MeetID: meetID}, plan.stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}
