package interchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/scoring"
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
	if head.Version < 8 {
		var err error
		if data, err = withoutNewerFields(data, head.Version); err != nil {
			return nil, fmt.Errorf("不是有效的 JSON:%w", err)
		}
	}
	var f File
	var decodeErr error
	if head.Version < 3 {
		// New fields are unknown to published formats, regardless of their JSON types.
		var legacy struct {
			File
			Combineds json.RawMessage `json:"combineds"`
			Events    []struct {
				Event
				ParticipantMode json.RawMessage `json:"participantMode"`
				TeamSize        json.RawMessage `json:"teamSize"`
				ScoreDecimals   json.RawMessage `json:"scoreDecimals"`
				RankDirection   json.RawMessage `json:"rankDirection"`
				AttemptMode     json.RawMessage `json:"attemptMode"`
				CombinedID      json.RawMessage `json:"combinedId"`
				StartMode       json.RawMessage `json:"startMode"`
				DistanceMode    json.RawMessage `json:"distanceMode"`
				RelayLegCount   json.RawMessage `json:"relayLegCount"`
			} `json:"events"`
		}
		decodeErr = json.Unmarshal(data, &legacy)
		f = legacy.File
		if legacy.Events != nil {
			f.Events = make([]Event, len(legacy.Events))
			for i, event := range legacy.Events {
				f.Events[i] = event.Event
				if head.Version == 2 && decodeErr == nil {
					for _, field := range []struct {
						Raw    json.RawMessage
						Target any
					}{{event.StartMode, &f.Events[i].StartMode}, {event.DistanceMode, &f.Events[i].DistanceMode}, {event.RelayLegCount, &f.Events[i].RelayLegCount}} {
						if len(field.Raw) > 0 {
							if err := json.Unmarshal(field.Raw, field.Target); err != nil {
								decodeErr = err
								break
							}
						}
					}
				}
			}
		}
	} else {
		decodeErr = json.Unmarshal(data, &f)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("字段类型不正确:%w", decodeErr)
	}
	p := Validate(&f)
	if head.Version >= 7 && f.Meet.TeamScoring != nil {
		p = append(p, validateTeamScoringShape(data)...)
	}
	if len(p) > 0 {
		return nil, p
	}
	return &f, nil
}

// withoutNewerFields ignores fields outside the file's declared format,
// regardless of their JSON types. Validate handles malformed structures.
func withoutNewerFields(data []byte, version int) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if version < 8 {
		delete(doc, "groups")
		for _, key := range []string{"units", "athletes", "combineds"} {
			list, _ := doc[key].([]any)
			for _, v := range list {
				if obj, ok := v.(map[string]any); ok {
					delete(obj, "groupId")
					delete(obj, "groupIds")
				}
			}
		}
	}
	if version < 7 {
		if meet, ok := doc["meet"].(map[string]any); ok {
			delete(meet, "teamScoring")
		}
	}
	events, _ := doc["events"].([]any)
	for _, e := range events {
		event, _ := e.(map[string]any)
		if version < 8 {
			delete(event, "groupId")
		}
		rounds, _ := event["rounds"].([]any)
		for _, r := range rounds {
			round, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if version < 4 {
				delete(round, "scheduledAt")
			}
			heats, _ := round["heats"].([]any)
			for _, h := range heats {
				if heat, ok := h.(map[string]any); ok {
					if version < 4 {
						delete(heat, "durationMinutes")
					}
					if version < 5 {
						delete(heat, "startedAt")
					}
					if version < 6 {
						slots, _ := heat["slots"].([]any)
						for _, s := range slots {
							if slot, ok := s.(map[string]any); ok {
								if result, ok := slot["result"].(map[string]any); ok {
									delete(result, "heightPending")
								}
							}
						}
					}
				}
			}
		}
	}
	return json.Marshal(doc)
}

var (
	trialsRe = regexp.MustCompile(`^[OX-]{1,3}$`)
)

// Validate checks spec §6 rules 2–6 (structure, references, uniqueness,
// result consistency).
func Validate(f *File) Problems {
	p := resolveFileGroups(f)
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }

	if strings.TrimSpace(f.Meet.Name) == "" {
		add("meet.name 不能为空")
	}
	if !domain.ValidDate(f.Meet.StartDate) {
		add("meet.startDate 应为 YYYY-MM-DD")
	}
	if f.Meet.EndDate != nil && !domain.ValidDate(*f.Meet.EndDate) {
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
		if a.BirthDate != nil && !domain.ValidDate(*a.BirthDate) {
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
		item := e.CombinedID + "|" + e.Gender + "|" + domain.DisciplineKey(e.Discipline) + "|" + domain.NormalizeText(e.GroupLabel)
		if items[item] {
			add("%s 与其他项目的性别、项目名、组别重复", at)
		}
		items[item] = true
		kind := domain.EventKind(e.Kind)
		if !kind.Valid() || (f.Version < 3 && kind.Scalar()) {
			add("%s.kind 项目类型不正确", at)
			continue
		}
		if e.Date != nil && !domain.ValidDate(*e.Date) {
			add("%s.date 应为 YYYY-MM-DD", at)
		}
		p = append(p, validateEvent(at, kind, &e, athletes, units, f.Version >= 2)...)
	}
	p = append(p, validateCombineds(f, athletes)...)
	p = append(p, validateTeamScoring(f)...)
	return p
}

