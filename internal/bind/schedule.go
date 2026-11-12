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

// SetRoundStart writes a round's start time; its warnings never block it.
func (a Arrange) SetRoundStart(ctx context.Context, in service.RoundStart) (*service.RoundStartResult, error) {
	return a.Svc.SetRoundStart(ctx, ui, in)
}

// ScheduleWarning is what a time dialog shows before saving: "" or a
// warning such as a day outside the meet.
func (a Arrange) ScheduleWarning(ctx context.Context, eventID, at string) (string, error) {
	return a.Svc.ScheduleWarning(ctx, eventID, at)
}

// HeatPlan checks the heat dialog's time and duration (minutes as typed)
// and shows them as the event detail does, from the round's start and the
// event's shown day.
func (Arrange) HeatPlan(roundStart, at, minutes, day string) (*service.HeatPlan, error) {
	return service.CheckHeatPlan(roundStart, at, minutes, day)
}

// MeetSchedule is the meet's timetable by day with its conflicts.
func (a Arrange) MeetSchedule(ctx context.Context, meetID string) (*service.Timetable, error) {
	return a.Svc.Schedule(ctx, meetID)
}

// HeatConflicts lists the athlete time conflicts of an event's heats.
func (a Arrange) HeatConflicts(ctx context.Context, eventID string) ([]service.HeatConflicts, error) {
	return a.Svc.EventConflicts(ctx, eventID)
}

// EditHeat saves one heat's name, time and duration from the schedule page.
func (a Arrange) EditHeat(ctx context.Context, in service.HeatEdit) error {
	return a.Svc.EditHeat(ctx, ui, in)
}
