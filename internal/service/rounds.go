package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Arrange states of a round (RoundRow.Arrange).
const (
	RoundEmpty    = "empty"    // first round, nobody entered
	RoundTodo     = "todo"     // first round with entries, no heats
	RoundArranged = "arranged" // has heats
	RoundWaiting  = "waiting"  // a later round whose round before is not finished
	RoundReady    = "ready"    // a later round whose round before is finished
)

// Result states of a round with heats (RoundRow.Results).
const (
	ResultsNone    = "none"    // no result yet
	ResultsDoing   = "doing"   // some results in
	ResultsDone    = "done"    // every slot has a result
	ResultsTies    = "ties"    // done, a tie only a ruling can settle changes who advances
	ResultsAdvance = "advance" // done, the next round has no heats yet
)

// MeetRounds is every round of a meet with where it stands: by event for
// the 编排 page, by day and time for the 成绩 page.
type MeetRounds struct {
	// Rounds are in the 项目 page's order, each event's rounds in order.
	Rounds []RoundRow `json:"rounds"`
	// Days hold the rounds with a planned time by their first time;
	// Unscheduled the rounds with heats but no time.
	Days        []RoundDay `json:"days"`
	Unscheduled []RoundRow `json:"unscheduled"`
	// Day is the day the 成绩 page opens on: today when the meet has
	// rounds then, else the first day with a round not finished, else the
	// first day; "" without days.
	Day string `json:"day"`
}

// RoundDay is one day's rounds.
type RoundDay struct {
	Day    string     `json:"day"`  // YYYY-MM-DD
	Text   string     `json:"text"` // 2026-11-12 周四
	Rounds []RoundRow `json:"rounds"`
}

// RoundRow is one round and where it stands.
type RoundRow struct {
	EventID    string `json:"eventId"`
	EventName  string `json:"eventName"`
	CombinedID string `json:"combinedId"`
	GroupID    string `json:"groupId"`
	RoundID    string `json:"roundId"`
	RoundLabel string `json:"roundLabel"`
	// Text names the round: 男子100m 预赛, or the event alone when it has
	// one round.
	Text  string `json:"text"`
	First bool   `json:"first"`
	// PrevLabel and NextLabel name the rounds before and after; "" = none.
	PrevLabel string `json:"prevLabel"`
	NextLabel string `json:"nextLabel"`
	Team      bool   `json:"team"`
	// StartAt is the round's first planned time; StartText shows it
	// relative to the day the round is listed on.
	StartAt   string `json:"startAt"`
	StartText string `json:"startText"`
	// TimeText shows StartAt with its date unless on the meet's first day.
	TimeText  string `json:"timeText"`
	Heats     int    `json:"heats"`
	HeatsDone int    `json:"heatsDone"` // heats with a result in every slot
	// People is how many are placed in its heats; Unplaced how many more
	// the round would take: the entries of a first round, those who
	// advanced into a later one, not placed yet.
	People   int    `json:"people"`
	Unplaced int    `json:"unplaced"`
	Arrange  string `json:"arrange"`
	Results  string `json:"results"` // "" without heats
	// Source is who an automatic arrangement takes: 报名, or the round
	// before's qualifiers (预赛晋级者).
	Source string `json:"source"`
	// CanArrange: someone is left to place and the round can take them now.
	CanArrange bool `json:"canArrange"`
}

// MeetRounds reads the meet once, marks included, so its queries do not
// grow with the rounds.
func (s *Service) MeetRounds(ctx context.Context, meetID string) (*MeetRounds, error) {
	p, err := s.loadMeetPlan(ctx, meetID)
	if err != nil {
		return nil, err
	}
	sc := store.Scope{MeetID: meetID}
	full, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}
	entered := map[string][]string{}
	for _, e := range entries {
		entered[e.EventID] = append(entered[e.EventID], e.CompetitorID())
	}
	return p.meetRounds(full, entered, time.Now().Format("2006-01-02")), nil
}

