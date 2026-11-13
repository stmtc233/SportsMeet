package service

import (
	"context"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// AthleteInput creates or updates an athlete.
type AthleteInput struct {
	Name          string `json:"name"`
	Gender        string `json:"gender,omitempty"`
	Bib           string `json:"bib,omitempty"`
	StudentNumber string `json:"studentNumber,omitempty"`
	UnitID        string `json:"unitId,omitempty"`
	ClassName     string `json:"className,omitempty"`
	Grade         string `json:"grade,omitempty"`
	BirthDate     string `json:"birthDate,omitempty"`
	Note          string `json:"note,omitempty"`
}

// AthleteRow is an athlete in the meet's list.
type AthleteRow struct {
	TeamMember bool           `json:"teamMember"`
	Athlete    domain.Athlete `json:"athlete"`
	UnitName   string         `json:"unitName"`
	Entries    int            `json:"entries"` // events entered
	Placed     bool           `json:"placed"`  // occupies a slot somewhere
}

// ListAthletes returns a meet's athletes with their unit and usage.
func (s *Service) ListAthletes(ctx context.Context, meetID string) ([]AthleteRow, error) {
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return nil, err
	}
	units, err := s.store.ListUnits(ctx, meetID)
	if err != nil {
		return nil, err
	}
	entries, slots, err := s.store.AthleteUsage(ctx, meetID)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, u := range units {
		names[u.ID] = u.Name
	}
	teams, err := s.store.ListTeams(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return nil, err
	}
	members := map[string]bool{}
	for _, t := range teams {
		for _, id := range t.MemberIDs {
			members[id] = true
		}
	}
	out := make([]AthleteRow, len(athletes))
	for i, a := range athletes {
		out[i] = AthleteRow{TeamMember: members[a.ID], Athlete: a, UnitName: names[a.UnitID], Entries: entries[a.ID], Placed: slots[a.ID] > 0}
	}
	return out, nil
}

func (in AthleteInput) athlete() (domain.Athlete, error) {
	a := domain.Athlete{
		Name:          strings.TrimSpace(in.Name),
		Bib:           strings.TrimSpace(in.Bib),
		StudentNumber: strings.TrimSpace(in.StudentNumber),
		UnitID:        in.UnitID,
		ClassName:     strings.TrimSpace(in.ClassName),
		Grade:         strings.TrimSpace(in.Grade),
		BirthDate:     strings.TrimSpace(in.BirthDate),
		Note:          strings.TrimSpace(in.Note),
	}
	g, ok := domain.NormalizeAthleteGender(in.Gender)
	switch {
	case a.Name == "":
		return a, invalid("请填写姓名")
	case !ok:
		return a, invalid("性别只能为男或女")
	case a.BirthDate != "" && !domain.ValidDate(a.BirthDate):
		return a, invalid("出生日期格式不正确")
	}
	a.Gender = g
	return a, nil
}

// identityClash reports the athlete (other than a) holding a's student
// number or bib: both are unique in a meet when not blank.
func identityClash(a domain.Athlete, others []domain.Athlete) error {
	num, bib := domain.NormalizeText(a.StudentNumber), domain.NormalizeText(a.Bib)
	for _, o := range others {
		if o.ID == a.ID {
			continue
		}
		if num != "" && domain.NormalizeText(o.StudentNumber) == num {
			return invalid("学号 %s 已属于「%s」，请改为更新", a.StudentNumber, o.Name)
		}
		if bib != "" && domain.NormalizeText(o.Bib) == bib {
			return invalid("号码 %s 已属于「%s」，请改为更新", a.Bib, o.Name)
		}
	}
	return nil
}

