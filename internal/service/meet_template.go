package service

import (
	"context"
	"sort"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/store"
)

type meetCombinedDefinition struct {
	Combined   domain.Combined            `json:"combined"`
	Components []domain.CombinedComponent `json:"components"`
}

type meetProjectDefinitions struct {
	Source      domain.Meet               `json:"source"`
	Groups      []domain.CompetitionGroup `json:"groups"`
	Events      []domain.Event            `json:"events"`
	Rounds      map[string][]domain.Round `json:"rounds"`
	Combineds   []meetCombinedDefinition  `json:"combineds"`
	TeamScoring domain.TeamScoringRules   `json:"teamScoring"`
	// Print style choice, front pages and the meet's images (named, not
	// their bytes: the hash covers their contents) come along.
	PrintStyleID string          `json:"printStyleId"`
	FrontPages   []FrontPageView `json:"frontPages"`
	Assets       []AssetInfo     `json:"assets"`
}

// MeetCreationPlan shows the new meet's fields and the source definitions to copy.
type MeetCreationPlan struct {
	Meet     *domain.Meet            `json:"meet"`
	Projects *meetProjectDefinitions `json:"projects"`
}

type meetCreationState struct {
	Projects *meetProjectDefinitions
	Meets    []MeetView // A committed creation invalidates reuse of the preview.
}

func (s *Service) meetProjects(ctx context.Context, sourceID string) (*meetProjectDefinitions, error) {
	source, err := s.store.GetMeet(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, invalid("来源赛事不存在或已删除")
	}
	groups, err := s.store.ListGroups(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, store.Scope{MeetID: sourceID})
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.ListRounds(ctx, store.Scope{MeetID: sourceID})
	if err != nil {
		return nil, err
	}
	combineds, err := s.store.ListCombined(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	rules, err := s.store.TeamScoringRules(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	// Keep the snapshot limited to definitions: participant and schedule changes
	// do not alter what this operation creates.
	liveEvents, liveCombineds := map[string]bool{}, map[string]bool{}
	for i, e := range events {
		events[i].Date = ""
		if e.CombinedID == "" {
			liveEvents[e.ID] = true
		}
	}
	for _, list := range rounds {
		for i := range list {
			list[i].ScheduledAt = ""
		}
	}
	parents := []meetCombinedDefinition{}
	for _, p := range combineds {
		parents = append(parents, meetCombinedDefinition{Combined: p.Combined, Components: p.Components})
		liveCombineds[p.Combined.ID] = true
	}
	overrides := []domain.TeamScoreOverride{}
	for _, o := range rules.Overrides {
		if liveEvents[o.EventID] || liveCombineds[o.CombinedID] {
			overrides = append(overrides, o)
		}
	}
	rules.Overrides = overrides
	prefs, err := s.MeetSettings(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	frontPages, err := s.store.ListFrontPages(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	assets, err := s.store.ListAssets(ctx, store.AssetOfMeet, sourceID, false)
	if err != nil {
		return nil, err
	}
	pages := make([]FrontPageView, len(frontPages))
	for i, p := range frontPages {
		pages[i] = frontPageView(p)
	}
	return &meetProjectDefinitions{Source: *source, Groups: groups, Events: events, Rounds: rounds, Combineds: parents, TeamScoring: rules,
		PrintStyleID: prefs.PrintStyleID, FrontPages: pages, Assets: assetInfos(assets)}, nil
}

// copyMeetProjects prepares independent definitions while the caller holds the
// service write lock. The new meet and definitions commit together; participant
// data, dates and planned times are not part of the copy.
func (s *Service) copyMeetProjects(ctx context.Context, sourceID string, meet domain.Meet) ([]sqlite.Statement, error) {
	defs, err := s.meetProjects(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	stmts := []sqlite.Statement{}
	groupIDs, eventIDs, combinedIDs := map[string]string{}, map[string]string{}, map[string]string{}
	for _, g := range defs.Groups {
		id := newID()
		groupIDs[g.ID] = id
		g.ID, g.MeetID = id, meet.ID
		stmts = append(stmts, insertGroup(g))
	}
	for _, p := range defs.Combineds {
		c := p.Combined
		id := newID()
		combinedIDs[c.ID] = id
		c.ID, c.MeetID, c.GroupID, c.RuleVersion = id, meet.ID, groupIDs[c.GroupID], 1
		stmts = append(stmts, insertCombined(c))
	}
	// ListEvents preserves creation order, including component events.
	for _, e := range defs.Events {
		oldID := e.ID
		e.ID, e.MeetID, e.GroupID, e.CombinedID = newID(), meet.ID, groupIDs[e.GroupID], combinedIDs[e.CombinedID]
		e.Date = ""
		eventIDs[oldID] = e.ID
		stmts = append(stmts, insertEvent(e, meet.CreatedAt))
		for _, r := range defs.Rounds[oldID] {
			r.ID, r.EventID, r.ScheduledAt = newID(), e.ID, ""
			stmts = append(stmts, insertRound(r))
		}
	}
	for _, p := range defs.Combineds {
		for _, c := range p.Components {
			c.CombinedID, c.EventID = combinedIDs[c.CombinedID], eventIDs[c.EventID]
			stmts = append(stmts, insertCombinedComponent(c))
		}
	}
	rules := defs.TeamScoring
	for i, o := range rules.Overrides {
		rules.Overrides[i].EventID, rules.Overrides[i].CombinedID = eventIDs[o.EventID], combinedIDs[o.CombinedID]
	}
	sort.Slice(rules.Overrides, func(i, j int) bool {
		a, b := rules.Overrides[i], rules.Overrides[j]
		if a.EventID != b.EventID {
			return a.EventID < b.EventID
		}
		return a.CombinedID < b.CombinedID
	})
	stmts = append(stmts, store.Stmt(`INSERT INTO meet_team_scoring(meet_id,rules_json) VALUES (?,?)`, meet.ID, jsonValue(rules)))
	// The print style the source uses is chosen again; the other settings
	// stay the defaults.
	if defs.PrintStyleID != "" {
		prefs := DefaultMeetSettings()
		prefs.PrintStyleID = defs.PrintStyleID
		stmts = append(stmts, store.Stmt(`UPDATE meets SET settings = ? WHERE id = ?`, jsonValue(prefs), meet.ID))
	}
	now := s.store.Now()
	sourcePages, err := s.store.ListFrontPages(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	for _, p := range sourcePages {
		p.ID, p.MeetID, p.CreatedAt, p.UpdatedAt = newID(), meet.ID, now, now
		stmts = append(stmts, store.InsertFrontPage(p))
	}
	sourceAssets, err := s.store.ListAssets(ctx, store.AssetOfMeet, sourceID, true)
	if err != nil {
		return nil, err
	}
	for _, a := range sourceAssets {
		a.ID, a.OwnerID = newID(), meet.ID
		stmts = append(stmts, store.UpsertAsset(store.AssetOfMeet, a))
	}
	return stmts, nil
}
