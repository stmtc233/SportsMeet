package bind

import (
	"context"
	"github.com/stmtc233/sportsmeet/internal/service"
)

type TeamScoring struct{ *Core }

func (t TeamScoring) Get(ctx context.Context, filter service.TeamScoreFilter) (*service.TeamScoreDetail, error) {
	return t.Svc.GetTeamScores(ctx, filter)
}
func (t TeamScoring) Preview(ctx context.Context, meetID string, in service.TeamScoringInput) (*service.TeamScoringPreview, error) {
	rules, err := service.ParseTeamScoringInput(in)
	if err != nil {
		return nil, err
	}
	return t.Svc.PreviewTeamScoring(ctx, service.TeamScoringRequest{MeetID: meetID, Rules: rules})
}
func (t TeamScoring) Apply(ctx context.Context, req service.TeamScoringRequest, hash string, confirmed bool) (*service.TeamScoringPreview, error) {
	return t.Svc.ApplyTeamScoring(ctx, ui, req, hash, confirmed)
}
