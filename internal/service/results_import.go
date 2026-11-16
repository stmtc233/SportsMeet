package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// ResultsImportPreview is what importing a results file
// (docs/results-format.md) into a meet would do.
type ResultsImportPreview struct {
	MeetName           string         `json:"meetName"`
	Heats              int            `json:"heats"`
	ResultsNew         int            `json:"resultsNew"`
	ResultsOverwritten int            `json:"resultsOverwritten"`
	ResultsUnchanged   int            `json:"resultsUnchanged"`
	Overwrites         []ResultChange `json:"overwrites"`
	Warnings           []string       `json:"warnings"`
	Errors             []string       `json:"errors"`
	Hash               string         `json:"hash"`
}

type resultsPlan struct {
	preview ResultsImportPreview
	stmts   []sqlite.Statement
}

// importRound is one round the file writes into, with its heats as they
// will be after the import.
type importRound struct {
	st     *eventState
	round  *domain.Round
	before map[slotRef]*domain.Result
	heats  []domain.Heat
}

// planResultsImport works out the import. File problems land in
// preview.Errors, data that no longer fits in preview.Warnings.
func (s *Service) planResultsImport(ctx context.Context, data []byte, meetID string) (*resultsPlan, error) {
	if meetID == "" {
		return nil, invalid("请选择赛事")
	}
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.ExportMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	snapshot.ExportedAt = ""
	plan := &resultsPlan{preview: ResultsImportPreview{MeetName: meet.Name, Overwrites: []ResultChange{}, Warnings: []string{}, Errors: []string{}}}
	p := &plan.preview
	finish := func() (*resultsPlan, error) {
		hashed := *p
		hashed.Hash = ""
		b, _ := json.Marshal(struct {
			Preview ResultsImportPreview
			Data    string
			Target  *interchange.File
		}{hashed, string(data), snapshot})
		sum := sha256.Sum256(b)
		p.Hash = hex.EncodeToString(sum[:])
		return plan, nil
	}

	f, err := interchange.DecodeResults(data)
	if err != nil {
		var problems interchange.Problems
		if errors.As(err, &problems) {
			p.Errors = append(p.Errors, problems...)
		} else {
			p.Errors = append(p.Errors, err.Error())
		}
		return finish()
	}
	if f.MeetID != meetID {
		p.Errors = append(p.Errors, "文件不属于本赛事")
		return finish()
	}

	events := map[string]*eventState{}
	rounds := map[string]*importRound{}
	var order []*importRound
	started := map[string]string{} // heat id → started_at to record
	now := s.store.Now()
	byHeat, err := s.planPromotions(ctx, plan, f, meetID, events, rounds, &order)
	if err != nil {
		return nil, err
	}
	if len(p.Errors) > 0 {
		return finish()
	}
	heatIndex := map[string][2]string{}
	if len(f.Heats) > 0 {
		ids := make([]string, 0, len(f.Heats))
		for _, h := range f.Heats {
			ids = append(ids, h.HeatID)
		}
		raw, _ := json.Marshal(ids)
		rows, err := s.store.Query(ctx, `SELECT h.id,r.event_id,r.id FROM heats h JOIN rounds r ON r.id=h.round_id JOIN events e ON e.id=r.event_id WHERE e.meet_id=? AND h.id IN (SELECT value FROM json_each(?))`, meetID, string(raw))
		if err != nil {
			return nil, err
		}
		for _, row := range rows.Values {
			heatIndex[row[0].(string)] = [2]string{row[1].(string), row[2].(string)}
		}
	}

	for _, fh := range f.Heats {
		label := fh.Label
		if label == "" {
			label = fh.HeatID
		}
		ir := byHeat[fh.HeatID]
		var st *eventState
		if ir != nil {
			st = ir.st
		} else {

			ids, ok := heatIndex[fh.HeatID]
			if !ok {
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」不在本赛事，未导入", label))
				continue
			}
			eventID, roundID := ids[0], ids[1]
			st = events[eventID]
			if st == nil {
				if st, err = s.loadEventParticipants(ctx, eventID); err != nil {
					return nil, err
				}
				events[eventID] = st
			}
			if !st.event.Kind.Timed() {
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」不是计时项目，未导入", label))
				continue
			}
			ir = rounds[roundID]
			if ir == nil {
				round := st.round(roundID)
				ir = &importRound{st: st, round: round, before: storedResults(round.Heats), heats: copyHeats(round.Heats)}
				rounds[roundID] = ir
				order = append(order, ir)
			}
		}

		var heat *domain.Heat
		for i := range ir.heats {
			if ir.heats[i].ID == fh.HeatID {
				heat = &ir.heats[i]
			}
		}
		if heat == nil {
			return nil, ErrStale
		}
		label = st.event.Name + " " + ir.round.Label() + " " + domain.HeatLabel(heat.Ordinal, heat.Name)
		changed := false
		for _, fr := range fh.Results {
			slot, how := findImportSlot(st, heat, fr)
			if slot == nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」第 %d 位的参赛者不在本组，未导入", label, fr.Position))
				continue
			}
			who := st.competitorName(slot.CompetitorID())
			switch {
			case how == "check":
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」%s 按姓名、学号或号码布对应", label, who))
			case slot.Position != fr.Position:
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」%s 的位置与文件不同，按本组位置写入", label, who))
			}
			r := &domain.Result{Status: domain.Status(fr.Status), Mark: fr.Mark, Note: fr.Note}
			old := slot.Result
			if old != nil {
				r.Wind = old.Wind
				if r.Note == "" {
					r.Note = old.Note // the file adds notes, never clears them
				}
				if sameStanding(old, r) {
					r.TieBreak = old.TieBreak // a ruling holds while the marks it ruled on stay
				}
			}
			before, after := resultTextEvent(st.event, old), resultTextEvent(st.event, r)
			switch {
			case old != nil && before == after && old.Note == r.Note:
				p.ResultsUnchanged++
				continue
			case old == nil:
				p.ResultsNew++
			default:
				p.ResultsOverwritten++
				p.Overwrites = append(p.Overwrites, ResultChange{Athlete: label + " " + who, Before: before, After: after, BeforeRecord: old, AfterRecord: r})
			}
			slot.Result = r
			changed = true
		}
		if !changed {
			continue
		}
		p.Heats++
		if heat.StartedAt == "" {
			at := domain.WallClock(now, s.zone)
			if fh.StartedAt != nil {
				at = *fh.StartedAt
			}
			started[heat.ID] = at
		}
	}
	for _, ir := range order {
		rescoreEvent(ir.st.event, ir.round.Advance, ir.heats)
		plan.stmts = append(plan.stmts, resultWrites(ir.before, ir.heats, nil, now)...)
	}
	p.Errors = append(p.Errors, validatePromotionQualifiers(f, rounds)...)
	for heatID, at := range started {
		plan.stmts = append(plan.stmts, store.Stmt(`UPDATE heats SET started_at = ? WHERE id = ? AND started_at = ''`, at, heatID))
	}
	return finish()
}

