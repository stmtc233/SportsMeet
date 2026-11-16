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

func TestTeamRankMedals(t *testing.T) {
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled, rules.Mode = true, domain.TeamModeMedals
	units := []domain.Unit{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	// Tied first place: both golds, the next place is third (bronze). The
	// outsider takes a place but no unit gets its medal; DNF has no rank.
	tied := TeamScoreSource{EventID: "e1", Gender: domain.GenderMale, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{
		{CompetitorID: "a1", UnitID: "a", Rank: teamPlace(1)},
		{CompetitorID: "b1", UnitID: "b", Rank: teamPlace(1)},
		{CompetitorID: "c1", UnitID: "c", Rank: teamPlace(3)},
		{CompetitorID: "a2", UnitID: "a", Rank: teamPlace(4)},
		{CompetitorID: "dnf", UnitID: "c"},
	}}
	relay := TeamScoreSource{EventID: "relay", Gender: domain.GenderFemale, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{
		{CompetitorID: "team", UnitID: "b", Rank: teamPlace(2)},
		{CompetitorID: "ghost", Rank: teamPlace(1)},
	}}
	pending := TeamScoreSource{EventID: "pending", Gender: domain.GenderMale, Multiplier: 1, Competitors: []TeamScoreCompetitor{{CompetitorID: "x", UnitID: "c", Rank: teamPlace(1)}}}
	excluded := TeamScoreSource{EventID: "excluded", Gender: domain.GenderMale, Complete: true, Multiplier: 0, Competitors: []TeamScoreCompetitor{{CompetitorID: "y", UnitID: "c", Rank: teamPlace(1)}}}
	out := TeamRank(rules, units, []TeamScoreSource{tied, relay, pending, excluded})
	got := map[string][4]int{}
	for _, r := range out.Rows {
		got[r.UnitID] = [4]int{r.Gold, r.Silver, r.Bronze, r.Medals}
		if r.Total.Text != "0" {
			t.Fatal("medals mode has no points", r.Total)
		}
	}
	if got["a"] != [4]int{1, 0, 0, 1} || got["b"] != [4]int{1, 1, 0, 2} || got["c"] != [4]int{0, 0, 1, 1} {
		t.Fatalf("medals: %+v", got)
	}
	if out.Counted != 2 || out.Total != 3 || out.Complete || len(out.Unassigned) != 1 || out.Unassigned[0].Medal != "金" {
		t.Fatalf("progress or outsider: %+v", out)
	}
	// Silver and bronze places show in the lines; non-medal places are not listed.
	for _, r := range out.Rows {
		for _, l := range r.Lines {
			if l.Medal == "" {
				t.Fatal("non-medal place listed", l)
			}
		}
	}
	// Gold first: b (1,1,0) leads a (1,0,0) and c (0,0,1); a and c then differ.
	if out.Rows[0].UnitID != "b" || out.Rows[1].UnitID != "a" || out.Rows[2].UnitID != "c" || *out.Rows[0].Rank != 1 || *out.Rows[1].Rank != 2 || *out.Rows[2].Rank != 3 {
		t.Fatalf("gold order: %+v", out.Rows)
	}
	// The medal total first puts a and c level at one medal each, behind b; a still leads on gold.
	rules.MedalOrder = domain.MedalOrderTotal
	out = TeamRank(rules, units, []TeamScoreSource{tied, relay})
	if out.Rows[0].UnitID != "b" || out.Rows[1].UnitID != "a" || *out.Rows[2].Rank != 3 {
		t.Fatalf("total order: %+v", out.Rows)
	}
	// Units with identical counts tie and the next one follows the skip.
	out = TeamRank(rules, []domain.Unit{{ID: "a"}, {ID: "b"}, {ID: "c"}}, []TeamScoreSource{{Gender: domain.GenderMale, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{
		{UnitID: "a", Rank: teamPlace(1)}, {UnitID: "b", Rank: teamPlace(1)}, {UnitID: "c", Rank: teamPlace(3)}}}})
	if *out.Rows[0].Rank != 1 || *out.Rows[1].Rank != 1 || *out.Rows[2].Rank != 3 {
		t.Fatalf("tie: %+v", out.Rows)
	}
}

func TestTeamRankPointsModeStillCountsMedals(t *testing.T) {
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	out := TeamRank(rules, []domain.Unit{{ID: "a"}}, []TeamScoreSource{{Gender: domain.GenderMale, Complete: true, Multiplier: 1, Competitors: []TeamScoreCompetitor{{UnitID: "a", Rank: teamPlace(2)}, {UnitID: "a", Rank: teamPlace(5)}}}})
	if r := out.Rows[0]; r.Silver != 1 || r.Medals != 1 || len(r.Lines) != 2 || r.Total.Text != "11" || r.Lines[0].Medal != "银" {
		t.Fatalf("%+v", r)
	}
}
