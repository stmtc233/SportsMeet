package interchange

import (
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Existing is what a meet already holds, for matching a file against it.
type Existing struct {
	Combineds []domain.Combined
	Units     []domain.Unit
	Athletes  []domain.Athlete
	Events    []domain.Event
}

// Match maps the file's ids to the meet's: a missing key means the entity
// is new (spec §7).
type Match struct {
	Combineds map[string]string
	Units     map[string]string
	Athletes  map[string]string
	Events    map[string]string
	// Conflicts lists athletes that fit more than one existing athlete;
	// any conflict rejects the whole file.
	Conflicts []string
}

// AffiliationOf is the athlete's unit name: the unit it refers to, else
// its affiliation text.
func AffiliationOf(f *File, a Athlete) string {
	if a.UnitID != nil {
		for _, u := range f.Units {
			if u.ID == *a.UnitID {
				return u.Name
			}
		}
	}
	return a.Affiliation
}

// MatchExisting finds which of the file's units, athletes and events are
// already in the meet.
//
// Athletes are identified by the first non-blank of: student number, bib,
// name + affiliation. Units match by name, events by gender + discipline +
// group label with disciplines compared loosely. Texts are compared
// trimmed, with runs of spaces merged and ASCII case ignored.
func MatchExisting(f *File, ex Existing) Match {
	m := Match{Combineds: map[string]string{}, Units: map[string]string{}, Athletes: map[string]string{}, Events: map[string]string{}}

	unitNames := map[string]string{}
	unitByID := map[string]string{}
	for _, u := range ex.Units {
		unitNames[domain.NormalizeText(u.Name)] = u.ID
		unitByID[u.ID] = u.Name
	}
	for _, u := range f.Units {
		if id, ok := unitNames[domain.NormalizeText(u.Name)]; ok {
			m.Units[u.ID] = id
		}
	}

	byNumber, byBib, byName := map[string][]string{}, map[string][]string{}, map[string][]string{}
	for _, a := range ex.Athletes {
		if k := domain.NormalizeText(a.StudentNumber); k != "" {
			byNumber[k] = append(byNumber[k], a.ID)
		}
		if k := domain.NormalizeText(a.Bib); k != "" {
			byBib[k] = append(byBib[k], a.ID)
		}
		k := domain.NormalizeText(a.Name) + "|" + domain.NormalizeText(unitByID[a.UnitID])
		byName[k] = append(byName[k], a.ID)
	}
	for _, a := range f.Athletes {
		var found []string
		switch {
		case domain.NormalizeText(a.StudentNumber) != "":
			found = byNumber[domain.NormalizeText(a.StudentNumber)]
		case domain.NormalizeText(a.Bib) != "":
			found = byBib[domain.NormalizeText(a.Bib)]
		default:
			found = byName[domain.NormalizeText(a.Name)+"|"+domain.NormalizeText(AffiliationOf(f, a))]
		}
		switch len(found) {
		case 0:
		case 1:
			m.Athletes[a.ID] = found[0]
		default:
			m.Conflicts = append(m.Conflicts, fmt.Sprintf("运动员「%s」与已有的 %d 人相符", a.Name, len(found)))
		}
	}

	combineds := map[string]string{}
	for _, c := range ex.Combineds {
		combineds[domain.CombinedKey(c.Gender, c.Name, c.GroupLabel)] = c.ID
	}
	for _, c := range f.Combineds {
		if id, ok := combineds[domain.CombinedKey(domain.Gender(c.Gender), c.Name, c.GroupLabel)]; ok {
			m.Combineds[c.ID] = id
		}
	}
	events := map[string]string{}
	for _, e := range ex.Events {
		events[e.CombinedID+"|"+domain.EventKey(e.Gender, e.Discipline, e.GroupLabel)] = e.ID
	}
	for _, e := range f.Events {
		if e.CombinedID != "" && m.Combineds[e.CombinedID] == "" {
			continue
		}
		if id, ok := events[m.Combineds[e.CombinedID]+"|"+domain.EventKey(domain.Gender(e.Gender), e.Discipline, e.GroupLabel)]; ok {
			m.Events[e.ID] = id
		}
	}
	return m
}
