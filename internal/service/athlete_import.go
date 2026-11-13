package service

import (
	"context"
	"encoding/csv"
	"fmt"
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
	r.TrimLeadingSpace = true
	switch {
	case strings.Contains(first, "\t"):
		r.Comma = '\t'
	case !strings.Contains(first, ",") && strings.Contains(first, "，"):
		r.Comma = '，'
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
	Action  string `json:"action"` // create | update | error
	Name    string `json:"name"`
	Message string `json:"message"` // the error, or a warning
}

// AthleteImportPreview says what an import would do.
type AthleteImportPreview struct {
	Rows     []ImportRow `json:"rows"`
	Creates  int         `json:"creates"`
	Updates  int         `json:"updates"`
	Errors   int         `json:"errors"`
	NewUnits []string    `json:"newUnits"`
	Entries  int         `json:"entries"` // new entries
}

type athleteImportPlan struct {
	preview AthleteImportPreview
	stmts   []sqlite.Statement
}

var eventSeparators = strings.NewReplacer("，", "\n", ",", "\n", "、", "\n", ";", "\n", "；", "\n", "/", "\n", "|", "\n")

// planAthleteImport validates every row and builds the statements of the
// import; the preview and the import share it, so they cannot disagree.
func (s *Service) planAthleteImport(ctx context.Context, meetID string, in AthleteImport) (*athleteImportPlan, error) {
	if _, err := s.GetMeet(ctx, meetID); err != nil {
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
	groupByName := map[string]string{}
	for _, g := range groups {
		groupByName[domain.NormalizeText(g.Name)] = g.ID
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
	touched := map[string]int{} // existing athlete id → line that updates it
	var unitStmts, stmts []sqlite.Statement

	for i, row := range in.Rows {
		line := i + 1
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
		var eventIDs []string
		bad := ""
		for _, token := range strings.Split(eventSeparators.Replace(cell(ColEvents)), "\n") {
			if token = strings.TrimSpace(token); token == "" {
				continue
			}
			id := matchEvent(events, token, a.Gender, a.GroupID)
			if id == "" {
				bad = token
				break
			}
			eventIDs = append(eventIDs, id)
		}
		if bad != "" {
			fail("找不到项目「%s」", bad)
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
		for _, eventID := range eventIDs {
			if k := eventID + "|" + a.ID; !entered[k] {
				entered[k] = true
				p.Entries++
				stmts = append(stmts, store.Stmt(`INSERT INTO entries(event_id, athlete_id) VALUES (?, ?)`, eventID, a.ID))
			}
		}
		for _, id := range eventIDs {
			for _, e := range events {
				if e.ID == id && e.GroupID != "" && a.GroupID != "" && e.GroupID != a.GroupID {
					out.Message = "跨组报名"
				}
			}
		}
		if a.GroupID == "" && len(groups) > 0 && out.Message == "" {
			out.Message = "未指定参赛组别"
		}
		p.Rows = append(p.Rows, out)
	}
	plan.stmts = append(unitStmts, stmts...)
	return plan, nil
}

// matchEvent finds the event a cell names: its full name, or its
// discipline among the events of the athlete's gender (and mixed ones)
// when that is unambiguous. Relays take teams, not athletes.
func matchEvent(events []domain.Event, token string, gender domain.Gender, groupID ...string) string {
	name, disc := domain.NormalizeText(token), domain.DisciplineKey(token)
	exact, candidates := []domain.Event{}, []domain.Event{}
	for _, e := range events {
		if e.IsTeam() || e.CombinedID != "" {
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
		return exact[0].ID
	}
	if len(exact) > 1 {
		candidates = exact
	}
	found := ""
	for _, e := range candidates {
		if e.Gender != gender && e.Gender != domain.GenderMixed {
			continue
		}
		if len(groupID) > 0 && groupID[0] != "" && e.GroupID != groupID[0] {
			continue
		}
		if found != "" {
			return ""
		}
		found = e.ID
	}
	return found
}

// PreviewAthleteImport reports what importing the table would do.
func (s *Service) PreviewAthleteImport(ctx context.Context, meetID string, in AthleteImport) (*AthleteImportPreview, error) {
	plan, err := s.planAthleteImport(ctx, meetID, in)
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
	plan, err := s.planAthleteImport(ctx, meetID, in)
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
			Detail:  map[string]int{"creates": p.Creates, "updates": p.Updates, "units": len(p.NewUnits), "entries": p.Entries}},
		Change{Kind: "athletes", MeetID: meetID}, plan.stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}
