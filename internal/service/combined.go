package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type PointRuleInput struct {
	Table           string            `json:"table"`
	InvalidPolicies map[string]string `json:"invalidPolicies"`
}
type CombinedComponentInput struct {
	EventID string         `json:"eventId,omitempty"`
	Event   EventInput     `json:"event"`
	Rule    PointRuleInput `json:"rule"`
}
type CombinedInput struct {
	Name       string                   `json:"name"`
	Gender     string                   `json:"gender"`
	GroupLabel string                   `json:"groupLabel,omitempty"`
	Note       string                   `json:"note,omitempty"`
	Components []CombinedComponentInput `json:"components"`
}
type CombinedComponentView struct {
	Event   domain.Event     `json:"event"`
	Ordinal int              `json:"ordinal"`
	Rule    domain.PointRule `json:"rule"`
	Input   PointRuleInput   `json:"input"`
}
type CombinedRow struct {
	GenderMismatch bool `json:"genderMismatch"`
	scoring.CombinedStanding
	Name     string `json:"name"`
	Bib      string `json:"bib"`
	UnitName string `json:"unitName"`
}
type CombinedDetail struct {
	Combined      domain.Combined         `json:"combined"`
	Components    []CombinedComponentView `json:"components"`
	AthleteIDs    []string                `json:"athleteIds"`
	Athletes      []domain.Athlete        `json:"athletes"`
	Rows          []CombinedRow           `json:"rows"`
	Complete      bool                    `json:"complete"`
	HasPlacements bool                    `json:"hasPlacements"`
	Heats         int                     `json:"heats"`
	Results       int                     `json:"results"`
}
type CombinedRowSummary struct {
	Combined   domain.Combined `json:"combined"`
	Components int             `json:"components"`
	Entries    int             `json:"entries"`
	Results    int             `json:"results"`
}

type combinedMeetState struct {
	parents  []store.CombinedData
	events   map[string]domain.Event
	rounds   map[string][]domain.Round
	athletes []domain.Athlete
	units    map[string]string
}

