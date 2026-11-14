package domain

import (
	"reflect"
	"testing"
)

func TestCheckEntryLimits(t *testing.T) {
	list := []LimitEntry{
		{Item: "e1", AthleteID: "a", UnitID: "u"},
		{Item: "e1", AthleteID: "a", UnitID: "u"}, // repeated: counts once
		{Item: "e2", AthleteID: "a", UnitID: "u"},
		{Item: "e1", AthleteID: "b", UnitID: "u"},
		{Item: "e1", AthleteID: "c"}, // no unit
		{Item: "r", TeamID: "t1", UnitID: "u"},
		{Item: "r", TeamID: "t2", UnitID: "u"},
	}
	got := CheckEntryLimits(EntryLimits{AthleteEvents: 1, UnitAthletes: 1, UnitTeams: 2}, list)
	want := EntryLimitReport{Athletes: map[string]int{"a": 2}, Units: map[UnitItem]int{{"e1", "u"}: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	none := CheckEntryLimits(EntryLimits{}, list)
	if len(none.Athletes)+len(none.Units) != 0 {
		t.Fatal("zero limits reported")
	}
	if (EntryLimits{UnitTeams: MaxEntryLimit + 1}).Validate() == nil || (EntryLimits{AthleteEvents: -1}).Validate() == nil || (EntryLimits{AthleteEvents: 99}).Validate() != nil {
		t.Fatal("validate")
	}
}
