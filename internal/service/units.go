package service

import (
	"context"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// UnitInput creates or updates a 参赛单位.
type UnitInput struct {
	Name      string `json:"name"`
	ShortName string `json:"shortName,omitempty"`
}

// ListUnits returns a meet's units.
func (s *Service) ListUnits(ctx context.Context, meetID string) ([]domain.Unit, error) {
	return s.store.ListUnits(ctx, meetID)
}

// SaveUnit creates a unit (id "") or updates one. Names are unique in a
// meet, compared loosely.
func (s *Service) SaveUnit(ctx context.Context, src store.Source, meetID, id string, in UnitInput) (*domain.Unit, error) {
	u := domain.Unit{ID: id, MeetID: meetID, Name: strings.TrimSpace(in.Name), ShortName: strings.TrimSpace(in.ShortName)}
	if u.Name == "" {
		return nil, invalid("请填写单位名称")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	found := id == ""
	for _, o := range units {
		if o.ID == id {
			found = true
		} else if domain.NormalizeText(o.Name) == domain.NormalizeText(u.Name) {
			return nil, invalid("已有同名单位「%s」", o.Name)
		}
	}
	if !found {
		return nil, invalid("参赛单位不存在或已删除")
	}
	a := store.Audit{Source: src, Action: "unit.updated", MeetID: meetID, Message: u.Name}
	stmt := store.Stmt(`UPDATE units SET name = ?, short_name = ? WHERE id = ? AND meet_id = ?`, u.Name, u.ShortName, id, meetID)
	if id == "" {
		u.ID = newID()
		a.Action = "unit.created"
		stmt = store.Stmt(`INSERT INTO units(id, meet_id, name, short_name) VALUES (?, ?, ?, ?)`, u.ID, meetID, u.Name, u.ShortName)
	}
	if err := s.write(ctx, a, Change{Kind: "units", MeetID: meetID}, stmt); err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteUnit removes a unit; its athletes stay, without a unit.
func (s *Service) DeleteUnit(ctx context.Context, src store.Source, meetID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(ctx,
		store.Audit{Source: src, Action: "unit.deleted", MeetID: meetID, Detail: map[string]string{"unitId": id}},
		Change{Kind: "units", MeetID: meetID},
		store.Stmt(`DELETE FROM units WHERE id = ? AND meet_id = ?`, id, meetID))
}
