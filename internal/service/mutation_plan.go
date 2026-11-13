package service

import (
	"context"
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func (s *Service) planMutation(ctx context.Context, in Mutation, before any) (any, bool, error) {
	const src = store.SourceMCP
	switch in.Action {
	case "delete_group":
		return in.ID, false, s.DeleteGroup(ctx, src, in.MeetID, in.ID)
	case "reorder_groups":
		return in.IDs, false, s.ReorderGroups(ctx, src, in.MeetID, in.IDs)
	case "save_combined":
		c, err := s.SaveCombined(ctx, src, in.MeetID, in.CombinedID, in.Combined)
		return c, false, err
	case "combined_entries":
		err := s.SetCombinedEntries(ctx, src, in.CombinedID, in.IDs)
		return in.IDs, false, err
	case "delete_combined":
		detail, err := s.GetCombined(ctx, in.CombinedID)
		if err != nil {
			return nil, false, err
		}
		return detail, false, s.DeleteCombined(ctx, src, in.CombinedID)

	case "add_entries":
		err := s.AddEntries(ctx, src, in.EventID, in.Entries)
		return in.Entries, false, err
	case "remove_entries":
		err := s.RemoveEntries(ctx, src, in.EventID, in.IDs)
		return in.IDs, false, err
	case "import_athletes":
		p, err := s.PreviewAthleteImport(ctx, in.MeetID, in.Table)
		if err != nil {
			return nil, false, err
		}
		_, err = s.ImportAthletes(ctx, src, in.MeetID, in.Table)
		return p, false, err
	case "import_meet":
		p, err := s.PreviewImport(ctx, []byte(in.Data), in.MeetID)
		if err != nil {
			return nil, false, err
		}
		p.Errors = append(p.Errors, mcpImportErrors([]byte(in.Data))...)
		if len(p.Errors) > 0 {
			return p, false, invalid("文件有误，无法导入")
		}
		imported, err := s.ImportMeet(ctx, src, []byte(in.Data), in.MeetID, p.Hash)
		return imported, false, err
	case "auto_arrange", "advance":
		return s.prepareArrangement(ctx, in)
	case "set_results":
		return s.prepareResults(ctx, in)
	case "delete_meet":
		counts, err := s.CountMeet(ctx, in.MeetID)
		if err != nil {
			return nil, false, err
		}
		if counts.Results > 0 {
			return before, false, invalid("赛事已有成绩，只能在界面上删除")
		}
		return before, false, s.DeleteMeet(ctx, src, in.MeetID)
	case "delete_team":
		return before, false, s.DeleteTeam(ctx, src, in.EventID, in.ID)
	case "delete_event":
		return before, false, s.DeleteEvent(ctx, src, in.EventID, false)
	case "delete_unit":
		units, err := s.ListUnits(ctx, in.MeetID)
		if err != nil {
			return nil, false, err
		}
		for _, u := range units {
			if u.ID == in.ID {
				return u, false, s.DeleteUnit(ctx, src, in.MeetID, in.ID)
			}
		}
		return nil, false, invalid("参赛单位不存在或已删除")
	case "delete_athletes":
		athletes, err := s.ListAthletes(ctx, in.MeetID)
		if err != nil {
			return nil, false, err
		}
		selected := []AthleteRow{}
		for _, id := range in.IDs {
			found := false
			for _, a := range athletes {
				if a.Athlete.ID == id {
					selected = append(selected, a)
					found = true
					break
				}
			}
			if !found {
				return nil, false, invalid("运动员不存在或已删除")
			}
		}
		if len(selected) == 0 {
			return nil, false, invalid("请选择运动员")
		}
		return selected, false, s.DeleteAthletes(ctx, src, in.MeetID, in.IDs)
	case "delete_heat":
		d, err := s.eventForHeat(ctx, in.HeatID)
		if err != nil {
			return nil, false, err
		}
		for _, r := range d.Rounds {
			rs := RoundSave{RoundID: r.ID, Order: []string{}}
			var removed *HeatView
			for i := range r.Heats {
				h := &r.Heats[i]
				if h.ID == in.HeatID {
					removed = h
					rs.Deleted = []HeatRef{{ID: h.ID, Base: h.Base}}
				} else {
					rs.Order = append(rs.Order, h.ID)
				}
			}
			if removed != nil {
				return removed, false, s.SaveBoard(ctx, src, BoardSave{EventID: d.Event.ID, Rounds: []RoundSave{rs}})
			}
		}
	}
	return nil, false, invalid("操作不存在")
}

func (s *Service) prepareArrangement(ctx context.Context, in Mutation) (any, bool, error) {
	req := in.Plan
	req.FromAdvance = in.Action == "advance"
	d, err := s.GetEvent(ctx, req.EventID)
	if err != nil {
		return nil, false, err
	}
	for _, r := range d.Rounds {
		if r.ID != req.RoundID {
			continue
		}
		rs := RoundSave{RoundID: r.ID, Order: []string{}}
		// Always exclude persisted placements as well as explicitly excluded entrants.
		for _, h := range r.Heats {
			rs.Order = append(rs.Order, h.ID)
			for _, sl := range h.Slots {
				req.Exclude = append(req.Exclude, sl.CompetitorID)
			}
		}
		plan, err := s.PlanHeats(ctx, req)
		if err != nil {
			return nil, false, err
		}
		for i, h := range plan {
			key := fmt.Sprintf("mcp-new-%d", i)
			hs := HeatSave{Key: key, Slots: []SlotSave{}}
			for _, sl := range h.Slots {
				hs.Slots = append(hs.Slots, SlotSave{Position: sl.Position, ID: sl.ID})
			}
			rs.Added = append(rs.Added, hs)
			rs.Order = append(rs.Order, key)
		}
		return plan, false, s.SaveBoard(ctx, store.SourceMCP, BoardSave{EventID: req.EventID, Rounds: []RoundSave{rs}})
	}
	return nil, false, invalid("轮次不存在或已删除")
}

func (s *Service) eventForHeat(ctx context.Context, heatID string) (*EventDetail, error) {
	rows, err := s.store.Query(ctx, `SELECT r.event_id FROM heats h JOIN rounds r ON r.id = h.round_id WHERE h.id = ?`, heatID)
	if err != nil {
		return nil, err
	}
	if len(rows.Values) == 0 {
		return nil, invalid("组不存在或已删除")
	}
	return s.GetEvent(ctx, rows.Values[0][0].(string))
}

type ResultPreviewRow struct {
	Position int            `json:"position"`
	Athlete  string         `json:"athlete"`
	Before   *ResultView    `json:"before"`
	After    *domain.Result `json:"after"`
}

func (s *Service) prepareResults(ctx context.Context, in Mutation) (any, bool, error) {
	d, err := s.eventForHeat(ctx, in.HeatID)
	if err != nil {
		return nil, false, err
	}
	rows := []ResultPreviewRow{}
	overwrite := false
	seen := map[int]bool{}
	for _, r := range d.Rounds {
		for _, h := range r.Heats {
			if h.ID != in.HeatID {
				continue
			}
			for _, input := range in.Results {
				if seen[input.Position] {
					return nil, false, invalid("位置重复")
				}
				seen[input.Position] = true
				// MCP cannot clear a result: deletion of competition records stays in the UI.
				after, err := buildResultEvent(d.Event, input)
				if err != nil {
					return nil, false, err
				}
				if after == nil {
					return nil, false, invalid("请填写成绩或状态")
				}
				found := false
				for _, sl := range h.Slots {
					if sl.Position != input.Position {
						continue
					}
					found = true
					overwrite = overwrite || sl.Result != nil
					rows = append(rows, ResultPreviewRow{Position: input.Position, Athlete: athleteNameFromDetail(d, sl.CompetitorID), Before: sl.Result, After: after})
				}
				if !found {
					return nil, false, invalid("位置为空或已变更")
				}
			}
		}
	}
	if len(in.Results) == 0 {
		return nil, false, invalid("请填写成绩")
	}
	return rows, overwrite, s.SaveResults(ctx, store.SourceMCP, in.HeatID, in.Results)
}

func athleteNameFromDetail(d *EventDetail, id string) string {
	for _, e := range d.Entries {
		if e.CompetitorID == id {
			return e.Name
		}
	}
	return id
}
