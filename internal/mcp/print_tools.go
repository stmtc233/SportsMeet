package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// Print styles, front pages and images are written at once, without a
// preview and a confirmation (docs/agents/mcp.md「打印样式」): they are not
// competition records, and a style keeps its last versions to go back to.

// printStyleInput is service.PrintStyleInput with every field optional and
// the parameters as objects (paramsPatch may also be the text of one).
type printStyleInput struct {
	Name        *string           `json:"name,omitempty" jsonschema:"样式名称，1–40 个字"`
	Params      any               `json:"params,omitempty" jsonschema:"完整的版面参数对象（字段见 get_print_style 的 guide.params，缺的字段按零值算，通常改用 paramsPatch）"`
	ParamsPatch any               `json:"paramsPatch,omitempty" jsonschema:"只改其中给出的版面参数，如 {\"fontSize\":10,\"marginTop\":20}；与 params 二选一"`
	CSS         *string           `json:"css,omitempty" jsonschema:"自定义样式表，整体替换原有的；会被清洗"`
	Slots       map[string]string `json:"slots,omitempty" jsonschema:"按插槽名修改：键为 cover、before_rosters、after_events、header_left、header_right、footer；值为模板，空白即删除该插槽"`
	Templates   map[string]string `json:"templates,omitempty" jsonschema:"按报表种类修改整个报表正文的模板（只写 body 内容）：键为 entries、startlist、results、team_scores、checkin、recordsheet；空白即删除"`
}

func (in printStyleInput) toService() (service.PrintStyleInput, error) {
	out := service.PrintStyleInput{Name: in.Name, CSS: in.CSS, Slots: in.Slots, Templates: in.Templates}
	if in.Params != nil {
		// Decoded through the service type, so the parameters are exactly
		// its own and a misspelt field is an error.
		raw, err := json.Marshal(map[string]any{"params": in.Params})
		if err != nil {
			return out, errors.New("params 格式不对")
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var holder service.PrintStyleInput
		if err := dec.Decode(&holder); err != nil {
			return out, fmt.Errorf("版面参数格式不对：%v", err)
		}
		out.Params = holder.Params
	}
	switch v := in.ParamsPatch.(type) {
	case nil:
	case string:
		out.ParamsPatch = v
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return out, errors.New("paramsPatch 格式不对")
		}
		out.ParamsPatch = string(raw)
	}
	return out, nil
}

// reportArg is service.ReportRequest with the optional parts optional.
type reportArg struct {
	MeetID   string   `json:"meetId"`
	Kind     string   `json:"kind" jsonschema:"entries（报名表）、startlist（秩序册）、results（成绩册）、team_scores（团体总分）、checkin（检录单）或 recordsheet（成绩记录表）"`
	EventIDs []string `json:"eventIds,omitempty" jsonschema:"限定项目与全能（entries、startlist、results、team_scores），留空为全部"`
	GroupIDs []string `json:"groupIds,omitempty" jsonschema:"限定组别（results、team_scores），留空为全部，空字符串选未分组"`
	RoundIDs []string `json:"roundIds,omitempty" jsonschema:"限定轮次（checkin、recordsheet），留空为全部"`
	Day      string   `json:"day,omitempty" jsonschema:"只取该日计划的组，YYYY-MM-DD（checkin、recordsheet）"`
}

func (a reportArg) request() service.ReportRequest {
	return service.ReportRequest{Kind: a.Kind, MeetID: a.MeetID, EventIDs: a.EventIDs, GroupIDs: a.GroupIDs, RoundIDs: a.RoundIDs, Day: a.Day}
}

// frontPageInput is service.FrontPageInput with reports optional.
type frontPageInput struct {
	Title   string   `json:"title" jsonschema:"1–60 个字"`
	BodyMD  string   `json:"bodyMd" jsonschema:"Markdown 正文"`
	Reports []string `json:"reports,omitempty" jsonschema:"印在哪些报表里：entries、startlist、results、team_scores、checkin、recordsheet；留空为全部"`
}

