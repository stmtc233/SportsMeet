package domain

import "errors"

// StartMode describes the start independently of the unit used for marks.
type StartMode string

const (
	StartNone  StartMode = ""
	StartLanes StartMode = "lanes"
	StartGroup StartMode = "group"
	StartBreak StartMode = "break"
)

type DistanceMode string

const (
	DistanceNone  DistanceMode = ""
	DistanceJump  DistanceMode = "jump"
	DistanceThrow DistanceMode = "throw"
)

// NormalizeConfig supplies compatibility defaults without interpreting Discipline.
func (e *Event) NormalizeConfig() error {
	if e.Kind.Timed() {
		if e.StartMode == "" {
			e.StartMode = StartLanes
		}
		switch e.StartMode {
		case StartLanes, StartGroup, StartBreak:
		default:
			return errors.New("请选择起跑方式")
		}
	} else if e.StartMode != "" {
		return errors.New("田赛不能设置起跑方式")
	}
	if e.Kind == KindDistance {
		if e.DistanceMode == "" {
			e.DistanceMode = DistanceJump
		}
		if e.DistanceMode != DistanceJump && e.DistanceMode != DistanceThrow {
			return errors.New("请选择跳跃或投掷")
		}
	} else if e.DistanceMode != "" {
		return errors.New("该项目不能设置远度方式")
	}
	if e.RelayLegCount < 0 {
		return errors.New("建议棒数不能为负数")
	}
	if e.Kind != KindRelay && e.RelayLegCount != 0 {
		return errors.New("只有接力可设置建议棒数")
	}
	return nil
}

func (e Event) UsesLanes() bool { return e.Kind.Timed() && e.StartMode != StartGroup }

// EventRules is the presentation and arrangement configuration shared by clients.
type EventRules struct {
	Timed          bool   `json:"timed"`
	UsesLanes      bool   `json:"usesLanes"`
	PositionLabel  string `json:"positionLabel"`
	AttemptLabel   string `json:"attemptLabel"`
	StartLabel     string `json:"startLabel"`
	DefaultPerHeat int    `json:"defaultPerHeat"`
}

func (e Event) Rules() EventRules {
	r := EventRules{Timed: e.Kind.Timed(), UsesLanes: e.UsesLanes(), PositionLabel: "顺序", DefaultPerHeat: 12}
	if r.UsesLanes {
		r.PositionLabel, r.DefaultPerHeat = "道次", e.LaneCount
	}
	if e.Kind.Timed() {
		switch e.StartMode {
		case StartGroup:
			r.StartLabel = "不分道"
		case StartBreak:
			r.StartLabel = "分道后抢道"
		default:
			r.StartLabel = "分道"
		}
	}
	if e.Kind == KindDistance {
		r.AttemptLabel = "试跳"
		if e.DistanceMode == DistanceThrow {
			r.AttemptLabel = "试投"
		}
	}
	return r
}

func (e Event) TeamSizeWarning(count int) string {
	if e.Kind == KindRelay && e.RelayLegCount > 0 && count != e.RelayLegCount {
		return "棒数与项目建议不符"
	}
	return ""
}