// findImportSlot finds where a file result goes in its heat: by the
// competitor's id, else by the check fields when exactly one competitor
// of the heat fits (docs/results-format.md §6).
func findImportSlot(st *eventState, heat *domain.Heat, fr interchange.HeatResult) (*domain.Slot, string) {
	for i := range heat.Slots {
		if id := heat.Slots[i].CompetitorID(); id != "" && id == fr.CompetitorID() {
			return &heat.Slots[i], "id"
		}
	}
	if fr.Check == nil {
		return nil, ""
	}
	type key struct {
		of   func(id string) string
		want string
	}
	keys := []key{{func(id string) string { return st.teams[id].Name }, fr.Check.Name}}
	if !st.event.IsTeam() {
		keys = []key{
			{func(id string) string { return st.athletes[id].StudentNumber }, fr.Check.StudentNumber},
			{func(id string) string { return st.athletes[id].Bib }, fr.Check.Bib},
			{func(id string) string { return st.athletes[id].Name }, fr.Check.Name},
		}
	}
	for _, k := range keys {
		want := domain.NormalizeText(k.want)
		if want == "" {
			continue
		}
		var found []*domain.Slot
		for i := range heat.Slots {
			if id := heat.Slots[i].CompetitorID(); id != "" && domain.NormalizeText(k.of(id)) == want {
				found = append(found, &heat.Slots[i])
			}
		}
		if len(found) == 1 {
			return found[0], "check"
		}
		if len(found) > 1 {
			return nil, ""
		}
	}
	return nil, ""
}

// PreviewResultsImport reports what importing a results file into meetID
// would do. It writes nothing.
func (s *Service) PreviewResultsImport(ctx context.Context, data []byte, meetID string) (*ResultsImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planResultsImport(ctx, data, meetID)
	if err != nil {
		return nil, err
	}
	return &plan.preview, nil
}

// ImportResults writes the file's results in one transaction, then the
// places and Q / q of every round it touched. hash is the preview's; when
// the file or the meet changed since, the import is refused with ErrStale.
func (s *Service) ImportResults(ctx context.Context, src store.Source, data []byte, meetID, hash string) (*ResultsImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planResultsImport(ctx, data, meetID)
	if err != nil {
		return nil, err
	}
	p := &plan.preview
	if len(p.Errors) > 0 {
		return nil, invalid("文件有误，无法导入")
	}
	if p.Hash != hash {
		return nil, ErrStale
	}
	if len(plan.stmts) == 0 {
		return p, nil
	}
	err = s.write(ctx,
		store.Audit{Source: src, Action: "results.imported", MeetID: meetID, Message: p.MeetName, Detail: p},
		Change{Kind: "results", MeetID: meetID}, plan.stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}
