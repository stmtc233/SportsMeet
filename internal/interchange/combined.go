package interchange

import (
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func validateCombineds(f *File, athletes map[string]bool) Problems {
	p := Problems{}
	add := func(format string, args ...any) { p = append(p, fmt.Sprintf(format, args...)) }
	events := map[string]Event{}
	for _, e := range f.Events {
		events[e.ID] = e
	}
	parents := map[string]bool{}
	keys := map[string]bool{}
	owned := map[string]bool{}
	for i, c := range f.Combineds {
		at := fmt.Sprintf("combineds[%d]", i)
		if c.ID == "" || parents[c.ID] {
			add("%s.id 为空或重复", at)
		}
		parents[c.ID] = true
		g, ok := domain.NormalizeEventGender(c.Gender)
		if !ok || string(g) != c.Gender {
			add("%s.gender 不正确", at)
		}
		key := domain.CombinedKey(g, c.Name, c.GroupLabel)
		if domain.NormalizeText(c.Name) == "" || keys[key] {
			add("%s.name 为空或重复", at)
		}
		keys[key] = true
		if c.RuleVersion < 1 {
			add("%s.ruleVersion 须为正整数", at)
		}
		if len(c.Components) < 2 {
			add("%s 至少需要两个分项", at)
		}
		roster := map[string]bool{}
		for _, id := range c.AthleteIDs {
			if !athletes[id] || roster[id] {
				add("%s.athleteIds 不存在或重复", at)
			}
			roster[id] = true
		}
		for j, part := range c.Components {
			e, ok := events[part.EventID]
			if !ok || e.CombinedID != c.ID || owned[e.ID] {
				add("%s.components[%d] 分项不存在、重复或不属于本全能", at, j)
				continue
			}
			owned[e.ID] = true
			if part.Ordinal != j+1 {
				add("%s.components 顺序须连续", at)
			}
			cfg := domain.Event{CompetitionConfig: e.CompetitionConfig, Kind: domain.EventKind(e.Kind), StartMode: domain.StartMode(e.StartMode), DistanceMode: domain.DistanceMode(e.DistanceMode)}
			if cfg.IsTeam() || e.Gender != c.Gender || e.GroupLabel != c.GroupLabel {
				add("%s.components 分项性别、组别或参赛方式不符", at)
			}
			if err := part.Rule.Validate(cfg); err != nil {
				add("%s.components[%d]：%s", at, j, err)
			}
			if len(e.Rounds) != 1 || e.Rounds[0].Stage != "final" || e.Rounds[0].Advance != nil {
				add("%s.components 分项须直接决赛", at)
			}
			entryIDs := map[string]bool{}
			for _, en := range e.Entries {
				entryIDs[en.AthleteID] = true
				if !roster[en.AthleteID] {
					add("%s 分项报名与全能不符", at)
				}
			}
			for id := range roster {
				if !entryIDs[id] {
					add("%s 分项缺少全能报名者", at)
				}
			}
		}
	}
	for _, e := range f.Events {
		if e.CombinedID != "" && (!parents[e.CombinedID] || !owned[e.ID]) {
			add("events[%s].combinedId 找不到全能分项", e.ID)
		}
	}
	return p
}
