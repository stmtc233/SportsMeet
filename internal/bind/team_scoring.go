package bind

import (
	"context"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/service"
)

type TeamScoring struct{ *Core }

func (t TeamScoring) Get(ctx context.Context, filter service.TeamScoreFilter) (*service.TeamScoreDetail, error) {
	return t.Svc.GetTeamScores(ctx, filter)
}

func (t TeamScoring) Boards(ctx context.Context, filter service.TeamScoreFilter) ([]service.TeamScoreBoard, error) {
	return t.Svc.GetTeamScoreBoards(ctx, filter)
}

// Save parses the rules as typed and saves them right away.
func (t TeamScoring) Save(ctx context.Context, meetID string, in service.TeamScoringInput) (*domain.TeamScoringRules, error) {
	return t.Svc.SaveTeamScoring(ctx, ui, meetID, in)
}