func (p *meetPlan) meetRounds(full map[string][]domain.Round, entered map[string][]string, today string) *MeetRounds {
	out := &MeetRounds{Rounds: []RoundRow{}, Days: []RoundDay{}, Unscheduled: []RoundRow{}}
	type timed struct {
		row   RoundRow
		at    string
		order int
	}
	var list []timed
	for _, e := range p.events {
		rs := full[e.ID]
		for i, r := range rs {
			row := roundRow(e, rs, i, entered[e.ID])
			at := domain.EarliestScheduled([]domain.Round{r})
			day := domain.ScheduledDay(at)
			row.StartAt, row.StartText, row.TimeText = at, domain.ScheduledText(at, day), domain.ScheduledText(at, p.meet.StartDate)
			out.Rounds = append(out.Rounds, row)
			switch {
			case day != "":
				list = append(list, timed{row, at, len(list)})
			case row.Heats > 0:
				out.Unscheduled = append(out.Unscheduled, row)
			}
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].at < list[j].at })
	for _, t := range list {
		day := domain.ScheduledDay(t.at)
		if n := len(out.Days); n == 0 || out.Days[n-1].Day != day {
			out.Days = append(out.Days, RoundDay{Day: day, Text: domain.DayText(day), Rounds: []RoundRow{}})
		}
		last := &out.Days[len(out.Days)-1]
		last.Rounds = append(last.Rounds, t.row)
	}
	for _, d := range out.Days {
		if d.Day == today {
			out.Day = today
		}
	}
	for _, d := range out.Days {
		if out.Day != "" {
			break
		}
		for _, r := range d.Rounds {
			if r.Results != ResultsDone && r.Results != ResultsAdvance {
				out.Day = d.Day
				break
			}
		}
	}
	if out.Day == "" && len(out.Days) > 0 {
		out.Day = out.Days[0].Day
	}
	return out
}

// roundText names a round of an event: the event alone when it has one.
func roundText(e domain.Event, rounds int, r domain.Round) string {
	if rounds <= 1 {
		return e.Name
	}
	return e.Name + " " + r.Label()
}

// roundRow sums up rs[i] of an event read with its marks.
func roundRow(e domain.Event, rs []domain.Round, i int, entered []string) RoundRow {
	r := rs[i]
	row := RoundRow{EventID: e.ID, EventName: e.Name, CombinedID: e.CombinedID, GroupID: e.GroupID, RoundID: r.ID, RoundLabel: r.Label(),
		Text: roundText(e, len(rs), r), First: i == 0, Team: e.IsTeam(), Heats: len(r.Heats), Source: "报名"}
	if i > 0 {
		row.PrevLabel = rs[i-1].Label()
	}
	if i < len(rs)-1 {
		row.NextLabel = rs[i+1].Label()
	}
	placed := map[string]bool{}
	for _, h := range r.Heats {
		full := len(h.Slots) > 0
		for _, sl := range h.Slots {
			placed[sl.CompetitorID()] = true
			if sl.Result == nil || sl.Result.HeightPending {
				full = false
			}
		}
		if full {
			row.HeatsDone++
		}
	}
	row.People = len(placed)
	// Who the round takes: the entries, or the round before's qualifiers.
	var takes []string
	ready := true
	if i == 0 {
		takes = entered
	} else {
		prev := rs[i-1]
		row.Source = prev.Label() + "晋级者"
		done, _ := roundFinished(prev)
		ready = done && prev.Advance != nil
		if ready {
			for _, h := range prev.Heats {
				for _, sl := range h.Slots {
					if sl.Result != nil && sl.Result.Qualify != "" {
						takes = append(takes, sl.CompetitorID())
					}
				}
			}
		}
	}
	for _, id := range takes {
		if !placed[id] {
			row.Unplaced++
		}
	}
	row.CanArrange = ready && row.Unplaced > 0
	switch {
	case len(r.Heats) > 0:
		row.Arrange = RoundArranged
	case i == 0 && len(entered) == 0:
		row.Arrange = RoundEmpty
	case i == 0:
		row.Arrange = RoundTodo
	case ready:
		row.Arrange = RoundReady
	default:
		row.Arrange = RoundWaiting
	}
	if len(r.Heats) == 0 {
		return row
	}
	done, started := roundFinished(r)
	switch {
	case !done && started:
		row.Results = ResultsDoing
	case !done:
		row.Results = ResultsNone
	case hasBlockingTie(e, r):
		row.Results = ResultsTies
	case i < len(rs)-1 && len(rs[i+1].Heats) == 0:
		row.Results = ResultsAdvance
	default:
		row.Results = ResultsDone
	}
	return row
}

