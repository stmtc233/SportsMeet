package domain

import "errors"

// TeamScoringRules belongs to a meet. Multipliers of zero exclude a target.
// Component events never have their own rule: their combined is the target.
type TeamScoringRules struct {
	Enabled         bool                `json:"enabled"`
	Places          []int64             `json:"places"`
	TiePolicy       string              `json:"tiePolicy"` // average | same
	RelayMultiplier int64               `json:"relayMultiplier"`
	Overrides       []TeamScoreOverride `json:"overrides"`
}

type TeamScoreOverride struct {
	EventID    string `json:"eventId,omitempty"`
	CombinedID string `json:"combinedId,omitempty"`
	Multiplier int64  `json:"multiplier"`
}

func DefaultTeamScoringRules() TeamScoringRules {
	return TeamScoringRules{Places: []int64{9, 7, 6, 5, 4, 3, 2, 1}, TiePolicy: "average", RelayMultiplier: 2, Overrides: []TeamScoreOverride{}}
}

func (r TeamScoringRules) Validate() error {
	if len(r.Places) == 0 {
		return errors.New("请填写名次分值")
	}
	for i, p := range r.Places {
		if p < 0 {
			return errors.New("名次分值须为非负整数")
		}
		if p > 9007199254740991 {
			return errors.New("名次分值超出范围")
		}
		if i > 0 && p > r.Places[i-1] {
			return errors.New("名次分值须依次递减或相同")
		}
	}
	if r.TiePolicy != "average" && r.TiePolicy != "same" {
		return errors.New("请选择并列计分方式")
	}
	if r.RelayMultiplier < 0 {
		return errors.New("计分倍数须为非负整数")
	}
	if r.RelayMultiplier > 9007199254740991 {
		return errors.New("计分倍数超出范围")
	}
	seen := map[string]bool{}
	for _, o := range r.Overrides {
		if (o.EventID == "") == (o.CombinedID == "") {
			return errors.New("计分例外须指定项目或全能")
		}
		key := "event:" + o.EventID
		if o.CombinedID != "" {
			key = "combined:" + o.CombinedID
		}
		if seen[key] {
			return errors.New("计分例外重复")
		}
		seen[key] = true
		if o.Multiplier < 0 {
			return errors.New("计分倍数须为非负整数")
		}
		if o.Multiplier > 9007199254740991 {
			return errors.New("计分倍数超出范围")
		}
	}
	return nil
}
