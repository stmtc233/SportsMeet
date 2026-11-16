package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func TestCreateExampleMeet(t *testing.T) {
	s, changes := newTestService(t)
	ctx := context.Background()
	id, err := s.CreateExampleMeet(ctx, ui)
	must(t, err)
	if len(*changes) != 1 || (*changes)[0] != (Change{Kind: "meets", MeetID: id}) {
		t.Fatal("expected one meet notification", *changes)
	}
	logs, err := s.ListAudit(ctx, AuditQuery{MeetID: id})
	must(t, err)
	if len(logs.Rows) != 1 || logs.Rows[0].Action != "meet.example" || logs.Rows[0].Source != ui {
		t.Fatalf("audit: %+v", logs)
	}
	overview, err := s.Overview(ctx, id)
	must(t, err)
	if overview.Athletes != 1152 || overview.Units != 12 || overview.Groups != 3 || overview.Current != "" || overview.Badges != (OverviewBadges{}) {
		t.Fatalf("example is incomplete: %+v", overview)
	}
	boards, err := s.GetTeamScoreBoards(ctx, TeamScoreFilter{MeetID: id})
	must(t, err)
	if len(boards) != 3 {
		t.Fatalf("boards: %d", len(boards))
	}
	for _, b := range boards {
		if !b.Detail.Complete || b.Detail.Counted != 27 || len(b.Detail.Rows) != 12 || len(b.Detail.Warnings) > 0 {
			t.Fatalf("incomplete group scores: %+v", b)
		}
	}
	combineds, err := s.ListCombined(ctx, id)
	must(t, err)
	for _, c := range combineds {
		d, err := s.GetCombined(ctx, c.Combined.ID)
		must(t, err)
		if !d.Complete || len(d.Rows) != 24 {
			t.Fatalf("incomplete combined: %+v", d)
		}
	}
	for _, kind := range []string{"entries", "startlist", "results", "team_scores", "checkin", "recordsheet"} {
		t.Run(kind, func(t *testing.T) {
			r, err := s.RenderReportRequest(ctx, ReportRequest{MeetID: id, Kind: kind}, RenderOptions{})
			must(t, err)
			if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "青禾") {
				t.Fatalf("report warnings: %v", r.Warnings)
			}
		})
	}
	first, err := s.ExportMeet(ctx, id)
	must(t, err)
	if first.Meet.EntryLimits == nil || *first.Meet.EntryLimits != (domain.EntryLimits{AthleteEvents: 2, UnitAthletes: 4, UnitTeams: 1}) {
		t.Fatal("example lost entry limits")
	}
	data, err := interchange.Encode(first)
	must(t, err)
	_, err = interchange.Decode(data)
	must(t, err)
	preview, err := s.PreviewImport(ctx, data, "")
	must(t, err)
	if len(preview.Errors) != 0 || len(preview.Warnings) != 0 {
		t.Fatalf("round-trip: %+v", preview)
	}
	// Repeating the action must make a fully independent copy, even after edits.
	must(t, s.UpdateMeet(ctx, ui, id, MeetInput{Name: "已修改的示例", StartDate: first.Meet.StartDate}))
	secondID, err := s.CreateExampleMeet(ctx, ui)
	must(t, err)
	second, err := s.ExportMeet(ctx, secondID)
	must(t, err)
	if secondID == id || second.Meet.Name == "已修改的示例" {
		t.Fatal("example reused an existing meet")
	}
	seen := map[string]bool{}
	collect := func(f *interchange.File, check bool) {
		ids := []string{f.Meet.ID}
		for _, a := range f.Athletes {
			ids = append(ids, a.ID)
		}
		for _, g := range f.Groups {
			ids = append(ids, g.ID)
		}
		for _, u := range f.Units {
			ids = append(ids, u.ID)
		}
		for _, c := range f.Combineds {
			ids = append(ids, c.ID)
		}
		for _, e := range f.Events {
			ids = append(ids, e.ID)
			for _, team := range e.Teams {
				ids = append(ids, team.ID)
			}
			for _, r := range e.Rounds {
				ids = append(ids, r.ID)
				for _, h := range r.Heats {
					ids = append(ids, h.ID)
				}
			}
		}
		for _, v := range ids {
			if check && seen[v] {
				t.Fatal("copies share ID", v)
			}
			seen[v] = true
		}
	}
	collect(first, false)
	collect(second, true)
	must(t, s.DeleteMeet(ctx, ui, id))
	counts, err := s.CountMeet(ctx, secondID)
	must(t, err)
	if counts.Athletes != 1152 || counts.Events != 93 {
		t.Fatalf("deleting another copy damaged this one: %+v", counts)
	}
}

func TestCanceledExampleLeavesNoData(t *testing.T) {
	s, changes := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.CreateExampleMeet(ctx, ui)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled create: %v", err)
	}
	meets, err := s.ListMeets(context.Background())
	must(t, err)
	if len(meets) != 0 || len(*changes) != 0 {
		t.Fatal("canceled creation left data")
	}
}
