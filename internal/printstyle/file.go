package printstyle

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// The style file: JSON, extension FileExt. Only version 1 exists; a reader
// reads every version that has been published.
const (
	FileFormat  = "sportsmeet-print-style"
	FileVersion = 1
	FileExt     = ".smstyle.json"

	maxFileBytes  = 32 << 20
	maxFileAssets = 30
	// MaxNameRunes is the longest style name.
	MaxNameRunes = 40
	maxNameRunes = MaxNameRunes
	// MaxAssets is the most images a style or a meet holds, MaxAssetsBytes
	// their sum: a style file carries them base64-encoded within maxFileBytes.
	MaxAssets      = maxFileAssets
	MaxAssetsBytes = 16 << 20
)

// ReportKinds are the reports a template may replace (the keys of
// File.Templates).
var ReportKinds = []string{"entries", "startlist", "results", "team_scores", "checkin", "recordsheet"}

// SlotNames are the places a report offers for small custom pieces: the
// cover page, the spot before the 代表队名单, the spot after the events, and
// the running head's left and right text and the foot's text.
var SlotNames = []string{"cover", "before_rosters", "after_events", "header_left", "header_right", "footer"}

// File is a style as exchanged between machines.
type File struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Name    string `json:"name"`
	Params  Params `json:"params"`
	CSS     string `json:"css"`
	// Slots are templates by slot name; Templates replace a whole report,
	// by report kind. Both are written against the report data of
	// DataVersion.
	Slots       map[string]string `json:"slots"`
	Templates   map[string]string `json:"templates"`
	DataVersion int               `json:"dataVersion"`
	Assets      []FileAsset       `json:"assets"`
}

// FileAsset is an image in a style file; Data is base64.
type FileAsset struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data string `json:"data"`
}

// Encode writes the style file, validating it first.
func Encode(f File) ([]byte, error) {
	f.Format, f.Version = FileFormat, FileVersion
	if f.Slots == nil {
		f.Slots = map[string]string{}
	}
	if f.Templates == nil {
		f.Templates = map[string]string{}
	}
	if f.Assets == nil {
		f.Assets = []FileAsset{}
	}
	if _, _, err := f.check(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// AssetMap is the file's images by name.
func (f File) AssetMap() (map[string]Asset, error) {
	out := map[string]Asset{}
	for _, a := range f.Assets {
		data, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil {
			return nil, fmt.Errorf("图片「%s」的数据不是 base64", a.Name)
		}
		out[a.Name] = Asset{Name: a.Name, MIME: a.MIME, Data: data}
	}
	return out, nil
}

type fileWire struct {
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Params      json.RawMessage   `json:"params"`
	CSS         string            `json:"css"`
	Slots       map[string]string `json:"slots"`
	Templates   map[string]string `json:"templates"`
	DataVersion int               `json:"dataVersion"`
	Assets      []FileAsset       `json:"assets"`
}

// Decode reads a style file. Params that are missing fields take the
// defaults; the CSS comes back cleaned (what was removed is in warnings);
// templates are parsed; images are checked. Every problem is in the error,
// one a line.
func Decode(data []byte) (File, []string, error) {
	if len(data) > maxFileBytes {
		return File{}, nil, fmt.Errorf("样式文件超过 %d MB", maxFileBytes>>20)
	}
	var w fileWire
	if err := json.Unmarshal(data, &w); err != nil {
		return File{}, nil, errors.New("不是有效的样式文件（JSON 格式不对）")
	}
	if w.Format != FileFormat {
		return File{}, nil, errors.New("不是打印样式文件")
	}
	if w.Version < 1 || w.Version > FileVersion {
		return File{}, nil, fmt.Errorf("样式文件版本 %d 太新，请更新软件后再导入", w.Version)
	}
	f := File{Format: w.Format, Version: w.Version, Name: w.Name, CSS: w.CSS, Slots: w.Slots, Templates: w.Templates, DataVersion: w.DataVersion, Assets: w.Assets}
	var problems []string
	p, err := ParamsFromJSON(w.Params)
	if err != nil {
		problems = append(problems, err.Error())
	}
	f.Params = p
	if f.Slots == nil {
		f.Slots = map[string]string{}
	}
	if f.Templates == nil {
		f.Templates = map[string]string{}
	}
	if f.Assets == nil {
		f.Assets = []FileAsset{}
	}
	var warns []string
	var checkProblems error
	f, warns, checkProblems = f.check()
	if checkProblems != nil {
		problems = append(problems, strings.Split(checkProblems.Error(), "\n")...)
	}
	if len(problems) > 0 {
		return File{}, warns, errors.New(strings.Join(problems, "\n"))
	}
	return f, warns, nil
}

// check validates what Params.Validate and the file's other fields say,
// cleaning the CSS on the way.
func (f File) check() (File, []string, error) {
	var problems, warns []string
	if n := utf8.RuneCountInString(strings.TrimSpace(f.Name)); n < 1 || n > maxNameRunes {
		problems = append(problems, fmt.Sprintf("样式名称需要 1–%d 个字", maxNameRunes))
	}
	if err := f.Params.Validate(); err != nil {
		problems = append(problems, err.Error())
	}
	css, w, err := SanitizeCSS(f.CSS)
	warns = append(warns, w...)
	if err != nil {
		problems = append(problems, err.Error())
	}
	f.CSS = css
	for _, name := range sortedKeys(f.Slots) {
		if !slices.Contains(SlotNames, name) {
			problems = append(problems, fmt.Sprintf("没有名为「%s」的插槽，可用 %s", name, strings.Join(SlotNames, "、")))
			continue
		}
		if _, err := CompileTemplate(f.Slots[name]); err != nil {
			problems = append(problems, fmt.Sprintf("插槽「%s」：%v", name, err))
		}
	}
	for _, kind := range sortedKeys(f.Templates) {
		if !slices.Contains(ReportKinds, kind) {
			problems = append(problems, fmt.Sprintf("没有名为「%s」的报表，可用 %s", kind, strings.Join(ReportKinds, "、")))
			continue
		}
		if _, err := CompileTemplate(f.Templates[kind]); err != nil {
			problems = append(problems, fmt.Sprintf("报表模板「%s」：%v", kind, err))
		}
	}
	if (len(f.Slots) > 0 || len(f.Templates) > 0) && f.DataVersion < 1 {
		problems = append(problems, "有模板时需要写明 dataVersion")
	}
	if len(f.Assets) > maxFileAssets {
		problems = append(problems, fmt.Sprintf("图片最多 %d 张", maxFileAssets))
	}
	seen := map[string]bool{}
	for _, a := range f.Assets {
		if seen[a.Name] {
			problems = append(problems, fmt.Sprintf("图片「%s」重名", a.Name))
		}
		seen[a.Name] = true
		data, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("图片「%s」的数据不是 base64", a.Name))
			continue
		}
		if err := ValidateAsset(Asset{Name: a.Name, MIME: a.MIME, Data: data}); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return f, warns, errors.New(strings.Join(problems, "\n"))
	}
	return f, warns, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