type printStyleIDArg struct {
	ID string `json:"id" jsonschema:"样式 ID；内置默认样式为 default"`
}
type getPrintStyleArg struct {
	ID      string `json:"id" jsonschema:"样式 ID；内置默认样式为 default"`
	NoGuide bool   `json:"noGuide,omitempty" jsonschema:"为 true 时不附接口说明（已读过时省略以减少内容）"`
}
type savePrintStyleArg struct {
	ID    string          `json:"id,omitempty" jsonschema:"留空新建；内置默认样式不能改"`
	Input printStyleInput `json:"input"`
}
type printStyleNameArg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type copyPrintStyleArg struct {
	ID   string `json:"id" jsonschema:"被复制的样式 ID，内置默认样式也可以"`
	Name string `json:"name,omitempty" jsonschema:"新样式名称，留空自动命名；已有同名时报错"`
}
type printStyleVersionArg struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type importPrintStyleArg struct {
	File any `json:"file" jsonschema:"export_print_style 返回的样式文件对象（也可传其 JSON 文本）；总是新建一个样式"`
}
type reportTemplateArg struct {
	Kind       string `json:"kind" jsonschema:"entries、startlist、results、team_scores、checkin 或 recordsheet"`
	WithSample bool   `json:"withSample,omitempty" jsonschema:"附该报表的样例数据"`
}
type renderReportArg struct {
	reportArg
	StyleID string           `json:"styleId,omitempty" jsonschema:"用这个样式代替赛事选用的；default 为内置默认样式"`
	Draft   *printStyleInput `json:"draft,omitempty" jsonschema:"试写的样式改动（params、paramsPatch、css、slots、templates），叠在样式上渲染，不保存"`
}
type styleSampleArg struct {
	Kind    string           `json:"kind" jsonschema:"entries、startlist、results、team_scores、checkin 或 recordsheet"`
	StyleID string           `json:"styleId,omitempty" jsonschema:"留空为内置默认样式"`
	Draft   *printStyleInput `json:"draft,omitempty" jsonschema:"试写的样式改动，不保存"`
}
type meetStyleArg struct {
	MeetID  string `json:"meetId"`
	StyleID string `json:"styleId" jsonschema:"样式 ID；default 为内置默认样式"`
}
type frontPageArg struct {
	MeetID string         `json:"meetId"`
	ID     string         `json:"id,omitempty" jsonschema:"留空新增到最后"`
	Input  frontPageInput `json:"input"`
}
type frontPageIDArg struct {
	MeetID string `json:"meetId"`
	ID     string `json:"id"`
}
type frontPageOrderArg struct {
	MeetID string   `json:"meetId"`
	IDs    []string `json:"ids" jsonschema:"必须恰好包含赛事全部前置页，按印刷顺序"`
}
type assetScopeArg struct {
	Scope   string `json:"scope" jsonschema:"style 或 meet"`
	OwnerID string `json:"ownerId" jsonschema:"scope 为 style 时是样式 ID，为 meet 时是赛事 ID"`
}
type saveAssetArg struct {
	assetScopeArg
	Name       string `json:"name" jsonschema:"图片名称，1–40 个字，不含空格、引号、括号；引用写 asset:名称"`
	DataBase64 string `json:"dataBase64" jsonschema:"png、jpeg 或 webp 图片的 base64；整个请求至多 2 MB，所以图片约 1.5 MB 以内"`
	MIME       string `json:"mime,omitempty" jsonschema:"image/png、image/jpeg 或 image/webp；留空按内容判断"`
}
type deleteAssetArg struct {
	assetScopeArg
	Name string `json:"name"`
}

func (a renderReportArg) options() (service.RenderOptions, error) {
	opt := service.RenderOptions{StyleID: a.StyleID}
	if a.Draft != nil {
		d, err := a.Draft.toService()
		if err != nil {
			return opt, err
		}
		opt.Draft = &d
	}
	return opt, nil
}

// decodeBase64 accepts the plain text, or a data: URI.
func decodeBase64(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "data:") {
		if _, rest, ok := strings.Cut(text, ","); ok {
			text = rest
		}
	}
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		if data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(text, "=")); err != nil {
			return nil, errors.New("图片数据不是有效的 base64")
		}
	}
	return data, nil
}

