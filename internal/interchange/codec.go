package interchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Problems lists every validation error of a file (spec §6: report all,
// not just the first).
type Problems []string

func (p Problems) Error() string { return "交换文件有误:\n" + strings.Join(p, "\n") }

// ErrNewerVersion means the file was written by a newer app.
var ErrNewerVersion = errors.New("文件来自更新版本的软件,请先升级")

// Decode parses and validates a file. A non-nil error is either a JSON
// error, ErrNewerVersion, or Problems.
func Decode(data []byte) (*File, error) {
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("不是有效的 JSON:%w", err)
	}
	if head.Format != Format {
		return nil, Problems{"format 必须为 " + Format}
	}
	if head.Version > Version {
		return nil, ErrNewerVersion
	}
	if head.Version < 1 {
		return nil, Problems{"version 必须为正整数"}
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("字段类型不正确:%w", err)
	}
	if p := Validate(&f); len(p) > 0 {
		return nil, p
	}
	return &f, nil
}

// Encode writes f as indented JSON with a trailing newline.
func Encode(f *File) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	dateRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	minuteRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$`)
	trialsRe = regexp.MustCompile(`^[OX-]{1,3}$`)
)

// Validate checks spec §6 rules 2–6 (structure, references, uniqueness,
// result consistency).
func Validate(f *File) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }

	if strings.TrimSpace(f.Meet.Name) == "" {
		add("meet.name 不能为空")
	}
	if !dateRe.MatchString(f.Meet.StartDate) {
		add("meet.startDate 应为 YYYY-MM-DD")
	}
	if f.Meet.EndDate != nil && !dateRe.MatchString(*f.Meet.EndDate) {
		add("meet.endDate 应为 YYYY-MM-DD")
	}

	units := map[string]bool{}
	unitNames := map[string]bool{}
	for i, u := range f.Units {
		if u.ID == "" || units[u.ID] {
			add("units[%d].id 为空或重复", i)
		}
		units[u.ID] = true
		if n := domain.NormalizeText(u.Name); n == "" || unitNames[n] {
			add("units[%d].name 为空或重复", i)
		} else {
			unitNames[n] = true
		}
	}

	athletes := map[string]bool{}
	bibs, numbers := map[string]bool{}, map[string]bool{}
	for i, a := range f.Athletes {
		if a.ID == "" || athletes[a.ID] {
			add("athletes[%d].id 为空或重复", i)
		}
		athletes[a.ID] = true
		if strings.TrimSpace(a.Name) == "" {
			add("athletes[%d].name 不能为空", i)
		}
		if a.Gender != "" && a.Gender != string(domain.GenderMale) && a.Gender != string(domain.GenderFemale) {
			add("athletes[%d].gender 只能为 男、女 或空", i)
		}
		if a.UnitID != nil && !units[*a.UnitID] {
			add("athletes[%d].unitId 找不到参赛单位 %s", i, *a.UnitID)
		}
		if a.BirthDate != nil && !dateRe.MatchString(*a.BirthDate) {
			add("athletes[%d].birthDate 应为 YYYY-MM-DD", i)
		}
		if k := domain.NormalizeText(a.Bib); k != "" {
			if bibs[k] {
				add("athletes[%d].bib %s 重复", i, a.Bib)
			}
			bibs[k] = true
		}
		if k := domain.NormalizeText(a.StudentNumber); k != "" {
			if numbers[k] {
				add("athletes[%d].studentNumber %s 重复", i, a.StudentNumber)
			}
			numbers[k] = true
		}
	}

	events := map[string]bool{}
	items := map[string]bool{}
	for i, e := range f.Events {
		at := fmt.Sprintf("events[%d]", i)
		if e.ID == "" || events[e.ID] {
			add("%s.id 为空或重复", at)
		}
		events[e.ID] = true
		g, ok := domain.NormalizeEventGender(e.Gender)
		if !ok || string(g) != e.Gender {
			add("%s.gender 只能为 男、女 或 混合", at)
		}
		if strings.TrimSpace(e.Discipline) == "" {
			add("%s.discipline 不能为空", at)
		}
		item := e.Gender + "|" + domain.DisciplineKey(e.Discipline) + "|" + domain.NormalizeText(e.GroupLabel)
		if items[item] {
			add("%s 与其他项目的性别、项目名、组别重复", at)
		}
		items[item] = true
		kind := domain.EventKind(e.Kind)
		if !kind.Valid() {
			add("%s.kind 只能为 track、relay、distance 或 height", at)
			continue
		}
		if e.Date != nil && !dateRe.MatchString(*e.Date) {
			add("%s.date 应为 YYYY-MM-DD", at)
		}
		p = append(p, validateEvent(at, kind, &e, athletes, units)...)
	}
	return p
}

func validateEvent(at string, kind domain.EventKind, e *Event, athletes, units map[string]bool) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	relay := kind == domain.KindRelay

	teams := map[string]bool{}
	for i, t := range e.Teams {
		if !relay {
			add("%s.teams 只用于接力项目", at)
			break
		}
		if t.ID == "" || teams[t.ID] {
			add("%s.teams[%d].id 为空或重复", at, i)
		}
		teams[t.ID] = true
		if t.UnitID != nil && !units[*t.UnitID] {
			add("%s.teams[%d].unitId 找不到参赛单位", at, i)
		}
		for _, m := range t.MemberIDs {
			if !athletes[m] {
				add("%s.teams[%d].memberIds 找不到运动员 %s", at, i, m)
			}
		}
	}

	competitor := func(where, athleteID, teamID string) (string, bool) {
		switch {
		case athleteID != "" && teamID != "":
			add("%s 不能同时有 athleteId 与 teamId", where)
		case relay && teamID == "":
			add("%s 接力项目必须用 teamId", where)
		case !relay && athleteID == "":
			add("%s 个人项目必须用 athleteId", where)
		case relay && !teams[teamID]:
			add("%s.teamId 找不到队伍 %s", where, teamID)
		case !relay && !athletes[athleteID]:
			add("%s.athleteId 找不到运动员 %s", where, athleteID)
		default:
			return athleteID + teamID, true
		}
		return "", false
	}

	entered := map[string]bool{}
	for i, en := range e.Entries {
		if key, ok := competitor(fmt.Sprintf("%s.entries[%d]", at, i), en.AthleteID, en.TeamID); ok {
			if entered[key] {
				add("%s.entries[%d] 重复报名", at, i)
			}
			entered[key] = true
		}
	}

	rounds := map[int]bool{}
	for ri, r := range e.Rounds {
		rat := fmt.Sprintf("%s.rounds[%d]", at, ri)
		if !domain.Stage(r.Stage).Valid() {
			add("%s.stage 不正确", rat)
		}
		if r.Ordinal < 1 || rounds[r.Ordinal] {
			add("%s.ordinal 应从 1 开始且不重复", rat)
		}
		rounds[r.Ordinal] = true
		heats := map[int]bool{}
		inRound := map[string]bool{}
		for hi, h := range r.Heats {
			hat := fmt.Sprintf("%s.heats[%d]", rat, hi)
			if h.Ordinal < 1 || heats[h.Ordinal] {
				add("%s.ordinal 应从 1 开始且不重复", hat)
			}
			heats[h.Ordinal] = true
			if h.ScheduledAt != nil && !minuteRe.MatchString(*h.ScheduledAt) {
				add("%s.scheduledAt 应为 YYYY-MM-DDTHH:mm", hat)
			}
			positions := map[int]bool{}
			for si, s := range h.Slots {
				sat := fmt.Sprintf("%s.slots[%d]", hat, si)
				if s.Position < 1 || positions[s.Position] {
					add("%s.position 应从 1 开始且组内不重复", sat)
				}
				positions[s.Position] = true
				if key, ok := competitor(sat, s.AthleteID, s.TeamID); ok {
					if !entered[key] {
						add("%s 未在本项目报名", sat)
					}
					if inRound[key] {
						add("%s 在同一轮次中出现两次", sat)
					}
					inRound[key] = true
				}
				if s.Result != nil {
					p = append(p, validateResult(sat+".result", kind, s.Result)...)
				}
			}
		}
	}
	return p
}

func validateResult(at string, kind domain.EventKind, r *Result) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	status := domain.Status(r.Status)
	switch {
	case !status.Valid():
		add("%s.status 不正确", at)
	case status == domain.StatusOK && (r.Mark == nil || *r.Mark < 0):
		add("%s 状态为 OK 时 mark 必填", at)
	case status != domain.StatusOK && r.Mark != nil:
		add("%s 状态不是 OK 时 mark 应为 null", at)
	}
	if r.Qualify != nil && *r.Qualify != "Q" && *r.Qualify != "q" {
		add("%s.qualify 只能为 Q、q 或 null", at)
	}
	if len(r.Attempts) > 0 && kind != domain.KindDistance {
		add("%s.attempts 只用于远度项目", at)
	}
	for i, a := range r.Attempts {
		switch {
		case a.Outcome == "valid" && a.Mark == nil:
			add("%s.attempts[%d] 有效试跳必须有 mark", at, i)
		case a.Outcome != "valid" && a.Outcome != "foul" && a.Outcome != "pass":
			add("%s.attempts[%d].outcome 不正确", at, i)
		case a.Outcome != "valid" && a.Mark != nil:
			add("%s.attempts[%d] 非有效试跳 mark 应为 null", at, i)
		}
	}
	if len(r.Bars) > 0 && kind != domain.KindHeight {
		add("%s.bars 只用于高度项目", at)
	}
	for i, b := range r.Bars {
		if !trialsRe.MatchString(b.Trials) {
			add("%s.bars[%d].trials 不正确", at, i)
		}
		if i > 0 && b.Height <= r.Bars[i-1].Height {
			add("%s.bars 应按高度升序", at)
		}
	}
	return p
}
