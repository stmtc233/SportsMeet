package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/printstyle"
	"github.com/stmtc233/sportsmeet/internal/report"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Print styles (打印样式) are global to the app: any meet can pick one in
// its settings (MeetSettings.PrintStyleID). The built-in 默认 style is not
// stored; it is the look the reports have without a style, it cannot be
// changed, and a copy of it is how a new style starts.
//
// Every write takes effect at once, there is no preview step. Each save
// keeps a snapshot, the latest maxPrintStyleVersions of a style, and a
// snapshot can be restored (which is a save of its own).
const (
	// DefaultPrintStyleID is the built-in style's id.
	DefaultPrintStyleID   = "default"
	defaultPrintStyleName = "默认"

	maxPrintStyles        = 100
	maxPrintStyleVersions = 30
)

// PrintStyleSummary is a style in the list.
type PrintStyleSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Builtin bool   `json:"builtin"`
	// Version counts the saves, from 1; 0 for the built-in style.
	Version   int   `json:"version"`
	UpdatedAt int64 `json:"updatedAt"` // ms; 0 for the built-in style
	// UsedBy is how many meets use it.
	UsedBy       int  `json:"usedBy"`
	HasCustomCSS bool `json:"hasCustomCss"`
	// HasTemplates: it holds custom templates or slots.
	HasTemplates bool `json:"hasTemplates"`
}

