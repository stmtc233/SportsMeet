// Package domain holds the entities and enums of a meet, with validation
// and no I/O. Terms follow AGENTS.md §2; rules are in docs/agents/domain.md.
package domain

import (
	"regexp"
	"strconv"
	"strings"
)

// Gender of an athlete ("" = unknown) or of an event (男 / 女 / 混合).
type Gender string

const (
	GenderMale    Gender = "男"
	GenderFemale  Gender = "女"
	GenderMixed   Gender = "混合"
	GenderUnknown Gender = ""
)

// NormalizeEventGender accepts common spellings and returns the canonical
// event gender; ok is false when s is not one of them.
func NormalizeEventGender(s string) (g Gender, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "男", "男子", "男性", "m", "male", "men", "boys":
		return GenderMale, true
	case "女", "女子", "女性", "f", "female", "women", "girls":
		return GenderFemale, true
	case "混合", "mixed", "x":
		return GenderMixed, true
	}
	return "", false
}

// EventKind decides how marks are measured and ranked.
type EventKind string

const (
	KindTrack    EventKind = "track"    // timed, milliseconds, lower is better
	KindRelay    EventKind = "relay"    // timed teams
	KindDistance EventKind = "distance" // millimetres, higher is better, attempts
	KindHeight   EventKind = "height"   // millimetres, higher is better, bars
)

// Valid reports whether k is a known kind.
func (k EventKind) Valid() bool {
	switch k {
	case KindTrack, KindRelay, KindDistance, KindHeight:
		return true
	}
	return false
}

// Timed reports whether marks are times (lower is better).
func (k EventKind) Timed() bool { return k == KindTrack || k == KindRelay }

// Stage of a round.
type Stage string

const (
	StageHeat         Stage = "heat"
	StageQuarterfinal Stage = "quarterfinal"
	StageSemifinal    Stage = "semifinal"
	StageFinal        Stage = "final"
)

// Valid reports whether s is a known stage.
func (s Stage) Valid() bool {
	switch s {
	case StageHeat, StageQuarterfinal, StageSemifinal, StageFinal:
		return true
	}
	return false
}

// Label is the stage's display name.
func (s Stage) Label() string {
	switch s {
	case StageHeat:
		return "预赛"
	case StageQuarterfinal:
		return "次赛"
	case StageSemifinal:
		return "半决赛"
	case StageFinal:
		return "决赛"
	}
	return ""
}

// Status of a result.
type Status string

const (
	StatusOK  Status = "OK"
	StatusDNS Status = "DNS"
	StatusDNF Status = "DNF"
	StatusDQ  Status = "DQ"
	StatusNM  Status = "NM"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusOK, StatusDNS, StatusDNF, StatusDQ, StatusNM:
		return true
	}
	return false
}

// ComposeEventName builds 「男子100m 初中组」 from the item fields; the only
// implementation, the frontend preview calls it too.
func ComposeEventName(g Gender, discipline, group string) string {
	prefix := ""
	switch g {
	case GenderMale:
		prefix = "男子"
	case GenderFemale:
		prefix = "女子"
	case GenderMixed:
		prefix = "混合"
	}
	name := prefix + strings.TrimSpace(discipline)
	if group = strings.TrimSpace(group); group != "" {
		if name != "" {
			name += " "
		}
		name += group
	}
	return name
}

var spaceRun = regexp.MustCompile(`\s+`)

// DisciplineKey compares disciplines loosely: 100 米 = 100m, 4x100M = 4×100m.
func DisciplineKey(v string) string {
	v = strings.ToLower(spaceRun.ReplaceAllString(strings.TrimSpace(v), ""))
	return strings.NewReplacer("米", "m", "x", "×", "*", "×").Replace(v)
}

// NormalizeText trims, collapses whitespace and lower-cases ASCII, for
// identity matching (interchange-format.md §7).
func NormalizeText(v string) string {
	return strings.ToLower(spaceRun.ReplaceAllString(strings.TrimSpace(v), " "))
}

var ordinalOnly = regexp.MustCompile(`^第\s*\d+\s*组$`)

// NormalizeHeatName stores only a custom name: blank and plain 「第N组」
// become "", since the ordinal already says it.
func NormalizeHeatName(v string) string {
	v = strings.TrimSpace(v)
	if ordinalOnly.MatchString(v) {
		return ""
	}
	return v
}

// HeatLabel is the readable name of a heat: its custom name, or 「第N组」.
func HeatLabel(ordinal int, name string) string {
	if name != "" {
		return name
	}
	return "第" + strconv.Itoa(ordinal) + "组"
}

// NormalizeAthleteGender accepts common spellings; ok is false for
// anything that is neither a gender nor blank.
func NormalizeAthleteGender(s string) (Gender, bool) {
	if strings.TrimSpace(s) == "" {
		return GenderUnknown, true
	}
	g, ok := NormalizeEventGender(s)
	if !ok || g == GenderMixed {
		return "", false
	}
	return g, true
}

// EventKey identifies an item inside a meet: gender + discipline + group,
// disciplines compared loosely.
func EventKey(g Gender, discipline, group string) string {
	return string(g) + "|" + DisciplineKey(discipline) + "|" + NormalizeText(group)
}

// StageOrder sorts rounds: heats first, the final last.
func StageOrder(s Stage) int {
	switch s {
	case StageHeat:
		return 1
	case StageQuarterfinal:
		return 2
	case StageSemifinal:
		return 3
	case StageFinal:
		return 4
	}
	return 0
}

// Label is the status as shown on screen and in print; OK has none.
func (s Status) Label() string {
	switch s {
	case StatusDNS:
		return "未起跑"
	case StatusDNF:
		return "未完成"
	case StatusDQ:
		return "犯规取消"
	case StatusNM:
		return "无成绩"
	}
	return ""
}

// KindLabel is the event kind's display name.
func (k EventKind) Label() string {
	switch k {
	case KindTrack:
		return "径赛"
	case KindRelay:
		return "接力"
	case KindDistance:
		return "远度"
	case KindHeight:
		return "高度"
	}
	return ""
}
