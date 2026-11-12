package service

import (
	"context"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// EntryInput enters an athlete; Seed is the typed entry mark, "" = none.
type EntryInput struct {
	AthleteID string `json:"athleteId"`
	Seed      string `json:"seed,omitempty"`
}

func (st *eventState) parseSeed(text string) (any, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	m, err := scoring.ParseMark(st.event.Kind, text)
	if err != nil {
		return nil, invalid("报名成绩「%s」格式不正确", text)
	}
	return m, nil
}

// AddEntries enters athletes into an event; those already entered are
// left as they are.
func (s *Service) AddEntries(ctx context.Context, src store.Source, eventID string, list []EntryInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	var stmts []sqlite.Statement
	var names []string
	seen := map[string]bool{}
	for _, in := range list {
		a, ok := st.athletes[in.AthleteID]
		if !ok {
			return invalid("运动员不存在或已删除")
		}
		if st.entered[a.ID] != nil || seen[a.ID] {
			continue
		}
		seed, err := st.parseSeed(in.Seed)
		if err != nil {
			return err
		}
		seen[a.ID] = true
		names = append(names, a.Name)
		stmts = append(stmts, store.Stmt(`INSERT INTO entries(event_id, athlete_id, seed_mark) VALUES (?, ?, ?)`, eventID, a.ID, seed))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.added", MeetID: st.event.MeetID, EventID: eventID,
			Message: st.event.Name + ":" + summarize(names), Detail: names},
		Change{Kind: "entries", MeetID: st.event.MeetID, EventID: eventID}, stmts...)
}

// SetSeed changes an entry's mark; "" clears it.
func (s *Service) SetSeed(ctx context.Context, src store.Source, eventID, athleteID, seed string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if st.entered[athleteID] == nil {
		return invalid("该运动员未报名本项目")
	}
	mark, err := st.parseSeed(seed)
	if err != nil {
		return err
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.seed", MeetID: st.event.MeetID, EventID: eventID,
			Message: st.event.Name + ":" + st.athletes[athleteID].Name, Detail: map[string]any{"athleteId": athleteID, "seedMark": mark}},
		Change{Kind: "entries", MeetID: st.event.MeetID, EventID: eventID},
		store.Stmt(`UPDATE entries SET seed_mark = ? WHERE event_id = ? AND athlete_id = ?`, mark, eventID, athleteID))
}

// RemoveEntries withdraws athletes from an event. Anyone placed in one of
// its heats must be moved out first: withdrawing never touches slots.
func (s *Service) RemoveEntries(ctx context.Context, src store.Source, eventID string, athleteIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	placed := map[string]bool{}
	for _, r := range st.rounds {
		for _, h := range r.Heats {
			for _, sl := range h.Slots {
				placed[sl.AthleteID] = true
			}
		}
	}
	var stmts []sqlite.Statement
	var names []string
	for _, id := range athleteIDs {
		if st.entered[id] == nil {
			continue
		}
		if placed[id] {
			return invalid("「%s」已排进组，请先移出", st.athletes[id].Name)
		}
		names = append(names, st.athletes[id].Name)
		stmts = append(stmts, store.Stmt(`DELETE FROM entries WHERE event_id = ? AND athlete_id = ?`, eventID, id))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.removed", MeetID: st.event.MeetID, EventID: eventID,
			Message: st.event.Name + ":" + summarize(names), Detail: names},
		Change{Kind: "entries", MeetID: st.event.MeetID, EventID: eventID}, stmts...)
}
