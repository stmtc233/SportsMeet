package interchange

import "encoding/json"

// Rules reference ordinary events or combined parents by file-local IDs.
func validateTeamScoring(f *File) Problems {
	r := f.Meet.TeamScoring
	if r == nil {
		return nil
	}
	p := Problems{}
	if err := r.Validate(); err != nil {
		p = append(p, "meet.teamScoring："+err.Error())
	}
	events := map[string]bool{}
	for _, e := range f.Events {
		if e.CombinedID == "" {
			events[e.ID] = true
		}
	}
	combineds := map[string]bool{}
	for _, c := range f.Combineds {
		combineds[c.ID] = true
	}
	for _, o := range r.Overrides {
		if (o.EventID != "" && !events[o.EventID]) || (o.CombinedID != "" && !combineds[o.CombinedID]) {
			p = append(p, "meet.teamScoring.overrides 找不到普通项目或全能")
		}
	}
	return p
}

// Required zero-valued fields must be present: a missing relay multiplier
// must not silently exclude relay events when a v7 rule is imported.
func validateTeamScoringShape(data []byte) Problems {
	var doc struct {
		Meet struct {
			Rules map[string]json.RawMessage `json:"teamScoring"`
		} `json:"meet"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Problems{err.Error()}
	}
	p := Problems{}
	for _, key := range []string{"enabled", "places", "tiePolicy", "relayMultiplier", "overrides"} {
		raw := doc.Meet.Rules[key]
		if len(raw) == 0 || string(raw) == "null" {
			p = append(p, "meet.teamScoring."+key+" 必填")
		}
	}
	var overrides []map[string]json.RawMessage
	if err := json.Unmarshal(doc.Meet.Rules["overrides"], &overrides); err == nil {
		for _, o := range overrides {
			if raw := o["multiplier"]; len(raw) == 0 || string(raw) == "null" {
				p = append(p, "meet.teamScoring.overrides.multiplier 必填")
			}
		}
	}
	return p
}