// hasBlockingTie: a tie not ruled changes how many advance.
func hasBlockingTie(e domain.Event, r domain.Round) bool {
	for _, t := range roundTies(e, r) {
		if !t.Ruled && t.Extra > 0 {
			return true
		}
	}
	return false
}

// BatchArrangeRequest arranges several rounds at once with the same
// options, each event at its own default heat size: a first round takes
// its entries, a later one the qualifiers of the round before. Only those
// not placed yet are arranged, in new heats after the existing ones.
type BatchArrangeRequest struct {
	MeetID string     `json:"meetId"`
	Rounds []RoundRef `json:"rounds"`
	Order  string     `json:"order"` // seed | random
	// Lanes: center (good marks in the middle lanes; events without lanes
	// go in order), sequential or random.
	Lanes         string `json:"lanes"`
	SeparateUnits bool   `json:"separateUnits"`
	Balance       bool   `json:"balance"`
	Seed          int64  `json:"seed"`
}

// BatchArrangePreview is what a batch arrangement would add; rounds with
// an Error are left out of it.
type BatchArrangePreview struct {
	PreviewHash string              `json:"previewHash"`
	Errors      []string            `json:"errors"`
	Rounds      []BatchArrangeRound `json:"rounds"`
	Heats       int                 `json:"heats"`
	People      int                 `json:"people"`
}

// BatchArrangeRound is one round of the preview.
type BatchArrangeRound struct {
	EventID string `json:"eventId"`
	RoundID string `json:"roundId"`
	Text    string `json:"text"`
	Source  string `json:"source"`
	PerHeat int    `json:"perHeat"`
	Heats   int    `json:"heats"`
	People  int    `json:"people"`
	Error   string `json:"error"`
	plan    []arrange.PlannedHeat
	order   []string // the round's existing heats
}

// PreviewBatchArrange computes the arrangement without writing.
func (s *Service) PreviewBatchArrange(ctx context.Context, in BatchArrangeRequest) (*BatchArrangePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, err := s.planBatchArrange(ctx, in)
	return p, err
}

