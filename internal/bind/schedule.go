package bind

import (
	"context"
	"github.com/stmtc233/sportsmeet/internal/service"
)

func (a Arrange) PreviewSchedule(ctx context.Context, in service.ScheduleRequest) (*service.SchedulePreview, error) {
	return a.Svc.PreviewSchedule(ctx, in)
}

// Schedule confirms the preview shown in the desktop dialog.
func (a Arrange) Schedule(ctx context.Context, in service.ScheduleRequest, hash string) (*service.SchedulePreview, error) {
	return a.Svc.ApplySchedule(ctx, ui, in, hash, true)
}
