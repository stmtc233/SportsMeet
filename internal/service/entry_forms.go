package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/entryform"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// EntryFormFile is one blank 报名登记表.
type EntryFormFile struct {
	Name string
	Data []byte
}

// EntryForm is a filled 报名登记表 as read from a file. Problem says why
// the file cannot be imported at all; "" = none.
type EntryForm struct {
	File    string `json:"file"`
	Problem string `json:"problem,omitempty"`
	entryform.Form
}

// EntryFormPreview says what importing some forms would do, form by form.
type EntryFormPreview struct {
	Forms   []EntryFormResult `json:"forms"`
	Creates int               `json:"creates"`
	Updates int               `json:"updates"`
	Entries int               `json:"entries"`
	Teams   int               `json:"teams"`
	Errors  int               `json:"errors"`
}

// EntryFormResult is one form of the preview: its athlete rows and team
// rows, or the problem that keeps the whole form out.
type EntryFormResult struct {
	File     string      `json:"file"`
	Unit     string      `json:"unit"`            // "" on a group's form
	Group    string      `json:"group,omitempty"` // a group's form
	Problem  string      `json:"problem,omitempty"`
	Rows     []ImportRow `json:"rows"`
	TeamRows []ImportRow `json:"teamRows"`
}

// EntryForms writes the blank forms: one per unit, or with byGroup one per
// group listing the group's units, and one more for the units of no group
// (id ""); only those of ids unless it is empty. A form lists the individual and team
// events of its groups and of no group, men's before women's before mixed.
func (s *Service) EntryForms(ctx context.Context, meetID string, ids []string, byGroup bool) ([]EntryFormFile, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	gender := map[domain.Gender]int{domain.GenderMale: 0, domain.GenderFemale: 1, domain.GenderMixed: 2}
	slices.SortStableFunc(events, func(a, b domain.Event) int { return gender[a.Gender] - gender[b.Gender] })
	var notes []string
	if n := meet.EntryLimits.AthleteEvents; n > 0 {
		notes = append(notes, fmt.Sprintf("每人限报 %d 项", n))
	}
	if n := meet.EntryLimits.UnitAthletes; n > 0 {
		notes = append(notes, fmt.Sprintf("每项限报 %d 人", n))
	}
	if n := meet.EntryLimits.UnitTeams; n > 0 {
		notes = append(notes, fmt.Sprintf("每项限报 %d 队", n))
	}
	// spec lists the events of groupIDs and of no group; every event when
	// groupIDs is empty.
	spec := func(groupIDs []string) entryform.Spec {
		sp := entryform.Spec{Meet: meet.Name, Note: strings.Join(notes, "　")}
		for _, e := range events {
			if e.CombinedID != "" || (e.GroupID != "" && len(groupIDs) > 0 && !slices.Contains(groupIDs, e.GroupID)) {
				continue
			}
			if !e.IsTeam() {
				sp.Events = append(sp.Events, e.Name)
				continue
			}
			size := e.RelayLegCount
			if e.Kind != domain.KindRelay {
				size = e.TeamSize
			}
			if size <= 0 {
				size = 4
			}
			sp.Teams = append(sp.Teams, entryform.TeamSpec{Event: e.Name, Size: size, Count: max(1, meet.EntryLimits.UnitTeams)})
		}
		return sp
	}
	var out []EntryFormFile
	names := map[string]int{}
	add := func(name string, sp entryform.Spec) error {
		data, err := entryform.Build(sp)
		if err != nil {
			return err
		}
		name = fileName(name) + "-报名登记表"
		if names[name]++; names[name] > 1 {
			name = fmt.Sprintf("%s-%d", name, names[name])
		}
		out = append(out, EntryFormFile{Name: name + ".xlsx", Data: data})
		return nil
	}
	if byGroup {
		groups, err := s.store.ListGroups(ctx, meetID)
		if err != nil {
			return nil, err
		}
		if len(groups) == 0 {
			return nil, invalid("还没有组别")
		}
		var none []string
		for _, u := range units {
			if len(u.GroupIDs) == 0 {
				none = append(none, u.Name)
			}
		}
		for _, g := range groups {
			if len(ids) > 0 && !slices.Contains(ids, g.ID) {
				continue
			}
			sp := spec([]string{g.ID})
			sp.Group = g.Name
			for _, u := range units {
				if slices.Contains(u.GroupIDs, g.ID) {
					sp.Units = append(sp.Units, u.Name)
				}
			}
			if len(sp.Units) == 0 {
				continue
			}
			if err := add(g.Name, sp); err != nil {
				return nil, err
			}
		}
		if len(none) > 0 && (len(ids) == 0 || slices.Contains(ids, "")) {
			sp := spec(nil)
			sp.Group, sp.Units = entryform.NoGroup, none
			if err := add(entryform.NoGroup, sp); err != nil {
				return nil, err
			}
		}
	} else {
		for _, u := range units {
			if len(ids) > 0 && !slices.Contains(ids, u.ID) {
				continue
			}
			sp := spec(u.GroupIDs)
			sp.Unit = u.Name
			if err := add(u.Name, sp); err != nil {
				return nil, err
			}
		}
	}
	if len(out) == 0 {
		return nil, invalid("还没有参赛单位")
	}
	return out, nil
}

