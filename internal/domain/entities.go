package domain

// Meet is one 赛事.
type Meet struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"startDate"` // YYYY-MM-DD
	EndDate   string `json:"endDate"`   // YYYY-MM-DD, "" = same day
	Location  string `json:"location"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"createdAt"` // ms
}

// Unit is one 参赛单位.
type Unit struct {
	ID        string `json:"id"`
	MeetID    string `json:"meetId"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
}

// Athlete belongs to a meet and is shared by its events.
type Athlete struct {
	ID            string `json:"id"`
	MeetID        string `json:"meetId"`
	Name          string `json:"name"`
	Gender        Gender `json:"gender"`
	Bib           string `json:"bib"`
	StudentNumber string `json:"studentNumber"`
	UnitID        string `json:"unitId"` // "" = none
	ClassName     string `json:"className"`
	Grade         string `json:"grade"`
	BirthDate     string `json:"birthDate"` // YYYY-MM-DD or ""
	Note          string `json:"note"`
}

// Event is one single item: gender + discipline + optional group.
type Event struct {
	ID           string    `json:"id"`
	MeetID       string    `json:"meetId"`
	Name         string    `json:"name"`
	Gender       Gender    `json:"gender"`
	Discipline   string    `json:"discipline"`
	GroupLabel   string    `json:"groupLabel"`
	Kind         EventKind `json:"kind"`
	Date         string    `json:"date"` // YYYY-MM-DD, "" = the meet's start
	LaneCount    int       `json:"laneCount"`
	AttemptCount int       `json:"attemptCount"`
	Note         string    `json:"note"`
}

// Attempt is one try in a distance event.
type Attempt struct {
	Outcome string `json:"outcome"` // valid | foul | pass
	Mark    *int64 `json:"mark"`
	Wind    *int   `json:"wind"`
}

// Bar is one height in a height event; Trials is made of O, X and -.
type Bar struct {
	Height int64  `json:"height"`
	Trials string `json:"trials"`
}

// Result is the mark at a slot. Mark is ms (timed) or mm (field).
type Result struct {
	Status   Status    `json:"status"`
	Mark     *int64    `json:"mark"`
	Rank     *int      `json:"rank"`
	Qualify  string    `json:"qualify"` // Q | q | ""
	Wind     *int      `json:"wind"`    // 0.1 m/s
	Attempts []Attempt `json:"attempts"`
	Bars     []Bar     `json:"bars"`
	Note     string    `json:"note"`
}

// Entry puts an athlete into an event's pool.
type Entry struct {
	EventID   string `json:"eventId"`
	AthleteID string `json:"athleteId"`
	SeedMark  *int64 `json:"seedMark"`
}

// Advance is a round's rule for the next one: the first ByPlace of each
// heat (Q) plus the ByMark best of the rest (q).
type Advance struct {
	ByPlace int `json:"byPlace"`
	ByMark  int `json:"byMark"`
}

// Round is one 轮次 of an event with its heats.
type Round struct {
	ID      string   `json:"id"`
	EventID string   `json:"eventId"`
	Stage   Stage    `json:"stage"`
	Ordinal int      `json:"ordinal"`
	Name    string   `json:"name"`
	Advance *Advance `json:"advance"`
	Heats   []Heat   `json:"heats"`
}

// Label is the round's display name.
func (r Round) Label() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Stage.Label()
}

// Heat is one 组 with its slots, ordered by position.
type Heat struct {
	ID          string `json:"id"`
	RoundID     string `json:"roundId"`
	Ordinal     int    `json:"ordinal"`
	Name        string `json:"name"` // custom name only
	ScheduledAt string `json:"scheduledAt"`
	Wind        *int   `json:"wind"`
	Slots       []Slot `json:"slots"`
}

// Facts counts what decides whether the heat is locked.
func (h Heat) Facts() HeatFacts {
	f := HeatFacts{}
	for _, s := range h.Slots {
		if s.Result != nil {
			f.Results++
		}
	}
	return f
}

// Slot is one 位置: a lane (track) or an order (field).
type Slot struct {
	Position  int     `json:"position"`
	AthleteID string  `json:"athleteId"`
	Result    *Result `json:"result"`
}

// HeatFacts is what the lock rule looks at.
type HeatFacts struct {
	Results int
}

// KeepReason says why a heat must be kept as it is ("" = free to change):
// the only place the lock is decided, shared by arranging, deleting and
// importing (docs/agents/domain.md「组与位置」).
func (f HeatFacts) KeepReason() string {
	if f.Results > 0 {
		return "已有成绩"
	}
	return ""
}
