package service

import (
	"context"
	"errors"
	"sort"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// PlanRequest asks for an automatic arrangement of one round. The page
// keeps its arrangement in a local draft, so it says who is already placed
// there (Exclude); the plan covers everyone else and is appended to the
// draft, not saved.
type PlanRequest struct {
	EventID string   `json:"eventId"`
	RoundID string   `json:"roundId"`
	Exclude []string `json:"exclude,omitempty"`
	// FromAdvance fills the round with those who advanced from the round
	// before it, seeded by the marks they got there, instead of the entries.
	FromAdvance   bool   `json:"fromAdvance,omitempty"`
	PerHeat       int    `json:"perHeat"`
	Order         string `json:"order"`
	Lanes         string `json:"lanes"`
	SeparateUnits bool   `json:"separateUnits,omitempty"`
	Balance       bool   `json:"balance,omitempty"`
	Seed          int64  `json:"seed,omitempty"`
}

// PlanHeats computes new heats for the athletes of a round not yet
// placed. It writes nothing.
func (s *Service) PlanHeats(ctx context.Context, req PlanRequest) ([]arrange.PlannedHeat, error) {
	st, err := s.loadEventParticipants(ctx, req.EventID)
	if err != nil {
		return nil, err
	}
	round := st.round(req.RoundID)
	if round == nil {
		return nil, ErrStale
	}
	skip := map[string]bool{}
	for _, id := range req.Exclude {
		skip[id] = true
	}
	var entrants []arrange.Entrant
	add := func(id string, seed *int64) {
		if a, ok := st.competitor(id); ok && !skip[id] {
			entrants = append(entrants, arrange.Entrant{ID: id, Unit: a.UnitID, Seed: seed})
		}
	}
	if req.FromAdvance {
		var prev *domain.Round
		for i := range st.rounds {
			if st.rounds[i].Ordinal == round.Ordinal-1 {
				prev = &st.rounds[i]
			}
		}
		if prev == nil || prev.Advance == nil {
			return nil, invalid("上一轮没有晋级规则")
		}
		if _, complete := standingsEvent(st.event, *prev); !complete {
			return nil, invalid("%s成绩未录完", prev.Label())
		}
		for _, h := range prev.Heats {
			for _, sl := range h.Slots {
				if sl.Result != nil && sl.Result.Qualify != "" {
					add(sl.CompetitorID(), sl.Result.Mark)
				}
			}
		}
	} else {
		for _, e := range st.entries {
			add(e.CompetitorID(), e.SeedMark)
		}
	}
	plan, err := arrange.Plan(arrange.Options{
		Kind: st.event.Kind, StartMode: st.event.StartMode, RankDirection: st.event.RankDirection, LaneCount: st.event.LaneCount, PerHeat: req.PerHeat, Order: req.Order, Lanes: req.Lanes,
		SeparateUnits: req.SeparateUnits, Balance: req.Balance, Seed: req.Seed,
	}, entrants)
	if err != nil {
		return nil, &InputError{Msg: err.Error()}
	}
	return plan, nil
}

// BoardSave is the arrange page's draft: only the heats it changed, each
// with the fingerprint it had when loaded (docs/agents/arrange.md).
type BoardSave struct {
	EventID string      `json:"eventId"`
	Rounds  []RoundSave `json:"rounds"`
	// Unlocked lists heats with results whose slots may change.
	Unlocked []string `json:"unlocked"`
	// Reset allows deleting heats that have results (「全部重置」).
	Reset bool `json:"reset"`
}

// RoundSave is one round's changes.
type RoundSave struct {
	RoundID string `json:"roundId"`
	// Order lists every heat left in the round, existing ids and the keys
	// of added ones, in display order; ordinals are derived from it.
	Order   []string   `json:"order"`
	Changed []HeatSave `json:"changed"`
	Added   []HeatSave `json:"added"`
	Deleted []HeatRef  `json:"deleted"`
}

// HeatSave is a heat's new content. Changed heats carry ID and Base, added
// ones a Key that Order refers to.
type HeatSave struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Base        string `json:"base"`
	Name        string `json:"name"`
	ScheduledAt string `json:"scheduledAt"` // own time; "" = the round's start
	// DurationMinutes is the expected duration, 1–1440 minutes; nil = not given.
	DurationMinutes *int       `json:"durationMinutes"`
	Slots           []SlotSave `json:"slots"`
}

