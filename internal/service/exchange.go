package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Version is the app version, shown in「关于」and written into exported
// files. main sets it from the packaged app (mygo.config.ts); a plain
// executable, as in tests, keeps this value.
var Version = "dev"

func ptr[T any](v T) *T { return &v }

func optStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// ExportMeet returns the meet as an interchange file (always the current
// format version).
func (s *Service) ExportMeet(ctx context.Context, meetID string) (*interchange.File, error) {
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	sc := store.Scope{MeetID: meetID}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, sc)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListEntries(ctx, sc)
	if err != nil {
		return nil, err
	}
	teams, err := s.store.ListTeams(ctx, sc)
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.LoadRounds(ctx, sc)
	if err != nil {
		return nil, err
	}

	f := &interchange.File{
		Format: interchange.Format, Version: interchange.Version,
		ExportedAt: time.UnixMilli(s.store.Now()).Format(time.RFC3339),
		Generator:  interchange.Generator{Name: "SportsMeet", Version: Version},
		Meet: interchange.Meet{ID: meet.ID, Name: meet.Name, StartDate: meet.StartDate, EndDate: optStr(meet.EndDate),
			Location: meet.Location, Note: meet.Note},
		Combineds: []interchange.Combined{}, Units: []interchange.Unit{}, Athletes: []interchange.Athlete{}, Events: []interchange.Event{},
	}
	combineds, err := s.store.ListCombined(ctx, meetID)
	if err != nil {
		return nil, err
	}
	for _, p := range combineds {
		c := p.Combined
		out := interchange.Combined{ID: c.ID, Name: c.Name, Gender: string(c.Gender), GroupLabel: c.GroupLabel, Note: c.Note, RuleVersion: c.RuleVersion, AthleteIDs: append([]string{}, p.AthleteIDs...), Components: []interchange.CombinedComponent{}}
		for _, part := range p.Components {
			out.Components = append(out.Components, interchange.CombinedComponent{EventID: part.EventID, Ordinal: part.Ordinal, Rule: part.Rule})
		}
		f.Combineds = append(f.Combineds, out)
	}
	unitNames := map[string]string{}
	for _, u := range units {
		unitNames[u.ID] = u.Name
		f.Units = append(f.Units, interchange.Unit{ID: u.ID, Name: u.Name, ShortName: u.ShortName})
	}
	for _, a := range athletes {
		f.Athletes = append(f.Athletes, interchange.Athlete{ID: a.ID, Name: a.Name, Gender: string(a.Gender), Bib: a.Bib,
			StudentNumber: a.StudentNumber, UnitID: optStr(a.UnitID), Affiliation: unitNames[a.UnitID],
			ClassName: a.ClassName, Grade: a.Grade, BirthDate: optStr(a.BirthDate), Note: a.Note})
	}
	byEvent := map[string][]interchange.Entry{}
	for _, e := range entries {
		byEvent[e.EventID] = append(byEvent[e.EventID], interchange.Entry{AthleteID: e.AthleteID, TeamID: e.TeamID, SeedMark: e.SeedMark})
	}
	byTeams := map[string][]interchange.Team{}
	for _, t := range teams {
		byTeams[t.EventID] = append(byTeams[t.EventID], interchange.Team{ID: t.ID, Name: t.Name, UnitID: optStr(t.UnitID), MemberIDs: t.MemberIDs})
	}
	for _, e := range events {
		if err := e.NormalizeConfig(); err != nil {
			return nil, err
		}
		ev := interchange.Event{CompetitionConfig: e.CompetitionConfig, CombinedID: e.CombinedID, ID: e.ID, Name: e.Name, Gender: string(e.Gender), Discipline: e.Discipline,
			GroupLabel: e.GroupLabel, Kind: string(e.Kind), StartMode: string(e.StartMode), DistanceMode: string(e.DistanceMode), RelayLegCount: e.RelayLegCount, Date: optStr(e.Date), Note: e.Note,
			Teams: byTeams[e.ID], Entries: byEvent[e.ID], Rounds: []interchange.Round{}}
		if e.Kind.Timed() {
			ev.LaneCount = ptr(e.LaneCount)
		}
		if e.Kind == domain.KindDistance || e.Kind.Scalar() {
			ev.AttemptCount = ptr(e.AttemptCount)
		}
		if ev.Entries == nil {
			ev.Entries = []interchange.Entry{}
		}
		for _, r := range rounds[e.ID] {
			round := interchange.Round{ID: r.ID, Stage: string(r.Stage), Ordinal: r.Ordinal, Name: r.Name,
				ScheduledAt: optStr(r.ScheduledAt), Heats: []interchange.Heat{}}
			if r.Advance != nil {
				round.Advance = &interchange.Advance{ByPlace: r.Advance.ByPlace, ByMark: r.Advance.ByMark}
			}
			for _, h := range r.Heats {
				heat := interchange.Heat{ID: h.ID, Ordinal: h.Ordinal, Name: h.Name, ScheduledAt: optStr(h.ScheduledAt),
					DurationMinutes: h.DurationMinutes, StartedAt: optStr(h.StartedAt), Wind: h.Wind, Slots: []interchange.Slot{}}
				for _, sl := range h.Slots {
					if sl.CompetitorID() == "" {
						continue
					}
					slot := interchange.Slot{Position: sl.Position, AthleteID: sl.AthleteID, TeamID: sl.TeamID}
					if r := sl.Result; r != nil {
						res := &interchange.Result{Status: string(r.Status), Mark: r.Mark, Rank: r.Rank, Qualify: optStr(r.Qualify),
							Wind: r.Wind, Note: r.Note, HeightPending: r.HeightPending}
						for _, a := range r.Attempts {
							res.Attempts = append(res.Attempts, interchange.Attempt{Outcome: a.Outcome, Mark: a.Mark, Wind: a.Wind})
						}
						for _, b := range r.Bars {
							res.Bars = append(res.Bars, interchange.Bar{Height: b.Height, Trials: b.Trials})
						}
						slot.Result = res
					}
					heat.Slots = append(heat.Slots, slot)
				}
				round.Heats = append(round.Heats, heat)
			}
			ev.Rounds = append(ev.Rounds, round)
		}
		f.Events = append(f.Events, ev)
	}
	return f, nil
}

