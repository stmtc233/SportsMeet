package service

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

type TeamScoreBoard struct {
	GroupID   string           `json:"groupId"`
	GroupName string           `json:"groupName"`
	Detail    *TeamScoreDetail `json:"detail"`
}

func (st *teamScoreState) boards(filter TeamScoreFilter, rules domain.TeamScoringRules) ([]TeamScoreBoard, error) {
	if err := st.validateFilter(filter); err != nil {
		return nil, err
	}
	groups := append([]domain.CompetitionGroup{}, st.groups...)
	ungrouped := len(groups) == 0
	for _, t := range st.targets {
		ungrouped = ungrouped || t.GroupID == ""
	}
	if ungrouped {
		groups = append(groups, domain.CompetitionGroup{Name: "未分组"})
	}
	out := []TeamScoreBoard{}
	for _, g := range groups {
		if filter.GroupID != nil && *filter.GroupID != g.ID {
			continue
		}
		f := filter
		f.GroupID = ptr(g.ID)
		d, err := st.detail(f, rules)
		if err != nil {
			return nil, err
		}
		out = append(out, TeamScoreBoard{GroupID: g.ID, GroupName: g.Name, Detail: d})
	}
	return out, nil
}

// GetTeamScoreBoards reads on the read connections; a write landing midway
// sends a change event and the page reads again.
func (s *Service) GetTeamScoreBoards(ctx context.Context, filter TeamScoreFilter) ([]TeamScoreBoard, error) {
	st, err := s.loadTeamScoreState(ctx, filter.MeetID)
	if err != nil {
		return nil, err
	}
	if filter.GroupID != nil && *filter.GroupID != "" {
		if err := s.validateGroupIDs(ctx, filter.MeetID, []string{*filter.GroupID}); err != nil {
			return nil, err
		}
	}
	return st.boards(filter, st.rules)
}
