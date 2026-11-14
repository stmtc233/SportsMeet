// Package bind exposes the service to the page through mygo bindings.
// It is a thin layer: every method forwards to internal/service with
// source ui; the only things it adds are native dialogs and files.
// `mygo generate` turns these into src/mygo.ts.
package bind

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/mcp"
	"github.com/stmtc233/sportsmeet/internal/scoring"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// DataChanged is broadcast after every committed write (UI or MCP); pages
// reload the data it names.
var DataChanged = mygo.NewEvent[service.Change]("dataChanged")

// Notify is the service's change callback.
func Notify(c service.Change) { _ = DataChanged.Broadcast(c) }

// Core holds the service every binding forwards to; main fills it once
// the data directory is known.
type Core struct {
	Svc *service.Service
	MCP *mcp.Manager
}

const ui = store.SourceUI

// maxFile bounds files read from disk on the page's behalf.
const maxFile = 32 << 20

func readFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.New("文件无法读取")
	}
	if info.Size() > maxFile {
		return nil, errors.New("文件过大")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("文件无法读取")
	}
	return data, nil
}

func ignoreCancel(err error) error {
	if errors.Is(err, mygo.ErrCanceled) {
		return nil
	}
	return err
}

// Meets manages 赛事.
type Meets struct{ *Core }

// List returns all meets, newest first.
func (m Meets) List(ctx context.Context) ([]service.MeetView, error) { return m.Svc.ListMeets(ctx) }

// Get returns one meet.
func (m Meets) Get(ctx context.Context, id string) (*service.MeetView, error) {
	return m.Svc.MeetDetail(ctx, id)
}

// Count returns what deleting the meet would remove.
func (m Meets) Count(ctx context.Context, id string) (store.MeetCounts, error) {
	return m.Svc.CountMeet(ctx, id)
}

// Create creates a meet.
func (m Meets) Create(ctx context.Context, in service.MeetInput) (*domain.Meet, error) {
	return m.Svc.CreateMeet(ctx, ui, in)
}

// Update changes a meet.
func (m Meets) Update(ctx context.Context, id string, in service.MeetInput) error {
	return m.Svc.UpdateMeet(ctx, ui, id, in)
}

// Delete removes a meet and everything in it.
func (m Meets) Delete(ctx context.Context, id string) error { return m.Svc.DeleteMeet(ctx, ui, id) }

// Combined manages owned all-around components and their shared roster.
type Combined struct{ *Core }

func (c Combined) List(ctx context.Context, meetID string) ([]service.CombinedRowSummary, error) {
	return c.Svc.ListCombined(ctx, meetID)
}
func (c Combined) Get(ctx context.Context, id string) (*service.CombinedDetail, error) {
	return c.Svc.GetCombined(ctx, id)
}
func (c Combined) Save(ctx context.Context, meetID, id string, in service.CombinedInput) (*domain.Combined, error) {
	return c.Svc.SaveCombined(ctx, ui, meetID, id, in)
}
func (c Combined) Entries(ctx context.Context, id string, athleteIDs []string) error {
	return c.Svc.SetCombinedEntries(ctx, ui, id, athleteIDs)
}
// LimitWarnings says which 报名限制 the full new roster would exceed.
func (c Combined) LimitWarnings(ctx context.Context, id string, athleteIDs []string) ([]string, error) {
	return c.Svc.CombinedLimitWarnings(ctx, id, athleteIDs)
}
func (c Combined) Delete(ctx context.Context, id string) error {
	return c.Svc.DeleteCombined(ctx, ui, id)
}
func (c Combined) ParseRule(in service.EventInput, rule service.PointRuleInput) (*service.PointRulePreview, error) {
	return c.Svc.ParsePointRule(in, rule)
}
func (c Combined) PreviewRules(ctx context.Context, req service.CombinedRulesRequest) (*service.CombinedRulesPreview, error) {
	return c.Svc.PreviewCombinedRules(ctx, req)
}
func (c Combined) ApplyRules(ctx context.Context, req service.CombinedRulesRequest, hash string, confirmed bool) (*service.CombinedRulesPreview, error) {
	return c.Svc.ApplyCombinedRules(ctx, ui, req, hash, confirmed)
}

// Units manages 参赛单位.
type Units struct{ *Core }

// List returns a meet's units.
func (u Units) List(ctx context.Context, meetID string) ([]domain.Unit, error) {
	return u.Svc.ListUnits(ctx, meetID)
}

