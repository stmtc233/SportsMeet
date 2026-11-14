package service

import (
	"context"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// AthleteEntryOption is an individual event as the athlete dialog offers
// it: whether the athlete is in it, and whether that can be undone there.
type AthleteEntryOption struct {
	EventID string `json:"eventId"`
	Name    string `json:"name"`
	Entered bool   `json:"entered"`
	// Placed: in a heat of the event, so the entry stays until moved out.
	Placed bool `json:"placed"`
}

// AthleteEntryOptions lists the individual events (not team events, not
// combined components) an athlete of this gender and group can enter, in
// the 项目 page's order, and those they are already in. athleteID is ""
// for an athlete not saved yet.
func (s *Service) AthleteEntryOptions(ctx context.Context, meetID, athleteID string, gender domain.Gender, groupID string) ([]AthleteEntryOption, error) {
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	entered, err := s.athleteEntered(ctx, meetID, athleteID)
	if err != nil {
		return nil, err
	}
	out := []AthleteEntryOption{}
	for _, e := range p.events {
		if e.IsTeam() || e.CombinedID != "" {
			continue
		}
		fits := (e.Gender == domain.GenderMixed || gender == domain.GenderUnknown || e.Gender == gender) && (e.GroupID == "" || e.GroupID == groupID)
		if !fits && !entered[e.ID] {
			continue
		}
		out = append(out, AthleteEntryOption{EventID: e.ID, Name: e.Name, Entered: entered[e.ID], Placed: placedIn(p.rounds[e.ID], athleteID)})
	}
	return out, nil
}

func (s *Service) athleteEntered(ctx context.Context, meetID, athleteID string) (map[string]bool, error) {
	out := map[string]bool{}
	if athleteID == "" {
		return out, nil
	}
	entries, err := s.store.ListEntries(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	for _, en := range entries {
		if en.AthleteID == athleteID && en.TeamID == "" {
			out[en.EventID] = true
		}
	}
	return out, nil
}

func placedIn(rounds []domain.Round, athleteID string) bool {
	for _, r := range rounds {
		for _, h := range r.Heats {
			for _, sl := range h.Slots {
				if athleteID != "" && sl.AthleteID == athleteID {
					return true
				}
			}
		}
	}
	return false
}

// SetAthleteEntries makes the athlete's individual events exactly eventIDs
// in one write: new ones entered without an entry mark, the others
// withdrawn. An event they are placed in cannot be withdrawn here.
func (s *Service) SetAthleteEntries(ctx context.Context, src store.Source, meetID, athleteID string, eventIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return err
	}
	a, ok := p.athletes[athleteID]
	if !ok {
		return invalid("运动员不存在或已删除")
	}
	entered, err := s.athleteEntered(ctx, meetID, athleteID)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	var stmts []sqlite.Statement
	var added, removed []string
	for _, id := range eventIDs {
		e, ok := p.eventBy[id]
		switch {
		case !ok:
			return invalid("项目不存在或已删除")
		case e.IsTeam():
			return invalid("「%s」按队报名", e.Name)
		case e.CombinedID != "":
			return invalid("请在全能中统一报名")
		}
		if wanted[id] {
			continue
		}
		wanted[id] = true
		if !entered[id] {
			added = append(added, e.Name)
			stmts = append(stmts, insertEntry(id, athleteID, "", nil))
		}
	}
	for _, e := range p.events {
		if !entered[e.ID] || wanted[e.ID] || e.IsTeam() || e.CombinedID != "" {
			continue
		}
		if placedIn(p.rounds[e.ID], athleteID) {
			return invalid("「%s」已排进「%s」的组，请先移出", a.Name, e.Name)
		}
		removed = append(removed, e.Name)
		stmts = append(stmts, store.Stmt(`DELETE FROM entries WHERE event_id = ? AND athlete_id = ?`, e.ID, athleteID))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.athlete_set", MeetID: meetID, Message: a.Name,
			Detail: struct {
				Added   []string `json:"added"`
				Removed []string `json:"removed"`
			}{added, removed}},
		Change{Kind: "entries", MeetID: meetID}, stmts...)
}
