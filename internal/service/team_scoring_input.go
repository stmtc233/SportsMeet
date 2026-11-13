package service

import (
	"strconv"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// TeamScoringInput keeps editable numeric text intact until Go validates it.
type TeamScoringInput struct {
	Enabled         bool                     `json:"enabled"`
	Places          string                   `json:"places"`
	TiePolicy       string                   `json:"tiePolicy"`
	RelayMultiplier string                   `json:"relayMultiplier"`
	Overrides       []TeamScoreOverrideInput `json:"overrides"`
}
type TeamScoreOverrideInput struct {
	EventID    string `json:"eventId"`
	CombinedID string `json:"combinedId"`
	Multiplier string `json:"multiplier"`
}

func ParseTeamScoringInput(in TeamScoringInput) (domain.TeamScoringRules, error) {
	r := domain.TeamScoringRules{Enabled: in.Enabled, TiePolicy: in.TiePolicy, Places: []int64{}, Overrides: []domain.TeamScoreOverride{}}
	parse := func(s string) (int64, error) {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil || v < 0 {
			return 0, invalid("分值和倍数须为非负整数")
		}
		return v, nil
	}
	for _, p := range strings.FieldsFunc(in.Places, func(c rune) bool { return c == ' ' || c == '\n' || c == '\t' || c == ',' || c == '，' || c == '、' }) {
		v, err := parse(p)
		if err != nil {
			return r, err
		}
		r.Places = append(r.Places, v)
	}
	var err error
	if r.RelayMultiplier, err = parse(in.RelayMultiplier); err != nil {
		return r, err
	}
	for _, o := range in.Overrides {
		v, err := parse(o.Multiplier)
		if err != nil {
			return r, err
		}
		r.Overrides = append(r.Overrides, domain.TeamScoreOverride{EventID: o.EventID, CombinedID: o.CombinedID, Multiplier: v})
	}
	if err := r.Validate(); err != nil {
		return r, invalid("%s", err)
	}
	return r, nil
}