func validateEvent(at string, kind domain.EventKind, e *Event, athletes, units map[string]bool, strictPositions bool) Problems {
	var p Problems
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	config := domain.Event{CompetitionConfig: e.CompetitionConfig, Kind: kind, StartMode: domain.StartMode(e.StartMode), DistanceMode: domain.DistanceMode(e.DistanceMode), RelayLegCount: e.RelayLegCount}
	config.AttemptCount = 3
	if kind.Scalar() {
		config.AttemptCount = 1
	}
	if e.AttemptCount != nil {
		config.AttemptCount = *e.AttemptCount
	}
	if err := config.NormalizeConfig(); err != nil {
		add("%s：%s", at, err)
	}
	// Exchange files retain the published positive ranges, including existing data.
	if e.LaneCount != nil && *e.LaneCount < 1 {
		add("%s.laneCount 至少为 1", at)
	}
	if e.AttemptCount != nil && *e.AttemptCount < 1 {
		add("%s.attemptCount 至少为 1", at)
	}
	relay := config.IsTeam()

	teams := map[string]bool{}
	teamNames := map[string]bool{}
	memberTeam := map[string]string{}
	for i, t := range e.Teams {
		if !relay {
			add("%s.teams 只用于队伍项目", at)
			break
		}
		if t.ID == "" || teams[t.ID] {
			add("%s.teams[%d].id 为空或重复", at, i)
		}
		teams[t.ID] = true
		key := domain.NormalizeText(t.Name) + "|"
		if t.UnitID != nil {
			key += *t.UnitID
		}
		if strings.TrimSpace(t.Name) == "" || teamNames[key] {
			add("%s.teams[%d].name 为空或同单位重名", at, i)
		}
		teamNames[key] = true
		if t.UnitID != nil && !units[*t.UnitID] {
			add("%s.teams[%d].unitId 找不到参赛单位", at, i)
		}
		for _, m := range t.MemberIDs {
			if _, ok := memberTeam[m]; ok {
				add("%s.teams[%d].memberIds 成员重复或已在其他队伍", at, i)
			}
			memberTeam[m] = t.ID
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
			add("%s 队伍项目必须用 teamId", where)
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
		if r.ScheduledAt != nil && !domain.ValidScheduled(*r.ScheduledAt) {
			add("%s.scheduledAt 应为 YYYY-MM-DDTHH:mm", rat)
		}
		heats := map[int]bool{}
		inRound := map[string]bool{}
		for hi, h := range r.Heats {
			hat := fmt.Sprintf("%s.heats[%d]", rat, hi)
			if h.Ordinal < 1 || heats[h.Ordinal] {
				add("%s.ordinal 应从 1 开始且不重复", hat)
			}
			heats[h.Ordinal] = true
			if h.ScheduledAt != nil && !domain.ValidScheduled(*h.ScheduledAt) {
				add("%s.scheduledAt 应为 YYYY-MM-DDTHH:mm", hat)
			}
			if !domain.ValidDuration(h.DurationMinutes) {
				add("%s.durationMinutes 应为 1–%d", hat, domain.MaxDurationMinutes)
			}
			if h.StartedAt != nil && !domain.ValidScheduled(*h.StartedAt) {
				add("%s.startedAt 应为 YYYY-MM-DDTHH:mm", hat)
			}
			positions := map[int]bool{}
			for si, s := range h.Slots {
				sat := fmt.Sprintf("%s.slots[%d]", hat, si)
				if s.Position < 1 || positions[s.Position] {
					add("%s.position 应从 1 开始且组内不重复", sat)
				}
				if strictPositions && config.UsesLanes() {
					lanes := 8
					if e.LaneCount != nil {
						lanes = *e.LaneCount
					}
					if s.Position > lanes {
						add("%s.position 超过道数", sat)
					}
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
					p = append(p, validateResult(sat+".result", config, s.Result)...)
				}
			}
		}
	}
	return p
}

func validateResult(at string, event domain.Event, r *Result) Problems {
	var p Problems
	kind := event.Kind
	add := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if r.HeightPending && kind != domain.KindHeight {
		add("%s.heightPending 只用于高度项目", at)
	}
	if r.HeightPending && r.Qualify != nil && *r.Qualify != "" {
		add("%s 过程成绩不能指定晋级", at)
	}
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
	if len(r.Attempts) > 0 && !event.MultipleAttempts() {
		add("%s.attempts 不适用于该项目", at)
	}
	if event.Kind.Scalar() && len(r.Attempts) > event.AttemptCount {
		add("%s.attempts 超过设置次数", at)
	}
	if event.Kind.Scalar() && len(r.Attempts) > 0 && r.Status == "OK" {
		attempts := make([]domain.Attempt, len(r.Attempts))
		for i, a := range r.Attempts {
			attempts[i] = domain.Attempt{Outcome: a.Outcome, Mark: a.Mark, Wind: a.Wind}
		}
		mark, ok, err := scoring.Aggregate(event, attempts)
		if err != nil || !ok || r.Mark == nil || mark != *r.Mark {
			add("%s.mark 与逐次成绩不符", at)
		}
	}
	for i, a := range r.Attempts {
		switch {
		case a.Outcome == "valid" && (a.Mark == nil || *a.Mark < 0):
			add("%s.attempts[%d] 有效尝试必须有 mark", at, i)
		case a.Outcome != "valid" && a.Outcome != "foul" && a.Outcome != "pass":
			add("%s.attempts[%d].outcome 不正确", at, i)
		case a.Outcome != "valid" && a.Mark != nil:
			add("%s.attempts[%d] 非有效尝试 mark 应为 null", at, i)
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
