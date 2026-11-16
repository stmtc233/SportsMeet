package service

import (
	"context"
	"slices"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// RenderOptions choose the print style a report is rendered with. Without
// options a report uses the style its meet selected (MeetSettings.PrintStyleID).
type RenderOptions struct {
	// StyleID uses this style instead of the meet's; DefaultPrintStyleID
	// for the built-in one. An id that does not exist is an error here
	// (a meet that names a vanished style reads as the default).
	StyleID string `json:"styleId"`
	// Draft is applied over the style without being saved, to try changes:
	// it is checked as a save is, and its Name is ignored. Images stay the
	// style's and the meet's.
	Draft *PrintStyleInput `json:"draft"`
}

// RenderedReport is a printable document and what the style could not do
// (CSS that was cleaned, a missing image, a template that failed and was
// replaced by the built-in layout).
type RenderedReport struct {
	HTML     string   `json:"html"`
	Warnings []string `json:"warnings"`
}

func rendered(html string, warns []string, err error) (*RenderedReport, error) {
	if err != nil {
		return nil, err
	}
	return &RenderedReport{HTML: html, Warnings: nonNilStrings(warns)}, nil
}

// htmlOnly is what the render methods without options return: the document,
// the warnings dropped.
func htmlOnly(r *RenderedReport, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return r.HTML, nil
}

// assembledReport is a report's content, gathered from the meet and shaped
// by its settings but not yet by a print style. Rendering and
// GetReportData both start from it, so the data a custom template receives
// and the data a caller reads are the same.
type assembledReport struct {
	kind string
	// book is a report.Book, report.CheckInBook or report.RecordBook.
	book any
	// styleID is the style the meet chose (MeetSettings.PrintStyleID).
	styleID string
}

// render renders the report in a style.
func (a *assembledReport) render(st report.Style) (string, []string, error) {
	switch b := a.book.(type) {
	case report.CheckInBook:
		return report.CheckIn(b, st)
	case report.RecordBook:
		return report.RecordSheets(b, st)
	case report.Book:
		switch a.kind {
		case ReportEntries:
			return report.Entries(b, st)
		case ReportStartList:
			return report.StartList(b, st)
		case ReportResults:
			return report.Results(b, st)
		case ReportTeamScores:
			return report.TeamScores(b, st)
		}
	}
	return "", nil, invalid("报表类型不正确")
}

// renderAssembled renders an assembled report in the style opt chooses
// (the meet's without options), with the style's warnings.
func (s *Service) renderAssembled(ctx context.Context, meetID string, a *assembledReport, opt RenderOptions) (*RenderedReport, error) {
	st, warns, err := s.reportStyle(ctx, meetID, a.styleID, opt)
	if err != nil {
		return nil, err
	}
	html, w, err := a.render(st)
	return rendered(html, append(warns, w...), err)
}

// reportStyle is the report.Style a meet's report is rendered with: the
// style chosen by options or by the meet (styleID, from settings the
// caller has read), with the draft over it, the meet's front pages and the
// images the style's text refers to (the meet's over the style's). The
// built-in style with no front pages is the zero-value look.
func (s *Service) reportStyle(ctx context.Context, meetID, settingsStyleID string, opt RenderOptions) (report.Style, []string, error) {
	var warns []string
	id := opt.StyleID
	explicit := id != ""
	if !explicit {
		id = settingsStyleID
	}
	c := builtinContent()
	styleOwner := ""
	if id != "" && id != DefaultPrintStyleID {
		row, err := s.store.GetPrintStyle(ctx, id)
		if err != nil {
			return report.Style{}, nil, err
		}
		switch {
		case row != nil:
			var w []string
			c, w = contentOf(*row)
			warns = append(warns, w...)
			styleOwner = row.ID
		case explicit:
			return report.Style{}, nil, errNoStyle()
		}
	}
	if opt.Draft != nil {
		draft := *opt.Draft
		draft.Name = nil
		next, w, err := applyStyleChange(c, draft)
		if err != nil {
			return report.Style{}, nil, err
		}
		c = next
		warns = append(warns, w...)
	}
	pages, pageRows, w, err := s.frontPagesForReport(ctx, meetID)
	if err != nil {
		return report.Style{}, nil, err
	}
	warns = append(warns, w...)

	st := report.Style{Params: c.Params, CSS: c.CSS, Slots: c.Slots, Templates: c.Templates, DataVersion: c.DataVersion, FrontPages: pages}
	// Images are read (they are the bulk of a style) only when something
	// refers to one.
	texts := []string{c.CSS}
	for _, v := range c.Slots {
		texts = append(texts, v)
	}
	for _, v := range c.Templates {
		texts = append(texts, v)
	}
	for _, p := range pageRows {
		texts = append(texts, p.BodyMD)
	}
	if strings.Contains(strings.Join(texts, "\n"), "asset:") {
		st.Assets = map[string]printstyle.Asset{}
		if styleOwner != "" {
			rows, err := s.store.ListAssets(ctx, store.AssetOfStyle, styleOwner, true)
			if err != nil {
				return report.Style{}, nil, err
			}
			for _, a := range rows {
				st.Assets[a.Name] = printstyle.Asset{Name: a.Name, MIME: a.MIME, Data: a.Data}
			}
		}
		rows, err := s.store.ListAssets(ctx, store.AssetOfMeet, meetID, true)
		if err != nil {
			return report.Style{}, nil, err
		}
		for _, a := range rows {
			st.Assets[a.Name] = printstyle.Asset{Name: a.Name, MIME: a.MIME, Data: a.Data}
		}
	}
	return st, warns, nil
}

// RenderStyleSample renders a report kind's built-in sample data with a
// style (styleID "" is the default) and an optional unsaved draft over it:
// the way to try a style before any meet's data is involved. Front pages
// are a meet's and not shown. Images are the style's.
func (s *Service) RenderStyleSample(ctx context.Context, kind, styleID string, draft *PrintStyleInput) (*RenderedReport, error) {
	if !slices.Contains(printstyle.ReportKinds, kind) {
		return nil, invalid("没有名为「%s」的报表，可用 %s", kind, strings.Join(printstyle.ReportKinds, "、"))
	}
	c := builtinContent()
	var assets []store.AssetRow
	if styleID != "" && styleID != DefaultPrintStyleID {
		row, err := s.store.GetPrintStyle(ctx, styleID)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, errNoStyle()
		}
		c, _ = contentOf(*row)
		if assets, err = s.store.ListAssets(ctx, store.AssetOfStyle, styleID, true); err != nil {
			return nil, err
		}
	}
	var warns []string
	if draft != nil {
		d := *draft
		d.Name = nil
		next, w, err := applyStyleChange(c, d)
		if err != nil {
			return nil, err
		}
		c, warns = next, w
	}
	st := report.Style{Params: c.Params, CSS: c.CSS, Slots: c.Slots, Templates: c.Templates, DataVersion: c.DataVersion, Assets: map[string]printstyle.Asset{}}
	for _, a := range assets {
		st.Assets[a.Name] = printstyle.Asset{Name: a.Name, MIME: a.MIME, Data: a.Data}
	}
	html, w, err := report.RenderSample(kind, st)
	return rendered(html, append(warns, w...), err)
}