func registerPrint(s *sdk.Server, svc *service.Service) {
	addTool(s, "list_print_styles", "列出打印样式（全局，跨赛事）：内置「默认」样式及自定义样式，含版本、被几个赛事使用、是否有自定义 CSS 与模板。", true, func(ctx context.Context, _ empty) (any, error) { return svc.ListPrintStyles(ctx) })
	addTool(s, "get_print_style", "读取一个打印样式（params、css、slots、templates、图片、使用它的赛事），并附接口说明 guide：参数取值范围、CSS 变量、报表类名、插槽位置、分页约定、模板写法、前置页 Markdown 语法、图片引用与内容清洗规则。修改样式前先读一次。", true, func(ctx context.Context, in getPrintStyleArg) (any, error) {
		detail, err := svc.GetPrintStyle(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		out := map[string]any{"style": detail}
		if !in.NoGuide {
			out["guide"] = svc.PrintGuide()
		}
		return out, nil
	})
	addTool(s, "save_print_style", "创建或修改打印样式，直接生效，不需要预览确认（每次保存留一个版本，可用 restore_print_style_version 回退）。id 留空新建（新样式从内置默认起步）；内置默认样式不能改。input 各字段省略即不改：name；params 完整版面参数，或 paramsPatch 只改给出的字段；css 自定义样式表（整体替换，会被清洗）；slots 与 templates 按键修改，键对应的值为空白即删除该键，没列出的键不变。slots 键为 cover、before_rosters、after_events、header_left、header_right、footer；templates 键为报表种类 entries、startlist、results、team_scores、checkin、recordsheet，值是只含 body 内容的 Go 模板。返回 style、version、changed（false 表示没有改动、不产生版本）与 warnings（被清洗掉的内容等）。", false, func(ctx context.Context, in savePrintStyleArg) (any, error) {
		input, err := in.Input.toService()
		if err != nil {
			return nil, err
		}
		return svc.SavePrintStyle(ctx, store.SourceMCP, in.ID, input)
	})
	addTool(s, "copy_print_style", "复制一个打印样式（含图片）为新样式，name 留空时自动命名，已有同名时报错。内置默认样式也可复制。", false, func(ctx context.Context, in copyPrintStyleArg) (any, error) {
		return svc.CopyPrintStyle(ctx, store.SourceMCP, in.ID, in.Name)
	})
	addTool(s, "rename_print_style", "给打印样式改名。内置默认样式不能改名。", false, func(ctx context.Context, in printStyleNameArg) (any, error) {
		return svc.RenamePrintStyle(ctx, store.SourceMCP, in.ID, in.Name)
	})
	addTool(s, "delete_print_style", "删除打印样式，直接生效；使用它的赛事改回内置默认样式。内置默认样式不能删。", false, func(ctx context.Context, in printStyleIDArg) (any, error) {
		err := svc.DeletePrintStyle(ctx, store.SourceMCP, in.ID)
		return map[string]bool{"deleted": err == nil}, err
	})
	addTool(s, "list_print_style_versions", "列出打印样式保留的最近版本（新的在前），含谁保存的、原因、是否是当前版本。", true, func(ctx context.Context, in printStyleIDArg) (any, error) {
		return svc.ListPrintStyleVersions(ctx, in.ID)
	})
	addTool(s, "restore_print_style_version", "把样式的参数、CSS、插槽与模板恢复到某个保留版本，作为新的一次保存（版本号继续增加）；名称与图片保持现状。", false, func(ctx context.Context, in printStyleVersionArg) (any, error) {
		return svc.RestorePrintStyleVersion(ctx, store.SourceMCP, in.ID, in.Version)
	})
	addTool(s, "export_print_style", "返回一个打印样式的样式文件（含图片的 base64），可用 import_print_style 导入。内置默认样式也可导出。", true, func(ctx context.Context, in printStyleIDArg) (any, error) {
		data, err := svc.ExportPrintStyle(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(data), nil
	})
	addTool(s, "import_print_style", "导入样式文件，直接新建一个样式（重名时名称后加序号），不改动已有样式；文件按保存时同样的规则检查与清洗。整个请求至多 2 MB。", false, func(ctx context.Context, in importPrintStyleArg) (any, error) {
		var data []byte
		switch v := in.File.(type) {
		case nil:
			return nil, errors.New("请提供样式文件 file")
		case string:
			data = []byte(v)
		default:
			var err error
			if data, err = json.Marshal(v); err != nil {
				return nil, errors.New("样式文件格式不对")
			}
		}
		return svc.ImportPrintStyle(ctx, store.SourceMCP, data)
	})
	addTool(s, "get_report_template", "返回某报表种类的内置模板源码、它调用的子模板 parts、数据类型 dataTypes（每个字段给出模板里写的 Go 名与 JSON 键名，以及可调用的方法）、可用的模板函数；withSample 时附样例数据。写自定义 templates 前先读，从内置模板改起。", true, func(_ context.Context, in reportTemplateArg) (any, error) {
		return svc.GetReportTemplate(in.Kind, in.WithSample)
	})
	addTool(s, "get_report_data", "读取某赛事某报表在范围内的数据（JSON），即自定义模板收到的 Book、CheckInBook 或 RecordBook，附 dataVersion。kind 为 entries、startlist、results、team_scores、checkin、recordsheet；eventIds 限定项目与全能，groupIds 限定组别（results、team_scores，空字符串为未分组），roundIds 与 day（YYYY-MM-DD）限定检录单与成绩记录表的组。只读。", true, func(ctx context.Context, in reportArg) (any, error) {
		return svc.GetReportData(ctx, in.request())
	})
	addTool(s, "render_report", "生成 HTML 报表，用赛事选用的打印样式，返回 {html, warnings}（警告是被清洗的内容、找不到的图片、出错而回退的模板等）。kind 为 entries（报名表）、startlist（秩序册）、results（成绩册）、team_scores（团体总分）、checkin（检录单）、recordsheet（成绩记录表）；eventIds 留空选全部，groupIds 限定组别，roundIds 与 day 限定检录单与成绩记录表的组。styleId 改用另一个样式；draft 试写样式改动（同 save_print_style 的 input，不含 name）叠在样式上渲染，不保存。不操作系统打印窗口。", true, func(ctx context.Context, in renderReportArg) (any, error) {
		opt, err := in.options()
		if err != nil {
			return nil, err
		}
		return svc.RenderReportRequest(ctx, in.request(), opt)
	})
	addTool(s, "render_style_sample", "用内置样例数据渲染某报表种类，预览样式或草稿，不依赖赛事，返回 {html, warnings}。styleId 留空为内置默认样式；draft 同 save_print_style 的 input（不含 name），叠在样式上渲染，不保存。前置页是赛事的，样例里不显示。", true, func(ctx context.Context, in styleSampleArg) (any, error) {
		var draft *service.PrintStyleInput
		if in.Draft != nil {
			d, err := in.Draft.toService()
			if err != nil {
				return nil, err
			}
			draft = &d
		}
		return svc.RenderStyleSample(ctx, in.Kind, in.StyleID, draft)
	})
	addTool(s, "set_meet_print_style", "让赛事的报表使用某个打印样式，直接生效；default 为内置默认样式。", false, func(ctx context.Context, in meetStyleArg) (any, error) {
		err := svc.SetMeetPrintStyle(ctx, store.SourceMCP, in.MeetID, in.StyleID)
		return map[string]bool{"saved": err == nil}, err
	})
	addTool(s, "list_front_pages", "列出赛事的前置页（致辞、规程、须知等 Markdown 页面，印在报表最前面，换样式不丢），按印刷顺序。", true, func(ctx context.Context, in meetArg) (any, error) { return svc.ListFrontPages(ctx, in.MeetID) })
	addTool(s, "save_front_page", "新增（id 留空，加到最后）或修改一个前置页，直接生效。input：title 1–60 个字；bodyMd 是 Markdown（表格、删除线；一行 ---pagebreak--- 另起一页；以 -> 开头、<- 结尾的段落居中；图片 ![说明](asset:名称)；不输出原始 HTML）；reports 是印在哪些报表里（entries、startlist、results、team_scores、checkin、recordsheet），空为全部。每个赛事至多 20 页。返回 page 与 warnings（找不到的图片）。", false, func(ctx context.Context, in frontPageArg) (any, error) {
		return svc.SaveFrontPage(ctx, store.SourceMCP, in.MeetID, in.ID, service.FrontPageInput{Title: in.Input.Title, BodyMD: in.Input.BodyMD, Reports: in.Input.Reports})
	})
	addTool(s, "delete_front_page", "删除一个前置页，直接生效；后面的页顺序前移。", false, func(ctx context.Context, in frontPageIDArg) (any, error) {
		err := svc.DeleteFrontPage(ctx, store.SourceMCP, in.MeetID, in.ID)
		return map[string]bool{"deleted": err == nil}, err
	})
	addTool(s, "reorder_front_pages", "设置前置页的印刷顺序，ids 必须恰好包含赛事全部前置页。", false, func(ctx context.Context, in frontPageOrderArg) (any, error) {
		err := svc.ReorderFrontPages(ctx, store.SourceMCP, in.MeetID, in.IDs)
		return map[string]bool{"saved": err == nil}, err
	})
	addTool(s, "list_assets", "列出样式或赛事的图片（名称、类型、大小、指纹，不含数据）。在 CSS、模板、Markdown 里写 asset:名称 引用。", true, func(ctx context.Context, in assetScopeArg) (any, error) {
		return svc.ListAssets(ctx, in.Scope, in.OwnerID)
	})
	addTool(s, "save_asset", "保存一张图片到样式（scope=style）或赛事（scope=meet），同名即替换，直接生效；样式的图片变化会产生新版本。只接受 png、jpeg、webp，单张至多 2 MB，每个样式或赛事至多 30 张。", false, func(ctx context.Context, in saveAssetArg) (any, error) {
		data, err := decodeBase64(in.DataBase64)
		if err != nil {
			return nil, err
		}
		return svc.SaveAsset(ctx, store.SourceMCP, in.Scope, in.OwnerID, in.Name, in.MIME, data)
	})
	addTool(s, "delete_asset", "删除样式或赛事的一张图片，直接生效；引用它的地方显示为空，报表的 warnings 会提示。", false, func(ctx context.Context, in deleteAssetArg) (any, error) {
		err := svc.DeleteAsset(ctx, store.SourceMCP, in.Scope, in.OwnerID, in.Name)
		return map[string]bool{"deleted": err == nil}, err
	})
}