// ApplyBatchArrange recomputes the preview under the write lock and, when
// it still matches, writes every round in one transaction.
func (s *Service) ApplyBatchArrange(ctx context.Context, src store.Source, in BatchArrangeRequest, hash string, confirmed bool) (*BatchArrangePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !confirmed || hash == "" {
		return nil, invalid("请先预览并确认")
	}
	p, states, err := s.planBatchArrange(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(p.Errors) > 0 {
		return nil, invalid("%s", p.Errors[0])
	}
	if p.PreviewHash != hash {
		return nil, ErrStale
	}
	saves := map[string]*BoardSave{}
	var eventOrder []string
	for _, r := range p.Rounds {
		if r.Error != "" {
			continue
		}
		b := saves[r.EventID]
		if b == nil {
			b = &BoardSave{EventID: r.EventID}
			saves[r.EventID] = b
			eventOrder = append(eventOrder, r.EventID)
		}
		rs := RoundSave{RoundID: r.RoundID, Order: append([]string{}, r.order...)}
		for i, h := range r.plan {
			key := fmt.Sprintf("batch-%d", i)
			hs := HeatSave{Key: key, Slots: []SlotSave{}}
			for _, sl := range h.Slots {
				hs.Slots = append(hs.Slots, SlotSave{Position: sl.Position, ID: sl.ID})
			}
			rs.Added = append(rs.Added, hs)
			rs.Order = append(rs.Order, key)
		}
		b.Rounds = append(b.Rounds, rs)
	}
	var stmts []sqlite.Statement
	for _, id := range eventOrder {
		w, _, err := s.boardWrites(states[id], *saves[id])
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, w...)
	}
	err = s.write(ctx, store.Audit{Source: src, Action: "board.batch_arranged", MeetID: in.MeetID,
		Message: fmt.Sprintf("%d 个轮次", len(eventRounds(p))), Detail: struct {
			Request BatchArrangeRequest `json:"request"`
			Heats   int                 `json:"heats"`
			People  int                 `json:"people"`
		}{in, p.Heats, p.People}},
		Change{Kind: "heats", MeetID: in.MeetID}, stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func eventRounds(p *BatchArrangePreview) []BatchArrangeRound {
	var out []BatchArrangeRound
	for _, r := range p.Rounds {
		if r.Error == "" {
			out = append(out, r)
		}
	}
	return out
}

// planBatchArrange reads each event of the request once, with all its
// athletes as the save needs them. Its caller holds the write lock.
func (s *Service) planBatchArrange(ctx context.Context, in BatchArrangeRequest) (*BatchArrangePreview, map[string]*eventState, error) {
	p := &BatchArrangePreview{Errors: []string{}, Rounds: []BatchArrangeRound{}}
	if len(in.Rounds) == 0 {
		p.Errors = append(p.Errors, "请选择轮次")
		return p, nil, nil
	}
	if in.Order != "seed" && in.Order != "random" {
		return nil, nil, invalid("排列顺序不正确")
	}
	if in.Lanes != "center" && in.Lanes != "sequential" && in.Lanes != "random" {
		return nil, nil, invalid("位置分配不正确")
	}
	states := map[string]*eventState{}
	seen := map[string]bool{}
	type stamp struct {
		RoundID string
		Heats   []string
		Plan    []arrange.PlannedHeat
	}
	var stamps []stamp
	for _, ref := range in.Rounds {
		if seen[ref.RoundID] {
			continue
		}
		seen[ref.RoundID] = true
		st := states[ref.EventID]
		if st == nil {
			var err error
			if st, err = s.loadEvent(ctx, ref.EventID); err != nil {
				return nil, nil, err
			}
			if st.event.MeetID != in.MeetID {
				return nil, nil, invalid("项目不存在或已删除")
			}
			states[ref.EventID] = st
		}
		round := st.round(ref.RoundID)
		if round == nil {
			return nil, nil, ErrStale
		}
		idx := 0
		for i := range st.rounds {
			if st.rounds[i].ID == round.ID {
				idx = i
			}
		}
		row := BatchArrangeRound{EventID: ref.EventID, RoundID: ref.RoundID, Text: roundText(st.event, len(st.rounds), *round),
			Source: "报名", PerHeat: st.event.Rules().DefaultPerHeat, order: []string{}}
		if idx > 0 {
			row.Source = st.rounds[idx-1].Label() + "晋级者"
		}
		req := PlanRequest{EventID: ref.EventID, RoundID: ref.RoundID, FromAdvance: idx > 0, PerHeat: row.PerHeat, Order: in.Order,
			Lanes: in.Lanes, SeparateUnits: in.SeparateUnits, Balance: in.Balance, Seed: in.Seed}
		if req.Lanes == "center" && !st.event.UsesLanes() {
			req.Lanes = "sequential"
		}
		var heats []string
		for _, h := range round.Heats {
			row.order = append(row.order, h.ID)
			heats = append(heats, arrange.Fingerprint(h))
			for _, sl := range h.Slots {
				req.Exclude = append(req.Exclude, sl.CompetitorID())
			}
		}
		plan, err := st.planRound(req)
		var ie *InputError
		switch {
		case errors.As(err, &ie):
			row.Error = ie.Msg
		case err != nil:
			return nil, nil, err
		case len(plan) == 0:
			row.Error = "没有要编排的人"
		}
		row.plan = plan
		for _, h := range plan {
			row.Heats++
			row.People += len(h.Slots)
		}
		p.Heats += row.Heats
		p.People += row.People
		p.Rounds = append(p.Rounds, row)
		stamps = append(stamps, stamp{ref.RoundID, heats, plan})
	}
	if len(eventRounds(p)) == 0 {
		p.Errors = append(p.Errors, "没有可编排的轮次")
	}
	raw, err := json.Marshal(struct {
		Request BatchArrangeRequest
		Rounds  []stamp
	}{in, stamps})
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(raw)
	p.PreviewHash = hex.EncodeToString(sum[:])
	return p, states, nil
}