// Save creates (id "") or updates a unit.
func (u Units) Save(ctx context.Context, meetID, id string, in service.UnitInput) (*domain.Unit, error) {
	return u.Svc.SaveUnit(ctx, ui, meetID, id, in)
}

// Delete removes a unit.
func (u Units) Delete(ctx context.Context, meetID, id string) error {
	return u.Svc.DeleteUnit(ctx, ui, meetID, id)
}

// Athletes manages 运动员 and their table import.
type Athletes struct{ *Core }

// List returns a meet's athletes.
func (a Athletes) List(ctx context.Context, meetID string) ([]service.AthleteRow, error) {
	return a.Svc.ListAthletes(ctx, meetID)
}

// Save creates (id "") or updates an athlete.
func (a Athletes) Save(ctx context.Context, meetID, id string, in service.AthleteInput) (*service.AthleteSave, error) {
	return a.Svc.SaveAthleteWithWarnings(ctx, ui, meetID, id, in)
}

// Delete removes athletes.
func (a Athletes) Delete(ctx context.Context, meetID string, ids []string) error {
	return a.Svc.DeleteAthletes(ctx, ui, meetID, ids)
}

// ParseTable splits pasted text into rows and guesses its columns.
func (Athletes) ParseTable(text string) (service.Table, error) { return service.ParseTable(text) }

// PickTable asks for a CSV or text file and returns its content, or ""
// when the user canceled.
func (Athletes) PickTable(ctx context.Context) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:  mygo.CallerWindow(ctx),
		Filters: []mygo.FileFilter{{Name: "表格", Extensions: []string{"csv", "tsv", "txt"}}},
	})
	if err != nil || len(paths) == 0 {
		return "", ignoreCancel(err)
	}
	data, err := readFile(paths[0])
	return string(data), err
}

// PreviewImport reports what importing the table would do.
func (a Athletes) PreviewImport(ctx context.Context, meetID string, in service.AthleteImport) (*service.AthleteImportPreview, error) {
	return a.Svc.PreviewAthleteImport(ctx, meetID, in)
}

// Import imports the table.
func (a Athletes) Import(ctx context.Context, meetID string, in service.AthleteImport) (*service.AthleteImportPreview, error) {
	return a.Svc.ImportAthletes(ctx, ui, meetID, in)
}

// Events manages 项目.
type Events struct{ *Core }

func (e Events) TeamWarnings(ctx context.Context, eventID string, memberIDs []string) ([]string, error) {
	return e.Svc.TeamWarnings(ctx, eventID, memberIDs)
}

func (e Events) Presets() []service.EventPreset { return e.Svc.EventPresets() }

func (e Events) Rules(in service.EventInput) (*domain.EventRules, error) { return e.Svc.EventRules(in) }

func (e Events) TeamSizeWarning(event domain.Event, count int) string {
	return e.Svc.TeamSizeWarning(event, count)
}

// List returns a meet's events with their progress.
func (e Events) List(ctx context.Context, meetID string) ([]service.EventRow, error) {
	return e.Svc.ListEvents(ctx, meetID)
}

// Get returns one event with its entries, rounds, heats and results.
func (e Events) Get(ctx context.Context, id string) (*service.EventDetail, error) {
	return e.Svc.GetEvent(ctx, id)
}

// Count returns what deleting the event would remove.
func (e Events) Count(ctx context.Context, id string) (store.MeetCounts, error) {
	return e.Svc.CountEvent(ctx, id)
}

// Save creates (id "") or updates an event.
func (e Events) Save(ctx context.Context, meetID, id string, in service.EventInput) (*domain.Event, error) {
	return e.Svc.SaveEvent(ctx, ui, meetID, id, in)
}

// Delete removes an event with its heats and results.
func (e Events) Delete(ctx context.Context, id string) error {
	return e.Svc.DeleteEvent(ctx, ui, id, true)
}

// Entries manages 报名.
type Entries struct{ *Core }

// Add enters athletes into an event.
func (e Entries) Add(ctx context.Context, eventID string, list []service.EntryInput) error {
	return e.Svc.AddEntries(ctx, ui, eventID, list)
}

// LimitWarnings says which 报名限制 entering the athletes or teams would exceed.
func (e Entries) LimitWarnings(ctx context.Context, eventID string, ids []string) ([]string, error) {
	return e.Svc.EntryLimitWarnings(ctx, eventID, ids)
}

// SetSeed changes an entry mark; "" clears it.
func (e Entries) SetSeed(ctx context.Context, eventID, competitorID, seed string) error {
	return e.Svc.SetSeed(ctx, ui, eventID, competitorID, seed)
}

