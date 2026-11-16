package service

import (
	"context"
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/examplemeet"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// CreateExampleMeet creates an independent copy of the bundled fictional meet.
// The ordinary import planner validates and remaps every ID. One transaction,
// audit entry and notification cover the entire copy, with no startup seeding.
func (s *Service) CreateExampleMeet(ctx context.Context, src store.Source) (string, error) {
	data, err := interchange.Encode(examplemeet.Build())
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	plan, err := s.planImport(ctx, data, "")
	if err != nil {
		return "", err
	}
	if len(plan.preview.Errors) > 0 {
		return "", fmt.Errorf("示例赛事有误：%s", interchange.Problems(plan.preview.Errors))
	}
	plan.preview.MeetID = plan.meetID
	if err := s.write(ctx,
		store.Audit{Source: src, Action: "meet.example", MeetID: plan.meetID, Message: plan.preview.MeetName, Detail: plan.preview},
		Change{Kind: "meets", MeetID: plan.meetID}, plan.stmts...); err != nil {
		return "", err
	}
	return plan.meetID, nil
}
