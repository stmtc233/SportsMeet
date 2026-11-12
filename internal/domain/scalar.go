package domain

import "errors"

func (k EventKind) Scalar() bool { return k == KindCount || k == KindScore }
func (e Event) IsTeam() bool     { return e.Kind == KindRelay || e.ParticipantMode == "team" }
func (e Event) Ascending() bool {
	return e.Kind.Timed() || (e.Kind.Scalar() && e.RankDirection == "asc")
}
func (e Event) MultipleAttempts() bool {
	return e.Kind == KindDistance || (e.Kind.Scalar() && e.AttemptMode != "single" && e.AttemptMode != "")
}

func (e *Event) normalizeCompetition() error {
	if e.ParticipantMode == "" {
		if e.Kind == KindRelay {
			e.ParticipantMode = "team"
		} else {
			e.ParticipantMode = "athlete"
		}
	}
	if e.ParticipantMode != "athlete" && e.ParticipantMode != "team" {
		return errors.New("请选择参赛方式")
	}
	if e.Kind == KindRelay && e.ParticipantMode != "team" {
		return errors.New("接力须按队伍参赛")
	}
	if e.IsTeam() && !e.Kind.Timed() && !e.Kind.Scalar() {
		return errors.New("请选择计时、计次或计分")
	}
	if e.Kind == KindRelay && e.TeamSize != 0 {
		return errors.New("接力请设置建议棒数")
	}
	if e.TeamSize < 0 {
		return errors.New("建议人数不能为负数")
	}
	if !e.IsTeam() && e.TeamSize != 0 {
		return errors.New("个人项目不能设置建议人数")
	}
	if e.Kind.Scalar() {
		if e.RankDirection == "" {
			e.RankDirection = "desc"
		}
		if e.RankDirection != "asc" && e.RankDirection != "desc" {
			return errors.New("请选择排名方向")
		}
		if e.AttemptMode == "" {
			e.AttemptMode = "single"
		}
		if e.AttemptMode != "single" && e.AttemptMode != "best" && e.AttemptMode != "sum" {
			return errors.New("请选择成绩汇总方式")
		}
		if e.ScoreDecimals < 0 || e.ScoreDecimals > 3 || (e.Kind == KindCount && e.ScoreDecimals != 0) {
			return errors.New("成绩小数位应为 0 到 3")
		}
	} else if e.RankDirection != "" || e.AttemptMode != "" || e.ScoreDecimals != 0 {
		return errors.New("该项目不能设置计次或计分规则")
	}
	return nil
}

// CompetitionConfig is persisted as one JSON column; all values are event-local.
type CompetitionConfig struct {
	ParticipantMode string `json:"participantMode,omitempty"`
	TeamSize        int    `json:"teamSize,omitempty"`
	ScoreDecimals   int    `json:"scoreDecimals,omitempty"`
	RankDirection   string `json:"rankDirection,omitempty"`
	AttemptMode     string `json:"attemptMode,omitempty"`
}
