package service

import (
	"testing"

	"github.com/stmtc233/sportsmeet/internal/store"
)

// tieFixture: two heats, Q = the winner of each, q = the next best one;
// 选手02 and 选手04 tie on 12.50 for that one q.
func tieFixture(t *testing.T) *fixture {
	t.Helper()
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1, AdvanceByMark: 1}}
	f := newFixture(t, 4, in)
	f.addHeat(0, f.athletes[0], f.athletes[1])
	f.addHeat(0, f.athletes[2], f.athletes[3])
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.00"}, {Position: 2, Mark: "12.50"}}))
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[1].ID, []ResultInput{{Position: 1, Mark: "12.10"}, {Position: 2, Mark: "12.50"}}))
	return f
}

func qualifies(r RoundView) map[string]string {
	out := map[string]string{}
	for _, s := range r.Standings {
		out[s.CompetitorID] = s.Qualify
	}
	return out
}

func places(r RoundView) map[string]int {
	out := map[string]int{}
	for _, s := range r.Standings {
		if s.Rank != nil {
			out[s.CompetitorID] = *s.Rank
		}
	}
	return out
}

func TestRuleTieOnLastQualifyingPlace(t *testing.T) {
	f := tieFixture(t)
	a := f.athletes
	r := f.detail().Rounds[0]
	if len(r.Ties) != 1 || r.Ties[0].Place != 3 || r.Ties[0].Ruled || r.Ties[0].Extra != 1 || len(r.Ties[0].CompetitorIDs) != 2 {
		t.Fatalf("ties = %+v", r.Ties)
	}
	if q := qualifies(r); q[a[1]] != "q" || q[a[3]] != "q" {
		t.Fatalf("before the ruling both advance: %v", q)
	}

	wantInvalid(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}}}), "须一起裁定")
	wantInvalid(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}, {a[1], 5}}}), "第 3–4 名")
	if err := f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[0], 1}}}); err != ErrStale {
		t.Fatalf("an untied competitor: %v", err)
	}

	before := len(*f.changes)
	must(t, f.svc.RuleTies(f.ctx, store.SourceUI, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}, {a[1], 4}}}))
	r = f.detail().Rounds[0]
	if p := places(r); p[a[3]] != 3 || p[a[1]] != 4 {
		t.Fatalf("places = %v", p)
	}
	if q := qualifies(r); q[a[3]] != "q" || q[a[1]] != "" || q[a[0]] != "Q" || q[a[2]] != "Q" {
		t.Fatalf("after the ruling = %v", q)
	}
	if len(r.Ties) != 1 || !r.Ties[0].Ruled || r.Ties[0].Extra != 0 || r.Ties[0].CompetitorIDs[0] != a[3] {
		t.Fatalf("ruled tie = %+v", r.Ties)
	}
	if n := len(*f.changes); n != before+1 || (*f.changes)[n-1].Kind != "results" {
		t.Fatalf("changes = %+v", (*f.changes)[before:])
	}
	log, err := f.svc.ListAudit(f.ctx, AuditQuery{})
	must(t, err)
	if log.Rows[0].Action != "results.tie" || log.Rows[0].ActionText != "裁定并列" {
		t.Fatalf("log = %+v", log.Rows[0])
	}

	// The same ruling writes nothing; a note leaves it standing.
	must(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}, {a[1], 4}}}))
	if len(*f.changes) != before+1 {
		t.Fatal("an unchanged ruling was written")
	}
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[0].ID, []ResultInput{{Position: 2, Mark: "12.50", Note: "复核"}}))
	if q := qualifies(f.detail().Rounds[0]); q[a[1]] != "" {
		t.Fatal("a note dropped the ruling")
	}

	// A changed mark drops that competitor's ruling: the tie is whole again.
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[0].ID, []ResultInput{{Position: 2, Mark: "12.40"}}))
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[0].ID, []ResultInput{{Position: 2, Mark: "12.50"}}))
	r = f.detail().Rounds[0]
	if q := qualifies(r); q[a[1]] != "q" || q[a[3]] != "q" || r.Ties[0].Ruled {
		t.Fatalf("after a new mark = %v, %+v", q, r.Ties)
	}

	// All at the shared place clears a ruling.
	must(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}, {a[1], 4}}}))
	must(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[3], 3}, {a[1], 3}}}))
	r = f.detail().Rounds[0]
	if p := places(r); p[a[1]] != 3 || p[a[3]] != 3 || r.Ties[0].Ruled {
		t.Fatalf("cleared = %v, %+v", p, r.Ties)
	}
}

func TestRuleTieInsideHeat(t *testing.T) {
	in := track()
	in.Rounds = []RoundInput{{Stage: "heat", AdvanceByPlace: 1}}
	f := newFixture(t, 3, in)
	a := f.athletes
	f.addHeat(0, a[0], a[1], a[2])
	r := f.detail().Rounds[0]
	must(t, f.svc.SaveResults(f.ctx, ui, r.Heats[0].ID, []ResultInput{{Position: 1, Mark: "12.00"}, {Position: 2, Mark: "12.00"}, {Position: 3, Mark: "12.30"}}))
	r = f.detail().Rounds[0]
	if len(r.Ties) != 1 || r.Ties[0].Extra != 1 {
		t.Fatalf("ties = %+v", r.Ties)
	}
	must(t, f.svc.RuleTies(f.ctx, ui, TieRulingInput{RoundID: r.ID, Places: []TiePlace{{a[1], 1}, {a[0], 2}}}))
	h := f.detail().Rounds[0].Heats[0]
	if *h.Slots[0].Result.Rank != 2 || h.Slots[0].Result.Qualify != "" || *h.Slots[1].Result.Rank != 1 || h.Slots[1].Result.Qualify != "Q" || *h.Slots[2].Result.Rank != 3 {
		t.Fatalf("heat = %+v %+v %+v", h.Slots[0].Result, h.Slots[1].Result, h.Slots[2].Result)
	}
}
