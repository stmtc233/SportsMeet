package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type CombinedRuleChange struct {
	EventID string         `json:"eventId"`
	Rule    PointRuleInput `json:"rule"`
}
type CombinedRulesRequest struct {
	CombinedID string               `json:"combinedId"`
	Rules      []CombinedRuleChange `json:"rules"`
}
type CombinedRulesPreview struct {
	PreviewHash string        `json:"previewHash"`
	RuleVersion int           `json:"ruleVersion"`
	Before      []CombinedRow `json:"before"`
	After       []CombinedRow `json:"after"`
	Changed     bool          `json:"changed"`
}

type PointBandView struct {
	MarkText string `json:"markText"`
	Points   int64  `json:"points"`
}
type PointRulePreview struct {
	Rule domain.PointRule `json:"rule"`
	Rows []PointBandView  `json:"rows"`
}

func (s *Service) ParsePointRule(in EventInput, rule PointRuleInput) (*PointRulePreview, error) {
	e, err := in.event()
	if err != nil {
		return nil, err
	}
	r, err := parsePointRule(e, rule)
	if err != nil {
		return nil, err
	}
	out := &PointRulePreview{Rule: r, Rows: []PointBandView{}}
	for _, b := range r.Bands {
		out.Rows = append(out.Rows, PointBandView{MarkText: scoring.FormatRuleMark(e, b.Mark), Points: b.Points})
	}
	return out, nil
}
func (s *Service) prepareCombinedRules(ctx context.Context, req CombinedRulesRequest) (*CombinedRulesPreview, *store.CombinedData, []sqlite.Statement, error) {
	st, p, err := s.combinedState(ctx, req.CombinedID)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(req.Rules) != len(p.Components) {
		return nil, nil, nil, invalid("请提供全部分项计分规则")
	}
	rules := map[string]domain.PointRule{}
	allowed := map[string]bool{}
	for _, c := range p.Components {
		allowed[c.EventID] = true
	}
	for _, change := range req.Rules {
		if !allowed[change.EventID] {
			return nil, nil, nil, invalid("分项不属于本全能")
		}
		if _, seen := rules[change.EventID]; seen {
			return nil, nil, nil, invalid("分项规则重复")
		}
		r, err := parsePointRule(st.events[change.EventID], change.Rule)
		if err != nil {
			return nil, nil, nil, err
		}
		rules[change.EventID] = r
	}
	before, err := st.detail(*p, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	after, err := st.detail(*p, rules)
	if err != nil {
		return nil, nil, nil, err
	}
	preview := &CombinedRulesPreview{RuleVersion: p.Combined.RuleVersion, Before: before.Rows, After: after.Rows}
	stmts := []sqlite.Statement{}
	for _, c := range p.Components {
		if !reflect.DeepEqual(c.Rule, rules[c.EventID]) {
			preview.Changed = true
			stmts = append(stmts, store.Stmt(`UPDATE combined_components SET rule_json=? WHERE combined_id=? AND event_id=?`, jsonValue(rules[c.EventID]), p.Combined.ID, c.EventID))
		}
	}
	if preview.Changed {
		preview.RuleVersion++
		stmts = append(stmts, store.Stmt(`UPDATE combined_events SET rule_version=? WHERE id=?`, preview.RuleVersion, p.Combined.ID))
	}
	raw, err := json.Marshal(struct {
		Request  CombinedRulesRequest
		Parents  []store.CombinedData
		Events   map[string]domain.Event
		Rounds   map[string][]domain.Round
		Athletes []domain.Athlete
		Units    map[string]string
	}{req, st.parents, st.events, st.rounds, st.athletes, st.units})
	if err != nil {
		return nil, nil, nil, err
	}
	hash := sha256.Sum256(raw)
	preview.PreviewHash = hex.EncodeToString(hash[:])
	return preview, p, stmts, nil
}
func (s *Service) PreviewCombinedRules(ctx context.Context, req CombinedRulesRequest) (*CombinedRulesPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, _, err := s.prepareCombinedRules(ctx, req)
	return p, err
}
func (s *Service) ApplyCombinedRules(ctx context.Context, src store.Source, req CombinedRulesRequest, hash string, confirmed bool) (*CombinedRulesPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, p, stmts, err := s.prepareCombinedRules(ctx, req)
	if err != nil {
		return nil, err
	}
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	if hash != preview.PreviewHash {
		return nil, ErrStale
	}
	if !preview.Changed {
		return preview, nil
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "combined.rules", MeetID: p.Combined.MeetID, Message: p.Combined.Name, Detail: struct {
		Request CombinedRulesRequest
		Preview *CombinedRulesPreview
	}{req, preview}}, Change{Kind: "combined", MeetID: p.Combined.MeetID}, stmts...)
	return preview, err
}
