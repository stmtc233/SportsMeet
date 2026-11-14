package service

import (
	"context"
	"fmt"
	"reflect"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// TieView is a tie in a round's order that only a referee can break
// (docs/agents/domain.md「并列裁定」).
type TieView struct {
	Place         int      `json:"place"`         // the place they share without a ruling
	CompetitorIDs []string `json:"competitorIds"` // in ruled order when ruled
	Ruled         bool     `json:"ruled"`
	// Extra is how many more advance than the round's rule allows because
	// of this tie; 0 = it does not touch qualification.
	Extra int `json:"extra"`
}

// TieRulingInput rules ties of one round. Places gives every competitor
// of each tie ruled its place from the shared one up; equal places stay
// tied, and all of a tie at its shared place clears its ruling.
type TieRulingInput struct {
	RoundID string     `json:"roundId"`
	Places  []TiePlace `json:"places"`
}

// TiePlace is one competitor's ruled place.
type TiePlace struct {
	CompetitorID string `json:"competitorId"`
	Place        int    `json:"place"`
}

// sameStanding reports whether a rewritten result ranks on what the old
// one did, so a ruling made on it still holds.
func sameStanding(a, b *domain.Result) bool {
	return a.Status == b.Status && samePtr(a.Mark, b.Mark) && reflect.DeepEqual(a.Attempts, b.Attempts) && reflect.DeepEqual(a.Bars, b.Bars)
}

func roundRanked(r domain.Round) []scoring.Ranked {
	var all []scoring.Ranked
	for _, h := range r.Heats {
		for _, sl := range h.Slots {
			if sl.Result != nil {
				all = append(all, ranked(sl.CompetitorID(), sl.Result))
			}
		}
	}
	return all
}

// qualified counts who a round's results send on.
func qualified(heats []domain.Heat) int {
	n := 0
	for _, h := range heats {
		for _, sl := range h.Slots {
			if sl.Result != nil && sl.Result.Qualify != "" {
				n++
			}
		}
	}
	return n
}

// withRuling copies heats with the given rulings set and the round
// rescored on them.
func withRuling(event domain.Event, advance *domain.Advance, heats []domain.Heat, ruling map[string]int) []domain.Heat {
	out := copyHeats(heats)
	for i, h := range out {
		for k, sl := range h.Slots {
			if t, ok := ruling[sl.CompetitorID()]; ok && sl.Result != nil {
				r := *sl.Result
				r.TieBreak = t
				out[i].Slots[k].Result = &r
			}
		}
	}
	rescoreEvent(event, advance, out)
	return out
}

// roundTies lists a round's ties. Extra compares who advances now with who
// would if the tie were ruled in its current order.
func roundTies(event domain.Event, r domain.Round) []TieView {
	ties := scoring.TiesEvent(event, roundRanked(r))
	out := make([]TieView, 0, len(ties))
	var now int
	if r.Advance != nil && len(ties) > 0 {
		now = qualified(withRuling(event, r.Advance, r.Heats, nil))
	}
	for _, t := range ties {
		v := TieView{Place: t.Place, CompetitorIDs: t.Keys, Ruled: t.Ruled}
		if now > 0 {
			strict := map[string]int{}
			for i, id := range t.Keys {
				strict[id] = i + 1
			}
			v.Extra = max(0, now-qualified(withRuling(event, r.Advance, r.Heats, strict)))
		}
		out = append(out, v)
	}
	return out
}

// RuleTies records a referee's ruling on ties of one round, then
// recomputes its places and Q / q. Every competitor of a tie it touches
// must be given a place within the tie; the marks stay as they are.
func (s *Service) RuleTies(ctx context.Context, src store.Source, in TieRulingInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.store.Query(ctx, `SELECT event_id FROM rounds WHERE id = ?`, in.RoundID)
	if err != nil {
		return err
	}
	if len(rows.Values) == 0 {
		return ErrStale
	}
	st, err := s.loadEventParticipants(ctx, rows.Values[0][0].(string))
	if err != nil {
		return err
	}
	round := st.round(in.RoundID)
	given := map[string]int{}
	for _, p := range in.Places {
		if _, dup := given[p.CompetitorID]; dup {
			return invalid("裁定名单重复")
		}
		given[p.CompetitorID] = p.Place
	}
	ruling := map[string]int{}
	type change struct {
		Athlete string `json:"athlete"`
		Before  int    `json:"before"`
		After   int    `json:"after"`
	}
	var changes []change
	before := scoring.RankEvent(st.event, roundRanked(*round))
	for _, t := range scoring.TiesEvent(st.event, roundRanked(*round)) {
		covered := 0
		for _, id := range t.Keys {
			if _, ok := given[id]; ok {
				covered++
			}
		}
		if covered == 0 {
			continue
		}
		if covered < len(t.Keys) {
			return invalid("第 %d 名并列的 %d 人须一起裁定", t.Place, len(t.Keys))
		}
		same := true
		for _, id := range t.Keys {
			p := given[id]
			if p < t.Place || p >= t.Place+len(t.Keys) {
				return invalid("「%s」的名次应在第 %d–%d 名之间", st.competitorName(id), t.Place, t.Place+len(t.Keys)-1)
			}
			same = same && p == given[t.Keys[0]]
		}
		for _, id := range t.Keys {
			ruling[id] = 0
			if !same {
				ruling[id] = given[id] - t.Place + 1
			}
			delete(given, id)
		}
	}
	if len(given) > 0 {
		return ErrStale // someone named is no longer tied
	}
	stored := storedResults(round.Heats)
	heats := withRuling(st.event, round.Advance, round.Heats, ruling)
	stmts := resultWrites(stored, heats, nil, s.store.Now())
	if len(stmts) == 0 {
		return nil
	}
	after := scoring.RankEvent(st.event, roundRanked(domain.Round{Heats: heats}))
	for _, h := range heats {
		for _, sl := range h.Slots {
			if _, ok := ruling[sl.CompetitorID()]; ok {
				changes = append(changes, change{st.competitorName(sl.CompetitorID()), before[sl.CompetitorID()], after[sl.CompetitorID()]})
			}
		}
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "results.tie", MeetID: st.event.MeetID, EventID: st.event.ID,
			Message: fmt.Sprintf("%s %s", st.event.Name, round.Label()), Detail: changes},
		Change{Kind: "results", MeetID: st.event.MeetID, EventID: st.event.ID}, stmts...)
}