// AssetInfo is an image without its bytes.
type AssetInfo struct {
	Name   string `json:"name"`
	MIME   string `json:"mime"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

// PrintStyleMeet is a meet that uses a style.
type PrintStyleMeet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PrintStyleDetail is a whole style.
type PrintStyleDetail struct {
	PrintStyleSummary
	CreatedAt int64             `json:"createdAt"`
	Params    printstyle.Params `json:"params"`
	CSS       string            `json:"css"`
	// Slots and Templates are written for report data version DataVersion
	// (the current one is report.DataVersion).
	Slots       map[string]string `json:"slots"`
	Templates   map[string]string `json:"templates"`
	DataVersion int               `json:"dataVersion"`
	Assets      []AssetInfo       `json:"assets"`
	Meets       []PrintStyleMeet  `json:"meets"`
}

// PrintStyleInput creates a style (no id) or changes one. A nil field is
// left as it is; on creation the default's value stands.
type PrintStyleInput struct {
	Name *string `json:"name"`
	// Params replace the layout parameters. ParamsPatch (a JSON object)
	// changes only the fields it names; give one of the two.
	Params      *printstyle.Params `json:"params"`
	ParamsPatch string             `json:"paramsPatch"`
	CSS         *string            `json:"css"`
	// Slots and Templates are changed by key: a key given is set, a key
	// given with white space only is removed, keys not given stay.
	Slots     map[string]string `json:"slots"`
	Templates map[string]string `json:"templates"`
}

// PrintStyleSaved is the result of a save.
type PrintStyleSaved struct {
	Style   *PrintStyleDetail `json:"style"`
	Version int               `json:"version"`
	// Changed is false when the input changed nothing: no version was made.
	Changed bool `json:"changed"`
	// Warnings say what was cleaned from the CSS and the like.
	Warnings []string `json:"warnings"`
}

// styleContent is what a style holds apart from its images.
type styleContent struct {
	Name        string
	Params      printstyle.Params
	CSS         string
	Slots       map[string]string
	Templates   map[string]string
	DataVersion int
}

func builtinContent() styleContent {
	return styleContent{Name: defaultPrintStyleName, Params: printstyle.DefaultParams(), Slots: map[string]string{}, Templates: map[string]string{}}
}

func (c styleContent) clone() styleContent {
	c.Slots, c.Templates = cloneMap(c.Slots), cloneMap(c.Templates)
	return c
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// contentOf reads a stored style. A column that does not parse reads as
// empty, with a warning.
func contentOf(row store.PrintStyleRow) (styleContent, []string) {
	c := styleContent{Name: row.Name, CSS: row.CSS, DataVersion: row.DataVersion}
	var warns []string
	p, err := printstyle.ParamsFromJSON([]byte(row.ParamsJSON))
	if err != nil {
		warns = append(warns, "样式的版面参数无法读取，已用默认值："+err.Error())
		p = printstyle.DefaultParams()
	}
	c.Params = p
	read := func(raw, what string) map[string]string {
		m := map[string]string{}
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
				warns = append(warns, "样式的"+what+"无法读取，已忽略")
				m = map[string]string{}
			}
		}
		return m
	}
	c.Slots, c.Templates = read(row.SlotsJSON, "插槽"), read(row.TemplatesJSON, "模板")
	return c, warns
}

func (c styleContent) row(id string, version int, created, updated int64) store.PrintStyleRow {
	return store.PrintStyleRow{ID: id, Name: c.Name, ParamsJSON: jsonValue(c.Params), CSS: c.CSS, SlotsJSON: jsonValue(nonNil(c.Slots)),
		TemplatesJSON: jsonValue(nonNil(c.Templates)), DataVersion: c.DataVersion, Version: version, CreatedAt: created, UpdatedAt: updated}
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (c styleContent) hasTemplates() bool { return len(c.Slots)+len(c.Templates) > 0 }

// styleSnapshot is what a version keeps. Images are only named with their
// hash: restoring a version does not bring images back (the images stay as
// they are now), it warns about the ones that differ.
type styleSnapshot struct {
	Reason      string            `json:"reason"`
	Name        string            `json:"name"`
	Params      printstyle.Params `json:"params"`
	CSS         string            `json:"css"`
	Slots       map[string]string `json:"slots"`
	Templates   map[string]string `json:"templates"`
	DataVersion int               `json:"dataVersion"`
	Assets      []snapshotAsset   `json:"assets"`
}

type snapshotAsset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

func snapshotJSON(c styleContent, reason string, assets []store.AssetRow) string {
	snap := styleSnapshot{Reason: reason, Name: c.Name, Params: c.Params, CSS: c.CSS, Slots: nonNil(c.Slots), Templates: nonNil(c.Templates),
		DataVersion: c.DataVersion, Assets: []snapshotAsset{}}
	for _, a := range assets {
		snap.Assets = append(snap.Assets, snapshotAsset{Name: a.Name, SHA256: a.SHA256})
	}
	return jsonValue(snap)
}

func validStyleName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > printstyle.MaxNameRunes {
		return "", invalid("样式名称需要 1–%d 个字", printstyle.MaxNameRunes)
	}
	if strings.ContainsFunc(name, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return "", invalid("样式名称不能含控制字符")
	}
	return name, nil
}

// styleNameTaken: names compare the way the app compares names elsewhere,
// and 默认 belongs to the built-in style.
func styleNameTaken(rows []store.PrintStyleRow, name, exceptID string) bool {
	key := domain.NormalizeText(name)
	if key == domain.NormalizeText(defaultPrintStyleName) {
		return true
	}
	for _, r := range rows {
		if r.ID != exceptID && domain.NormalizeText(r.Name) == key {
			return true
		}
	}
	return false
}

// uniqueStyleName returns name cut to the name length, or with (2), (3) …
// appended when it is taken.
func uniqueStyleName(rows []store.PrintStyleRow, name string) string {
	fit := func(n int) string {
		r := []rune(strings.TrimSpace(name))
		if len(r) > n {
			r = r[:n]
		}
		return strings.TrimSpace(string(r))
	}
	if whole := fit(printstyle.MaxNameRunes); !styleNameTaken(rows, whole, "") {
		return whole
	}
	base := fit(printstyle.MaxNameRunes - 4)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s(%d)", base, n)
		if !styleNameTaken(rows, candidate, "") {
			return candidate
		}
	}
}

// applyStyleChange applies in over base, checking what it changes in this
// order: parameters, CSS (cleaned; what was removed is a warning), the
// slots, the templates. A problem is an error and nothing is applied.
// Slots and templates are validated, and stamped with the current report
// data version, only when the input names some: a style whose templates
// are stale stays as it is until they are touched.
func applyStyleChange(base styleContent, in PrintStyleInput) (styleContent, []string, error) {
	c := base.clone()
	var warns []string
	if in.Params != nil && strings.TrimSpace(in.ParamsPatch) != "" {
		return c, nil, invalid("版面参数只能给 params 或 paramsPatch 之一")
	}
	switch {
	case in.Params != nil:
		if err := in.Params.Validate(); err != nil {
			return c, nil, invalid("%s", err)
		}
		c.Params = *in.Params
	case strings.TrimSpace(in.ParamsPatch) != "":
		p := c.Params
		dec := json.NewDecoder(strings.NewReader(in.ParamsPatch))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return c, nil, invalid("版面参数格式不对：%v", err)
		}
		if err := p.Validate(); err != nil {
			return c, nil, invalid("%s", err)
		}
		c.Params = p
	}
	if in.CSS != nil {
		clean, w, err := printstyle.SanitizeCSS(*in.CSS)
		if err != nil {
			return c, nil, invalid("%s", err)
		}
		c.CSS = clean
		warns = append(warns, w...)
	}
	// An empty patch (the form sends {} for both) leaves stale templates alone.
	touched := len(in.Slots) > 0 || len(in.Templates) > 0
	merge := func(dst map[string]string, patch map[string]string) {
		for k, v := range patch {
			if strings.TrimSpace(v) == "" {
				delete(dst, k)
			} else {
				dst[k] = v
			}
		}
	}
	merge(c.Slots, in.Slots)
	merge(c.Templates, in.Templates)
	if touched {
		for _, name := range sortedStrings(c.Slots) {
			if err := report.ValidateSlot(name, c.Slots[name]); err != nil {
				return c, nil, invalid("插槽「%s」：%v", name, err)
			}
		}
		for _, kind := range sortedStrings(c.Templates) {
			if err := report.ValidateTemplate(kind, c.Templates[kind]); err != nil {
				return c, nil, invalid("报表模板「%s」：%v", kind, err)
			}
		}
		if c.hasTemplates() {
			c.DataVersion = report.DataVersion
		} else {
			c.DataVersion = 0
		}
	}
	if c.hasTemplates() && c.DataVersion != report.DataVersion {
		warns = append(warns, "自定义模板写于旧的报表数据版本，不会使用，请更新模板")
	}
	return c, warns, nil
}

func sortedStrings(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (s *Service) allPrintStyles(ctx context.Context) ([]store.PrintStyleRow, error) {
	return s.store.ListPrintStyles(ctx)
}

func findStyle(rows []store.PrintStyleRow, id string) *store.PrintStyleRow {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

var errBuiltinStyle = invalid("内置的默认样式不能修改，可以复制后再改")

func errNoStyle() error { return invalid("打印样式不存在或已删除") }

// useCounts counts the meets by the style they use; the meets of the
// default style are those naming no style, or one that is gone.
func (s *Service) styleUse(ctx context.Context, rows []store.PrintStyleRow) (byStyle map[string][]PrintStyleMeet, err error) {
	uses, err := s.store.ListMeetStyleUse(ctx)
	if err != nil {
		return nil, err
	}
	byStyle = map[string][]PrintStyleMeet{}
	for _, u := range uses {
		id := u.StyleID
		if id == "" || findStyle(rows, id) == nil {
			id = DefaultPrintStyleID
		}
		byStyle[id] = append(byStyle[id], PrintStyleMeet{ID: u.MeetID, Name: u.MeetName})
	}
	return byStyle, nil
}

func summaryOf(id string, c styleContent, version int, updated int64, used int) PrintStyleSummary {
	return PrintStyleSummary{ID: id, Name: c.Name, Builtin: id == DefaultPrintStyleID, Version: version, UpdatedAt: updated, UsedBy: used,
		HasCustomCSS: c.CSS != "", HasTemplates: c.hasTemplates()}
}

// ListPrintStyles returns the built-in default first, then the saved
// styles, oldest first.
func (s *Service) ListPrintStyles(ctx context.Context) ([]PrintStyleSummary, error) {
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	use, err := s.styleUse(ctx, rows)
	if err != nil {
		return nil, err
	}
	out := []PrintStyleSummary{summaryOf(DefaultPrintStyleID, builtinContent(), 0, 0, len(use[DefaultPrintStyleID]))}
	for _, r := range rows {
		c, _ := contentOf(r)
		out = append(out, summaryOf(r.ID, c, r.Version, r.UpdatedAt, len(use[r.ID])))
	}
	return out, nil
}

func assetInfos(rows []store.AssetRow) []AssetInfo {
	out := make([]AssetInfo, 0, len(rows))
	for _, a := range rows {
		out = append(out, AssetInfo{Name: a.Name, MIME: a.MIME, SHA256: a.SHA256, Size: a.Size})
	}
	return out
}

func (s *Service) styleDetail(ctx context.Context, rows []store.PrintStyleRow, id string) (*PrintStyleDetail, error) {
	use, err := s.styleUse(ctx, rows)
	if err != nil {
		return nil, err
	}
	d := &PrintStyleDetail{Assets: []AssetInfo{}, Meets: use[id]}
	if d.Meets == nil {
		d.Meets = []PrintStyleMeet{}
	}
	var c styleContent
	if id == DefaultPrintStyleID {
		c = builtinContent()
		d.PrintStyleSummary = summaryOf(id, c, 0, 0, len(d.Meets))
	} else {
		row := findStyle(rows, id)
		if row == nil {
			return nil, errNoStyle()
		}
		c, _ = contentOf(*row)
		d.PrintStyleSummary = summaryOf(id, c, row.Version, row.UpdatedAt, len(d.Meets))
		d.CreatedAt = row.CreatedAt
		assets, err := s.store.ListAssets(ctx, store.AssetOfStyle, id, false)
		if err != nil {
			return nil, err
		}
		d.Assets = assetInfos(assets)
	}
	d.Params, d.CSS, d.Slots, d.Templates, d.DataVersion = c.Params, c.CSS, nonNil(c.Slots), nonNil(c.Templates), c.DataVersion
	return d, nil
}

// GetPrintStyle returns a whole style; DefaultPrintStyleID is the built-in
// one.
func (s *Service) GetPrintStyle(ctx context.Context, id string) (*PrintStyleDetail, error) {
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	return s.styleDetail(ctx, rows, id)
}

// SavePrintStyle creates a style (id "") or changes one, at once. The
// checks run in this order and a failure writes nothing: the name (1–40
// characters, not taken), the parameters, the CSS (cleaned), the slots, the
// templates. The saved CSS is the cleaned one. A save that changes nothing
// makes no version. The built-in style cannot be changed.
func (s *Service) SavePrintStyle(ctx context.Context, src store.Source, id string, in PrintStyleInput) (*PrintStyleSaved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	if id == "" {
		if in.Name == nil {
			return nil, invalid("请填写样式名称")
		}
		name, err := validStyleName(*in.Name)
		if err != nil {
			return nil, err
		}
		if styleNameTaken(rows, name, "") {
			return nil, invalid("已有名为「%s」的样式", name)
		}
		if len(rows) >= maxPrintStyles {
			return nil, invalid("样式最多 %d 个", maxPrintStyles)
		}
		c := builtinContent()
		c.Name = name
		c, warns, err := applyStyleChange(c, in)
		if err != nil {
			return nil, err
		}
		return s.createStyle(ctx, src, rows, c, nil, "printstyle.created", "新建", warns)
	}
	if id == DefaultPrintStyleID {
		return nil, errBuiltinStyle
	}
	row := findStyle(rows, id)
	if row == nil {
		return nil, errNoStyle()
	}
	base, _ := contentOf(*row)
	next := base.clone()
	if in.Name != nil {
		name, err := validStyleName(*in.Name)
		if err != nil {
			return nil, err
		}
		if styleNameTaken(rows, name, id) {
			return nil, invalid("已有名为「%s」的样式", name)
		}
		next.Name = name
	}
	next, warns, err := applyStyleChange(next, in)
	if err != nil {
		return nil, err
	}
	reason := "保存"
	if in.Name != nil && next.Name != base.Name && in.Params == nil && in.ParamsPatch == "" && in.CSS == nil && in.Slots == nil && in.Templates == nil {
		reason = "改名"
	}
	return s.updateStyle(ctx, src, rows, *row, base, next, "printstyle.updated", reason, warns)
}

// createStyle writes a new style with its first version (and images, for a
// copy or an import).
func (s *Service) createStyle(ctx context.Context, src store.Source, rows []store.PrintStyleRow, c styleContent, assets []store.AssetRow, action, reason string, warns []string) (*PrintStyleSaved, error) {
	now := s.store.Now()
	id := newID()
	stmts := []sqlite.Statement{store.InsertPrintStyle(c.row(id, 1, now, now))}
	stored := make([]store.AssetRow, 0, len(assets))
	for _, a := range assets {
		a.ID, a.OwnerID = newID(), id
		stored = append(stored, a)
		stmts = append(stmts, store.UpsertAsset(store.AssetOfStyle, a))
	}
	stmts = append(stmts, store.InsertPrintStyleVersion(id, 1, src, snapshotJSON(c, reason, stored), now))
	err := s.write(ctx, store.Audit{Source: src, Action: action, Message: c.Name, Detail: map[string]any{"styleId": id, "reason": reason}},
		Change{Kind: "printStyles"}, stmts...)
	if err != nil {
		return nil, err
	}
	rows = append(rows, c.row(id, 1, now, now))
	d, err := s.styleDetail(ctx, rows, id)
	if err != nil {
		return nil, err
	}
	return &PrintStyleSaved{Style: d, Version: 1, Changed: true, Warnings: nonNilStrings(warns)}, nil
}

func nonNilStrings(w []string) []string {
	if w == nil {
		return []string{}
	}
	return w
}

func sameContent(a, b styleContent) bool {
	a.Slots, a.Templates, b.Slots, b.Templates = nonNil(a.Slots), nonNil(a.Templates), nonNil(b.Slots), nonNil(b.Templates)
	return reflect.DeepEqual(a, b)
}

// updateStyle writes next over a style when it differs: the new content,
// the version bumped, its snapshot kept and the oldest snapshots beyond
// maxPrintStyleVersions dropped, in one batch.
func (s *Service) updateStyle(ctx context.Context, src store.Source, rows []store.PrintStyleRow, row store.PrintStyleRow, base, next styleContent, action, reason string, warns []string) (*PrintStyleSaved, error) {
	if sameContent(base, next) {
		d, err := s.styleDetail(ctx, rows, row.ID)
		if err != nil {
			return nil, err
		}
		return &PrintStyleSaved{Style: d, Version: row.Version, Changed: false, Warnings: nonNilStrings(warns)}, nil
	}
	assets, err := s.store.ListAssets(ctx, store.AssetOfStyle, row.ID, false)
	if err != nil {
		return nil, err
	}
	version := row.Version + 1
	now := s.store.Now()
	updated := next.row(row.ID, version, row.CreatedAt, now)
	stmts := []sqlite.Statement{store.UpdatePrintStyle(updated),
		store.InsertPrintStyleVersion(row.ID, version, src, snapshotJSON(next, reason, assets), now),
		store.TrimPrintStyleVersions(row.ID, version-maxPrintStyleVersions)}
	err = s.write(ctx, store.Audit{Source: src, Action: action, Message: next.Name, Detail: map[string]any{"styleId": row.ID, "version": version, "reason": reason}},
		Change{Kind: "printStyles"}, stmts...)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == row.ID {
			rows[i] = updated
		}
	}
	d, err := s.styleDetail(ctx, rows, row.ID)
	if err != nil {
		return nil, err
	}
	return &PrintStyleSaved{Style: d, Version: version, Changed: true, Warnings: nonNilStrings(warns)}, nil
}

// CopyPrintStyle makes a new style of a style (the built-in default
// included) with its images. name "" is the source's name with 「 副本」
// (and a number when taken); a name that is taken is an error.
func (s *Service) CopyPrintStyle(ctx context.Context, src store.Source, id, name string) (*PrintStyleSaved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) >= maxPrintStyles {
		return nil, invalid("样式最多 %d 个", maxPrintStyles)
	}
	var c styleContent
	var assets []store.AssetRow
	if id == DefaultPrintStyleID {
		c = builtinContent()
	} else {
		row := findStyle(rows, id)
		if row == nil {
			return nil, errNoStyle()
		}
		c, _ = contentOf(*row)
		if assets, err = s.store.ListAssets(ctx, store.AssetOfStyle, id, true); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(name) == "" {
		c.Name = uniqueStyleName(rows, c.Name+" 副本")
	} else {
		n, err := validStyleName(name)
		if err != nil {
			return nil, err
		}
		if styleNameTaken(rows, n, "") {
			return nil, invalid("已有名为「%s」的样式", n)
		}
		c.Name = n
	}
	return s.createStyle(ctx, src, rows, c, assets, "printstyle.created", "复制自「"+defaultOr(rows, id)+"」", nil)
}

func defaultOr(rows []store.PrintStyleRow, id string) string {
	if r := findStyle(rows, id); r != nil {
		return r.Name
	}
	return defaultPrintStyleName
}

// RenamePrintStyle renames a style; it is a save, with a version of its own.
func (s *Service) RenamePrintStyle(ctx context.Context, src store.Source, id, name string) (*PrintStyleSaved, error) {
	return s.SavePrintStyle(ctx, src, id, PrintStyleInput{Name: &name})
}

// DeletePrintStyle removes a style with its versions and images, at once.
// The meets that used it read as the default style from then on.
func (s *Service) DeletePrintStyle(ctx context.Context, src store.Source, id string) error {
	if id == DefaultPrintStyleID {
		return errBuiltinStyle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row, err := s.store.GetPrintStyle(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return errNoStyle()
	}
	return s.write(ctx, store.Audit{Source: src, Action: "printstyle.deleted", Message: row.Name, Detail: map[string]any{"styleId": id, "version": row.Version}},
		Change{Kind: "printStyles"}, store.DeletePrintStyle(id))
}

// PrintStyleVersion is a kept snapshot of a style.
type PrintStyleVersion struct {
	Version int   `json:"version"`
	Current bool  `json:"current"`
	At      int64 `json:"at"` // ms
	// AtText is At shown to the minute in the service's time zone.
	AtText string `json:"atText"`
	// Source is who saved it: ui, mcp or system.
	Source string `json:"source"`
	// Reason says what the save was: 新建, 保存, 改名, 图片, 恢复 …
	Reason       string `json:"reason"`
	Name         string `json:"name"`
	HasCustomCSS bool   `json:"hasCustomCss"`
	HasTemplates bool   `json:"hasTemplates"`
	Assets       int    `json:"assets"`
}

// ListPrintStyleVersions returns the kept snapshots of a style, newest
// first (at most maxPrintStyleVersions). The built-in style has none.
func (s *Service) ListPrintStyleVersions(ctx context.Context, id string) ([]PrintStyleVersion, error) {
	if id == DefaultPrintStyleID {
		return []PrintStyleVersion{}, nil
	}
	row, err := s.store.GetPrintStyle(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNoStyle()
	}
	list, err := s.store.ListPrintStyleVersions(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]PrintStyleVersion, 0, len(list))
	for _, v := range list {
		var snap styleSnapshot
		_ = json.Unmarshal([]byte(v.SnapshotJSON), &snap)
		out = append(out, PrintStyleVersion{Version: v.Version, Current: v.Version == row.Version, At: v.CreatedAt, AtText: domain.RecordedText(v.CreatedAt, s.zone), Source: string(v.Source), Reason: snap.Reason,
			Name: snap.Name, HasCustomCSS: snap.CSS != "", HasTemplates: len(snap.Slots)+len(snap.Templates) > 0, Assets: len(snap.Assets)})
	}
	return out, nil
}

// RestorePrintStyleVersion brings a kept version's parameters, CSS, slots
// and templates back, as a new save (the version number goes on). The name
// stays the current one, so a restore cannot collide with another style.
// Images are not part of what is restored: they stay as they are now, and
// the warnings name the ones the version was saved with that are gone or
// have changed.
func (s *Service) RestorePrintStyleVersion(ctx context.Context, src store.Source, id string, version int) (*PrintStyleSaved, error) {
	if id == DefaultPrintStyleID {
		return nil, errBuiltinStyle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.allPrintStyles(ctx)
	if err != nil {
		return nil, err
	}
	row := findStyle(rows, id)
	if row == nil {
		return nil, errNoStyle()
	}
	v, err := s.store.GetPrintStyleVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, invalid("没有第 %d 版（只保留最近 %d 版）", version, maxPrintStyleVersions)
	}
	var snap styleSnapshot
	if err := json.Unmarshal([]byte(v.SnapshotJSON), &snap); err != nil {
		return nil, invalid("第 %d 版的内容无法读取", version)
	}
	base, _ := contentOf(*row)
	next := base.clone()
	next.Params, next.CSS, next.Slots, next.Templates, next.DataVersion = snap.Params, snap.CSS, nonNil(snap.Slots), nonNil(snap.Templates), snap.DataVersion
	if err := next.Params.Validate(); err != nil {
		return nil, invalid("第 %d 版的版面参数现在不合格：%v", version, err)
	}
	css, warns, err := printstyle.SanitizeCSS(next.CSS)
	if err != nil {
		return nil, invalid("第 %d 版的 CSS 现在不合格：%v", version, err)
	}
	next.CSS = css
	if next.hasTemplates() {
		if next.DataVersion == report.DataVersion {
			for _, name := range sortedStrings(next.Slots) {
				if err := report.ValidateSlot(name, next.Slots[name]); err != nil {
					return nil, invalid("第 %d 版的插槽「%s」现在不合格：%v", version, name, err)
				}
			}
			for _, kind := range sortedStrings(next.Templates) {
				if err := report.ValidateTemplate(kind, next.Templates[kind]); err != nil {
					return nil, invalid("第 %d 版的报表模板「%s」现在不合格：%v", version, kind, err)
				}
			}
		} else {
			warns = append(warns, "自定义模板写于旧的报表数据版本，不会使用，请更新模板")
		}
	}
	assets, err := s.store.ListAssets(ctx, store.AssetOfStyle, id, false)
	if err != nil {
		return nil, err
	}
	have := map[string]string{}
	for _, a := range assets {
		have[a.Name] = a.SHA256
	}
	for _, a := range snap.Assets {
		switch sha, ok := have[a.Name]; {
		case !ok:
			warns = append(warns, fmt.Sprintf("图片「%s」现在不存在", a.Name))
		case sha != a.SHA256:
			warns = append(warns, fmt.Sprintf("图片「%s」已被替换，内容与第 %d 版时不同", a.Name, version))
		}
	}
	return s.updateStyle(ctx, src, rows, *row, base, next, "printstyle.restored", fmt.Sprintf("恢复第 %d 版", version), warns)
}

func assetHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// checkAssetTotals refuses images beyond the count and size an owner may
// hold; replacing an image counts the new bytes only.
func checkAssetTotals(existing []store.AssetRow, name string, newSize int) error {
	count, total := 1, newSize
	for _, a := range existing {
		if a.Name == name {
			continue
		}
		count++
		total += a.Size
	}
	if count > printstyle.MaxAssets {
		return invalid("图片最多 %d 张", printstyle.MaxAssets)
	}
	if total > printstyle.MaxAssetsBytes {
		return invalid("图片合计不能超过 %d MB", printstyle.MaxAssetsBytes>>20)
	}
	return nil
}

// assetOwner turns the scope of the asset methods: "style" or "meet".
func assetOwner(scope string) (store.AssetOwner, error) {
	switch scope {
	case string(store.AssetOfStyle):
		return store.AssetOfStyle, nil
	case string(store.AssetOfMeet):
		return store.AssetOfMeet, nil
	}
	return "", invalid("图片范围只能是 style 或 meet")
}

// ownerExists checks the style (not the built-in one) or the meet an image
// belongs to.
func (s *Service) ownerExists(ctx context.Context, owner store.AssetOwner, ownerID string) (rows []store.PrintStyleRow, meetName string, err error) {
	if owner == store.AssetOfMeet {
		m, err := s.GetMeet(ctx, ownerID)
		if err != nil {
			return nil, "", err
		}
		return nil, m.Name, nil
	}
	if ownerID == DefaultPrintStyleID {
		return nil, "", errBuiltinStyle
	}
	rows, err = s.allPrintStyles(ctx)
	if err != nil {
		return nil, "", err
	}
	if findStyle(rows, ownerID) == nil {
		return nil, "", errNoStyle()
	}
	return rows, "", nil
}

// ListAssets returns the images of a style or a meet, without their bytes.
func (s *Service) ListAssets(ctx context.Context, scope, ownerID string) ([]AssetInfo, error) {
	owner, err := assetOwner(scope)
	if err != nil {
		return nil, err
	}
	if owner == store.AssetOfStyle && ownerID == DefaultPrintStyleID {
		return []AssetInfo{}, nil
	}
	if _, _, err := s.ownerExists(ctx, owner, ownerID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListAssets(ctx, owner, ownerID, false)
	if err != nil {
		return nil, err
	}
	return assetInfos(rows), nil
}

// AssetDataURI returns an image as an embedded data URI, for a thumbnail.
func (s *Service) AssetDataURI(ctx context.Context, scope, ownerID, name string) (string, error) {
	owner, err := assetOwner(scope)
	if err != nil {
		return "", err
	}
	a, err := s.store.GetAsset(ctx, owner, ownerID, name)
	if err != nil {
		return "", err
	}
	if a == nil {
		return "", invalid("图片「%s」不存在", name)
	}
	return printstyle.Asset{Name: a.Name, MIME: a.MIME, Data: a.Data}.DataURI(), nil
}

// SaveAsset adds an image to a style or a meet, or replaces the one of the
// same name, at once. The image is checked by printstyle.ValidateAsset (png,
// jpeg or webp, at most 2 MB, the type agreeing with the bytes; mime ""
// is taken from the bytes), and an owner holds at most printstyle.MaxAssets
// images of printstyle.MaxAssetsBytes together. For a style the change is
// a save with a new version; a meet's images have no versions. An image
// that is already there unchanged writes nothing.
func (s *Service) SaveAsset(ctx context.Context, src store.Source, scope, ownerID, name, mime string, data []byte) (*AssetInfo, error) {
	owner, err := assetOwner(scope)
	if err != nil {
		return nil, err
	}
	if mime == "" {
		mime = printstyle.SniffImage(data)
	}
	if err := printstyle.ValidateAsset(printstyle.Asset{Name: name, MIME: mime, Data: data}); err != nil {
		return nil, invalid("%s", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, meetName, err := s.ownerExists(ctx, owner, ownerID)
	if err != nil {
		return nil, err
	}
	existing, err := s.store.ListAssets(ctx, owner, ownerID, false)
	if err != nil {
		return nil, err
	}
	sha := assetHash(data)
	info := &AssetInfo{Name: name, MIME: mime, SHA256: sha, Size: len(data)}
	for _, a := range existing {
		if a.Name == name && a.SHA256 == sha && a.MIME == mime {
			return info, nil
		}
	}
	if err := checkAssetTotals(existing, name, len(data)); err != nil {
		return nil, err
	}
	stmt := store.UpsertAsset(owner, store.AssetRow{ID: newID(), OwnerID: ownerID, Name: name, MIME: mime, SHA256: sha, Data: data})
	if owner == store.AssetOfMeet {
		return info, s.write(ctx, store.Audit{Source: src, Action: "asset.saved", MeetID: ownerID, Message: meetName, Detail: map[string]any{"name": name, "size": len(data)}},
			Change{Kind: "meets", MeetID: ownerID}, stmt)
	}
	after := replaceAsset(existing, store.AssetRow{Name: name, MIME: mime, SHA256: sha, Size: len(data)})
	if err := s.styleAssetsChanged(ctx, src, rows, ownerID, after, "图片「"+name+"」", "printstyle.asset", []sqlite.Statement{stmt}); err != nil {
		return nil, err
	}
	return info, nil
}

func replaceAsset(list []store.AssetRow, a store.AssetRow) []store.AssetRow {
	out := make([]store.AssetRow, 0, len(list)+1)
	replaced := false
	for _, e := range list {
		if e.Name == a.Name {
			out, replaced = append(out, a), true
			continue
		}
		out = append(out, e)
	}
	if !replaced {
		out = append(out, a)
	}
	slices.SortFunc(out, func(x, y store.AssetRow) int { return strings.Compare(x.Name, y.Name) })
	return out
}

// styleAssetsChanged writes an image change of a style together with the
// style's new version and its snapshot.
func (s *Service) styleAssetsChanged(ctx context.Context, src store.Source, rows []store.PrintStyleRow, id string, after []store.AssetRow, what, action string, stmts []sqlite.Statement) error {
	row := findStyle(rows, id)
	c, _ := contentOf(*row)
	version := row.Version + 1
	now := s.store.Now()
	updated := c.row(id, version, row.CreatedAt, now)
	stmts = append(stmts, store.UpdatePrintStyle(updated),
		store.InsertPrintStyleVersion(id, version, src, snapshotJSON(c, what, after), now),
		store.TrimPrintStyleVersions(id, version-maxPrintStyleVersions))
	return s.write(ctx, store.Audit{Source: src, Action: action, Message: c.Name, Detail: map[string]any{"styleId": id, "version": version, "reason": what}},
		Change{Kind: "printStyles"}, stmts...)
}

// DeleteAsset removes an image by name from a style or a meet, at once.
// References to it in CSS, templates and Markdown then show nothing and
// the report says so in its warnings.
func (s *Service) DeleteAsset(ctx context.Context, src store.Source, scope, ownerID, name string) error {
	owner, err := assetOwner(scope)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, meetName, err := s.ownerExists(ctx, owner, ownerID)
	if err != nil {
		return err
	}
	existing, err := s.store.ListAssets(ctx, owner, ownerID, false)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(existing, func(a store.AssetRow) bool { return a.Name == name })
	if i < 0 {
		return invalid("图片「%s」不存在", name)
	}
	stmt := store.DeleteAsset(owner, ownerID, name)
	if owner == store.AssetOfMeet {
		return s.write(ctx, store.Audit{Source: src, Action: "asset.deleted", MeetID: ownerID, Message: meetName, Detail: map[string]any{"name": name}},
			Change{Kind: "meets", MeetID: ownerID}, stmt)
	}
	after := slices.Delete(slices.Clone(existing), i, i+1)
	return s.styleAssetsChanged(ctx, src, rows, ownerID, after, "删除图片「"+name+"」", "printstyle.asset", []sqlite.Statement{stmt})
}

var assetRef = regexp.MustCompile(`(?i)asset:([^\s"'()<>]+)`)

// referencedAssets lists the asset: names a text refers to.
func referencedAssets(texts ...string) []string {
	var names []string
	for _, t := range texts {
		for _, m := range assetRef.FindAllStringSubmatch(t, -1) {
			name := m[1]
			if un, err := url.PathUnescape(name); err == nil {
				name = un
			}
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}