// fileName keeps a name usable as a file name on every system.
func fileName(name string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-").Replace(name)
}

// ReadEntryForm reads one picked file; a file that is not a form comes
// back with its Problem set, so the others can still be previewed.
func ReadEntryForm(file string, data []byte) EntryForm {
	f, err := entryform.Parse(data)
	if err != nil {
		msg := "文件无法读取"
		if errors.Is(err, entryform.ErrNotForm) {
			msg = "不是报名登记表"
		}
		return EntryForm{File: file, Problem: msg, Form: entryform.Form{Athletes: []entryform.FormAthlete{}, Teams: []entryform.FormTeam{}}}
	}
	return EntryForm{File: file, Form: *f}
}

// findUnit finds a unit by its name or short name.
func findUnit(units []domain.Unit, name string) *domain.Unit {
	key := domain.NormalizeText(name)
	if key == "" {
		return nil
	}
	var found *domain.Unit
	for i, u := range units {
		if domain.NormalizeText(u.Name) == key {
			return &units[i]
		}
		if domain.NormalizeText(u.ShortName) == key {
			found = &units[i]
		}
	}
	return found
}

// planEntryForms checks each form against the meet, then plans all forms
// that pass as one table import with their team rows.
func (s *Service) planEntryForms(ctx context.Context, meetID string, forms []EntryForm) (*EntryFormPreview, *athleteImportPlan, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, nil, err
	}
	if len(forms) == 0 {
		return nil, nil, invalid("请选择报名登记表")
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, nil, err
	}
	groups, err := s.store.ListGroups(ctx, meetID)
	if err != nil {
		return nil, nil, err
	}
	preview := &EntryFormPreview{Forms: []EntryFormResult{}}
	in := AthleteImport{Mapping: []string{ColName, ColGender, ColStudentNumber, ColUnit, ColGroup}, Rows: [][]string{}}
	rows := &formRows{}
	var planned []int           // indexes into preview.Forms
	seen := map[string]string{} // unit id or group:<id> → file
	for _, f := range forms {
		res := EntryFormResult{File: f.File, Unit: f.Unit, Group: f.Group, Problem: f.Problem, Rows: []ImportRow{}, TeamRows: []ImportRow{}}
		// A group's form names its group; its rows name their units.
		var unit *domain.Unit
		var group *domain.CompetitionGroup
		byGroup := f.Group != ""
		key := ""
		if byGroup {
			for i, g := range groups {
				if domain.NormalizeText(g.Name) == domain.NormalizeText(f.Group) {
					group = &groups[i]
				}
			}
			if group != nil {
				key = "group:" + group.ID
			} else if f.Group == entryform.NoGroup {
				key = "group:"
			}
		} else if unit = findUnit(units, f.Unit); unit != nil {
			key = unit.ID
		}
		switch {
		case res.Problem != "":
		case domain.NormalizeText(f.Meet) != domain.NormalizeText(meet.Name):
			res.Problem = fmt.Sprintf("这是「%s」的表", f.Meet)
		case byGroup && key == "":
			res.Problem = fmt.Sprintf("没有组别「%s」", f.Group)
		case key == "":
			res.Problem = fmt.Sprintf("没有参赛单位「%s」", f.Unit)
		case seen[key] != "" && byGroup:
			res.Problem = fmt.Sprintf("与「%s」是同一组别", seen[key])
		case seen[key] != "":
			res.Problem = fmt.Sprintf("与「%s」是同一单位", seen[key])
		case len(f.Athletes) == 0 && len(f.Teams) == 0:
			res.Problem = "表里还没有填写"
		}
		if res.Problem != "" {
			preview.Errors++
			preview.Forms = append(preview.Forms, res)
			continue
		}
		seen[key] = f.File
		groupName := ""
		if group != nil {
			groupName, res.Group = group.Name, group.Name
		}
		// rowUnit is the unit a row belongs to, or why it has none.
		rowUnit := func(name string) (string, string) {
			if !byGroup {
				return unit.Name, ""
			}
			u := findUnit(units, name)
			switch {
			case name == "":
				return "", "请填写参赛单位"
			case u == nil:
				return name, fmt.Sprintf("没有参赛单位「%s」", name)
			case group != nil && !slices.Contains(u.GroupIDs, group.ID):
				return u.Name, fmt.Sprintf("「%s」不属于「%s」", u.Name, group.Name)
			}
			return u.Name, ""
		}
		if unit != nil {
			res.Unit = unit.Name
		}
		for _, a := range f.Athletes {
			name, problem := rowUnit(a.Unit)
			if a.Problem != "" {
				problem = a.Problem
			}
			in.Rows = append(in.Rows, []string{a.Name, a.Gender, a.StudentNumber, name, groupName})
			rows.lines = append(rows.lines, a.Line)
			rows.problems = append(rows.problems, problem)
			rows.events = append(rows.events, a.Events)
			rows.units = append(rows.units, name)
		}
		for _, t := range f.Teams {
			name, problem := rowUnit(t.Unit)
			rows.teams = append(rows.teams, formTeam{line: t.Line, unit: name, event: t.Event, members: t.Members, problem: problem})
		}
		planned = append(planned, len(preview.Forms))
		preview.Forms = append(preview.Forms, res)
	}
	plan, err := s.planAthleteImport(ctx, meetID, in, rows)
	if err != nil {
		return nil, nil, err
	}
	// Each athlete row and each team row gives exactly one preview row,
	// in order, so the plan's rows split back into the forms.
	p := plan.preview
	r, t := 0, 0
	for _, i := range planned {
		form := forms[i]
		res := &preview.Forms[i]
		res.Rows = p.Rows[r : r+len(form.Athletes)]
		res.TeamRows = p.TeamRows[t : t+len(form.Teams)]
		if form.Group != "" {
			for k := range res.Rows {
				res.Rows[k].Unit = rows.units[r+k]
			}
			for k := range res.TeamRows {
				res.TeamRows[k].Unit = rows.teams[t+k].unit
			}
		}
		r, t = r+len(form.Athletes), t+len(form.Teams)
	}
	preview.Creates, preview.Updates, preview.Entries, preview.Teams = p.Creates, p.Updates, p.Entries, p.Teams
	preview.Errors += p.Errors
	return preview, plan, nil
}