// SlotSave places an athlete or relay team; ID is the UI's opaque competitor ID.
type SlotSave struct {
	Position  int    `json:"position"`
	AthleteID string `json:"athleteId,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	ID        string `json:"competitorId,omitempty"`
}

// HeatRef names a heat to delete, with the fingerprint it was loaded with.
type HeatRef struct {
	ID   string `json:"id"`
	Base string `json:"base"`
}

type boardHeat struct {
	heat      domain.Heat
	old       *domain.Heat // nil for an added heat
	submitted bool
	slots     bool // its slots differ from what is stored
}

func sameSlots(a, b []domain.Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Position != b[i].Position || a[i].AthleteID != b[i].AthleteID || a[i].TeamID != b[i].TeamID {
			return false
		}
	}
	return true
}

// buildSlots validates a heat's slots and carries each athlete's result
// along to the new position.
func (st *eventState) buildSlots(label string, in []SlotSave, old *domain.Heat) ([]domain.Slot, error) {
	results := map[string]*domain.Result{}
	if old != nil {
		for _, sl := range old.Slots {
			results[sl.CompetitorID()] = sl.Result
		}
	}
	out := make([]domain.Slot, 0, len(in))
	positions, members := map[int]bool{}, map[string]bool{}
	for _, sl := range in {
		if (sl.TeamID != "" && !st.event.IsTeam()) || (sl.AthleteID != "" && st.event.IsTeam()) || (sl.TeamID != "" && sl.AthleteID != "") || (sl.ID != "" && ((sl.TeamID != "" && sl.ID != sl.TeamID) || (sl.AthleteID != "" && sl.ID != sl.AthleteID))) {
			return nil, invalid("报名类型与项目不符")
		}
		a, ok := st.competitor(sl.CompetitorID())
		switch {
		case st.event.CombinedID != "" && st.entered[sl.CompetitorID()] == nil:
			return nil, invalid("请先在全能中报名")
		case !ok:
			return nil, invalid("%s不存在或已删除", st.competitorLabel())
		case sl.Position < 1 || positions[sl.Position]:
			return nil, invalid("「%s」的位置重复或不正确", label)
		case st.event.UsesLanes() && sl.Position > st.event.LaneCount:
			return nil, invalid("「%s」的道次超过 %d 道", label, st.event.LaneCount)
		case members[a.ID]:
			return nil, invalid("「%s」在同一轮次排了两次", a.Name)
		}
		positions[sl.Position], members[a.ID] = true, true
		out = append(out, st.slot(sl.Position, a.ID, results[a.ID]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

// SaveBoard applies the arrange page's draft in one transaction. It
// returns ErrStale when a submitted heat no longer matches its
// fingerprint or the round's heats changed; the page then reloads and
// merges.
func (s *Service) SaveBoard(ctx context.Context, src store.Source, in BoardSave) error {
	if src == store.SourceMCP && (in.Reset || len(in.Unlocked) > 0) {
		return invalid("请在界面上解锁或重置")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveBoard(ctx, src, in)
}

// saveBoard is SaveBoard with the write lock held.
func (s *Service) saveBoard(ctx context.Context, src store.Source, in BoardSave) error {
	st, err := s.loadEvent(ctx, in.EventID)
	if err != nil {
		return err
	}
	unlocked := map[string]bool{}
	for _, id := range in.Unlocked {
		unlocked[id] = true
	}
	now := s.store.Now()
	var stmts []sqlite.Statement
	entering := map[string]bool{}
	detail := struct {
		Added   int `json:"added"`
		Changed int `json:"changed"`
		Deleted int `json:"deleted"`
		Results int `json:"resultsDeleted"`
	}{}

	for _, rs := range in.Rounds {
		round := st.round(rs.RoundID)
		if round == nil {
			return ErrStale
		}
		current := map[string]*domain.Heat{}
		for i := range round.Heats {
			current[round.Heats[i].ID] = &round.Heats[i]
		}
		deleted := map[string]bool{}
		for _, d := range rs.Deleted {
			h := current[d.ID]
			if h == nil || arrange.Fingerprint(*h) != d.Base {
				return ErrStale
			}
			if reason := h.Facts().KeepReason(); reason != "" && !in.Reset {
				return invalid("「%s」%s，不能删除", domain.HeatLabel(h.Ordinal, h.Name), reason)
			}
			deleted[d.ID] = true
			detail.Deleted++
			detail.Results += h.Facts().Results
			stmts = append(stmts, store.Stmt(`DELETE FROM heats WHERE id = ?`, d.ID))
		}
		changed := map[string]HeatSave{}
		for _, c := range rs.Changed {
			h := current[c.ID]
			if h == nil || deleted[c.ID] || arrange.Fingerprint(*h) != c.Base {
				return ErrStale
			}
			changed[c.ID] = c
		}
		added := map[string]HeatSave{}
		for _, a := range rs.Added {
			if a.Key == "" || current[a.Key] != nil {
				return errors.New("board: an added heat needs a key of its own")
			}
			added[a.Key] = a
		}
		if len(rs.Order) != len(current)-len(deleted)+len(added) {
			return ErrStale
		}

		// The round as it will be, in the new order.
		heats := make([]boardHeat, 0, len(rs.Order))
		ordinals := make([]int, 0, len(rs.Order))
		listed := map[string]bool{}
		for _, key := range rs.Order {
			if listed[key] {
				return ErrStale
			}
			listed[key] = true
			var bh boardHeat
			var save *HeatSave
			if a, ok := added[key]; ok {
				bh = boardHeat{heat: domain.Heat{ID: newID(), RoundID: round.ID}, submitted: true, slots: true}
				save = &a
				ordinals = append(ordinals, 0)
			} else {
				h := current[key]
				if h == nil || deleted[key] {
					return ErrStale
				}
				bh = boardHeat{heat: *h, old: h}
				bh.heat.Slots = append([]domain.Slot(nil), h.Slots...)
				if c, ok := changed[key]; ok {
					save, bh.submitted = &c, true
				}
				ordinals = append(ordinals, h.Ordinal)
			}
			if save != nil {
				if save.ScheduledAt != "" && !domain.ValidScheduled(save.ScheduledAt) {
					return invalid("计划时间格式不正确")
				}
				if !domain.ValidDuration(save.DurationMinutes) {
					return invalid("预计用时须为 1–%d 分钟", domain.MaxDurationMinutes)
				}
				bh.heat.Name, bh.heat.ScheduledAt = domain.NormalizeHeatName(save.Name), save.ScheduledAt
				bh.heat.DurationMinutes = save.DurationMinutes
				label := domain.HeatLabel(len(heats)+1, bh.heat.Name)
				if bh.old != nil {
					label = domain.HeatLabel(bh.old.Ordinal, bh.heat.Name)
				}
				slots, err := st.buildSlots(label, save.Slots, bh.old)
				if err != nil {
					return err
				}
				if bh.old != nil {
					bh.slots = !sameSlots(bh.old.Slots, slots)
					if reason := bh.old.Facts().KeepReason(); bh.slots && reason != "" && !unlocked[key] && !in.Reset {
						return invalid("「%s」%s，请先解锁位置", label, reason)
					}
				}
				bh.heat.Slots = slots
			}
			heats = append(heats, bh)
		}

		// One slot per athlete in a round: placing someone in a submitted
		// heat takes them out of the heats the page did not submit.
		placed := map[string]bool{}
		for _, bh := range heats {
			if !bh.submitted {
				continue
			}
			for _, sl := range bh.heat.Slots {
				if placed[sl.CompetitorID()] {
					return invalid("「%s」在同一轮次排了两次", st.competitorName(sl.CompetitorID()))
				}
				placed[sl.CompetitorID()] = true
			}
		}
		for i := range heats {
			bh := &heats[i]
			if bh.submitted {
				continue
			}
			kept := bh.heat.Slots[:0:0]
			for _, sl := range bh.heat.Slots {
				if !placed[sl.CompetitorID()] {
					kept = append(kept, sl)
					continue
				}
				if reason := bh.old.Facts().KeepReason(); reason != "" {
					return invalid("「%s」在%s的「%s」里，请先解锁位置", st.competitorName(sl.CompetitorID()), reason,
						domain.HeatLabel(bh.old.Ordinal, bh.old.Name))
				}
				bh.slots = true
			}
			bh.heat.Slots = kept
		}

		before := storedResults(round.Heats)
		final := make([]domain.Heat, len(heats))
		rewritten := map[string]bool{}
		for i, ord := range arrange.DeriveOrdinals(ordinals) {
			bh := &heats[i]
			bh.heat.Ordinal = ord
			final[i] = bh.heat
			h := bh.heat
			switch {
			case bh.old == nil:
				detail.Added++
				stmts = append(stmts, store.Stmt(`INSERT INTO heats(id, round_id, ordinal, name, scheduled_at, duration_minutes) VALUES (?, ?, ?, ?, ?, ?)`,
					h.ID, h.RoundID, h.Ordinal, h.Name, h.ScheduledAt, optArg(h.DurationMinutes)))
			case h.Ordinal != bh.old.Ordinal || h.Name != bh.old.Name || h.ScheduledAt != bh.old.ScheduledAt ||
				!samePtr(h.DurationMinutes, bh.old.DurationMinutes):
				stmts = append(stmts, store.Stmt(`UPDATE heats SET ordinal = ?, name = ?, scheduled_at = ?, duration_minutes = ? WHERE id = ?`,
					h.Ordinal, h.Name, h.ScheduledAt, optArg(h.DurationMinutes), h.ID))
			}
			if bh.old != nil && (bh.slots || bh.submitted) {
				detail.Changed++
			}
			if !bh.slots {
				continue
			}
			if bh.old != nil {
				// Deleting the slots drops their results too; the ones
				// that stay are written again below.
				rewritten[h.ID] = true
				detail.Results += bh.old.Facts().Results - h.Facts().Results
				stmts = append(stmts, store.Stmt(`DELETE FROM slots WHERE heat_id = ?`, h.ID))
			}
			for _, sl := range h.Slots {
				stmts = append(stmts, insertSlot(h.ID, sl))
				if st.entered[sl.CompetitorID()] == nil && !entering[sl.CompetitorID()] {
					entering[sl.CompetitorID()] = true // placing enters (docs/agents/domain.md「报名」)
					stmts = append(stmts, insertEntry(in.EventID, sl.AthleteID, sl.TeamID, nil))
				}
			}
		}
		rescoreEvent(st.event, round.Advance, final)
		stmts = append(stmts, resultWrites(before, final, rewritten, now)...)
	}
	if len(stmts) == 0 {
		return nil
	}
	action := "board.saved"
	if in.Reset {
		action = "board.reset"
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: action, MeetID: st.event.MeetID, EventID: in.EventID, Message: st.event.Name, Detail: detail},
		Change{Kind: "heats", MeetID: st.event.MeetID, EventID: in.EventID}, stmts...)
}

func (sl SlotSave) CompetitorID() string {
	if sl.ID != "" {
		return sl.ID
	}
	if sl.TeamID != "" {
		return sl.TeamID
	}
	return sl.AthleteID
}
