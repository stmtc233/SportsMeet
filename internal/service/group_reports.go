package service

import (
	"context"
	"slices"
	"strconv"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
)

type GroupReportRequest struct {
	Kind     string   `json:"kind"`
	MeetID   string   `json:"meetId"`
	GroupIDs []string `json:"groupIds"` // empty = all; "" selects ungrouped
	EventIDs []string `json:"eventIds"`
}

func (s *Service) RenderGroupedReport(ctx context.Context, req GroupReportRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Kind != ReportResults && req.Kind != ReportTeamScores {
		return "", invalid("报表类型不正确")
	}
	st, err := s.loadTeamScoreState(ctx, req.MeetID)
	if err != nil {
		return "", err
	}
	for _, id := range req.GroupIDs {
		if id != "" && !slices.ContainsFunc(st.groups, func(g domain.CompetitionGroup) bool { return g.ID == id }) {
			return "", invalid("组别不属于本赛事")
		}
	}
	for _, id := range req.EventIDs {
		_, eventOK := st.combined.events[id]
		combinedOK := false
		for _, parent := range st.combined.parents {
			if parent.Combined.ID == id {
				combinedOK = true
				break
			}
		}
		if !eventOK && !combinedOK {
			return "", invalid("项目或全能不属于本赛事")
		}
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	root := report.Book{Meet: st.meet.Name, Dates: domain.DateRangeText(st.meet.StartDate, st.meet.EndDate), Location: st.meet.Location, PrintedAt: domain.RecordedText(s.store.Now(), s.zone), NewPage: prefs.PrintNewPage, GroupBooks: []report.Book{}}
	groups := append([]domain.CompetitionGroup{}, st.groups...)
	ungrouped := len(groups) == 0
	for _, t := range st.targets {
		ungrouped = ungrouped || t.GroupID == ""
	}
	if ungrouped {
		groups = append(groups, domain.CompetitionGroup{Name: "未分组"})
	}
	for _, g := range groups {
		if len(req.GroupIDs) > 0 && !slices.Contains(req.GroupIDs, g.ID) {
			continue
		}
		ids := []string{}
		for _, t := range st.targets {
			if t.GroupID == g.ID && (len(req.EventIDs) == 0 || slices.Contains(req.EventIDs, t.id())) {
				ids = append(ids, t.id())
			}
		}
		// Existing callers can print a component on its own; it earns no team points.
		for _, e := range st.combined.events {
			if e.CombinedID != "" && e.GroupID == g.ID && slices.Contains(req.EventIDs, e.ID) {
				ids = append(ids, e.ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		book := &report.Book{}
		if req.Kind == ReportResults {
			book, err = s.reportBook(ctx, req.MeetID, ids, prefs.PrintResultsTime, true)
			if err != nil {
				return "", err
			}
		}
		book.GroupName = g.Name
		book.PrintedAt = ""
		book.TeamOnly = req.Kind == ReportTeamScores
		if st.rules.Enabled && (req.Kind == ReportTeamScores || prefs.PrintTeamScores) {
			targets := []string{}
			for _, t := range st.targets {
				if slices.Contains(ids, t.id()) {
					targets = append(targets, t.id())
				}
			}
			if len(targets) > 0 {
				d, err := st.detail(TeamScoreFilter{MeetID: req.MeetID, GroupID: ptr(g.ID), TargetIDs: targets}, st.rules)
				if err != nil {
					return "", err
				}
				table := &report.TeamScoreTable{State: d.State, Counted: d.Counted, Total: d.Total, Scope: "全部项目", Rows: []report.TeamScoreRow{}}
				if len(req.EventIDs) > 0 {
					table.Scope = "所选项目"
					for _, src := range d.Sources {
						if src.Multiplier > 0 {
							table.Targets = append(table.Targets, src.Name)
						}
					}
				}
				for _, r := range d.Rows {
					row := report.TeamScoreRow{Unit: r.UnitName, Male: r.Male.Text, Female: r.Female.Text, Mixed: r.Mixed.Text, Total: r.Total.Text}
					if r.Rank != nil {
						row.Rank = strconv.Itoa(*r.Rank)
					}
					table.Rows = append(table.Rows, row)
				}
				book.TeamScores = table
			}
		}
		root.GroupBooks = append(root.GroupBooks, *book)
	}
	if req.Kind == ReportTeamScores {
		return report.TeamScores(root)
	}
	return report.Results(root)
}
