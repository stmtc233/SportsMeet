package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type TeamScoreFilter struct {
	MeetID     string   `json:"meetId"`
	Gender     string   `json:"gender,omitempty"`
	GroupLabel *string  `json:"groupLabel,omitempty"`
	TargetIDs  []string `json:"targetIds,omitempty"`
}
type TeamScoreTarget struct {
	EventID    string        `json:"eventId"`
	CombinedID string        `json:"combinedId"`
	Name       string        `json:"name"`
	Gender     domain.Gender `json:"gender"`
	GroupLabel string        `json:"groupLabel"`
	Relay      bool          `json:"relay"`
}

func (t TeamScoreTarget) id() string {
	if t.EventID != "" {
		return t.EventID
	}
	return t.CombinedID
}

type TeamScoreDetail struct {
	scoring.TeamScores
	Rules   domain.TeamScoringRules `json:"rules"`
	Groups  []string                `json:"groups"`
	Targets []TeamScoreTarget       `json:"targets"`
}
type TeamScoringRequest struct {
	MeetID string                  `json:"meetId"`
	Rules  domain.TeamScoringRules `json:"rules"`
}
type TeamScoringPreview struct {
	PreviewHash string           `json:"previewHash"`
	Changed     bool             `json:"changed"`
	Before      *TeamScoreDetail `json:"before"`
	After       *TeamScoreDetail `json:"after"`
}

type teamScoreState struct {
	meet     domain.Meet
	combined *combinedMeetState
	units    []domain.Unit
	teams    []domain.Team
	rules    domain.TeamScoringRules
	targets  []TeamScoreTarget
}

func (s *Service) loadTeamScoreState(ctx context.Context, meetID string) (*teamScoreState, error) {
	m, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	st := &teamScoreState{meet: *m, targets: []TeamScoreTarget{}}
	if st.combined, err = s.loadCombinedMeet(ctx, meetID); err != nil {
		return nil, err
	}
	if st.units, err = s.store.ListUnits(ctx, meetID); err != nil {
		return nil, err
	}
	if st.teams, err = s.store.ListTeams(ctx, store.Scope{MeetID: meetID}); err != nil {
		return nil, err
	}
	if st.rules, err = s.store.TeamScoringRules(ctx, meetID); err != nil {
		return nil, err
	}
	// ListEvents keeps creation order for schedule tie breakers.
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	for _, e := range domain.ScheduleOrder(events, st.combined.rounds, m.StartDate) {
		if e.CombinedID == "" {
			st.targets = append(st.targets, TeamScoreTarget{EventID: e.ID, Name: e.Name, Gender: e.Gender, GroupLabel: e.GroupLabel, Relay: e.Kind == domain.KindRelay})
		}
	}
	for _, p := range st.combined.parents {
		c := p.Combined
		st.targets = append(st.targets, TeamScoreTarget{CombinedID: c.ID, Name: c.Name, Gender: c.Gender, GroupLabel: c.GroupLabel})
	}
	// Deleted targets have no effect and are omitted from reads/exports.
	live := []domain.TeamScoreOverride{}
	for _, o := range st.rules.Overrides {
		for _, t := range st.targets {
			if o.EventID == t.EventID && o.CombinedID == t.CombinedID {
				live = append(live, o)
				break
			}
		}
	}
	st.rules.Overrides = live
	st.rules, err = st.normalizeRules(st.rules)
	return st, err
}

func (st *teamScoreState) normalizeRules(r domain.TeamScoringRules) (domain.TeamScoringRules, error) {
	if err := r.Validate(); err != nil {
		return r, invalid("%s", err)
	}
	overrides := []domain.TeamScoreOverride{}
	for _, o := range r.Overrides {
		found := false
		for _, t := range st.targets {
			if o.EventID == t.EventID && o.CombinedID == t.CombinedID {
				found = true
				base := int64(1)
				if t.Relay {
					base = r.RelayMultiplier
				}
				if o.Multiplier != base {
					overrides = append(overrides, o)
				}
				break
			}
		}
		if !found {
			return r, invalid("计分项目不属于本赛事")
		}
	}
	sort.Slice(overrides, func(i, j int) bool {
		a, b := overrides[i], overrides[j]
		if a.EventID != b.EventID {
			return a.EventID < b.EventID
		}
		return a.CombinedID < b.CombinedID
	})
	r.Overrides = overrides
	return r, nil
}

