package examplemeet

import (
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func makeResult(spec discipline, category, round, i, count int) *interchange.Result {
	// A permutation gives schools different strengths without random global state.
	v := int64((i*17 + category*7 + len(spec.name)) % count)
	base := spec.base
	if category%2 == 1 {
		if spec.kind == "track" || spec.kind == "relay" {
			base = base * 114 / 100
		}
		if spec.kind == "distance" {
			base = base * 82 / 100
		}
	}
	r := &interchange.Result{Status: "OK"}
	if round == 0 {
		switch {
		case i == 22:
			r.Status = "DNS"
			return r
		case i == 29 && spec.kind == "track":
			r.Status = "DNF"
			return r
		case i == 43 && spec.kind == "track":
			r.Status = "DQ"
			return r
		}
	}
	switch spec.kind {
	case "track", "relay":
		step := int64(30)
		if spec.base > 20000 {
			step = 90
		}
		if spec.base > 100000 {
			step = 1200
		}
		r.Mark = ptr(base + v*step - int64(round)*80)
	case "distance":
		mark := base + v*30
		if i == 21 {
			r.Status = "NM"
			r.Attempts = []interchange.Attempt{{Outcome: "foul"}, {Outcome: "foul"}, {Outcome: "foul"}}
			return r
		}
		r.Attempts = []interchange.Attempt{{Outcome: "valid", Mark: ptr(mark - 150)}, {Outcome: "valid", Mark: ptr(mark)}, {Outcome: "valid", Mark: ptr(mark - 70)}}
		if i%4 == 0 {
			r.Attempts[0] = interchange.Attempt{Outcome: "foul"}
		}
		if i%5 == 0 {
			r.Attempts[2] = interchange.Attempt{Outcome: "pass"}
		}
		if spec.distance == "jump" {
			for a := range r.Attempts {
				if r.Attempts[a].Mark != nil {
					r.Attempts[a].Wind = ptr(10 + a*2)
				}
			}
			r.Wind = ptr(12)
		}
		r.Mark = ptr(mark)
	case "height":
		start := int64(1200)
		if category%2 == 1 {
			start = 1050
		}
		if i == 21 {
			r.Status, r.Bars = "NM", []interchange.Bar{{Height: start, Trials: "XXX"}}
			return r
		}
		last := start + (2+v%7)*50
		for height := start; height <= last; height += 50 {
			trials := "O"
			if height == last && i%3 == 0 {
				trials = "XO"
			}
			r.Bars = append(r.Bars, interchange.Bar{Height: height, Trials: trials})
		}
		r.Bars = append(r.Bars, interchange.Bar{Height: last + 50, Trials: "XXX"})
		r.Mark = ptr(last)
	case "count":
		mark := base + v*2
		if i == 2 || i == 3 {
			mark = 164
		} // An unresolved, legitimate tie.
		if i == 19 {
			mark = 0
		} // Valid zero is distinct from a missing mark.
		first := max(int64(0), mark-8)
		r.Attempts = []interchange.Attempt{{Outcome: "valid", Mark: ptr(first)}, {Outcome: "valid", Mark: ptr(mark)}}
		r.Mark = ptr(mark)
	case "score":
		first, second := base+v*25, base+v*20+75
		r.Attempts = []interchange.Attempt{{Outcome: "valid", Mark: ptr(first)}, {Outcome: "valid", Mark: ptr(second)}}
		r.Mark = ptr(first + second)
	}
	return r
}

func addCombined(f *interchange.File, group, sex int) {
	category := group*2 + sex
	c := interchange.Combined{ID: fmt.Sprintf("c%d", len(f.Combineds)+1), Name: "校园三项全能", Gender: []string{"男", "女"}[sex],
		GroupID: f.Groups[group].ID, GroupLabel: f.Groups[group].Name, RuleVersion: 1, Note: "示例分档计分表，仅用于演示。"}
	for p := 14; p < 16; p++ {
		for u := range f.Units {
			c.AthleteIDs = append(c.AthleteIDs, athleteID(group, sex, u, p))
		}
	}
	specs := []discipline{
		{name: "100m", kind: "track", start: "lanes", from: 14, to: 16, base: 12500, stages: []roundPlan{{day: 0, minute: 720 + category*15, duration: 4}}},
		{name: "跳远", kind: "distance", distance: "jump", from: 14, to: 16, base: 4900, stages: []roundPlan{{day: 1, minute: 780 + category*30, duration: 12}}},
		{name: "800m", kind: "track", start: "break", from: 14, to: 16, base: 140000, stages: []roundPlan{{day: 2, minute: 940 + category*15, duration: 4}}},
	}
	for index, spec := range specs {
		e := makeEvent(f, group, sex, spec)
		e.CombinedID = c.ID
		finishEvent(&e, spec, category)
		rule := domain.PointRule{InvalidPolicies: map[string]string{"DNS": "exclude", "DNF": "exclude", "DQ": "exclude", "NM": "zero"}}
		for b := range 20 {
			mark := int64(12000 + b*350)
			if index == 1 {
				mark = int64(7000 - b*250)
			}
			if index == 2 {
				mark = int64(130000 + b*6000)
			}
			rule.Bands = append(rule.Bands, domain.PointBand{Mark: mark, Points: int64(1000 - b*50)})
		}
		c.Components = append(c.Components, interchange.CombinedComponent{EventID: e.ID, Ordinal: index + 1, Rule: rule})
		f.Events = append(f.Events, e)
	}
	f.Combineds = append(f.Combineds, c)
}