// Remove withdraws athletes from an event.
func (e Entries) Remove(ctx context.Context, eventID string, competitorIDs []string) error {
	return e.Svc.RemoveEntries(ctx, ui, eventID, competitorIDs)
}

// Arrange plans and saves heats.
type Arrange struct{ *Core }

// Plan computes new heats for those not yet placed; it writes nothing.
func (a Arrange) Plan(ctx context.Context, req service.PlanRequest) ([]arrange.PlannedHeat, error) {
	return a.Svc.PlanHeats(ctx, req)
}

// Save applies the arrange page's draft.
func (a Arrange) Save(ctx context.Context, in service.BoardSave) (service.EventDetail, error) {
	if err := a.Svc.SaveBoard(ctx, ui, in); err != nil {
		return service.EventDetail{}, err
	}
	saved, err := a.Svc.GetEvent(ctx, in.EventID)
	if err != nil {
		return service.EventDetail{}, err
	}
	return *saved, nil
}

// Results records marks.
type Results struct{ *Core }

// Save records what was typed for slots of a heat.
func (r Results) Save(ctx context.Context, heatID string, inputs []service.ResultInput) error {
	return r.Svc.SaveResults(ctx, ui, heatID, inputs)
}

func (r Results) PreviewHeight(inputs []service.ResultInput) service.HeightPreview {
	return service.PreviewHeight(inputs)
}

// SetStarted records, changes or clears when a heat actually started.
func (r Results) SetStarted(ctx context.Context, in service.HeatStartInput) (*service.HeatStartResult, error) {
	return r.Svc.SetHeatStarted(ctx, ui, in)
}

// RuleTies records a referee's ruling on ties of a round.
func (r Results) RuleTies(ctx context.Context, in service.TieRulingInput) error {
	return r.Svc.RuleTies(ctx, ui, in)
}

// Reports renders and prints 秩序册, 成绩册, 检录单 and 成绩记录表.
type Reports struct{ *Core }

func (r Reports) Targets(ctx context.Context, meetID string) ([]service.ReportTarget, error) {
	return r.Svc.ReportTargets(ctx, meetID)
}

// Render returns the report as an HTML document, for the preview.
func (r Reports) Render(ctx context.Context, kind, meetID string, eventIDs []string) (string, error) {
	return r.Svc.RenderReport(ctx, kind, meetID, eventIDs)
}

// CheckInTargets lists the rounds the 检录单 can print and its days.
func (r Reports) CheckInTargets(ctx context.Context, meetID string) (*service.CheckInTargets, error) {
	return r.Svc.CheckInTargets(ctx, meetID)
}

// RenderCheckIn returns the 检录单 as an HTML document, for the preview.
func (r Reports) RenderCheckIn(ctx context.Context, req service.CheckInRequest) (string, error) {
	return r.Svc.RenderCheckIn(ctx, req)
}

// RenderRecordSheet returns the 成绩记录表 as an HTML document, for the preview.
func (r Reports) RenderRecordSheet(ctx context.Context, req service.CheckInRequest) (string, error) {
	return r.Svc.RenderRecordSheet(ctx, req)
}

// PrintPages opens the preview's laid-out sheets (paginate.ts printable) in
// a window of its own and shows the system's print dialog, where they can
// also be saved as a PDF. The sheets are printed rather than Go's document
// so that the pages are the ones the preview shows. The main window is not
// printed: it would print the app's sidebar too.
func (r Reports) PrintPages(title, html string) error {
	win := mygo.NewWindow(mygo.WindowOptions{Title: title, Width: 900, Height: 760})
	page := win.Page()
	var once sync.Once
	page.OnDidFinishLoad(func() { once.Do(page.Print) })
	page.LoadHTML(html, "")
	return nil
}

// Exchange imports and exports interchange files (.meet.json).
type Exchange struct {
	*Core
	mu     sync.Mutex
	picked map[string][]byte // the file the user chose, by path
}

var meetFilter = []mygo.FileFilter{{Name: "赛事文件", Extensions: []string{"json"}}}

// Export asks where to save the meet and writes it there. It returns the
// chosen path, or "" when the user canceled.
func (x *Exchange) Export(ctx context.Context, meetID string) (string, error) {
	f, err := x.Svc.ExportMeet(ctx, meetID)
	if err != nil {
		return "", err
	}
	data, err := interchange.Encode(f)
	if err != nil {
		return "", err
	}
	name := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(f.Meet.Name) + ".meet.json"
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: mygo.CallerWindow(ctx), DefaultPath: name, Filters: meetFilter})
	if err != nil || path == "" {
		return "", ignoreCancel(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", errors.New("文件无法写入")
	}
	return path, nil
}