// PreviewEntryForms reports what importing the forms would do.
func (s *Service) PreviewEntryForms(ctx context.Context, meetID string, forms []EntryForm) (*EntryFormPreview, error) {
	preview, _, err := s.planEntryForms(ctx, meetID, forms)
	return preview, err
}

// ImportEntryForms imports the forms in one transaction: athletes, entries
// and teams. Anything in error stops all of it.
func (s *Service) ImportEntryForms(ctx context.Context, src store.Source, meetID string, forms []EntryForm) (*EntryFormPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, plan, err := s.planEntryForms(ctx, meetID, forms)
	if err != nil {
		return nil, err
	}
	if preview.Errors > 0 {
		return nil, invalid("有 %d 处有误，请修正后再导入", preview.Errors)
	}
	if len(plan.stmts) == 0 {
		return preview, nil
	}
	err = s.write(ctx,
		store.Audit{Source: src, Action: "athlete.imported", MeetID: meetID,
			Message: fmt.Sprintf("导入 %d 份报名登记表：新建 %d 人，更新 %d 人", len(forms), preview.Creates, preview.Updates),
			Detail:  map[string]int{"forms": len(forms), "creates": preview.Creates, "updates": preview.Updates, "entries": preview.Entries, "teams": preview.Teams}},
		Change{Kind: "athletes", MeetID: meetID}, plan.stmts...)
	if err != nil {
		return nil, err
	}
	return preview, nil
}