// Tally counts new and reused entities of one kind.
type Tally struct {
	New    int `json:"new"`
	Reused int `json:"reused"`
}

// ImportPreview says what importing a file would do. Hash identifies it:
// the import must present the same one, so what is written is what was
// shown.
type ImportPreview struct {
	Combineds          Tally          `json:"combineds"`
	MeetName           string         `json:"meetName"`
	NewMeet            bool           `json:"newMeet"`
	Teams              Tally          `json:"teams"`
	Units              Tally          `json:"units"`
	Athletes           Tally          `json:"athletes"`
	Events             Tally          `json:"events"`
	Entries            int            `json:"entries"`
	Heats              int            `json:"heats"`
	ResultsNew         int            `json:"resultsNew"`
	ResultsOverwritten int            `json:"resultsOverwritten"`
	Overwrites         []ResultChange `json:"overwrites"`
	Warnings           []string       `json:"warnings"`
	Errors             []string       `json:"errors"`
	Hash               string         `json:"hash"`
	MeetID             string         `json:"meetId"` // set once imported
}

type importPlan struct {
	preview ImportPreview
	meetID  string
	stmts   []sqlite.Statement
}

func resultFromFile(r *interchange.Result) domain.Result {
	out := domain.Result{Status: domain.Status(r.Status), Mark: r.Mark, Rank: r.Rank, Wind: r.Wind, Note: r.Note, HeightPending: r.HeightPending}
	if r.Qualify != nil {
		out.Qualify = *r.Qualify
	}
	for _, a := range r.Attempts {
		out.Attempts = append(out.Attempts, domain.Attempt{Outcome: a.Outcome, Mark: a.Mark, Wind: a.Wind})
	}
	for _, b := range r.Bars {
		out.Bars = append(out.Bars, domain.Bar{Height: b.Height, Trials: b.Trials})
	}
	return out
}

