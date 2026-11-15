package domain

import (
	"reflect"
	"testing"
)

func TestFollowUnitGroups(t *testing.T) {
	ath := func(id, unit, group string) Athlete { return Athlete{ID: id, UnitID: unit, GroupID: group} }
	tests := []struct {
		name     string
		units    []Unit
		athletes []Athlete
		units2   map[string]bool
		ath2     map[string]bool
		adds     []UnitGroupAdd
		moves    []AthleteGroupMove
	}{
		{"unit joins its first group: its athletes follow", []Unit{{ID: "u", GroupIDs: []string{"g1"}}},
			[]Athlete{ath("a", "u", ""), ath("b", "u", "")}, map[string]bool{"u": true}, nil,
			nil, []AthleteGroupMove{{"a", "", "g1"}, {"b", "", "g1"}}},
		{"unit gains a second group: athletes stay", []Unit{{ID: "u", GroupIDs: []string{"g1", "g2"}}},
			[]Athlete{ath("a", "u", "g1"), ath("b", "u", "")}, map[string]bool{"u": true}, nil, nil, nil},
		{"unit leaves a group: its athletes go to the one left", []Unit{{ID: "u", GroupIDs: []string{"g1"}}},
			[]Athlete{ath("a", "u", "g2"), ath("b", "u", "g1")}, map[string]bool{"u": true}, nil,
			nil, []AthleteGroupMove{{"a", "g2", "g1"}}},
		{"unit leaves a group of three: its athletes are ungrouped", []Unit{{ID: "u", GroupIDs: []string{"g1", "g3"}}},
			[]Athlete{ath("a", "u", "g2")}, map[string]bool{"u": true}, nil,
			nil, []AthleteGroupMove{{"a", "g2", ""}}},
		{"unit leaves every group", []Unit{{ID: "u"}}, []Athlete{ath("a", "u", "g1")}, map[string]bool{"u": true}, nil,
			nil, []AthleteGroupMove{{"a", "g1", ""}}},
		{"athlete into a group its unit lacks adds it", []Unit{{ID: "u", GroupIDs: []string{"g1"}}},
			[]Athlete{ath("a", "u", "g2"), ath("b", "u", "g1")}, nil, map[string]bool{"a": true},
			[]UnitGroupAdd{{"u", "g2"}}, nil},
		{"athlete into a group of a unit without groups: the unit's other athletes follow", []Unit{{ID: "u"}},
			[]Athlete{ath("a", "u", "g1"), ath("b", "u", "")}, nil, map[string]bool{"a": true},
			[]UnitGroupAdd{{"u", "g1"}}, []AthleteGroupMove{{"b", "", "g1"}}},
		{"athlete ungrouped in a unit of one group stays in it", []Unit{{ID: "u", GroupIDs: []string{"g1"}}},
			[]Athlete{ath("a", "u", "")}, nil, map[string]bool{"a": true}, nil, []AthleteGroupMove{{"a", "", "g1"}}},
		{"athlete without unit keeps its group", nil, []Athlete{ath("a", "", "g1")}, nil, map[string]bool{"a": true}, nil, nil},
		{"other units are left alone", []Unit{{ID: "u", GroupIDs: []string{"g1"}}, {ID: "v", GroupIDs: []string{"g1"}}},
			[]Athlete{ath("a", "v", "")}, map[string]bool{"u": true}, nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adds, moves := FollowUnitGroups(tt.units, tt.athletes, tt.units2, tt.ath2)
			if !reflect.DeepEqual(adds, tt.adds) || !reflect.DeepEqual(moves, tt.moves) {
				t.Fatalf("got %v %v, want %v %v", adds, moves, tt.adds, tt.moves)
			}
		})
	}
}
