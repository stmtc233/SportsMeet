package interchange

import (
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Labels are derived display values; group IDs are the v8 identity source.
func resolveFileGroups(f *File) Problems {
	if f.Version < 8 {
		return nil
	}
	p := Problems{}
	ids := map[string]string{}
	names := map[string]bool{}
	ordinals := map[int]bool{}
	for _, g := range f.Groups {
		key := domain.NormalizeText(g.Name)
		if g.ID == "" || ids[g.ID] != "" || key == "" || names[key] || g.Ordinal < 1 || ordinals[g.Ordinal] {
			p = append(p, "groups 组别标识、名称或顺序为空或重复")
		}
		ids[g.ID] = g.Name
		names[key] = true
		ordinals[g.Ordinal] = true
	}
	label := func(id string, old *string) {
		name := ids[id]
		if id != "" && name == "" {
			p = append(p, "groupId 找不到组别")
		}
		if *old != "" && domain.NormalizeText(*old) != domain.NormalizeText(name) {
			p = append(p, "groupLabel 与组别引用不符")
		}
		*old = name
	}
	for i := range f.Events {
		label(f.Events[i].GroupID, &f.Events[i].GroupLabel)
	}
	for i := range f.Combineds {
		label(f.Combineds[i].GroupID, &f.Combineds[i].GroupLabel)
	}
	for _, a := range f.Athletes {
		if a.GroupID != "" && ids[a.GroupID] == "" {
			p = append(p, fmt.Sprintf("athletes[%s].groupId 找不到组别", a.ID))
		}
	}
	for _, u := range f.Units {
		seen := map[string]bool{}
		for _, id := range u.GroupIDs {
			if id == "" || ids[id] == "" || seen[id] {
				p = append(p, "units.groupIds 找不到组别或重复")
			}
			seen[id] = true
		}
	}
	for _, c := range f.Combineds {
		for _, part := range c.Components {
			for _, e := range f.Events {
				if e.ID == part.EventID && e.GroupID != c.GroupID {
					p = append(p, "全能分项组别引用不符")
				}
			}
		}
	}
	return p
}
