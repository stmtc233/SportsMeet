package domain

import "errors"

// EntryLimits are a meet's 报名限制; 0 means no limit. Exceeding them is
// only pointed out, never refused (docs/agents/domain.md「报名限制」).
type EntryLimits struct {
	// AthleteEvents is how many events one athlete may enter. Individual
	// events count; a combined counts once; team events do not count.
	AthleteEvents int `json:"athleteEvents"`
	// UnitAthletes is how many athletes of one unit may enter one
	// individual event or combined.
	UnitAthletes int `json:"unitAthletes"`
	// UnitTeams is how many teams of one unit may enter one team event.
	UnitTeams int `json:"unitTeams"`
}

// MaxEntryLimit is the largest limit that can be set.
const MaxEntryLimit = 99

func (l EntryLimits) Validate() error {
	for _, n := range []int{l.AthleteEvents, l.UnitAthletes, l.UnitTeams} {
		if n < 0 || n > MaxEntryLimit {
			return errors.New("报名限制须为 0–99")
		}
	}
	return nil
}

// Any reports whether some limit is set.
func (l EntryLimits) Any() bool {
	return l.AthleteEvents > 0 || l.UnitAthletes > 0 || l.UnitTeams > 0
}

// LimitEntry is one entry as the limits count it: Item is the individual
// or team event, or the combined (its components are not items).
// UnitID is the athlete's or the team's unit, "" = none.
type LimitEntry struct {
	Item      string
	AthleteID string
	TeamID    string
	UnitID    string
}

// UnitItem is one unit in one item.
type UnitItem struct {
	Item   string
	UnitID string
}

// EntryLimitReport lists only what is over a limit.
type EntryLimitReport struct {
	// Athletes maps an athlete to the number of items entered.
	Athletes map[string]int
	// Units maps a unit in an item to its athletes (or teams) there.
	Units map[UnitItem]int
}

// CheckEntryLimits counts the entries against the limits. Entries without a
// unit never count for a unit; a repeated entry counts once.
func CheckEntryLimits(l EntryLimits, list []LimitEntry) EntryLimitReport {
	out := EntryLimitReport{Athletes: map[string]int{}, Units: map[UnitItem]int{}}
	seen := map[LimitEntry]bool{}
	items := map[string]int{}
	units := map[UnitItem]int{}
	teamItems := map[UnitItem]bool{}
	for _, e := range list {
		key := LimitEntry{Item: e.Item, AthleteID: e.AthleteID, TeamID: e.TeamID}
		if seen[key] {
			continue
		}
		seen[key] = true
		if e.TeamID == "" {
			items[e.AthleteID]++
		}
		if e.UnitID != "" {
			k := UnitItem{e.Item, e.UnitID}
			units[k]++
			teamItems[k] = e.TeamID != ""
		}
	}
	if l.AthleteEvents > 0 {
		for id, n := range items {
			if n > l.AthleteEvents {
				out.Athletes[id] = n
			}
		}
	}
	for k, n := range units {
		limit := l.UnitAthletes
		if teamItems[k] {
			limit = l.UnitTeams
		}
		if limit > 0 && n > limit {
			out.Units[k] = n
		}
	}
	return out
}
