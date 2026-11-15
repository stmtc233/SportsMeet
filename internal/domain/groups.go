package domain

import "slices"

// CompetitionGroup is a meet's independently ranked category, such as 初一.
// A heat remains a 组; a CompetitionGroup is always called 组别.
type CompetitionGroup struct {
	ID      string `json:"id"`
	MeetID  string `json:"meetId"`
	Name    string `json:"name"`
	Ordinal int    `json:"ordinal"`
}

// UnitGroupAdd is a group a unit joins because one of its athletes is in it.
type UnitGroupAdd struct {
	UnitID  string
	GroupID string
}

// AthleteGroupMove is an athlete whose group follows its unit's groups.
type AthleteGroupMove struct {
	AthleteID string
	From      string
	To        string
}

// AthleteGroupIn is the group an athlete in group g has once its unit is in
// groups: the only one when the unit has one, g while it is one of them,
// else none.
func AthleteGroupIn(groups []string, g string) string {
	if len(groups) == 1 {
		return groups[0]
	}
	if slices.Contains(groups, g) {
		return g
	}
	return ""
}

// FollowUnitGroups keeps every athlete with a unit in one of the unit's
// groups, and in the only one when the unit has one. An athlete changed
// into a group its unit lacks adds the group to the unit; then the athletes
// of each unit changed or added to, and the athletes changed, follow their
// unit's groups. Athletes without a unit keep their group.
func FollowUnitGroups(units []Unit, athletes []Athlete, changedUnits, changedAthletes map[string]bool) ([]UnitGroupAdd, []AthleteGroupMove) {
	groups := map[string][]string{}
	for _, u := range units {
		groups[u.ID] = append([]string{}, u.GroupIDs...)
	}
	touched := map[string]bool{}
	for id := range changedUnits {
		touched[id] = true
	}
	var adds []UnitGroupAdd
	for _, a := range athletes {
		have, ok := groups[a.UnitID]
		if !changedAthletes[a.ID] || !ok || a.GroupID == "" || slices.Contains(have, a.GroupID) {
			continue
		}
		groups[a.UnitID] = append(have, a.GroupID)
		touched[a.UnitID] = true
		adds = append(adds, UnitGroupAdd{UnitID: a.UnitID, GroupID: a.GroupID})
	}
	var moves []AthleteGroupMove
	for _, a := range athletes {
		have, ok := groups[a.UnitID]
		if !ok || !touched[a.UnitID] && !changedAthletes[a.ID] {
			continue
		}
		if to := AthleteGroupIn(have, a.GroupID); to != a.GroupID {
			moves = append(moves, AthleteGroupMove{AthleteID: a.ID, From: a.GroupID, To: to})
		}
	}
	return adds, moves
}