// planImport decodes data and works out the import into meetID ("" = a
// new meet). File problems land in preview.Errors, not in the error.
func (s *Service) planImport(ctx context.Context, data []byte, meetID string) (*importPlan, error) {
	var snapshot *interchange.File
	if meetID != "" {
		var err error
		snapshot, err = s.ExportMeet(ctx, meetID)
		if err != nil {
			return nil, err
		}
		snapshot.ExportedAt = ""
	}
	plan := &importPlan{meetID: meetID, preview: ImportPreview{Overwrites: []ResultChange{}, Warnings: []string{}, Errors: []string{}}}
	p := &plan.preview
	finish := func() (*importPlan, error) {
		hashed := *p
		hashed.Hash, hashed.MeetID = "", ""
		b, _ := json.Marshal(struct {
			Preview ImportPreview
			Data    string
			Target  *interchange.File
		}{hashed, string(data), snapshot})
		sum := sha256.Sum256(b)
		p.Hash = hex.EncodeToString(sum[:])
		return plan, nil
	}
	f, err := interchange.Decode(data)
	var problems interchange.Problems
	switch {
	case errors.As(err, &problems):
		p.Errors = problems
		return finish()
	case err != nil:
		p.Errors = []string{err.Error()}
		return finish()
	}
	now := s.store.Now()
	add := func(sql string, args ...any) { plan.stmts = append(plan.stmts, store.Stmt(sql, args...)) }

	var ex interchange.Existing
	existingRounds := map[string][]domain.Round{}
	existingTeams := []domain.Team{}
	existingCombineds := []store.CombinedData{}
	entered := map[string]bool{}
	p.MeetName = f.Meet.Name
	if meetID == "" {
		p.NewMeet = true
		plan.meetID = newID()
		m := domain.Meet{ID: plan.meetID, Name: strings.TrimSpace(f.Meet.Name), StartDate: f.Meet.StartDate,
			Location: f.Meet.Location, Note: f.Meet.Note, CreatedAt: now}
		if f.Meet.EndDate != nil {
			m.EndDate = *f.Meet.EndDate
		}
		plan.stmts = append(plan.stmts, store.InsertMeet(m))
	} else {
		meet, err := s.GetMeet(ctx, meetID)
		if err != nil {
			return nil, err
		}
		p.MeetName = meet.Name
		sc := store.Scope{MeetID: meetID}
		if ex.Units, err = s.store.ListUnits(ctx, meetID); err != nil {
			return nil, err
		}
		if ex.Athletes, err = s.store.ListAthletes(ctx, meetID); err != nil {
			return nil, err
		}
		if existingCombineds, err = s.store.ListCombined(ctx, meetID); err != nil {
			return nil, err
		}
		for _, c := range existingCombineds {
			ex.Combineds = append(ex.Combineds, c.Combined)
		}
		if ex.Events, err = s.store.ListEvents(ctx, sc); err != nil {
			return nil, err
		}
		entries, err := s.store.ListEntries(ctx, sc)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			entered[e.EventID+"|"+e.CompetitorID()] = true
		}
		if existingTeams, err = s.store.ListTeams(ctx, sc); err != nil {
			return nil, err
		}
		if existingRounds, err = s.store.LoadRounds(ctx, sc); err != nil {
			return nil, err
		}
	}
	meetID = plan.meetID
	match := interchange.MatchExisting(f, ex)
	if len(match.Conflicts) > 0 {
		p.Errors = match.Conflicts
		return finish()
	}

	// Units, then athletes; an affiliation without a unit becomes one.
	unitByName := map[string]string{}
	for _, u := range ex.Units {
		unitByName[domain.NormalizeText(u.Name)] = u.ID
	}
	unitID := func(name, short string) string {
		key := domain.NormalizeText(name)
		if key == "" {
			return ""
		}
		if id, ok := unitByName[key]; ok {
			return id
		}
		id := newID()
		unitByName[key] = id
		p.Units.New++
		add(`INSERT INTO units(id, meet_id, name, short_name) VALUES (?, ?, ?, ?)`, id, meetID, strings.TrimSpace(name), short)
		return id
	}
	units := map[string]string{}
	for _, u := range f.Units {
		if id, ok := match.Units[u.ID]; ok {
			units[u.ID] = id
			p.Units.Reused++
			continue
		}
		units[u.ID] = unitID(u.Name, u.ShortName)
	}
	athletes := map[string]string{}
	names := map[string]string{}
	genders := map[string]string{}
	for _, a := range f.Athletes {
		names[a.ID], genders[a.ID] = a.Name, a.Gender
		if id, ok := match.Athletes[a.ID]; ok {
			athletes[a.ID] = id
			p.Athletes.Reused++
			continue
		}
		n := domain.Athlete{ID: newID(), MeetID: meetID, Name: strings.TrimSpace(a.Name), Gender: domain.Gender(a.Gender),
			Bib: strings.TrimSpace(a.Bib), StudentNumber: strings.TrimSpace(a.StudentNumber),
			ClassName: a.ClassName, Grade: a.Grade, Note: a.Note}
		if a.UnitID != nil {
			n.UnitID = units[*a.UnitID]
		} else {
			n.UnitID = unitID(a.Affiliation, "")
		}
		if a.BirthDate != nil {
			n.BirthDate = *a.BirthDate
		}
		athletes[a.ID] = n.ID
		p.Athletes.New++
		plan.stmts = append(plan.stmts, insertAthlete(n, now))
	}
	// A new athlete must not take the student number or bib of an athlete
	// the file did not match (it matched someone else by an earlier field).
	taken := map[string]string{}
	for _, a := range ex.Athletes {
		if k := domain.NormalizeText(a.StudentNumber); k != "" {
			taken["n|"+k] = a.ID
		}
		if k := domain.NormalizeText(a.Bib); k != "" {
			taken["b|"+k] = a.ID
		}
	}
	for _, a := range f.Athletes {
		for _, k := range []string{"n|" + domain.NormalizeText(a.StudentNumber), "b|" + domain.NormalizeText(a.Bib)} {
			if owner, ok := taken[k]; ok && len(k) > 2 && owner != athletes[a.ID] {
				p.Errors = append(p.Errors, fmt.Sprintf("运动员「%s」的学号或号码已属于赛事里的另一人", a.Name))
			}
		}
	}

	combinedIDs := map[string]string{}
	eventIDs := map[string]string{}
	for _, c := range f.Combineds {
		if id, ok := match.Combineds[c.ID]; ok {
			combinedIDs[c.ID] = id
			p.Combineds.Reused++
		} else {
			id := newID()
			combinedIDs[c.ID] = id
			p.Combineds.New++
			plan.stmts = append(plan.stmts, insertCombined(domain.Combined{ID: id, MeetID: meetID, Name: c.Name, Gender: domain.Gender(c.Gender), GroupLabel: c.GroupLabel, Note: c.Note, RuleVersion: c.RuleVersion}))
		}
	}
	entries := map[string]bool{}
	for _, e := range f.Events {
		kind := domain.EventKind(e.Kind)
		gender := domain.Gender(e.Gender)
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = domain.ComposeEventName(gender, e.Discipline, e.GroupLabel)
		}
		config := domain.Event{CompetitionConfig: e.CompetitionConfig, Kind: kind, StartMode: domain.StartMode(e.StartMode), DistanceMode: domain.DistanceMode(e.DistanceMode), RelayLegCount: e.RelayLegCount, LaneCount: 8, AttemptCount: 3}
		if kind.Scalar() {
			config.AttemptCount = 1
		}
		if e.LaneCount != nil {
			config.LaneCount = *e.LaneCount
		}
		if e.AttemptCount != nil {
			config.AttemptCount = *e.AttemptCount
		}
		if err := config.NormalizeConfig(); err != nil {
			p.Errors = append(p.Errors, err.Error())
			return finish()
		}
		eventID, reused := match.Events[e.ID]
		if reused {
			for _, old := range ex.Events {
				if old.ID == eventID {
					if err := old.NormalizeConfig(); err != nil {
						p.Errors = append(p.Errors, err.Error())
						return finish()
					}
					if f.Version >= 3 && !reflect.DeepEqual(old.CompetitionConfig, config.CompetitionConfig) {
						p.Errors = append(p.Errors, "项目计分或参赛配置与文件不符")
						return finish()
					}
					if f.Version >= 2 && (old.StartMode != config.StartMode || old.DistanceMode != config.DistanceMode || old.RelayLegCount != config.RelayLegCount || (kind.Timed() && old.LaneCount != config.LaneCount) || ((kind == domain.KindDistance || kind.Scalar()) && old.AttemptCount != config.AttemptCount)) {
						p.Errors = append(p.Errors, "项目配置与文件不符")
						return finish()
					}
				}
				if old.ID == eventID && old.Kind != kind {
					p.Errors = append(p.Errors, "项目类型与文件不符")
					return finish()
				}
			}
			p.Events.Reused++
		} else {
			eventID = newID()
			p.Events.New++
			n := domain.Event{CompetitionConfig: config.CompetitionConfig, CombinedID: combinedIDs[e.CombinedID], ID: eventID, MeetID: meetID, Name: name, Gender: gender, Discipline: strings.TrimSpace(e.Discipline),
				GroupLabel: strings.TrimSpace(e.GroupLabel), Kind: kind, LaneCount: config.LaneCount, AttemptCount: config.AttemptCount, Note: e.Note, StartMode: config.StartMode, DistanceMode: config.DistanceMode, RelayLegCount: config.RelayLegCount}
			if e.Date != nil {
				n.Date = *e.Date
			}
			if e.LaneCount != nil && *e.LaneCount > 0 {
				n.LaneCount = *e.LaneCount
			}
			if e.AttemptCount != nil && *e.AttemptCount > 0 {
				n.AttemptCount = *e.AttemptCount
			}
			plan.stmts = append(plan.stmts, insertEvent(n, now))
		}
		eventIDs[e.ID] = eventID
		teams := map[string]string{}
		teamNames := map[string]string{}
		mappedMembers := map[string]bool{}
		for _, t := range e.Teams {
			members := make([]string, 0, len(t.MemberIDs))
			for _, m := range t.MemberIDs {
				if mappedMembers[athletes[m]] {
					p.Errors = append(p.Errors, "队伍成员匹配到同一运动员")
					return finish()
				}
				mappedMembers[athletes[m]] = true
				members = append(members, athletes[m])
				if g := genders[m]; g != "" && gender != domain.GenderMixed && g != string(gender) {
					p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」的性别与「%s」不符", names[m], name))
				}
			}
			unit := ""
			if t.UnitID != nil {
				unit = units[*t.UnitID]
			}
			var found *domain.Team
			for _, old := range existingTeams {
				if old.EventID == eventID && old.UnitID == unit && domain.NormalizeText(old.Name) == domain.NormalizeText(t.Name) {
					if found != nil {
						p.Errors = append(p.Errors, "队伍匹配不唯一")
						return finish()
					}
					found = &old
				}
			}
			if found != nil {
				if !slices.Equal(found.MemberIDs, members) {
					p.Errors = append(p.Errors, fmt.Sprintf("队伍「%s」名单与文件不符", t.Name))
					return finish()
				}
				teams[t.ID] = found.ID
				p.Teams.Reused++
			} else {
				// A member can represent only one team in the same event.
				for _, old := range existingTeams {
					if old.EventID == eventID {
						for _, m := range members {
							if slices.Contains(old.MemberIDs, m) {
								p.Errors = append(p.Errors, fmt.Sprintf("队伍「%s」的成员已在其他队伍", t.Name))
								return finish()
							}
						}
					}
				}
				nt := domain.Team{ID: newID(), EventID: eventID, Name: strings.TrimSpace(t.Name), UnitID: unit, MemberIDs: members}
				teams[t.ID] = nt.ID
				p.Teams.New++
				plan.stmts = append(plan.stmts, insertTeam(nt)...)
				existingTeams = append(existingTeams, nt)
			}
			teamNames[teams[t.ID]] = t.Name
		}
		for _, en := range e.Entries {
			id := athletes[en.AthleteID]
			if config.IsTeam() {
				id = teams[en.TeamID]
			}
			if g := genders[en.AthleteID]; g != "" && gender != domain.GenderMixed && g != string(gender) {
				p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」的性别与「%s」不符", names[en.AthleteID], name))
			}
			if k := eventID + "|" + id; !entered[k] && !entries[k] {
				entries[k] = true
				p.Entries++
				plan.stmts = append(plan.stmts, insertEntry(eventID, athletes[en.AthleteID], teams[en.TeamID], optArg(en.SeedMark)))
			}
		}

		// Rounds match by stage, heats by ordinal.
		have := map[domain.Stage]*domain.Round{}
		stages := map[domain.Stage]string{} // every round of the event after the import
		for i := range existingRounds[eventID] {
			r := &existingRounds[eventID][i]
			have[r.Stage], stages[r.Stage] = r, r.ID
		}
		inFile := map[domain.Stage]bool{}
		for _, r := range e.Rounds {
			stage := domain.Stage(r.Stage)
			if inFile[stage] {
				p.Errors = append(p.Errors, fmt.Sprintf("「%s」有两个%s轮次", name, stage.Label()))
				continue
			}
			inFile[stage] = true
			old := have[stage]
			roundID := newID()
			if old != nil {
				roundID = old.ID
			} else {
				n := domain.Round{ID: roundID, EventID: eventID, Stage: stage, Name: r.Name}
				if r.ScheduledAt != nil {
					n.ScheduledAt = *r.ScheduledAt
				}
				if r.Advance != nil {
					n.Advance = &domain.Advance{ByPlace: r.Advance.ByPlace, ByMark: r.Advance.ByMark}
				}
				stages[stage] = roundID
				plan.stmts = append(plan.stmts, insertRound(n))
			}
			placedElsewhere := map[string]int{} // athlete → ordinal of the heat holding them
			oldHeats := map[int]*domain.Heat{}
			if old != nil {
				for i := range old.Heats {
					h := &old.Heats[i]
					oldHeats[h.Ordinal] = h
					for _, sl := range h.Slots {
						placedElsewhere[sl.CompetitorID()] = h.Ordinal
					}
				}
			}
			for _, h := range r.Heats {
				label := name + " " + stage.Label() + " " + domain.HeatLabel(h.Ordinal, domain.NormalizeHeatName(h.Name))
				slots := make([]domain.Slot, 0, len(h.Slots))
				for _, sl := range h.Slots {
					slot := domain.Slot{Position: sl.Position, AthleteID: athletes[sl.AthleteID], TeamID: teams[sl.TeamID]}
					if sl.Result != nil {
						slot.Result = ptr(resultFromFile(sl.Result))
					}
					slots = append(slots, slot)
				}
				sort.Slice(slots, func(i, j int) bool { return slots[i].Position < slots[j].Position })

				if oh := oldHeats[h.Ordinal]; oh != nil {
					if !sameSlots(oh.Slots, slots) {
						p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」的编排与文件不同，未导入", label))
						continue
					}
					for i, sl := range slots {
						if sl.Result == nil {
							continue
						}
						if was := oh.Slots[i].Result; was == nil {
							p.ResultsNew++
						} else if a, b := resultTextEvent(config, was), resultTextEvent(config, sl.Result); a != b {
							p.ResultsOverwritten++
							p.Overwrites = append(p.Overwrites, ResultChange{Athlete: label + " " + athleteName(ex.Athletes, sl.CompetitorID()) + teamNames[sl.TeamID], Before: a, After: b})
						} else {
							continue
						}
						plan.stmts = append(plan.stmts, upsertResult(oh.ID, sl.Position, *sl.Result, now))
					}
					continue
				}
				clash := false
				for _, sl := range slots {
					if _, ok := placedElsewhere[sl.CompetitorID()]; ok {
						clash = true
					}
				}
				if clash {
					p.Warnings = append(p.Warnings, fmt.Sprintf("「%s」里有人已排在本轮次其他组，未导入", label))
					continue
				}
				heatID := newID()
				p.Heats++
				sched, started := "", ""
				if h.ScheduledAt != nil {
					sched = *h.ScheduledAt
				}
				if h.StartedAt != nil {
					started = *h.StartedAt
				}
				add(`INSERT INTO heats(id, round_id, ordinal, name, scheduled_at, duration_minutes, started_at, wind) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					heatID, roundID, h.Ordinal, domain.NormalizeHeatName(h.Name), sched, optArg(h.DurationMinutes), started, optArg(h.Wind))
				for _, sl := range slots {
					placedElsewhere[sl.CompetitorID()] = h.Ordinal
					plan.stmts = append(plan.stmts, insertSlot(heatID, sl))
					if sl.Result != nil {
						p.ResultsNew++
						plan.stmts = append(plan.stmts, upsertResult(heatID, sl.Position, *sl.Result, now))
					}
				}
			}
		}
		// Every event has a final, and rounds are numbered by stage.
		if _, ok := stages[domain.StageFinal]; !ok && len(stages) == 0 {
			id := newID()
			stages[domain.StageFinal] = id
			plan.stmts = append(plan.stmts, insertRound(domain.Round{ID: id, EventID: eventID, Stage: domain.StageFinal, Ordinal: 1}))
		}
		order := make([]domain.Stage, 0, len(stages))
		for st := range stages {
			order = append(order, st)
		}
		sort.Slice(order, func(i, j int) bool { return domain.StageOrder(order[i]) < domain.StageOrder(order[j]) })
		for i, st := range order {
			add(`UPDATE rounds SET ordinal = ? WHERE id = ?`, i+1, stages[st])
		}
	}
	for _, c := range f.Combineds {
		id := combinedIDs[c.ID]
		var existing *store.CombinedData
		for i := range existingCombineds {
			if existingCombineds[i].Combined.ID == id {
				existing = &existingCombineds[i]
			}
		}
		if existing != nil {
			if len(existing.Components) != len(c.Components) {
				p.Errors = append(p.Errors, "全能分项与文件不符")
				return finish()
			}
			for i, part := range c.Components {
				was := existing.Components[i]
				if was.EventID != eventIDs[part.EventID] || !reflect.DeepEqual(was.Rule, part.Rule) {
					p.Errors = append(p.Errors, "全能计分规则与文件不符")
					return finish()
				}
			}
		} else {
			for _, part := range c.Components {
				plan.stmts = append(plan.stmts, insertCombinedComponent(domain.CombinedComponent{CombinedID: id, EventID: eventIDs[part.EventID], Ordinal: part.Ordinal, Rule: part.Rule}))
			}
		}
		have := map[string]bool{}
		if existing != nil {
			for _, a := range existing.AthleteIDs {
				have[a] = true
			}
		}
		for _, a := range c.AthleteIDs {
			if !have[athletes[a]] {
				plan.stmts = append(plan.stmts, store.Stmt(`INSERT INTO combined_entries(combined_id,athlete_id) VALUES (?,?)`, id, athletes[a]))
				have[athletes[a]] = true
			}
		}
	}
	return finish()
}

func athleteName(list []domain.Athlete, id string) string {
	for _, a := range list {
		if a.ID == id {
			return a.Name
		}
	}
	return ""
}

// PreviewImport reports what importing an interchange file into meetID
// ("" = as a new meet) would do. It writes nothing.
func (s *Service) PreviewImport(ctx context.Context, data []byte, meetID string) (*ImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planImport(ctx, data, meetID)
	if err != nil {
		return nil, err
	}
	return &plan.preview, nil
}

// ImportMeet imports the file in one transaction. hash is the preview's:
// when the data or the file changed since, the import is refused with
// ErrStale and must be previewed again.
func (s *Service) ImportMeet(ctx context.Context, src store.Source, data []byte, meetID, hash string) (*ImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planImport(ctx, data, meetID)
	if err != nil {
		return nil, err
	}
	p := &plan.preview
	if len(p.Errors) > 0 {
		return nil, invalid("文件有误，无法导入")
	}
	if p.Hash != hash {
		return nil, ErrStale
	}
	p.MeetID = plan.meetID
	if len(plan.stmts) == 0 {
		return p, nil
	}
	err = s.write(ctx,
		store.Audit{Source: src, Action: "meet.imported", MeetID: plan.meetID, Message: p.MeetName, Detail: p},
		Change{Kind: "meets", MeetID: plan.meetID}, plan.stmts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}
