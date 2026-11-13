package scoring

import (
	"math/big"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func teamPlace(p int) *int { return &p }

func TestTeamRankTiesCutoffUnassignedAndMultipliers(t *testing.T) {
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	rules.Places = []int64{9, 7, 6}
	units := []domain.Unit{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	src := TeamScoreSource{EventID: "final", Gender: domain.GenderMale, Complete: true, Multiplier: 2, Competitors: []TeamScoreCompetitor{
		{CompetitorID: "outsider", Rank: teamPlace(1)},
		{CompetitorID: "a1", UnitID: "a", Rank: teamPlace(2)},
		{CompetitorID: "b1", UnitID: "b", Rank: teamPlace(2)},
		{CompetitorID: "a2", UnitID: "a", Rank: teamPlace(4)},
		{CompetitorID: "dq", UnitID: "c"},
	}}
	out := TeamRank(rules, units, []TeamScoreSource{src, {EventID: "pending", Gender: domain.GenderFemale, Multiplier: 1}, {EventID: "excluded", Gender: domain.GenderMixed, Multiplier: 0}})
	if out.Complete || out.Counted != 1 || out.Total != 2 || out.Rows[0].Total.Exact != "13" || out.Rows[1].Total.Exact != "13" || *out.Rows[0].Rank != 1 || *out.Rows[1].Rank != 1 || *out.Rows[2].Rank != 3 || len(out.Unassigned) != 1 || out.Unassigned[0].Score.Text != "18" {
		t.Fatalf("totals: %+v", out)
	}
	if out.Rows[0].Lines[0].Base.Text != "6.5" || out.Rows[0].Lines[1].Score.Text != "0" {
		t.Fatal("averaged or out-of-range points")
	}
	rules.TiePolicy = "same"
	out = TeamRank(rules, units, []TeamScoreSource{src})
	if !out.Complete || out.Rows[0].Total.Text != "14" || out.Rows[1].Total.Text != "14" {
		t.Fatal("same-place policy")
	}
	// A tie spanning the last scoring place includes zero for the places beyond it.
	rules.TiePolicy = "average"
	rules.Places = []int64{9, 7}
	out = TeamRank(rules, units, []TeamScoreSource{src})
	if out.Rows[0].Lines[0].Base.Text != "3.5" || out.Rows[0].Total.Text != "7" {
		t.Fatal("tie crossing cutoff")
	}
}

func TestTeamRankExactFractionsAndDirectTotalTies(t *testing.T) {
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	rules.Places = []int64{1}
	sources := []TeamScoreSource{}
	for range 3 {
		sources = append(sources, TeamScoreSource{Gender: domain.GenderMixed, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{{UnitID: "a", Rank: teamPlace(1)}, {Rank: teamPlace(1)}, {Rank: teamPlace(1)}}})
	}
	sources = append(sources, TeamScoreSource{Gender: domain.GenderFemale, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{{UnitID: "b", Rank: teamPlace(1)}}})
	out := TeamRank(rules, []domain.Unit{{ID: "a"}, {ID: "b"}}, sources)
	if out.Rows[0].Total.Text != "1" || out.Rows[1].Total.Text != "1" || *out.Rows[0].Rank != 1 || *out.Rows[1].Rank != 1 || out.Rows[0].Lines[0].Base.Text != "1/3" {
		t.Fatalf("fraction sum: %+v", out.Rows)
	}
	// A terminating decimal remains exact regardless of denominator size.
	if s := scoreValue(big.NewRat(1, 8)); s.Text != "0.125" || s.Exact != "1/8" {
		t.Fatal(s)
	}
	rules.Enabled = false
	out = TeamRank(rules, []domain.Unit{{ID: "a"}}, sources)
	if out.Counted != 0 || out.Complete || out.Rows[0].Rank != nil || out.Rows[0].Total.Text != "0" {
		t.Fatal("disabled rules counted")
	}
}
