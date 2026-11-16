package domain

import "errors"

// Team scoring modes. The empty Mode is TeamModePoints and the empty
// MedalOrder is MedalOrderGold, so rules written before medals existed read
// the same and canonical rules omit both.
const (
	TeamModePoints  = "points"
	TeamModeMedals  = "medals"
	MedalOrderGold  = "gold"  // gold, then silver, then bronze
	MedalOrderTotal = "total" // medal total, then gold, silver, bronze
)

// TeamScoringRules belongs to a meet. Multipliers of zero exclude a target.
// Component events never have their own rule: their combined is the target.
// In medals mode places, tie policy and multipliers play no part: a target
// counts once, and only a zero override excludes it.
type TeamScoringRules struct {
	Enabled         bool                `json:"enabled"`
	Mode            string              `json:"mode,omitempty"`       // points | medals
	MedalOrder      string              `json:"medalOrder,omitempty"` // gold | total, medals mode only
	Places          []int64             `json:"places"`
	TiePolicy       string              `json:"tiePolicy"` // average | same
	RelayMultiplier int64               `json:"relayMultiplier"`
	Overrides       []TeamScoreOverride `json:"overrides"`
}

// Medals reports whether units are ranked by medals instead of points.
func (r TeamScoringRules) Medals() bool { return r.Mode == TeamModeMedals }

// Canonical writes the rules in their one stored form: the default mode and
// order are left empty, the order only exists in medals mode, and a medals
// rule that never typed places or a tie policy keeps the defaults, so
// switching back to points finds usable values.
func (r TeamScoringRules) Canonical() TeamScoringRules {
	if r.Mode == TeamModePoints {
		r.Mode = ""
	}
	if !r.Medals() || r.MedalOrder == MedalOrderGold {
		r.MedalOrder = ""
	}
	if r.Medals() {
		d := DefaultTeamScoringRules()
		if len(r.Places) == 0 {
			r.Places = d.Places
		}
		if r.TiePolicy == "" {
			r.TiePolicy = d.TiePolicy
		}
	}
	return r
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
	if r.Mode != "" && r.Mode != TeamModePoints && r.Mode != TeamModeMedals {
		return errors.New("请选择计分方式")
	}
	if r.MedalOrder != "" && r.MedalOrder != MedalOrderGold && r.MedalOrder != MedalOrderTotal {
		return errors.New("请选择奖牌排序方式")
	}
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