// PickedFile is a file chosen for import.
type PickedFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// Pick asks for an interchange file and remembers its content; Path is ""
// when the user canceled.
func (x *Exchange) Pick(ctx context.Context) (PickedFile, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: mygo.CallerWindow(ctx), Filters: meetFilter})
	if err != nil || len(paths) == 0 {
		return PickedFile{}, ignoreCancel(err)
	}
	data, err := readFile(paths[0])
	if err != nil {
		return PickedFile{}, err
	}
	x.mu.Lock()
	x.picked = map[string][]byte{paths[0]: data}
	x.mu.Unlock()
	return PickedFile{Path: paths[0], Name: filepath.Base(paths[0])}, nil
}

func (x *Exchange) data(path string) ([]byte, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	data, ok := x.picked[path]
	if !ok {
		return nil, errors.New("请重新选择文件")
	}
	return data, nil
}

// Preview reports what importing the picked file into meetID ("" = as a
// new meet) would do.
func (x *Exchange) Preview(ctx context.Context, path, meetID string) (*service.ImportPreview, error) {
	data, err := x.data(path)
	if err != nil {
		return nil, err
	}
	return x.Svc.PreviewImport(ctx, data, meetID)
}

// Import imports the picked file as previewed (hash is the preview's).
func (x *Exchange) Import(ctx context.Context, path, meetID, hash string) (*service.ImportPreview, error) {
	data, err := x.data(path)
	if err != nil {
		return nil, err
	}
	return x.Svc.ImportMeet(ctx, ui, data, meetID, hash)
}

// Marks parses and formats marks; the page never does it itself.
type Marks struct{}

func (Marks) NormalizeHeight(input string) (string, error) {
	h, err := scoring.ParseMark(domain.KindHeight, input)
	if err != nil || h <= 0 || h%10 != 0 {
		return "", errors.New("高度需为正数，精确到厘米")
	}
	return scoring.FormatMark(domain.KindHeight, h), nil
}

// Parse reads typed input as a mark of kind.
func (Marks) Parse(kind domain.EventKind, input string) (int64, error) {
	return scoring.ParseMark(kind, input)
}

// Format shows a mark of kind.
func (Marks) NormalizeEvent(event domain.Event, input string) (string, error) {
	if err := event.NormalizeConfig(); err != nil {
		return "", err
	}
	mark, err := scoring.ParseEventMark(event, input)
	if err != nil {
		return "", err
	}
	return scoring.FormatEventMark(event, mark), nil
}

func (Marks) Format(kind domain.EventKind, mark int64) string { return scoring.FormatMark(kind, mark) }

// Normalize parses typed input and returns it as marks are shown, so a
// field can tidy what was typed.
func (Marks) Normalize(kind domain.EventKind, input string) (string, error) {
	m, err := scoring.ParseMark(kind, input)
	if err != nil {
		return "", err
	}
	return scoring.FormatMark(kind, m), nil
}

// EventName composes 「男子100m 初中组」 for previews.
func (Marks) EventName(gender domain.Gender, discipline, group string) string {
	return domain.ComposeEventName(gender, discipline, group)
}

// AppInfo describes the running app.
type AppInfo struct {
	Version       string `json:"version"`
	CanSelfUpdate bool   `json:"canSelfUpdate"`
	UpdateMessage string `json:"updateMessage"`
	RepositoryURL string `json:"repositoryURL"`
}

const repositoryURL = "https://github.com/stmtc233/SportsMeet"

// App tells the page about the app itself.
type App struct{}

// Info describes the running build and how it can receive updates.
func (App) Info() AppInfo {
	info := AppInfo{Version: service.Version, CanSelfUpdate: mygo.Updater.Enabled(), RepositoryURL: repositoryURL}
	if !info.CanSelfUpdate {
		info.UpdateMessage = "当前安装方式需手动下载安装"
		if mygo.IsDev() {
			info.UpdateMessage = "开发运行，请从发布页下载安装"
		}
	}
	return info
}

// CheckForUpdates uses mygo's update window for installable builds. Other
// builds open releases in the system browser so users can choose a download.
func (App) CheckForUpdates() error {
	if !mygo.Updater.Enabled() {
		return mygo.Shell.OpenExternal(repositoryURL + "/releases")
	}
	updater.CheckForUpdates()
	return nil
}

// OpenRepository opens the project's repository in the system browser.
func (App) OpenRepository() error { return mygo.Shell.OpenExternal(repositoryURL) }