func (s *Service) loadCombinedMeet(ctx context.Context, meetID string) (*combinedMeetState, error) {
	st := &combinedMeetState{events: map[string]domain.Event{}, units: map[string]string{}}
	var err error
	if st.parents, err = s.store.ListCombined(ctx, meetID); err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		st.events[e.ID] = e
	}
	if st.rounds, err = s.store.LoadRounds(ctx, store.Scope{MeetID: meetID}); err != nil {
		return nil, err
	}
	if st.athletes, err = s.store.ListAthletes(ctx, meetID); err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	for _, u := range units {
		st.units[u.ID] = u.Name
	}
	return st, nil
}
func (s *Service) combinedState(ctx context.Context, id string) (*combinedMeetState, *store.CombinedData, error) {
	meetID, err := s.store.CombinedMeet(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if meetID == "" {
		return nil, nil, invalid("全能不存在或已删除")
	}
	st, err := s.loadCombinedMeet(ctx, meetID)
	if err != nil {
		return nil, nil, err
	}
	for i := range st.parents {
		if st.parents[i].Combined.ID == id {
			return st, &st.parents[i], nil
		}
	}
	return nil, nil, ErrStale
}
func (s *Service) ListCombined(ctx context.Context, meetID string) ([]CombinedRowSummary, error) {
	st, err := s.loadCombinedMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := []CombinedRowSummary{}
	for _, p := range st.parents {
		row := CombinedRowSummary{Combined: p.Combined, Components: len(p.Components), Entries: len(p.AthleteIDs)}
		for _, c := range p.Components {
			for _, r := range st.rounds[c.EventID] {
				for _, h := range r.Heats {
					row.Results += h.Facts().Results
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}
func (s *Service) GetCombined(ctx context.Context, id string) (*CombinedDetail, error) {
	st, p, err := s.combinedState(ctx, id)
	if err != nil {
		return nil, err
	}
	return st.detail(*p, nil)
}
func (st *combinedMeetState) detail(p store.CombinedData, rules map[string]domain.PointRule) (*CombinedDetail, error) {
	d := &CombinedDetail{Combined: p.Combined, Components: []CombinedComponentView{}, AthleteIDs: append([]string{}, p.AthleteIDs...), Athletes: st.athletes, Rows: []CombinedRow{}}
	sources := []scoring.CombinedSource{}
	for _, c := range p.Components {
		e, ok := st.events[c.EventID]
		if !ok || e.CombinedID != p.Combined.ID {
			return nil, invalid("全能分项不完整")
		}
		rule := c.Rule
		if override, ok := rules[e.ID]; ok {
			rule = override
		}
		d.Components = append(d.Components, CombinedComponentView{Event: e, Ordinal: c.Ordinal, Rule: rule, Input: pointRuleInput(e, rule)})
		source := scoring.CombinedSource{Event: e, Rule: rule, Results: map[string]*domain.Result{}}
		for _, r := range st.rounds[e.ID] {
			for _, h := range r.Heats {
				d.Heats++
				d.Results += h.Facts().Results
				if len(h.Slots) > 0 {
					d.HasPlacements = true
				}
				if r.Stage != domain.StageFinal {
					continue
				}
				for _, sl := range h.Slots {
					if _, exists := source.Results[sl.AthleteID]; exists {
						return nil, invalid("全能分项成绩重复")
					}
					source.Results[sl.AthleteID] = sl.Result
				}
			}
		}
		sources = append(sources, source)
	}
	rows, complete, err := scoring.CombinedRank(p.AthleteIDs, sources)
	if err != nil {
		return nil, invalid("%s", err)
	}
	d.Complete = complete
	athletes := map[string]domain.Athlete{}
	for _, a := range st.athletes {
		athletes[a.ID] = a
	}
	for _, r := range rows {
		a := athletes[r.AthleteID]
		d.Rows = append(d.Rows, CombinedRow{CombinedStanding: r, GenderMismatch: a.Gender != "" && p.Combined.Gender != domain.GenderMixed && a.Gender != p.Combined.Gender, Name: a.Name, Bib: a.Bib, UnitName: st.units[a.UnitID]})
	}
	return d, nil
}
func parsePointRule(e domain.Event, in PointRuleInput) (domain.PointRule, error) {
	r := domain.PointRule{Bands: []domain.PointBand{}, InvalidPolicies: in.InvalidPolicies}
	text := strings.TrimSpace(in.Table)
	for i, line := range strings.Split(text, "\n") {
		parts := strings.FieldsFunc(strings.TrimSpace(line), func(c rune) bool { return c == '\t' || c == ',' || c == '，' || c == ' ' })
		if len(parts) != 2 {
			return r, invalid("计分表第 %d 行须有成绩和积分", i+1)
		}
		if i == 0 && (parts[0] == "成绩" || parts[0] == "成绩标准" || strings.EqualFold(parts[0], "mark")) && (parts[1] == "积分" || strings.EqualFold(parts[1], "points")) {
			continue
		}
		mark, err := scoring.ParseEventMark(e, parts[0])
		if err != nil {
			return r, invalid("计分表第 %d 行成绩有误", i+1)
		}
		points, err := scoring.ParseEventMark(domain.Event{Kind: domain.KindCount}, parts[1])
		if err != nil {
			return r, invalid("计分表第 %d 行积分有误", i+1)
		}
		r.Bands = append(r.Bands, domain.PointBand{Mark: mark, Points: points})
	}
	if err := r.Validate(e); err != nil {
		return r, invalid("%s", err)
	}
	return r, nil
}
func pointRuleInput(e domain.Event, r domain.PointRule) PointRuleInput {
	lines := []string{}
	for _, b := range r.Bands {
		lines = append(lines, fmt.Sprintf("%s\t%d", scoring.FormatRuleMark(e, b.Mark), b.Points))
	}
	return PointRuleInput{Table: strings.Join(lines, "\n"), InvalidPolicies: r.InvalidPolicies}
}
func jsonValue(v any) string { b, _ := json.Marshal(v); return string(b) }
func insertCombined(c domain.Combined) sqlite.Statement {
	return store.Stmt(`INSERT INTO combined_events(id,meet_id,name,gender,group_label,note,rule_version) VALUES (?,?,?,?,?,?,?)`, c.ID, c.MeetID, c.Name, string(c.Gender), c.GroupLabel, c.Note, c.RuleVersion)
}
func insertCombinedComponent(c domain.CombinedComponent) sqlite.Statement {
	return store.Stmt(`INSERT INTO combined_components(combined_id,event_id,ordinal,rule_json) VALUES (?,?,?,?)`, c.CombinedID, c.EventID, c.Ordinal, jsonValue(c.Rule))
}

func (s *Service) SaveCombined(ctx context.Context, src store.Source, meetID, id string, in CombinedInput) (*domain.Combined, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	gender, ok := domain.NormalizeEventGender(in.Gender)
	if !ok {
		return nil, invalid("请选择性别")
	}
	c := domain.Combined{ID: id, MeetID: meetID, Name: strings.TrimSpace(in.Name), Gender: gender, GroupLabel: strings.TrimSpace(in.GroupLabel), Note: strings.TrimSpace(in.Note), RuleVersion: 1}
	if c.Name == "" {
		return nil, invalid("请填写全能名称")
	}
	if len(in.Components) < 2 {
		return nil, invalid("全能至少需要两个分项")
	}
	st, err := s.loadCombinedMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	var old *store.CombinedData
	for i := range st.parents {
		p := &st.parents[i]
		if p.Combined.ID == id {
			old = p
		} else if domain.CombinedKey(p.Combined.Gender, p.Combined.Name, p.Combined.GroupLabel) == domain.CombinedKey(c.Gender, c.Name, c.GroupLabel) {
			return nil, invalid("已有相同的全能")
		}
	}
	if id != "" && old == nil {
		return nil, invalid("全能不存在或已删除")
	}
	events := []domain.Event{}
	components := []domain.CombinedComponent{}
	seen := map[string]bool{}
	if old == nil {
		c.ID = newID()
	} else {
		c.RuleVersion = old.Combined.RuleVersion
	}
	for i, input := range in.Components {
		input.Event.Gender, input.Event.GroupLabel = string(gender), c.GroupLabel
		if len(input.Event.Rounds) > 0 {
			return nil, invalid("全能分项须直接决赛")
		}
		e, err := input.Event.event()
		if err != nil {
			return nil, err
		}
		if e.IsTeam() {
			return nil, invalid("全能分项须按个人参赛")
		}
		key := domain.DisciplineKey(e.Discipline)
		if seen[key] {
			return nil, invalid("全能分项重复")
		}
		seen[key] = true
		e.ID, e.MeetID, e.CombinedID = newID(), meetID, c.ID
		e.Name = c.Name + " · " + domain.ComposeEventName(e.Gender, e.Discipline, e.GroupLabel)
		if old != nil && i < len(old.Components) {
			e.ID = old.Components[i].EventID
		}
		rule, err := parsePointRule(e, input.Rule)
		if err != nil {
			return nil, invalid("%s：%s", e.Discipline, err)
		}
		events = append(events, e)
		components = append(components, domain.CombinedComponent{CombinedID: c.ID, EventID: e.ID, Ordinal: i + 1, Rule: rule})
	}
	var stmts []sqlite.Statement
	action := "combined.created"
	roster := []string{}
	if old != nil {
		action = "combined.updated"
		roster = old.AthleteIDs
		d, err := st.detail(*old, nil)
		if err != nil {
			return nil, err
		}
		same := len(old.Components) == len(components)
		if same {
			for i, part := range components {
				was := st.events[old.Components[i].EventID]
				e := events[i]
				was.Name = e.Name
				if !reflect.DeepEqual(was, e) {
					same = false
				}
				if !reflect.DeepEqual(old.Components[i].Rule, part.Rule) {
					return nil, invalid("请先预览计分规则修改")
				}
			}
		}
		if !same && d.HasPlacements {
			return nil, invalid("已有编排，不能修改全能分项")
		}
		if same {
			stmts = append(stmts, store.Stmt(`UPDATE combined_events SET name=?,gender=?,group_label=?,note=? WHERE id=?`, c.Name, string(c.Gender), c.GroupLabel, c.Note, c.ID))
			for _, e := range events {
				stmts = append(stmts, store.Stmt(`UPDATE events SET name=? WHERE id=?`, e.Name, e.ID))
			}
			err = s.write(ctx, store.Audit{Source: src, Action: action, MeetID: meetID, Message: c.Name, Detail: in}, Change{Kind: "combined", MeetID: meetID}, stmts...)
			return &c, err
		}
		c.RuleVersion++
		// Empty component events can be rebuilt while preserving the shared roster.
		stmts = append(stmts, store.Stmt(`DELETE FROM events WHERE combined_id=?`, c.ID), store.Stmt(`UPDATE combined_events SET name=?,gender=?,group_label=?,note=?,rule_version=? WHERE id=?`, c.Name, string(c.Gender), c.GroupLabel, c.Note, c.RuleVersion, c.ID))
	} else {
		stmts = append(stmts, insertCombined(c))
	}
	for i, e := range events {
		stmts = append(stmts, insertEvent(e, s.store.Now()), insertRound(domain.Round{ID: newID(), EventID: e.ID, Stage: domain.StageFinal, Ordinal: 1}), insertCombinedComponent(components[i]))
		for _, a := range roster {
			stmts = append(stmts, insertEntry(e.ID, a, "", nil))
		}
	}
	err = s.write(ctx, store.Audit{Source: src, Action: action, MeetID: meetID, Message: c.Name, Detail: in}, Change{Kind: "combined", MeetID: meetID}, stmts...)
	return &c, err
}

func (s *Service) SetCombinedEntries(ctx context.Context, src store.Source, id string, athleteIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, p, err := s.combinedState(ctx, id)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, a := range st.athletes {
		allowed[a.ID] = true
	}
	wanted := map[string]bool{}
	for _, a := range athleteIDs {
		if !allowed[a] || wanted[a] {
			return invalid("报名运动员不属于赛事或重复")
		}
		wanted[a] = true
	}
	have := map[string]bool{}
	for _, a := range p.AthleteIDs {
		have[a] = true
	}
	for _, c := range p.Components {
		for _, r := range st.rounds[c.EventID] {
			for _, h := range r.Heats {
				for _, sl := range h.Slots {
					if !wanted[sl.AthleteID] {
						return invalid("已有分项编排，请先移出")
					}
				}
			}
		}
	}
	stmts := []sqlite.Statement{}
	for _, a := range p.AthleteIDs {
		if !wanted[a] {
			stmts = append(stmts, store.Stmt(`DELETE FROM combined_entries WHERE combined_id=? AND athlete_id=?`, id, a))
			for _, c := range p.Components {
				stmts = append(stmts, store.Stmt(`DELETE FROM entries WHERE event_id=? AND athlete_id=?`, c.EventID, a))
			}
		}
	}
	// Preserve the requested order for new entries; unchanged entry order is preserved.
	added := []string{}
	for _, a := range athleteIDs {
		if !have[a] {
			added = append(added, a)
		}
	}
	for _, a := range added {
		stmts = append(stmts, store.Stmt(`INSERT INTO combined_entries(combined_id,athlete_id) VALUES (?,?)`, id, a))
		for _, c := range p.Components {
			stmts = append(stmts, insertEntry(c.EventID, a, "", nil))
		}
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx, store.Audit{Source: src, Action: "combined.entries", MeetID: p.Combined.MeetID, Message: p.Combined.Name, Detail: athleteIDs}, Change{Kind: "combined", MeetID: p.Combined.MeetID}, stmts...)
}
func (s *Service) DeleteCombined(ctx context.Context, src store.Source, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, p, err := s.combinedState(ctx, id)
	if err != nil {
		return err
	}
	d, err := st.detail(*p, nil)
	if err != nil {
		return err
	}
	if src == store.SourceMCP && d.Results > 0 {
		return invalid("已有成绩，只能在界面上删除")
	}
	return s.write(ctx, store.Audit{Source: src, Action: "combined.deleted", MeetID: p.Combined.MeetID, Message: p.Combined.Name, Detail: d.Combined}, Change{Kind: "combined", MeetID: p.Combined.MeetID}, store.Stmt(`DELETE FROM combined_events WHERE id=?`, id))
}
