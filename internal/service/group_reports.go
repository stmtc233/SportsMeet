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

// RenderGroupedReport renders the 成绩册 or the 团体总分表 by group, in the
// meet's print style (RenderGroupedReportWith chooses the style).
func (s *Service) RenderGroupedReport(ctx context.Context, req GroupReportRequest) (string, error) {
	return htmlOnly(s.RenderGroupedReportWith(ctx, req, RenderOptions{}))
}

// RenderGroupedReportWith is RenderGroupedReport with a style chosen by opt
// instead of the meet's, and the style's warnings.
func (s *Service) RenderGroupedReportWith(ctx context.Context, req GroupReportRequest, opt RenderOptions) (*RenderedReport, error) {
	a, err := s.groupedBook(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.renderAssembled(ctx, req.MeetID, a, opt)
}

// groupedBook gathers the 成绩册 or 团体总分表 by group, under the write
// lock for one consistent read.
func (s *Service) groupedBook(ctx context.Context, req GroupReportRequest) (*assembledReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Kind != ReportResults && req.Kind != ReportTeamScores {
		return nil, invalid("报表类型不正确")
	}
	st, err := s.loadTeamScoreState(ctx, req.MeetID)
	if err != nil {
		return nil, err
	}
	for _, id := range req.GroupIDs {
		if id != "" && !slices.ContainsFunc(st.groups, func(g domain.CompetitionGroup) bool { return g.ID == id }) {
			return nil, invalid("组别不属于本赛事")
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
			return nil, invalid("项目或全能不属于本赛事")
		}
	}
	prefs, err := s.MeetSettings(ctx, req.MeetID)
	if err != nil {
		return nil, err
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
				return nil, err
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
					return nil, err
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
					// A column of zeros says nothing; the mixed one is printed only when some unit scored.
					table.Mixed = table.Mixed || r.Mixed.Text != "" && r.Mixed.Text != "0"
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
	return &assembledReport{kind: req.Kind, book: root, styleID: prefs.PrintStyleID}, nil
}
