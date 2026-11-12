package service

import (
	"context"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// MeetInput creates or updates a meet.
type MeetInput struct {
	Name      string `json:"name"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate,omitempty"`
	Location  string `json:"location,omitempty"`
	Note      string `json:"note,omitempty"`
}

func (in MeetInput) meet() (domain.Meet, error) {
	m := domain.Meet{
		Name:      strings.TrimSpace(in.Name),
		StartDate: strings.TrimSpace(in.StartDate),
		EndDate:   strings.TrimSpace(in.EndDate),
		Location:  strings.TrimSpace(in.Location),
		Note:      strings.TrimSpace(in.Note),
	}
	switch {
	case m.Name == "":
		return m, invalid("请填写赛事名称")
	case !domain.ValidDate(m.StartDate):
		return m, invalid("开始日期格式不正确")
	case m.EndDate != "" && !domain.ValidDate(m.EndDate):
		return m, invalid("结束日期格式不正确")
	case m.EndDate != "" && m.EndDate < m.StartDate:
		return m, invalid("结束日期早于开始日期")
	}
	return m, nil
}

// MeetView is a meet with its days as shown.
type MeetView struct {
	domain.Meet
	DatesText string `json:"datesText"` // domain.DateRangeText
}

func meetView(m domain.Meet) MeetView {
	return MeetView{Meet: m, DatesText: domain.DateRangeText(m.StartDate, m.EndDate)}
}

// ListMeets returns all meets.
func (s *Service) ListMeets(ctx context.Context) ([]MeetView, error) {
	list, err := s.store.ListMeets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MeetView, len(list))
	for i, m := range list {
		out[i] = meetView(m)
	}
	return out, nil
}

// MeetDetail returns one meet as the pages and MCP show it.
func (s *Service) MeetDetail(ctx context.Context, id string) (*MeetView, error) {
	m, err := s.GetMeet(ctx, id)
	if err != nil {
		return nil, err
	}
	v := meetView(*m)
	return &v, nil
}

// GetMeet returns one meet.
func (s *Service) GetMeet(ctx context.Context, id string) (*domain.Meet, error) {
	m, err := s.store.GetMeet(ctx, id)
	if err == nil && m == nil {
		err = invalid("赛事不存在或已删除")
	}
	return m, err
}

// CountMeet returns what deleting the meet would remove.
func (s *Service) CountMeet(ctx context.Context, id string) (store.MeetCounts, error) {
	return s.store.Count(ctx, store.Scope{MeetID: id})
}

// CreateMeet validates and creates a meet.
func (s *Service) CreateMeet(ctx context.Context, src store.Source, in MeetInput) (*domain.Meet, error) {
	m, err := in.meet()
	if err != nil {
		return nil, err
	}
	m.ID = newID()
	s.mu.Lock()
	defer s.mu.Unlock()
	m.CreatedAt = s.store.Now()
	err = s.write(ctx,
		store.Audit{Source: src, Action: "meet.created", MeetID: m.ID, Message: m.Name},
		Change{Kind: "meets", MeetID: m.ID},
		store.InsertMeet(m))
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMeet changes a meet's fields.
func (s *Service) UpdateMeet(ctx context.Context, src store.Source, id string, in MeetInput) error {
	m, err := in.meet()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, id); err != nil {
		return err
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "meet.updated", MeetID: id, Message: m.Name},
		Change{Kind: "meets", MeetID: id},
		store.Stmt(`UPDATE meets SET name = ?, start_date = ?, end_date = ?, location = ?, note = ? WHERE id = ?`,
			m.Name, m.StartDate, m.EndDate, m.Location, m.Note, id))
}

// DeleteMeet removes a meet with everything in it (foreign keys cascade).
func (s *Service) DeleteMeet(ctx context.Context, src store.Source, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.GetMeet(ctx, id)
	if err != nil {
		return err
	}
	counts, err := s.store.Count(ctx, store.Scope{MeetID: id})
	if err != nil {
		return err
	}
	if src == store.SourceMCP && counts.Results > 0 {
		return invalid("赛事已有成绩，只能在界面上删除")
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "meet.deleted", MeetID: id, Message: m.Name, Detail: counts},
		Change{Kind: "meets", MeetID: id},
		store.Stmt(`DELETE FROM meets WHERE id = ?`, id))
}
