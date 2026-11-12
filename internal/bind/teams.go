package bind

import (
	"context"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/service"
)

// SaveTeam creates or edits a relay team and its ordered roster.
func (e Events) SaveTeam(ctx context.Context, eventID, id string, in service.TeamInput) (*domain.Team, error) {
	return e.Svc.SaveTeam(ctx, ui, eventID, id, in)
}

// DeleteTeam removes an unplaced team and its entry.
func (e Events) DeleteTeam(ctx context.Context, eventID, id string) error {
	return e.Svc.DeleteTeam(ctx, ui, eventID, id)
}
