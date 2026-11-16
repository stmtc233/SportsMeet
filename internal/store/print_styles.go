package store

import (
	"context"

	"github.com/egoist/mygo/plugins/sqlite"
)

// PrintStyleRow is a stored print style (docs: print styles are global to
// the app, not to a meet). The JSON columns hold what internal/printstyle
// defines; the store does not read them.
type PrintStyleRow struct {
	ID            string
	Name          string
	ParamsJSON    string
	CSS           string
	SlotsJSON     string
	TemplatesJSON string
	DataVersion   int
	Version       int
	CreatedAt     int64
	UpdatedAt     int64
}

const printStyleColumns = `id, name, params_json, css, slots_json, templates_json, data_version, version, created_at, updated_at`

func scanPrintStyle(r []any) PrintStyleRow {
	return PrintStyleRow{ID: str(r[0]), Name: str(r[1]), ParamsJSON: str(r[2]), CSS: str(r[3]), SlotsJSON: str(r[4]), TemplatesJSON: str(r[5]),
		DataVersion: int(i64(r[6])), Version: int(i64(r[7])), CreatedAt: i64(r[8]), UpdatedAt: i64(r[9])}
}

// ListPrintStyles returns every style, oldest first.
func (s *Store) ListPrintStyles(ctx context.Context) ([]PrintStyleRow, error) {
	rows, err := s.Query(ctx, `SELECT `+printStyleColumns+` FROM print_styles ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	out := make([]PrintStyleRow, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, scanPrintStyle(r))
	}
	return out, nil
}

// GetPrintStyle returns one style, nil when there is none.
func (s *Store) GetPrintStyle(ctx context.Context, id string) (*PrintStyleRow, error) {
	rows, err := s.Query(ctx, `SELECT `+printStyleColumns+` FROM print_styles WHERE id = ?`, id)
	if err != nil || len(rows.Values) == 0 {
		return nil, err
	}
	r := scanPrintStyle(rows.Values[0])
	return &r, nil
}

// InsertPrintStyle is the statement creating r.
func InsertPrintStyle(r PrintStyleRow) sqlite.Statement {
	return Stmt(`INSERT INTO print_styles(`+printStyleColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.ParamsJSON, r.CSS, r.SlotsJSON, r.TemplatesJSON, r.DataVersion, r.Version, r.CreatedAt, r.UpdatedAt)
}

// UpdatePrintStyle is the statement writing every field of r but its id and
// creation time.
func UpdatePrintStyle(r PrintStyleRow) sqlite.Statement {
	return Stmt(`UPDATE print_styles SET name = ?, params_json = ?, css = ?, slots_json = ?, templates_json = ?,
		data_version = ?, version = ?, updated_at = ? WHERE id = ?`,
		r.Name, r.ParamsJSON, r.CSS, r.SlotsJSON, r.TemplatesJSON, r.DataVersion, r.Version, r.UpdatedAt, r.ID)
}

// DeletePrintStyle is the statement removing a style; its versions and
// images go with it (foreign keys cascade).
func DeletePrintStyle(id string) sqlite.Statement {
	return Stmt(`DELETE FROM print_styles WHERE id = ?`, id)
}

// PrintStyleVersionRow is a saved snapshot of a style.
type PrintStyleVersionRow struct {
	Version      int
	Source       Source
	SnapshotJSON string
	CreatedAt    int64
}

// InsertPrintStyleVersion is the statement keeping a snapshot.
func InsertPrintStyleVersion(styleID string, version int, src Source, snapshot string, now int64) sqlite.Statement {
	return Stmt(`INSERT INTO print_style_versions(style_id, version, source, snapshot_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		styleID, version, string(src), snapshot, now)
}

// TrimPrintStyleVersions is the statement dropping the snapshots numbered
// at most through.
func TrimPrintStyleVersions(styleID string, through int) sqlite.Statement {
	return Stmt(`DELETE FROM print_style_versions WHERE style_id = ? AND version <= ?`, styleID, through)
}

// ListPrintStyleVersions returns a style's snapshots, newest first.
func (s *Store) ListPrintStyleVersions(ctx context.Context, styleID string) ([]PrintStyleVersionRow, error) {
	rows, err := s.Query(ctx, `SELECT version, source, snapshot_json, created_at FROM print_style_versions WHERE style_id = ? ORDER BY version DESC`, styleID)
	if err != nil {
		return nil, err
	}
	out := make([]PrintStyleVersionRow, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, PrintStyleVersionRow{Version: int(i64(r[0])), Source: Source(str(r[1])), SnapshotJSON: str(r[2]), CreatedAt: i64(r[3])})
	}
	return out, nil
}

// GetPrintStyleVersion returns one snapshot, nil when it was never kept or
// has been trimmed.
func (s *Store) GetPrintStyleVersion(ctx context.Context, styleID string, version int) (*PrintStyleVersionRow, error) {
	rows, err := s.Query(ctx, `SELECT version, source, snapshot_json, created_at FROM print_style_versions WHERE style_id = ? AND version = ?`, styleID, version)
	if err != nil || len(rows.Values) == 0 {
		return nil, err
	}
	r := rows.Values[0]
	return &PrintStyleVersionRow{Version: int(i64(r[0])), Source: Source(str(r[1])), SnapshotJSON: str(r[2]), CreatedAt: i64(r[3])}, nil
}

// AssetOwner says whose image it is: a print style's or a meet's.
type AssetOwner string

const (
	AssetOfStyle AssetOwner = "style"
	AssetOfMeet  AssetOwner = "meet"
)

// table and column are fixed strings, never user input.
func (o AssetOwner) table() (table, column string) {
	if o == AssetOfMeet {
		return "meet_assets", "meet_id"
	}
	return "print_style_assets", "style_id"
}

// AssetRow is a stored image. ListAssets leaves Data nil unless asked.
type AssetRow struct {
	ID      string
	OwnerID string
	Name    string
	MIME    string
	SHA256  string
	Size    int
	Data    []byte
}

// ListAssets returns an owner's images by name; withData also reads the
// image bytes, which the list for a page does not need.
func (s *Store) ListAssets(ctx context.Context, owner AssetOwner, ownerID string, withData bool) ([]AssetRow, error) {
	table, col := owner.table()
	q := `SELECT id, name, mime, sha256, length(data) FROM ` + table + ` WHERE ` + col + ` = ? ORDER BY name`
	if withData {
		q = `SELECT id, name, mime, sha256, length(data), data FROM ` + table + ` WHERE ` + col + ` = ? ORDER BY name`
	}
	rows, err := s.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]AssetRow, 0, len(rows.Values))
	for _, r := range rows.Values {
		a := AssetRow{ID: str(r[0]), OwnerID: ownerID, Name: str(r[1]), MIME: str(r[2]), SHA256: str(r[3]), Size: int(i64(r[4]))}
		if withData {
			a.Data, _ = r[5].([]byte)
		}
		out = append(out, a)
	}
	return out, nil
}

