package scoring

import (
	"github.com/stmtc233/sportsmeet/internal/domain"
	"testing"
)

func TestCombinedZeroPendingInvalidAndThresholds(t *testing.T) {
	e := domain.Event{ID: "count", Kind: domain.KindCount}
	rule := domain.PointRule{Bands: []domain.PointBand{{Mark: 10, Points: 100}, {Mark: 5, Points: 50}}, InvalidPolicies: map[string]string{"DNS": "zero", "DNF": "zero", "DQ": "exclude", "NM": "zero"}}
	if err := rule.Validate(e); err != nil {
		t.Fatal(err)
	}
	if Points(e, rule, 10) != 100 || Points(e, rule, 9) != 50 || Points(e, rule, 0) != 0 {
		t.Fatal("threshold boundaries")
	}
	zero := int64(0)
	source := CombinedSource{Event: e, Rule: rule, Results: map[string]*domain.Result{"zero": {Status: domain.StatusOK, Mark: &zero}, "nm": {Status: domain.StatusNM}, "dq": {Status: domain.StatusDQ}}}
	rows, complete, err := CombinedRank([]string{"zero", "nm", "dq", "pending"}, []CombinedSource{source})
	if err != nil || complete {
		t.Fatal(err, complete)
	}
	byID := map[string]CombinedStanding{}
	for _, r := range rows {
		byID[r.AthleteID] = r
	}
	if byID["zero"].Cells[0].Points == nil || *byID["zero"].Cells[0].Points != 0 || byID["zero"].Pending {
		t.Fatal("valid zero")
	}
	if byID["nm"].Excluded || byID["nm"].Pending || *byID["nm"].Cells[0].Points != 0 {
		t.Fatal("NM zero policy")
	}
	if !byID["dq"].Excluded || byID["dq"].Rank != nil {
		t.Fatal("DQ exclusion")
	}
	if !byID["pending"].Pending || byID["pending"].Cells[0].Points != nil {
		t.Fatal("pending became zero")
	}
	_, complete, err = CombinedRank([]string{"zero", "nm", "dq"}, []CombinedSource{source})
	if err != nil || !complete {
		t.Fatal("complete statuses")
	}
}
