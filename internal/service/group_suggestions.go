package service

import (
	"context"
	"slices"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

type GroupSuggestion struct {
	Kind       string   `json:"kind"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Candidates []string `json:"candidates"`
	Reason     string   `json:"reason"`
}

func (s *Service) SuggestGroups(ctx context.Context, meetID string) ([]GroupSuggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadTeamScoreState(ctx, meetID)
	if err != nil {
		return nil, err
	}
	byAthlete := map[string][]string{}
	byUnit := map[string][]string{}
	add := func(m map[string][]string, id, gid string) {
		if id != "" && gid != "" && !slices.Contains(m[id], gid) {
			m[id] = append(m[id], gid)
		}
	}
	teams := map[string]struct {
		unit    string
		members []string
	}{}
	for _, t := range st.teams {
		teams[t.ID] = struct {
			unit    string
			members []string
		}{t.UnitID, t.MemberIDs}
	}
	athletes := map[string]domain.Athlete{}
	for _, a := range st.combined.athletes {
		athletes[a.ID] = a
		for _, g := range st.groups {
			if a.Grade != "" && domain.NormalizeText(a.Grade) == domain.NormalizeText(g.Name) {
				add(byAthlete, a.ID, g.ID)
			}
		}
	}
	for _, en := range st.entries {
		gid := st.combined.events[en.EventID].GroupID
		if en.AthleteID != "" {
			add(byAthlete, en.AthleteID, gid)
			add(byUnit, athletes[en.AthleteID].UnitID, gid)
		}
		if t, ok := teams[en.TeamID]; ok {
			add(byUnit, t.unit, gid)
			for _, id := range t.members {
				add(byAthlete, id, gid)
			}
		}
	}
	for _, a := range st.combined.athletes {
		add(byUnit, a.UnitID, a.GroupID)
	}
	out := []GroupSuggestion{}
	for _, u := range st.units {
		if len(u.GroupIDs) == 0 {
			out = append(out, GroupSuggestion{Kind: "unit", ID: u.ID, Name: u.Name, Candidates: append([]string{}, byUnit[u.ID]...), Reason: "已有报名与运动员归属"})
		}
	}
	for _, a := range st.combined.athletes {
		if a.GroupID == "" {
			for _, u := range st.units {
				if u.ID == a.UnitID && len(u.GroupIDs) == 1 {
					add(byAthlete, a.ID, u.GroupIDs[0])
				}
			}
			out = append(out, GroupSuggestion{Kind: "athlete", ID: a.ID, Name: a.Name, Candidates: append([]string{}, byAthlete[a.ID]...), Reason: "已有报名、年级与单位归属"})
		}
	}
	return out, nil
}