func (st *teamScoreState) detail(filter TeamScoreFilter, rules domain.TeamScoringRules) (*TeamScoreDetail, error) {
	if filter.Gender != "" && filter.Gender != "男" && filter.Gender != "女" && filter.Gender != "混合" {
		return nil, invalid("请选择性别")
	}
	for _, id := range filter.TargetIDs {
		if id == "" || !slices.ContainsFunc(st.targets, func(t TeamScoreTarget) bool { return t.id() == id }) {
			return nil, invalid("计分项目不存在或已删除")
		}
	}
	athletes := map[string]domain.Athlete{}
	for _, a := range st.combined.athletes {
		athletes[a.ID] = a
	}
	teams := map[string]domain.Team{}
	for _, t := range st.teams {
		teams[t.ID] = t
	}
	groups := []string{}
	sources := []scoring.TeamScoreSource{}
	for _, t := range st.targets {
		if !slices.Contains(groups, t.GroupLabel) {
			groups = append(groups, t.GroupLabel)
		}
		if filter.Gender != "" && filter.Gender != string(t.Gender) {
			continue
		}
		if filter.GroupLabel != nil && *filter.GroupLabel != t.GroupLabel {
			continue
		}
		if len(filter.TargetIDs) > 0 && !slices.Contains(filter.TargetIDs, t.id()) {
			continue
		}
		src := scoring.TeamScoreSource{EventID: t.EventID, CombinedID: t.CombinedID, Name: t.Name, Gender: t.Gender, GroupLabel: t.GroupLabel, Multiplier: 1, Competitors: []scoring.TeamScoreCompetitor{}}
		if t.Relay {
			src.Multiplier = rules.RelayMultiplier
		}
		for _, o := range rules.Overrides {
			if o.EventID == t.EventID && o.CombinedID == t.CombinedID {
				src.Multiplier = o.Multiplier
			}
		}
		if t.EventID != "" {
			for _, r := range st.combined.rounds[t.EventID] {
				if r.Stage != domain.StageFinal {
					continue
				}
				var rows []Standing
				rows, src.Complete = standingsEvent(st.combined.events[t.EventID], r)
				for _, row := range rows {
					c := scoring.TeamScoreCompetitor{CompetitorID: row.CompetitorID, AthleteID: row.AthleteID, TeamID: row.TeamID, Rank: row.Rank, MarkText: row.MarkText}
					if row.TeamID != "" {
						c.Name, c.UnitID = teams[row.TeamID].Name, teams[row.TeamID].UnitID
					} else {
						c.Name, c.UnitID = athletes[row.AthleteID].Name, athletes[row.AthleteID].UnitID
					}
					src.Competitors = append(src.Competitors, c)
				}
			}
		} else {
			for _, p := range st.combined.parents {
				if p.Combined.ID != t.CombinedID {
					continue
				}
				d, err := st.combined.detail(p, nil)
				if err != nil {
					return nil, err
				}
				src.Complete = d.Complete
				for _, row := range d.Rows {
					src.Competitors = append(src.Competitors, scoring.TeamScoreCompetitor{CompetitorID: row.AthleteID, AthleteID: row.AthleteID, Name: row.Name, UnitID: athletes[row.AthleteID].UnitID, Rank: row.Rank})
				}
			}
		}
		sources = append(sources, src)
	}
	sort.Strings(groups)
	return &TeamScoreDetail{TeamScores: scoring.TeamRank(rules, st.units, sources), Rules: rules, Groups: groups, Targets: st.targets}, nil
}

func (s *Service) GetTeamScores(ctx context.Context, filter TeamScoreFilter) (*TeamScoreDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadTeamScoreState(ctx, filter.MeetID)
	if err != nil {
		return nil, err
	}
	return st.detail(filter, st.rules)
}

func (s *Service) prepareTeamScoring(ctx context.Context, req TeamScoringRequest) (*TeamScoringPreview, error) {
	st, err := s.loadTeamScoreState(ctx, req.MeetID)
	if err != nil {
		return nil, err
	}
	req.Rules, err = st.normalizeRules(req.Rules)
	if err != nil {
		return nil, err
	}
	filter := TeamScoreFilter{MeetID: req.MeetID}
	before, err := st.detail(filter, st.rules)
	if err != nil {
		return nil, err
	}
	after, err := st.detail(filter, req.Rules)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(struct {
		Request  TeamScoringRequest
		Meet     domain.Meet
		Rules    domain.TeamScoringRules
		Parents  []store.CombinedData
		Events   map[string]domain.Event
		Rounds   map[string][]domain.Round
		Athletes []domain.Athlete
		Units    []domain.Unit
		Teams    []domain.Team
	}{req, st.meet, st.rules, st.combined.parents, st.combined.events, st.combined.rounds, st.combined.athletes, st.units, st.teams})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(raw)
	return &TeamScoringPreview{PreviewHash: hex.EncodeToString(hash[:]), Changed: !reflect.DeepEqual(st.rules, req.Rules), Before: before, After: after}, nil
}

func (s *Service) PreviewTeamScoring(ctx context.Context, req TeamScoringRequest) (*TeamScoringPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepareTeamScoring(ctx, req)
}

func (s *Service) ApplyTeamScoring(ctx context.Context, src store.Source, req TeamScoringRequest, hash string, confirmed bool) (*TeamScoringPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.prepareTeamScoring(ctx, req)
	if err != nil {
		return nil, err
	}
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	if hash != p.PreviewHash {
		return nil, ErrStale
	}
	if !p.Changed {
		return p, nil
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "team_scoring.rules", MeetID: req.MeetID, Message: "团体总分", Detail: p}, Change{Kind: "team_scoring", MeetID: req.MeetID}, store.Stmt(`INSERT INTO meet_team_scoring(meet_id,rules_json) VALUES (?,?) ON CONFLICT(meet_id) DO UPDATE SET rules_json=excluded.rules_json`, req.MeetID, jsonValue(p.After.Rules)))
	return p, err
}
