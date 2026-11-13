package domain

import "errors"

// Combined owns independently arranged component events and one shared roster.
type Combined struct {
	GroupID     string `json:"groupId"`
	ID          string `json:"id"`
	MeetID      string `json:"meetId"`
	Name        string `json:"name"`
	Gender      Gender `json:"gender"`
	GroupLabel  string `json:"groupLabel"`
	Note        string `json:"note"`
	RuleVersion int    `json:"ruleVersion"`
}
type PointBand struct {
	Mark   int64 `json:"mark"`
	Points int64 `json:"points"`
}
type PointRule struct {
	Bands           []PointBand       `json:"bands"`
	InvalidPolicies map[string]string `json:"invalidPolicies"`
}

// Bands are ordered from the hardest threshold to the easiest, without interpolation.
func (r PointRule) Validate(e Event) error {
	if len(r.Bands) == 0 {
		return errors.New("请填写计分表")
	}
	for i, b := range r.Bands {
		if b.Mark < 0 || b.Points < 0 {
			return errors.New("成绩和积分不能为负数")
		}
		if i > 0 {
			prev := r.Bands[i-1]
			if (e.Ascending() && b.Mark <= prev.Mark) || (!e.Ascending() && b.Mark >= prev.Mark) {
				return errors.New("档位须从高标准向低排列")
			}
			if b.Points > prev.Points {
				return errors.New("较低标准的积分不能更高")
			}
		}
	}
	for _, status := range []Status{StatusDNS, StatusDNF, StatusDQ, StatusNM} {
		policy := r.InvalidPolicies[string(status)]
		if policy != "zero" && policy != "exclude" {
			return errors.New("请设置无效成绩的计分方式")
		}
	}
	for status := range r.InvalidPolicies {
		if status != "DNS" && status != "DNF" && status != "DQ" && status != "NM" {
			return errors.New("无效成绩状态不正确")
		}
	}
	return nil
}

type CombinedComponent struct {
	CombinedID string    `json:"combinedId"`
	EventID    string    `json:"eventId"`
	Ordinal    int       `json:"ordinal"`
	Rule       PointRule `json:"rule"`
}

func CombinedKey(g Gender, name, group string) string {
	return string(g) + "|" + NormalizeText(name) + "|" + NormalizeText(group)
}
