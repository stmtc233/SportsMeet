package bind

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/service"
)

type Groups struct{ *Core }

func (g Groups) List(ctx context.Context, meetID string) ([]service.GroupView, error) {
	return g.Svc.ListGroups(ctx, meetID)
}
func (g Groups) Save(ctx context.Context, meetID, id string, in service.GroupInput) (*domain.CompetitionGroup, error) {
	return g.Svc.SaveGroup(ctx, ui, meetID, id, in)
}
func (g Groups) Delete(ctx context.Context, meetID, id string) error {
	return g.Svc.DeleteGroup(ctx, ui, meetID, id)
}
func (g Groups) Reorder(ctx context.Context, meetID string, ids []string) error {
	return g.Svc.ReorderGroups(ctx, ui, meetID, ids)
}
func (g Groups) Suggestions(ctx context.Context, meetID string) ([]service.GroupSuggestion, error) {
	return g.Svc.SuggestGroups(ctx, meetID)
}

// Assign saves unit and athlete groups right away.
func (g Groups) Assign(ctx context.Context, req service.GroupAssignmentsRequest) error {
	return g.Svc.AssignGroups(ctx, ui, req)
}

// Change moves events and combined events to a group right away.
func (g Groups) Change(ctx context.Context, req service.GroupChangeRequest) error {
	return g.Svc.ChangeGroup(ctx, ui, req)
}

func (r Reports) RenderGrouped(ctx context.Context, req service.GroupReportRequest) (string, error) {
	return r.Svc.RenderGroupedReport(ctx, req)
}
func (r Reports) PrintGrouped(ctx context.Context, req service.GroupReportRequest) error {
	html, err := r.Svc.RenderGroupedReport(ctx, req)
	if err != nil {
		return err
	}
	title := "成绩册"
	if req.Kind == service.ReportTeamScores {
		title = "团体总分表"
	}
	printWindow(title, html)
	return nil
}
