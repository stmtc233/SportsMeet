package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// A meet's front pages (前置页) are Markdown pages printed at the start of
// its reports, after the style's cover: 致辞, 规程, 须知. They belong to the
// meet, not to a style, so changing the style keeps them. Their Markdown
// is stored and turned into HTML when a report is rendered.
const (
	maxFrontPages     = 20
	maxFrontPageTitle = 60
)

// FrontPageInput is a front page's text and the reports it goes in.
type FrontPageInput struct {
	Title  string `json:"title"`
	BodyMD string `json:"bodyMd"`
	// Reports are report kinds (printstyle.ReportKinds: entries, startlist,
	// results, team_scores, checkin, recordsheet); empty = every report.
	Reports []string `json:"reports"`
}

// FrontPageView is a stored front page; Position counts from 1.
type FrontPageView struct {
	ID        string   `json:"id"`
	MeetID    string   `json:"meetId"`
	Position  int      `json:"position"`
	Title     string   `json:"title"`
	BodyMD    string   `json:"bodyMd"`
	Reports   []string `json:"reports"`
	UpdatedAt int64    `json:"updatedAt"`
}

// FrontPageSaved is a saved front page with what to tell the user.
type FrontPageSaved struct {
	Page FrontPageView `json:"page"`
	// Warnings name the images the text refers to that the meet and its
	// style do not have.
	Warnings []string `json:"warnings"`
}

func frontPageView(r store.FrontPageRow) FrontPageView {
	reports := []string{}
	_ = json.Unmarshal([]byte(r.ReportsJSON), &reports)
	if reports == nil {
		reports = []string{}
	}
	return FrontPageView{ID: r.ID, MeetID: r.MeetID, Position: r.Position, Title: r.Title, BodyMD: r.BodyMD, Reports: reports, UpdatedAt: r.UpdatedAt}
}

// ListFrontPages returns a meet's front pages in print order.
func (s *Service) ListFrontPages(ctx context.Context, meetID string) ([]FrontPageView, error) {
	if _, err := s.GetMeet(ctx, meetID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListFrontPages(ctx, meetID)
	if err != nil {
		return nil, err
	}
	out := make([]FrontPageView, len(rows))
	for i, r := range rows {
		out[i] = frontPageView(r)
	}
	return out, nil
}

// checkFrontPage validates and normalizes a front page: the title (1–60
// characters), the Markdown (printstyle.RenderMarkdown must accept it) and
// the report kinds (known, once each, in the reports' order).
func checkFrontPage(in FrontPageInput) (FrontPageInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if n := utf8.RuneCountInString(in.Title); n < 1 || n > maxFrontPageTitle {
		return in, invalid("前置页标题需要 1–%d 个字", maxFrontPageTitle)
	}
	if _, err := printstyle.RenderMarkdown(in.BodyMD); err != nil {
		return in, invalid("%s", err)
	}
	for _, k := range in.Reports {
		if !slices.Contains(printstyle.ReportKinds, k) {
			return in, invalid("没有名为「%s」的报表，可用 %s", k, strings.Join(printstyle.ReportKinds, "、"))
		}
	}
	in.Reports = slices.DeleteFunc(slices.Clone(printstyle.ReportKinds), func(k string) bool { return !slices.Contains(in.Reports, k) })
	return in, nil
}

// SaveFrontPage adds a front page to the end of a meet's pages (id ""), or
// changes one, at once. At most 20 per meet.
func (s *Service) SaveFrontPage(ctx context.Context, src store.Source, meetID, id string, in FrontPageInput) (*FrontPageSaved, error) {
	in, err := checkFrontPage(in)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListFrontPages(ctx, meetID)
	if err != nil {
		return nil, err
	}
	now := s.store.Now()
	row := store.FrontPageRow{ID: id, MeetID: meetID, Title: in.Title, BodyMD: in.BodyMD, ReportsJSON: jsonValue(in.Reports), UpdatedAt: now}
	var stmt sqlite.Statement
	action := "frontpage.saved"
	if id == "" {
		if len(rows) >= maxFrontPages {
			return nil, invalid("前置页最多 %d 页", maxFrontPages)
		}
		row.ID, row.Position, row.CreatedAt = newID(), len(rows)+1, now
		stmt = store.InsertFrontPage(row)
	} else {
		i := slices.IndexFunc(rows, func(r store.FrontPageRow) bool { return r.ID == id })
		if i < 0 {
			return nil, invalid("前置页不存在或已删除")
		}
		row.Position, row.CreatedAt = rows[i].Position, rows[i].CreatedAt
		stmt = store.UpdateFrontPage(row)
	}
	warns, err := s.missingAssets(ctx, meetID, in.BodyMD)
	if err != nil {
		return nil, err
	}
	err = s.write(ctx, store.Audit{Source: src, Action: action, MeetID: meetID, Message: meet.Name, Detail: map[string]any{"title": in.Title, "pageId": row.ID}},
		Change{Kind: "meets", MeetID: meetID}, stmt)
	if err != nil {
		return nil, err
	}
	return &FrontPageSaved{Page: frontPageView(row), Warnings: warns}, nil
}

// missingAssets names, as warnings, the images texts refer to that neither
// the meet nor the style the meet uses has.
func (s *Service) missingAssets(ctx context.Context, meetID string, texts ...string) ([]string, error) {
	warns := []string{}
	names := referencedAssets(texts...)
	if len(names) == 0 {
		return warns, nil
	}
	have := map[string]bool{}
	owners := []struct {
		owner store.AssetOwner
		id    string
	}{{store.AssetOfMeet, meetID}}
	prefs, err := s.MeetSettings(ctx, meetID)
	if err != nil {
		return nil, err
	}
	if prefs.PrintStyleID != "" {
		owners = append(owners, struct {
			owner store.AssetOwner
			id    string
		}{store.AssetOfStyle, prefs.PrintStyleID})
	}
	for _, o := range owners {
		list, err := s.store.ListAssets(ctx, o.owner, o.id, false)
		if err != nil {
			return nil, err
		}
		for _, a := range list {
			have[a.Name] = true
		}
	}
	for _, n := range names {
		if !have[n] {
			warns = append(warns, fmt.Sprintf("找不到图片「%s」，请先上传到赛事图片", n))
		}
	}
	return warns, nil
}

// DeleteFrontPage removes a front page, at once; the pages after it move up.
func (s *Service) DeleteFrontPage(ctx context.Context, src store.Source, meetID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return err
	}
	rows, err := s.store.ListFrontPages(ctx, meetID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rows, func(r store.FrontPageRow) bool { return r.ID == id })
	if i < 0 {
		return invalid("前置页不存在或已删除")
	}
	stmts := []sqlite.Statement{store.DeleteFrontPage(meetID, id)}
	for j, r := range slices.Delete(slices.Clone(rows), i, i+1) {
		if r.Position != j+1 {
			stmts = append(stmts, store.SetFrontPagePosition(meetID, r.ID, j+1))
		}
	}
	return s.write(ctx, store.Audit{Source: src, Action: "frontpage.deleted", MeetID: meetID, Message: meet.Name, Detail: map[string]any{"title": rows[i].Title}},
		Change{Kind: "meets", MeetID: meetID}, stmts...)
}

