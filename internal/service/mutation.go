package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Mutation is an internal union used by the MCP adapters. Each tool exposes only its own fields.
type Mutation struct {
	CombinedID string        `json:"combinedId,omitempty"`
	Combined   CombinedInput `json:"combined,omitempty"`
	Action     string        `json:"action"`
	MeetID     string        `json:"meetId,omitempty"`
	EventID    string        `json:"eventId,omitempty"`
	HeatID     string        `json:"heatId,omitempty"`
	ID         string        `json:"id,omitempty"`
	IDs        []string      `json:"ids,omitempty"`
	Entries    []EntryInput  `json:"entries,omitempty"`
	Results    []ResultInput `json:"results,omitempty"`
	Plan       PlanRequest   `json:"plan,omitempty"`
	Table      AthleteImport `json:"table,omitempty"`
	Data       string        `json:"data,omitempty"`
}

// MutationPreview shows the complete affected data and carries an opaque confirmation hash.
type MutationPreview struct {
	PreviewHash string   `json:"previewHash"`
	Errors      []string `json:"errors"`
	Result      any      `json:"result"`
}

type preparedWrite struct {
	audit  store.Audit
	change Change
	stmts  []sqlite.Statement
}

// prepareMutation runs the normal service validation on a request-local planner.
// Its write method collects one batch, without executing SQL, logging or broadcasting.
// The real service's write lock covers preparation, fingerprint checking and commit.
func (s *Service) prepareMutation(ctx context.Context, in Mutation) (*MutationPreview, *preparedWrite, bool, error) {
	before, err := s.mutationSnapshot(ctx, in)
	if err != nil {
		return nil, nil, false, err
	}
	pending := &preparedWrite{}
	planner := &Service{store: s.store, pending: pending, zone: s.zone}
	result, overwrite, err := planner.planMutation(ctx, in, before)
	p := &MutationPreview{Errors: []string{}, Result: result}
	if err != nil {
		if _, ok := err.(*InputError); !ok && err != ErrStale {
			return nil, nil, false, err
		}
		p.Errors = append(p.Errors, err.Error())
		return p, pending, overwrite, nil
	}
	raw, err := json.Marshal(struct {
		Request Mutation
		Before  any
	}{in, before})
	if err != nil {
		return nil, nil, false, err
	}
	sum := sha256.Sum256(raw)
	p.PreviewHash = hex.EncodeToString(sum[:])
	return p, pending, overwrite, nil
}

func (s *Service) PreviewMutation(ctx context.Context, in Mutation) (*MutationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, _, err := s.prepareMutation(ctx, in)
	return p, err
}

func (s *Service) ApplyMutation(ctx context.Context, in Mutation, hash string, confirmed bool) (*MutationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, pending, overwrite, err := s.prepareMutation(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(p.Errors) > 0 {
		return nil, invalid("%s", p.Errors[0])
	}
	// Only filling empty result slots can bypass explicit preview confirmation.
	if in.Action != "set_results" || overwrite {
		if !confirmed || hash == "" {
			return nil, invalid("请先预览并确认")
		}
		if hash != p.PreviewHash {
			return nil, ErrStale
		}
	} else if confirmed && hash != "" && hash != p.PreviewHash {
		return nil, ErrStale
	}
	if pending.audit.Action != "" {
		if err := s.write(ctx, pending.audit, pending.change, pending.stmts...); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *Service) mutationSnapshot(ctx context.Context, in Mutation) (any, error) {
	meetID := in.MeetID
	if in.CombinedID != "" {
		d, err := s.GetCombined(ctx, in.CombinedID)
		if err != nil {
			return nil, err
		}
		if meetID != "" && meetID != d.Combined.MeetID {
			return nil, invalid("全能不属于本赛事")
		}
		meetID = d.Combined.MeetID
	}
	eventID := in.EventID
	if in.Action == "auto_arrange" || in.Action == "advance" {
		eventID = in.Plan.EventID
	}
	if in.HeatID != "" {
		rows, err := s.store.Query(ctx, `SELECT r.event_id FROM heats h JOIN rounds r ON r.id = h.round_id WHERE h.id = ?`, in.HeatID)
		if err != nil {
			return nil, err
		}
		if len(rows.Values) == 0 {
			return nil, invalid("组不存在或已删除")
		}
		eventID = rows.Values[0][0].(string)
	}
	if eventID != "" {
		ev, err := s.GetEvent(ctx, eventID)
		if err != nil {
			return nil, err
		}
		if meetID != "" && meetID != ev.Event.MeetID {
			return nil, invalid("项目不属于本赛事")
		}
		meetID = ev.Event.MeetID
	}
	if meetID == "" {
		if in.Action != "import_meet" {
			return nil, invalid("请选择赛事")
		}
		return s.ListMeets(ctx)
	}
	f, err := s.ExportMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	f.ExportedAt = "" // Export time is not competition state.
	return f, nil
}
