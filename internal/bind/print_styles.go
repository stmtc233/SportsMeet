package bind

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/stmtc233/sportsmeet/internal/service"
)

// styleExt is the extension of a style file (printstyle.FileExt).
const styleExt = ".smstyle.json"

var styleFilter = []mygo.FileFilter{{Name: "打印样式", Extensions: []string{"json"}}}

var imageFilter = []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}

// PrintStyles manages 打印样式 (global to the app) and their images. The
// page edits the layout parameters, images and versions; CSS and templates
// are left to AI through MCP.
type PrintStyles struct{ *Core }

// List returns the built-in default first, then the saved styles.
func (p PrintStyles) List(ctx context.Context) ([]service.PrintStyleSummary, error) {
	return p.Svc.ListPrintStyles(ctx)
}

// ParamRules returns the limits of the layout parameters, for the form.
func (p PrintStyles) ParamRules() service.PrintParamRules {
	return p.Svc.PrintParamRules()
}

// Get returns one style; "default" is the built-in one.
func (p PrintStyles) Get(ctx context.Context, id string) (*service.PrintStyleDetail, error) {
	return p.Svc.GetPrintStyle(ctx, id)
}

// Save changes a style's parameters (or creates one with id ""), at once.
func (p PrintStyles) Save(ctx context.Context, id string, in service.PrintStyleInput) (*service.PrintStyleSaved, error) {
	return p.Svc.SavePrintStyle(ctx, ui, id, in)
}

// Rename gives a style another name.
func (p PrintStyles) Rename(ctx context.Context, id, name string) (*service.PrintStyleSaved, error) {
	return p.Svc.RenamePrintStyle(ctx, ui, id, name)
}

// Copy makes a new style of a style; name "" is the source's name with 「 副本」.
func (p PrintStyles) Copy(ctx context.Context, id, name string) (*service.PrintStyleSaved, error) {
	return p.Svc.CopyPrintStyle(ctx, ui, id, name)
}

// Delete removes a style; the meets that used it read as the default.
func (p PrintStyles) Delete(ctx context.Context, id string) error {
	return p.Svc.DeletePrintStyle(ctx, ui, id)
}

// Versions lists the kept snapshots of a style, newest first.
func (p PrintStyles) Versions(ctx context.Context, id string) ([]service.PrintStyleVersion, error) {
	return p.Svc.ListPrintStyleVersions(ctx, id)
}

// Restore brings a kept version back as a new save.
func (p PrintStyles) Restore(ctx context.Context, id string, version int) (*service.PrintStyleSaved, error) {
	return p.Svc.RestorePrintStyleVersion(ctx, ui, id, version)
}

// Assets lists the images of a style (scope "style") or a meet (scope "meet").
func (p PrintStyles) Assets(ctx context.Context, scope, ownerID string) ([]service.AssetInfo, error) {
	return p.Svc.ListAssets(ctx, scope, ownerID)
}

// AssetImage is an image as an embedded data URI, for a thumbnail.
func (p PrintStyles) AssetImage(ctx context.Context, scope, ownerID, name string) (string, error) {
	return p.Svc.AssetDataURI(ctx, scope, ownerID, name)
}

// UploadAsset asks for an image (png, jpeg or webp) and adds it to the
// style or the meet under its file name; nil when the user canceled.
func (p PrintStyles) UploadAsset(ctx context.Context, scope, ownerID string) (*service.AssetInfo, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: mygo.CallerWindow(ctx), Filters: imageFilter})
	if err != nil || len(paths) == 0 {
		return nil, ignoreCancel(err)
	}
	data, err := readFile(paths[0])
	if err != nil {
		return nil, err
	}
	return p.Svc.SaveAsset(ctx, ui, scope, ownerID, assetName(paths[0]), "", data)
}

// DeleteAsset removes an image by name.
func (p PrintStyles) DeleteAsset(ctx context.Context, scope, ownerID, name string) error {
	return p.Svc.DeleteAsset(ctx, ui, scope, ownerID, name)
}

// assetName is a file's name without its extension, with what an image's
// name cannot hold (white space, quotes, brackets, & % ; ,) turned into
// "-" and cut to 40 characters.
func assetName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	var b strings.Builder
	for _, r := range base {
		if r <= ' ' || r == 0x7f || r == 0x85 || r == 0xA0 || r == 0x3000 || r == 0x2028 || r == 0x2029 || strings.ContainsRune("\"'`()[]{}<>（）【】《》“”‘’「」\\&%;,", r) {
			r = '-'
		}
		b.WriteRune(r)
	}
	name := b.String()
	if utf8.RuneCountInString(name) > 40 {
		name = string([]rune(name)[:40])
	}
	return name
}

// Import asks for a style file and adds its style as a new one; nil when
// the user canceled.
func (p PrintStyles) Import(ctx context.Context) (*service.PrintStyleSaved, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: mygo.CallerWindow(ctx), Filters: styleFilter})
	if err != nil || len(paths) == 0 {
		return nil, ignoreCancel(err)
	}
	data, err := readFile(paths[0])
	if err != nil {
		return nil, err
	}
	return p.Svc.ImportPrintStyle(ctx, ui, data)
}

// Export asks where to save the style and writes it there. It returns the
// chosen path, or "" when the user canceled.
func (p PrintStyles) Export(ctx context.Context, id string) (string, error) {
	style, err := p.Svc.GetPrintStyle(ctx, id)
	if err != nil {
		return "", err
	}
	data, err := p.Svc.ExportPrintStyle(ctx, id)
	if err != nil {
		return "", err
	}
	name := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(style.Name) + styleExt
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: mygo.CallerWindow(ctx), DefaultPath: name, Filters: styleFilter})
	if err != nil || path == "" {
		return "", ignoreCancel(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", errors.New("文件无法写入")
	}
	return path, nil
}

// FrontPages manages a meet's 前置页.
type FrontPages struct{ *Core }

// List returns the meet's front pages in print order.
func (f FrontPages) List(ctx context.Context, meetID string) ([]service.FrontPageView, error) {
	return f.Svc.ListFrontPages(ctx, meetID)
}

// Save adds a front page (id "") or changes one; the result carries the
// warnings, such as an image the text refers to that is missing.
func (f FrontPages) Save(ctx context.Context, meetID, id string, in service.FrontPageInput) (*service.FrontPageSaved, error) {
	return f.Svc.SaveFrontPage(ctx, ui, meetID, id, in)
}

// Delete removes a front page.
func (f FrontPages) Delete(ctx context.Context, meetID, id string) error {
	return f.Svc.DeleteFrontPage(ctx, ui, meetID, id)
}

// Reorder sets the print order; ids name every front page once.
func (f FrontPages) Reorder(ctx context.Context, meetID string, ids []string) error {
	return f.Svc.ReorderFrontPages(ctx, ui, meetID, ids)
}

// SetPrintStyle makes the meet's reports use a print style ("" = 默认).
func (m Meets) SetPrintStyle(ctx context.Context, meetID, styleID string) error {
	return m.Svc.SetMeetPrintStyle(ctx, ui, meetID, styleID)
}