// GetAsset returns one image with its bytes, nil when there is none.
func (s *Store) GetAsset(ctx context.Context, owner AssetOwner, ownerID, name string) (*AssetRow, error) {
	table, col := owner.table()
	rows, err := s.Query(ctx, `SELECT id, name, mime, sha256, length(data), data FROM `+table+` WHERE `+col+` = ? AND name = ?`, ownerID, name)
	if err != nil || len(rows.Values) == 0 {
		return nil, err
	}
	r := rows.Values[0]
	a := AssetRow{ID: str(r[0]), OwnerID: ownerID, Name: str(r[1]), MIME: str(r[2]), SHA256: str(r[3]), Size: int(i64(r[4]))}
	a.Data, _ = r[5].([]byte)
	return &a, nil
}

// UpsertAsset is the statement adding an image, or replacing the one of the
// same name (its id stays).
func UpsertAsset(owner AssetOwner, a AssetRow) sqlite.Statement {
	table, col := owner.table()
	return Stmt(`INSERT INTO `+table+`(id, `+col+`, name, mime, data, sha256) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(`+col+`, name) DO UPDATE SET mime = excluded.mime, data = excluded.data, sha256 = excluded.sha256`,
		a.ID, a.OwnerID, a.Name, a.MIME, a.Data, a.SHA256)
}

