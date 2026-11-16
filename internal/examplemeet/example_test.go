package examplemeet

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/scoring"
)

func TestCompleteExample(t *testing.T) {
	f := Build()
	data, err := interchange.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interchange.Decode(data); err != nil {
		t.Fatal(err)
	}
	again, err := interchange.Encode(Build())
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("example is not reproducible", err)
	}
	if len(f.Units) != 12 || len(f.Groups) != 3 || len(f.Athletes) != 1152 || len(f.Events) != 93 || len(f.Combineds) != 6 {
		t.Fatal("example size changed; update the documented size")
	}
	athletes := map[string]interchange.Athlete{}
	for _, a := range f.Athletes {
		athletes[a.ID] = a
	}
	entered := map[string]bool{}
	statuses, kinds, teams := map[string]bool{}, map[string]bool{}, 0
	var times []arrange.TimedHeat
	for _, e := range f.Events {
		kinds[e.Kind] = true
		members := map[string][]string{}
		teams += len(e.Teams)
		for _, team := range e.Teams {
			if len(team.MemberIDs) != 4 {
				t.Fatalf("%s: incomplete team", e.ID)
			}
			members[team.ID] = team.MemberIDs
		}
		for _, entry := range e.Entries {
			if entry.AthleteID != "" {
				entered[entry.AthleteID] = true
			}
			for _, a := range members[entry.TeamID] {
				entered[a] = true
			}
		}
		for ri, r := range e.Rounds {
			next := map[string]bool{}
			for _, h := range r.Heats {
				at := r.ScheduledAt
				if h.ScheduledAt != nil {
					at = h.ScheduledAt
				}
				if at == nil || !domain.ValidScheduled(*at) || (*at)[:10] < f.Meet.StartDate || (*at)[:10] > *f.Meet.EndDate {
					t.Fatalf("%s: missing or out-of-range schedule", h.ID)
				}
				timed := arrange.TimedHeat{HeatID: h.ID, Start: *at, Minutes: h.DurationMinutes}
				for _, s := range h.Slots {
					if s.Result == nil || s.Result.HeightPending || h.StartedAt == nil {
						t.Fatalf("%s: unfinished result", h.ID)
					}
					if s.AthleteID != "" {
						timed.Athletes = append(timed.Athletes, s.AthleteID)
						if a := athletes[s.AthleteID]; a.GroupID != e.GroupID || a.CompetitionGender != e.Gender {
							t.Fatalf("%s: mismatched athlete", e.ID)
						}
					}
					timed.Athletes = append(timed.Athletes, members[s.TeamID]...)
					res := s.Result
					statuses[res.Status] = true
					if res.Qualify != nil {
						next[competitor(s)] = true
					}
					if e.Kind == "height" {
						var bars []domain.Bar
						for _, b := range res.Bars {
							bars = append(bars, domain.Bar{Height: b.Height, Trials: b.Trials})
						}
						if err := scoring.ValidateBars(bars); err != nil {
							t.Fatal(h.ID, err)
						}
					}
				}
				times = append(times, timed)
			}
			if ri+1 < len(e.Rounds) {
				actual := map[string]bool{}
				for _, h := range e.Rounds[ri+1].Heats {
					for _, s := range h.Slots {
						actual[competitor(s)] = true
					}
				}
				if !reflect.DeepEqual(next, actual) {
					t.Fatalf("%s: qualifiers differ from next round", r.ID)
				}
			}
		}
	}
	if len(entered) != 1152 || teams != 180 || len(kinds) != 6 || len(statuses) != 5 {
		t.Fatalf("coverage: entered=%d teams=%d kinds=%v statuses=%v", len(entered), teams, kinds, statuses)
	}
	if conflicts := arrange.Conflicts(times, 20); len(conflicts) != 0 {
		t.Fatalf("schedule has %d conflicts: %+v", len(conflicts), conflicts[0])
	}
}