func insertAthlete(a domain.Athlete, now int64) sqlite.Statement {
	return store.Stmt(`INSERT INTO athletes(id, meet_id, name, gender, bib, student_number, unit_id, class_name,
		grade, birth_date, note, created_at, bib_key, student_number_key, name_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.MeetID, a.Name, string(a.Gender), a.Bib, a.StudentNumber, nullable(a.UnitID), a.ClassName,
		a.Grade, a.BirthDate, a.Note, now, domain.NormalizeText(a.Bib), domain.NormalizeText(a.StudentNumber), domain.NormalizeText(a.Name))
}

func updateAthlete(a domain.Athlete) sqlite.Statement {
	return store.Stmt(`UPDATE athletes SET name = ?, gender = ?, bib = ?, student_number = ?, unit_id = ?,
		class_name = ?, grade = ?, birth_date = ?, note = ?, bib_key=?, student_number_key=?, name_key=? WHERE id = ?`,
		a.Name, string(a.Gender), a.Bib, a.StudentNumber, nullable(a.UnitID), a.ClassName, a.Grade, a.BirthDate, a.Note,
		domain.NormalizeText(a.Bib), domain.NormalizeText(a.StudentNumber), domain.NormalizeText(a.Name), a.ID)
}

// SaveAthlete creates an athlete (id "") or updates one.
func (s *Service) SaveAthlete(ctx context.Context, src store.Source, meetID, id string, in AthleteInput) (*domain.Athlete, error) {
	a, err := in.athlete()
	if err != nil {
		return nil, err
	}
	a.ID, a.MeetID = id, meetID
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	others, err := s.store.AthleteIdentityMatches(ctx, a)
	if err != nil {
		return nil, err
	}
	if id != "" {
		exists, err := s.store.AthleteExists(ctx, meetID, id)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, invalid("运动员不存在或已删除")
		}
	}
	if a.UnitID != "" {
		units, err := s.store.ListUnits(ctx, meetID)
		if err != nil {
			return nil, err
		}
		ok := false
		for _, u := range units {
			ok = ok || u.ID == a.UnitID
		}
		if !ok {
			return nil, invalid("参赛单位不存在或已删除")
		}
	}
	if err := identityClash(a, others); err != nil {
		return nil, err
	}
	audit := store.Audit{Source: src, Action: "athlete.updated", MeetID: meetID, Message: a.Name}
	stmt := updateAthlete(a)
	if id == "" {
		a.ID = newID()
		audit.Action = "athlete.created"
		stmt = insertAthlete(a, s.store.Now())
	}
	if err := s.write(ctx, audit, Change{Kind: "athletes", MeetID: meetID}, stmt); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteAthletes removes athletes and their entries. Anyone placed in a
// heat is refused: the slot would be left empty with its result.
func (s *Service) DeleteAthletes(ctx context.Context, src store.Source, meetID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	athletes, err := s.store.ListAthletes(ctx, meetID)
	if err != nil {
		return err
	}
	_, slots, err := s.store.AthleteUsage(ctx, meetID)
	if err != nil {
		return err
	}
	byID := map[string]domain.Athlete{}
	for _, a := range athletes {
		byID[a.ID] = a
	}
	teams, err := s.store.ListTeams(ctx, store.Scope{MeetID: meetID})
	if err != nil {
		return err
	}
	for _, t := range teams {
		for _, member := range t.MemberIDs {
			for _, id := range ids {
				if member == id {
					event, err := s.store.ListEvents(ctx, store.Scope{EventID: t.EventID})
					if err != nil {
						return err
					}
					if len(event) > 0 && event[0].Kind == domain.KindRelay {
						return invalid("运动员已在接力队，请先移出队伍")
					}
					return invalid("运动员已在队伍，请先移出")
				}
			}
		}
	}

	var stmts []sqlite.Statement
	var names []string
	for _, id := range ids {
		a, ok := byID[id]
		if !ok {
			continue
		}
		if slots[id] > 0 {
			return invalid("「%s」已排进组，请先移出", a.Name)
		}
		names = append(names, a.Name)
		stmts = append(stmts, store.Stmt(`DELETE FROM athletes WHERE id = ?`, id))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx,
		store.Audit{Source: src, Action: "athlete.deleted", MeetID: meetID, Message: summarize(names), Detail: names},
		Change{Kind: "athletes", MeetID: meetID}, stmts...)
}

// summarize names a few items and counts the rest, for log messages.
func summarize(names []string) string {
	if len(names) <= 3 {
		return strings.Join(names, "、")
	}
	return strings.Join(names[:3], "、") + " 等 " + itoa(len(names)) + " 人"
}

// AthleteSave includes advisory name collisions without weakening bib/student-number uniqueness.
type AthleteSave struct {
	Athlete  *domain.Athlete `json:"athlete"`
	Warnings []string        `json:"warnings"`
}

// SaveAthleteWithWarnings shares the normal save validation with an advisory duplicate-name check.
func (s *Service) SaveAthleteWithWarnings(ctx context.Context, src store.Source, meetID, id string, in AthleteInput) (*AthleteSave, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := &preparedWrite{}
	planner := &Service{store: s.store, pending: pending}
	athlete, err := planner.SaveAthlete(ctx, src, meetID, id, in)
	if err != nil {
		return nil, err
	}
	duplicate, err := s.store.AthleteNameExists(ctx, meetID, athlete.ID, athlete.Name)
	if err != nil {
		return nil, err
	}
	out := &AthleteSave{Athlete: athlete, Warnings: []string{}}
	if duplicate {
		out.Warnings = append(out.Warnings, "赛事中有同名运动员")
	}
	if err := s.write(ctx, pending.audit, pending.change, pending.stmts...); err != nil {
		return nil, err
	}
	return out, nil
}