// ReorderFrontPages sets the print order; ids must name every front page of
// the meet once.
func (s *Service) ReorderFrontPages(ctx context.Context, src store.Source, meetID string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meet, err := s.GetMeet(ctx, meetID)
	if err != nil {
		return err
	}
	rows, err := s.store.ListFrontPages(ctx, meetID)
	if err != nil {
		return err
	}
	if len(ids) != len(rows) {
		return invalid("排序须列出全部 %d 页前置页", len(rows))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !slices.ContainsFunc(rows, func(r store.FrontPageRow) bool { return r.ID == id }) {
			return invalid("排序里有不属于本赛事的前置页，或重复")
		}
		seen[id] = true
	}
	stmts := []sqlite.Statement{}
	for i, id := range ids {
		stmts = append(stmts, store.SetFrontPagePosition(meetID, id, i+1))
	}
	if len(stmts) == 0 {
		return nil
	}
	return s.write(ctx, store.Audit{Source: src, Action: "frontpage.reordered", MeetID: meetID, Message: meet.Name},
		Change{Kind: "meets", MeetID: meetID}, stmts...)
}

// frontPagesForReport renders a meet's front pages as the reports take
// them. A page whose text cannot be rendered is left out with a warning.
func (s *Service) frontPagesForReport(ctx context.Context, meetID string) ([]report.FrontPage, []store.FrontPageRow, []string, error) {
	rows, err := s.store.ListFrontPages(ctx, meetID)
	if err != nil {
		return nil, nil, nil, err
	}
	var pages []report.FrontPage
	var warns []string
	for _, r := range rows {
		html, err := printstyle.RenderMarkdown(r.BodyMD)
		if err != nil {
			warns = append(warns, fmt.Sprintf("前置页「%s」无法显示：%v", r.Title, err))
			continue
		}
		pages = append(pages, report.FrontPage{Title: r.Title, HTML: html, Reports: frontPageView(r).Reports})
	}
	return pages, rows, warns, nil
}
