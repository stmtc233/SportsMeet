// Package interchange reads and writes the meet interchange file
// (docs/interchange-format.md). Field names and order follow the spec.
package interchange

import "github.com/stmtc233/sportsmeet/internal/domain"

// Format and Version identify the file (spec §2).
const (
	Format  = "sportsmeet-interchange"
	Version = 7
)

// File is one meet's full snapshot.
type File struct {
	Combineds  []Combined `json:"combineds,omitempty"`
	Format     string     `json:"format"`
	Version    int        `json:"version"`
	ExportedAt string     `json:"exportedAt"`
	Generator  Generator  `json:"generator"`
	Meet       Meet       `json:"meet"`
	Units      []Unit     `json:"units"`
	Athletes   []Athlete  `json:"athletes"`
	Events     []Event    `json:"events"`
}

type Generator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Meet struct {
	TeamScoring *domain.TeamScoringRules `json:"teamScoring,omitempty"`
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	StartDate   string                   `json:"startDate"`
	EndDate     *string                  `json:"endDate,omitempty"`
	Location    string                   `json:"location,omitempty"`
	Note        string                   `json:"note,omitempty"`
}

type Unit struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName,omitempty"`
}

type Athlete struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Gender        string  `json:"gender,omitempty"`
	Bib           string  `json:"bib,omitempty"`
	StudentNumber string  `json:"studentNumber,omitempty"`
	UnitID        *string `json:"unitId,omitempty"`
	Affiliation   string  `json:"affiliation,omitempty"`
	ClassName     string  `json:"className,omitempty"`
	Grade         string  `json:"grade,omitempty"`
	BirthDate     *string `json:"birthDate,omitempty"`
	Note          string  `json:"note,omitempty"`
}

type Event struct {
	domain.CompetitionConfig
	CombinedID    string  `json:"combinedId,omitempty"`
	StartMode     string  `json:"startMode,omitempty"`
	DistanceMode  string  `json:"distanceMode,omitempty"`
	RelayLegCount int     `json:"relayLegCount,omitempty"`
	ID            string  `json:"id"`
	Name          string  `json:"name,omitempty"`
	Gender        string  `json:"gender"`
	Discipline    string  `json:"discipline"`
	GroupLabel    string  `json:"groupLabel,omitempty"`
	Kind          string  `json:"kind"`
	Date          *string `json:"date,omitempty"`
	LaneCount     *int    `json:"laneCount,omitempty"`
	AttemptCount  *int    `json:"attemptCount,omitempty"`
	Note          string  `json:"note,omitempty"`
	Teams         []Team  `json:"teams,omitempty"`
	Entries       []Entry `json:"entries"`
	Rounds        []Round `json:"rounds"`
}

type Team struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	UnitID    *string  `json:"unitId,omitempty"`
	MemberIDs []string `json:"memberIds"`
}

type Entry struct {
	AthleteID string `json:"athleteId,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	SeedMark  *int64 `json:"seedMark,omitempty"`
}

type Round struct {
	ID          string   `json:"id"`
	Stage       string   `json:"stage"`
	Ordinal     int      `json:"ordinal"`
	Name        string   `json:"name,omitempty"`
	ScheduledAt *string  `json:"scheduledAt,omitempty"`
	Advance     *Advance `json:"advance"`
	Heats       []Heat   `json:"heats"`
}

type Advance struct {
	ByPlace int `json:"byPlace"`
	ByMark  int `json:"byMark"`
}

type Heat struct {
	ID              string  `json:"id"`
	Ordinal         int     `json:"ordinal"`
	Name            string  `json:"name,omitempty"`
	ScheduledAt     *string `json:"scheduledAt,omitempty"`
	DurationMinutes *int    `json:"durationMinutes,omitempty"`
	StartedAt       *string `json:"startedAt,omitempty"`
	Wind            *int    `json:"wind,omitempty"`
	Slots           []Slot  `json:"slots"`
}

type Slot struct {
	Position  int     `json:"position"`
	AthleteID string  `json:"athleteId,omitempty"`
	TeamID    string  `json:"teamId,omitempty"`
	Result    *Result `json:"result"`
}

type Result struct {
	HeightPending bool      `json:"heightPending,omitempty"`
	Status        string    `json:"status"`
	Mark          *int64    `json:"mark"`
	Rank          *int      `json:"rank"`
	Qualify       *string   `json:"qualify,omitempty"`
	Wind          *int      `json:"wind,omitempty"`
	Attempts      []Attempt `json:"attempts,omitempty"`
	Bars          []Bar     `json:"bars,omitempty"`
	Note          string    `json:"note,omitempty"`
}

type Attempt struct {
	Outcome string `json:"outcome"`
	Mark    *int64 `json:"mark"`
	Wind    *int   `json:"wind,omitempty"`
}

type Bar struct {
	Height int64  `json:"height"`
	Trials string `json:"trials"`
}

// Combined references the owned component events in events[].
type Combined struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Gender      string              `json:"gender"`
	GroupLabel  string              `json:"groupLabel,omitempty"`
	Note        string              `json:"note,omitempty"`
	RuleVersion int                 `json:"ruleVersion"`
	Components  []CombinedComponent `json:"components"`
	AthleteIDs  []string            `json:"athleteIds"`
}
type CombinedComponent struct {
	EventID string           `json:"eventId"`
	Ordinal int              `json:"ordinal"`
	Rule    domain.PointRule `json:"rule"`
}
