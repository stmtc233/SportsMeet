package interchange

import (
	"fmt"
	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ResultsPromotion reuses the existing next round of the same event.
type ResultsPromotion struct {
	EventID       string          `json:"eventId"`
	SourceRoundID string          `json:"sourceRoundId"`
	TargetRoundID string          `json:"targetRoundId"`
	Count         int             `json:"count"`
	Heats         []PromotionHeat `json:"heats"`
}
type PromotionHeat struct {
	HeatID      string          `json:"heatId"`
	Ordinal     int             `json:"ordinal"`
	Name        string          `json:"name"`
	ScheduledAt *string         `json:"scheduledAt,omitempty"`
	Slots       []PromotionSlot `json:"slots"`
}
type PromotionSlot struct {
	Position  int    `json:"position"`
	AthleteID string `json:"athleteId"`
}

func validatePromotions(plans []ResultsPromotion) Problems {
	var p Problems
	sources, targets, heats := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, plan := range plans {
		add := func(msg string) { p = append(p, fmt.Sprintf("promotions[%d]：%s", i, msg)) }
		if plan.EventID == "" || plan.SourceRoundID == "" || plan.TargetRoundID == "" || plan.SourceRoundID == plan.TargetRoundID || plan.Count < 1 {
			add("晋级项目、轮次或人数不正确")
		}
		if sources[plan.SourceRoundID] || targets[plan.TargetRoundID] {
			add("晋级轮次重复")
		}
		sources[plan.SourceRoundID] = true
		targets[plan.TargetRoundID] = true
		if len(plan.Heats) == 0 {
			add("缺少晋级编排")
		}
		people, ordinals := map[string]bool{}, map[int]bool{}
		for _, h := range plan.Heats {
			if h.HeatID == "" || heats[h.HeatID] || h.Ordinal < 1 || ordinals[h.Ordinal] {
				add("晋级组 ID 或序号不正确")
			}
			heats[h.HeatID] = true
			ordinals[h.Ordinal] = true
			if h.ScheduledAt != nil && !domain.ValidScheduled(*h.ScheduledAt) {
				add("晋级组计划时间不正确")
			}
			if len(h.Slots) == 0 {
				add("晋级组没有运动员")
			}
			positions := map[int]bool{}
			for _, sl := range h.Slots {
				if sl.Position < 1 || positions[sl.Position] || sl.AthleteID == "" || people[sl.AthleteID] {
					add("晋级运动员或位置重复、缺失")
				}
				positions[sl.Position] = true
				people[sl.AthleteID] = true
			}
		}
		if len(people) < plan.Count {
			add("晋级人数不足")
		}
	}
	return p
}
