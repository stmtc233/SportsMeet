package service

import (
	"context"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// EntryInput enters an athlete or relay team; Seed is the typed entry mark, "" = none.
type EntryInput struct {
	AthleteID string `json:"athleteId,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	Seed      string `json:"seed,omitempty"`
}

func (st *eventState) parseSeed(text string) (any, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	m, err := scoring.ParseEventMark(st.event, text)
	if err != nil {
		return nil, invalid("报名成绩「%s」格式不正确", text)
	}
	return m, nil
}

// AddEntries enters athletes or relay teams into an event; those already entered are
// left as they are.
func (s *Service) AddEntries(ctx context.Context, src store.Source, eventID string, list []EntryInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if st.event.CombinedID != "" {
		return invalid("请在全能中统一报名")
	}
	var stmts []sqlite.Statement
	var names []string
	seen := map[string]bool{}
	for _, in := range list {
		if (in.AthleteID == "") == (in.TeamID == "") || (st.event.IsTeam()) != (in.TeamID != "") {
			return invalid("报名类型与项目不符")
		}
		id := in.AthleteID
		if in.TeamID != "" {
			id = in.TeamID
		}
		a, ok := st.competitor(id)
		if !ok {
			return invalid("%s不存在或已删除", st.competitorLabel())
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
		stmts = append(stmts, insertEntry(eventID, in.AthleteID, in.TeamID, seed))
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
func (s *Service) SetSeed(ctx context.Context, src store.Source, eventID, competitorID, seed string) error {
	return s.setSeed(ctx, src, eventID, competitorID, seed, nil)
}

// SetEntrySeed accepts the typed entry identity used by MCP.
func (s *Service) SetEntrySeed(ctx context.Context, src store.Source, eventID string, in EntryInput) error {
	return s.setSeed(ctx, src, eventID, "", in.Seed, &in)
}
func (s *Service) setSeed(ctx context.Context, src store.Source, eventID, competitorID, seed string, typed *EntryInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if typed != nil {
		if (typed.AthleteID == "") == (typed.TeamID == "") || (st.event.IsTeam()) != (typed.TeamID != "") {
			return invalid("报名类型与项目不符")
		}
		competitorID = typed.AthleteID
		if typed.TeamID != "" {
			competitorID = typed.TeamID
		}
	}

	if st.entered[competitorID] == nil {
		return invalid("该%s未报名本项目", st.competitorLabel())
	}
	mark, err := st.parseSeed(seed)
	if err != nil {
		return err
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.seed", MeetID: st.event.MeetID, EventID: eventID,
			Message: st.event.Name + ":" + st.competitorName(competitorID), Detail: map[string]any{"competitorId": competitorID, "seedMark": mark}},
		Change{Kind: "entries", MeetID: st.event.MeetID, EventID: eventID},
		store.Stmt(`UPDATE entries SET seed_mark = ? WHERE event_id = ? AND `+st.entryColumn()+` = ?`, mark, eventID, competitorID))
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
				placed[sl.CompetitorID()] = true
			}
		}
	}
	if st.event.CombinedID != "" {
		return invalid("请在全能中取消报名")
	}
	var stmts []sqlite.Statement
	var names []string
	for _, id := range athleteIDs {
		if st.entered[id] == nil {
			continue
		}
		if placed[id] {
			return invalid("「%s」已排进组，请先移出", st.competitorName(id))
		}
		names = append(names, st.competitorName(id))
		stmts = append(stmts, store.Stmt(`DELETE FROM entries WHERE event_id = ? AND `+st.entryColumn()+` = ?`, eventID, id))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "entry.removed", MeetID: st.event.MeetID, EventID: eventID,
			Message: st.event.Name + ":" + summarize(names), Detail: names},
		Change{Kind: "entries", MeetID: st.event.MeetID, EventID: eventID}, stmts...)
}

func (st *eventState) entryColumn() string {
	if st.event.IsTeam() {
		return "team_id"
	}
	return "athlete_id"
}