// DeleteAsset is the statement removing an image by name.
func DeleteAsset(owner AssetOwner, ownerID, name string) sqlite.Statement {
	table, col := owner.table()
	return Stmt(`DELETE FROM `+table+` WHERE `+col+` = ? AND name = ?`, ownerID, name)
}

// FrontPageRow is a meet's front page; its Markdown is rendered when a
// report is.
type FrontPageRow struct {
	ID          string
	MeetID      string
	Position    int
	Title       string
	BodyMD      string
	ReportsJSON string
	CreatedAt   int64
	UpdatedAt   int64
}

const frontPageColumns = `id, meet_id, position, title, body_md, reports_json, created_at, updated_at`

func scanFrontPage(r []any) FrontPageRow {
	return FrontPageRow{ID: str(r[0]), MeetID: str(r[1]), Position: int(i64(r[2])), Title: str(r[3]), BodyMD: str(r[4]), ReportsJSON: str(r[5]), CreatedAt: i64(r[6]), UpdatedAt: i64(r[7])}
}

// ListFrontPages returns a meet's front pages in print order.
func (s *Store) ListFrontPages(ctx context.Context, meetID string) ([]FrontPageRow, error) {
	rows, err := s.Query(ctx, `SELECT `+frontPageColumns+` FROM meet_front_pages WHERE meet_id = ? ORDER BY position, rowid`, meetID)
	if err != nil {
		return nil, err
	}
	out := make([]FrontPageRow, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, scanFrontPage(r))
	}
	return out, nil
}

// InsertFrontPage is the statement creating p.
func InsertFrontPage(p FrontPageRow) sqlite.Statement {
	return Stmt(`INSERT INTO meet_front_pages(`+frontPageColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.MeetID, p.Position, p.Title, p.BodyMD, p.ReportsJSON, p.CreatedAt, p.UpdatedAt)
}

// UpdateFrontPage is the statement writing p's text and reports.
func UpdateFrontPage(p FrontPageRow) sqlite.Statement {
	return Stmt(`UPDATE meet_front_pages SET title = ?, body_md = ?, reports_json = ?, updated_at = ? WHERE id = ? AND meet_id = ?`,
		p.Title, p.BodyMD, p.ReportsJSON, p.UpdatedAt, p.ID, p.MeetID)
}

// SetFrontPagePosition is the statement moving a front page.
func SetFrontPagePosition(meetID, id string, position int) sqlite.Statement {
	return Stmt(`UPDATE meet_front_pages SET position = ? WHERE id = ? AND meet_id = ?`, position, id, meetID)
}

// DeleteFrontPage is the statement removing a front page.
func DeleteFrontPage(meetID, id string) sqlite.Statement {
	return Stmt(`DELETE FROM meet_front_pages WHERE id = ? AND meet_id = ?`, id, meetID)
}

// MeetStyleUse is a meet and the print style its settings name.
type MeetStyleUse struct {
	MeetID   string
	MeetName string
	StyleID  string // "" = the default
}

// ListMeetStyleUse reads which style every meet uses, in one query. The
// style is a field of the meet's settings JSON, empty settings are valid.
func (s *Store) ListMeetStyleUse(ctx context.Context) ([]MeetStyleUse, error) {
	rows, err := s.Query(ctx, `SELECT id, name, CASE WHEN json_valid(settings) THEN json_extract(settings, '$.printStyleId') END
		FROM meets ORDER BY start_date DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]MeetStyleUse, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, MeetStyleUse{MeetID: str(r[0]), MeetName: str(r[1]), StyleID: str(r[2])})
	}
	return out, nil
}
