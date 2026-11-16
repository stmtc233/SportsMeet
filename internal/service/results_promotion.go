package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// planPromotions materializes new heats in the in-memory import snapshot. The
// caller adds marks and verifies qualification before committing all statements.
func (s *Service) planPromotions(ctx context.Context, plan *resultsPlan, f *interchange.ResultsFile, meetID string,
	events map[string]*eventState, rounds map[string]*importRound, order *[]*importRound) (map[string]*importRound, error) {
	ids := []string{}
	for _, p := range f.Promotions {
		for _, h := range p.Heats {
			ids = append(ids, h.HeatID)
		}
	}
	existingIDs := map[string]bool{}
	if len(ids) > 0 {
		raw, _ := json.Marshal(ids)
		rows, err := s.store.Query(ctx, "SELECT id FROM heats WHERE id IN (SELECT value FROM json_each(?))", string(raw))
		if err != nil {
			return nil, err
		}
		for _, row := range rows.Values {
			existingIDs[row[0].(string)] = true
		}
	}

	byHeat := map[string]*importRound{}
	touch := func(st *eventState, r *domain.Round) *importRound {
		if ir := rounds[r.ID]; ir != nil {
			return ir
		}
		ir := &importRound{st: st, round: r, before: storedResults(r.Heats), heats: copyHeats(r.Heats)}
		rounds[r.ID] = ir
		*order = append(*order, ir)
		return ir
	}
	for _, p := range f.Promotions {
		st := events[p.EventID]
		if st == nil {
			var err error
			st, err = s.loadEventParticipants(ctx, p.EventID)
			if err != nil {
				return nil, err
			}
			events[p.EventID] = st
		}
		if st.event.MeetID != meetID || st.event.Kind != domain.KindTrack || st.event.IsTeam() {
			plan.preview.Errors = append(plan.preview.Errors, "晋级项目不属于本赛事的个人计时项目")
			continue
		}
		source, target := st.round(p.SourceRoundID), st.round(p.TargetRoundID)
		if source == nil || target == nil || target.Ordinal != source.Ordinal+1 || source.Advance == nil {
			plan.preview.Errors = append(plan.preview.Errors, "晋级目标不是已有的下一轮")
			continue
		}
		src := touch(st, source)
		dst := touch(st, target)
		// Qualification is recalculated from imported marks, using the same rules as UI input.
		if source.Advance.ByPlace != 0 || source.Advance.ByMark != p.Count {
			plan.preview.Warnings = append(plan.preview.Warnings, fmt.Sprintf("「%s」按成绩晋级 %d 人", source.Label(), p.Count))
		}
		source.Advance = &domain.Advance{ByMark: p.Count}
		src.round = source
		plan.stmts = append(plan.stmts, store.Stmt("UPDATE rounds SET advance_by_place=0,advance_by_mark=? WHERE id=?", p.Count, source.ID))
		have := map[string]domain.Heat{}
		for _, h := range dst.heats {
			have[h.ID] = h
		}
		conflict := len(dst.heats) > 0 && len(dst.heats) != len(p.Heats)
		planned := []domain.Heat{}
		for _, h := range p.Heats {
			want := domain.Heat{ID: h.HeatID, RoundID: target.ID, Ordinal: h.Ordinal, Name: domain.NormalizeHeatName(h.Name), Slots: []domain.Slot{}}
			if h.ScheduledAt != nil {
				want.ScheduledAt = *h.ScheduledAt
			}
			for _, sl := range h.Slots {
				if _, ok := st.athletes[sl.AthleteID]; !ok {
					plan.preview.Errors = append(plan.preview.Errors, "晋级运动员不在项目名单中")
				}
				if st.event.UsesLanes() && sl.Position > st.event.LaneCount {
					plan.preview.Errors = append(plan.preview.Errors, "晋级道次超出项目道数")
				}
				want.Slots = append(want.Slots, domain.Slot{Position: sl.Position, AthleteID: sl.AthleteID})
			}
			sort.Slice(want.Slots, func(i, j int) bool { return want.Slots[i].Position < want.Slots[j].Position })
			if old, ok := have[h.HeatID]; ok {
				if !samePromotionSlots(old, want) {
					conflict = true
				}
				planned = append(planned, old) // preserve the target's times, names and existing results
			} else {
				if len(dst.heats) > 0 {
					conflict = true
				}
				if existingIDs[h.HeatID] {
					conflict = true
				}
				planned = append(planned, want)
			}
		}
		if conflict {
			plan.preview.Errors = append(plan.preview.Errors, fmt.Sprintf("「%s」已有不同编排，未覆盖", target.Label()))
			continue
		}
		if len(dst.heats) == 0 {
			for _, h := range planned {
				plan.stmts = append(plan.stmts, store.Stmt(`INSERT INTO heats(id,round_id,ordinal,name,scheduled_at) VALUES(?,?,?,?,?)`, h.ID, target.ID, h.Ordinal, h.Name, h.ScheduledAt))
				for _, sl := range h.Slots {
					plan.stmts = append(plan.stmts, insertSlot(h.ID, sl))
				}
			}
			plan.preview.Warnings = append(plan.preview.Warnings, fmt.Sprintf("复用「%s」，新增 %d 组晋级编排", target.Label(), len(planned)))
		}
		dst.heats = planned
		for _, h := range dst.heats {
			byHeat[h.ID] = dst
		}
	}
	return byHeat, nil
}
func samePromotionSlots(a, b domain.Heat) bool {
	if a.Ordinal != b.Ordinal || len(a.Slots) != len(b.Slots) {
		return false
	}
	byPos := map[int]string{}
	for _, s := range a.Slots {
		byPos[s.Position] = s.AthleteID
	}
	for _, s := range b.Slots {
		if byPos[s.Position] != s.AthleteID {
			return false
		}
	}
	return true
}
func validatePromotionQualifiers(f *interchange.ResultsFile, rounds map[string]*importRound) []string {
	var errors []string
	for _, p := range f.Promotions {
		src, dst := rounds[p.SourceRoundID], rounds[p.TargetRoundID]
		if src == nil || dst == nil {
			continue
		}
		want := map[string]bool{}
		complete := true
		for _, h := range src.heats {
			for _, sl := range h.Slots {
				if sl.CompetitorID() == "" {
					continue
				}
				if sl.Result == nil {
					complete = false
					continue
				}
				if sl.Result.Qualify != "" {
					want[sl.AthleteID] = true
				}
			}
		}
		got := map[string]bool{}
		for _, h := range dst.heats {
			for _, sl := range h.Slots {
				if sl.AthleteID != "" {
					got[sl.AthleteID] = true
				}
			}
		}
		same := len(want) == len(got) && len(want) >= p.Count
		for id := range want {
			if !got[id] {
				same = false
			}
		}
		if !complete || !same {
			errors = append(errors, "晋级名单与来源成绩或本机并列裁定不符，请重新核对")
		}
	}
	return errors
}
