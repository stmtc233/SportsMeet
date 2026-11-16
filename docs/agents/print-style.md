# 打印样式与前置页

样式文件、报表数据与排版约定的字段级规范见 [print-style-format.md](../print-style-format.md)（事实源）；本文写概念、规则与实现约束。MCP 工具见 [mcp.md](mcp.md)「打印样式」，界面见 [frontend.md](frontend.md)「设置页」「赛事设置」「打印」。

## 概念与归属

- **打印样式**（`print_styles` 表）是软件全局的：不属于任何赛事，所有赛事都能在「赛事设置」选用，可单独导出为 `.smstyle.json` 再导入。一个样式 = 版面参数（`Params`：纸张、边距、字体、字号、表格线、页眉页码、栏目开关）、自定义 CSS、插槽、整份报表的模板、图片。
- **内置「默认」样式**（id `default`）不存储，就是报表不带任何样式时的外观；只读，可复制、可导出，新样式从它复制或起步。它没有版本，也不能改名、删除。
- **前置页**（`meet_front_pages`）与**赛事图片**（`meet_assets`）属于赛事，不属于样式：换样式不丢，随赛事导出与沿用。前置页是 Markdown，印在所选报表的开头，在封面之后。
- 赛事选用哪个样式存在赛事设置的 `printStyleId`（空 = 默认）。使用它的赛事数由一次查询汇总（`ListMeetStyleUse`），样式列表的「N 个赛事使用」来自这里。
- **谁改什么（用户要求）**：自定义 CSS、插槽与报表模板只由 AI 经 MCP 写，普通用户在界面只改版面参数、图片、版本、导入导出与前置页，看到「含自定义 CSS / 模板」的标记但不编辑它们。原因：模板与 CSS 要对照报表数据结构与排版约定才写得对，界面不提供文本编辑，交给 AI 并由预览与版本兜底。

## 写入直接生效（用户要求）

样式、前置页与图片的写入与删除**直接生效，不走「预览指纹 + `confirmed=true`」**，MCP 也是（这是 [mcp.md](mcp.md) 预览确认规则的例外）。它们不是比赛记录，改坏不丢数据：样式每次保存留一个版本，写坏了回退；前置页与图片可以再次保存修正。界面只对删除样式、删除图片、删除前置页弹确认框。日志来源 `ui` / `mcp`，动作见 `service/audit.go`（`printstyle.*`、`frontpage.*`、`asset.*`、`meet.printstyle`）。

## 渲染流程

`service.reportStyle` 把一次渲染要用的样式合成 `report.Style`：先取样式（渲染选项的 `StyleID`，没有则取赛事设置选用的；赛事选用的样式已被删除读作默认，显式给出的不存在 id 才报错），再叠上不保存的草稿 `Draft`（按保存同样的规则检查与清洗），再加赛事的前置页与图片。图片只在 CSS、模板、插槽或 Markdown 里出现 `asset:` 时才读（图片是样式里最大的部分），赛事的同名图片盖过样式的。

`report.renderDoc` 按固定顺序组装文档：

1. **head**：`<meta charset>`；CSP `meta`；`<title>`；然后四段 `<style>`，顺序是 ① `base.css`（分页与预览纸面的必需规则，不论样式）（「每个项目另起一页」时附一条 `.event` 的另起页规则）② `Params.CSS()`（纸张、边距、字体、线宽等 CSS 变量与 `@page`）③ `default.css`（默认外观，读那些变量）④ 样式的自定义 CSS。后面的盖过前面的，所以自定义 CSS 能改外观，也能在 `:root` 里覆盖变量。
2. **body 属性**：`data-paper`、`data-margin`、`data-pagenum`，页眉页脚用 `data-meet` `data-title` `data-head-left` `data-head-right` `data-foot`（见下「页眉页脚与预览」）。
3. **body 内容**：封面（`cover` 插槽，`<section class="cover">`）→ 前置页（每页 `<section class="front">`，按 `reports` 过滤）→ 报表正文（自定义模板，没有或出错则内置模板）。
4. **清洗**：样式带 CSS、插槽或模板（`Style.custom()`）时，整份文档过 `printstyle.SanitizeDocument`；默认样式与只改版面参数的样式不经这一步，输出就是内置模板的原样（`testdata/report/default-*.body.html` 黄金文件由 `TestDefaultStyleKeepsTheOutput` 把关，改内置模板与默认 CSS 时对照）。前置页的 HTML 不论样式都单独清洗一次。
5. **图片解析**：清洗之后把 `asset:名称` 换成内嵌的 `data:` 图片（`printstyle.ResolveAssets`）。顺序不能反：清洗只放行 `asset:` 引用与内嵌图片，先换再清洗要把最多 16 MB 的 base64 逐字读一遍，且找不到的图片也无从提示。找不到的引用换成一个透明像素并进 `warnings`。

返回 `{html, warnings}`。`warnings` 汇集：被清洗掉的内容、找不到的图片、出错而被跳过的插槽、出错而回退默认版式的模板、因数据版本过旧而失效的模板、无法使用的版面参数（读作默认并提示）。界面「打印」页在预览上方显示前几条（`reports/ReportWarnings.tsx`，至多 3 条，其余写「另有 N 条提示」）；MCP 原样返回。

## 安全：为什么必须清洗，再加 CSP

自定义 CSS 与模板由 AI 写入，AI 可能被它读到的内容（报名表、网页）带偏；报表又是用户在系统 WebView 里打开、再交给打印窗口的 HTML 文档。不清洗，一个样式就能让报表联网把赛事数据发出去（外链图片、`@import`、`<link>`）、执行脚本或改写预览页面。所以：

- **清洗是第一道**（`internal/printstyle`）：CSS 按浏览器的方式分词、解转义后检查（`u\72l(` 同样是 `url(`），只放行内嵌图片、内嵌字体与 `asset:` 引用，删除 `@import`、`expression()` 等；HTML 只留版式元素与白名单属性，删除脚本、框架、表单、svg、外链与事件属性，`img` 只能指向内嵌图片或 `asset:`；Markdown 不输出原始 HTML，渲染结果同样清洗。规则细节见格式规范 §7。
- **CSP 是第二道**：每份报表带 `default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:`。清洗有漏洞时文档仍不能联网、不能跑脚本。预览与打印窗口载入的是同一份文档（`paginate.ts` 的 `printable` 序列化整份文档，head 里的 CSP 一并带走），打印窗口因此也受它约束。
- **预览 iframe 不放行脚本**（`sandbox="allow-same-origin"`），见 frontend.md「打印」。
- 清洗是**白名单**：没列出的元素、属性、at 规则一律删除并在 `warnings` 里说。不要为了让某个样式「正常显示」放宽它：放宽的是整个应用的安全边界，不是那一个样式的。
- 模板函数只有算术与字符串小工具，不含文件、网络与反射；`printf` 换成拒绝 3 位数以上宽度与精度的版本（`%0999999999d` 一次就要 1 GB 内存）。计步到不了函数内部，所以函数与模板能调用的报表方法各自限制按参数分配的大小：`seq` 先算项数（无符号，端点在整数边界也不溢出）至多 1 万项，`repeat` 结果至多 64 KB，`RowsOf` 每行至多 100 人；新加的函数或带整数参数的报表方法同样要自带上限，过大的分配是致命错误，`recover` 接不住，整个应用会退出。保存与导入时模板先在各报表的样例数据上试运行，通不过拒绝保存。
- **模板运行能被打断**：text/template 无法从外部停止，不写出内容的循环也碰不到输出上限，超时后放着不管它会一直占着 CPU，重复请求还会累积。所以编译时在每个 `range` 循环体和每个命名模板的开头插入 `{{$__tick := __tick}}`（声明不输出，html/template 不转义它），`tick` 计步并检查时限，超过 200 万步或 10 秒就返回错误，模板当场停在调用方的协程里。运行一次一个（`execMu`），`tick` 读的就是正在运行的那一次的状态；默认样式不经这里，不受影响。

## 保存、版本与快照

- `SavePrintStyle` 的检查顺序是：名称（1–40 字、不与已有样式或「默认」重名，名称按 `domain.NormalizeText` 比较）→ 版面参数 → CSS（清洗，删掉的进 `warnings`）→ 插槽 → 报表模板。任一项不合格整个保存拒绝，不写任何东西。保存的是**清洗后**的 CSS。`params` 整体替换，`paramsPatch` 只改给出的字段（未知字段拒绝），二者不能同时给；`slots` 与 `templates` 按键修改，值只有空白即删除该键，没给的键不变。
- **每次保存留一个版本**（`print_style_versions`，快照含名称、参数、CSS、插槽、模板、`dataVersion`、图片的名称与指纹），只保留最近 30 个，更早的随保存一起删除；版本号一直累加，不因删除回头。保存后与保存前内容相同时不产生版本（`changed=false`）。改名、图片增删与恢复版本也各是一次保存、各留一个版本，`reason` 分别写「改名」「图片「x」」「删除图片「x」」「恢复第 N 版」。
- **恢复**只恢复参数、CSS、插槽、模板与 `dataVersion`，是一次新的保存；名称不变（恢复不会撞上别的样式的名称），图片不变（图片体积大、不进快照），快照里有而现在没有或已被替换的图片以警告提示。版本里的内容按现在的规则重新检查，不合格则拒绝恢复。
- **模板与数据版本**：保存时只有这次改动了插槽或模板，才重新试运行并把 `dataVersion` 记为当前报表数据版本；只改参数或 CSS 不碰它，写于旧版本的模板保持原样并警告。渲染时 `dataVersion` 与当前不同，插槽与报表模板一律不使用，报表退回默认版式并警告「自定义模板已失效」；样式本身（参数、CSS、图片）仍生效。改报表数据（`report.Book` 等的导出字段与 json 名）即 `report.DataVersion` +1 并新增 `testdata/report/data-vN.json` 黄金文件，由 `TestDataGolden` 把关。
- **模板失效与出错的回退**：自定义报表模板运行出错 → 该报表用内置模板并警告；某个插槽出错 → 略过这一个并警告；版面参数读出来不合格 → 读作默认并警告。不让一个坏样式使整份报表打不出来。
- 删除样式连同它的版本与图片（外键级联）；使用它的赛事读作默认样式，不报错。内置默认样式不能改、不能删。

## 赛事选用样式：`SetMeetPrintStyle`

赛事选用的样式只由 `SetMeetPrintStyle` 改，`SaveMeetSettings` **不改**：保存赛事设置时服务端把 `printStyleId` 恢复成当前值。原因：赛事设置整体提交，提交里没带 `printStyleId` 或带着过期的值，就会把赛事的样式清成默认或改回旧的；「选样式」与「调编排、打印选项」是两件无关的事，不能互相覆盖。`MeetSettings` 读取时把已不存在的样式 id 读作空，赛事设置页因此总是显示一个真实存在的选项。

## 沿用新建赛事与交换格式

- **沿用项目与赛制新建赛事**一并复制：选用的样式 id（新赛事的其他设置仍用默认值）、全部前置页（新 ID，顺序不变）、赛事图片（新 ID，内容相同）；样式本身是全局的，不复制。`meetProjectDefinitions` 带这三项，MCP 的预览指纹据此覆盖前置页与图片（图片只带名称、大小与指纹，不带数据）。→ domain.md「沿用项目与赛制新建赛事」。
- **交换格式 v11**（→ [interchange-format.md](../interchange-format.md) §5.15）含 `frontPages` 与赛事 `assets`；**样式不进交换格式**，有自己的样式文件，版本号各自独立。导入新赛事全部写入；导入已有赛事只增不改（相同标题与正文的前置页、同名同内容的图片视为已有，同名不同内容的图片保留赛事里的并警告，前置页合计超过 20 页整份拒绝），规则在 `service/print_content_import.go`。赛事导入不带来样式选择：导入的赛事用默认样式。

## 事件与刷新

写样式、图片、版本、导入、删除广播 `Change{Kind: "printStyles"}`（应用级，不带赛事 ID）；写前置页、赛事图片、选用样式广播带赛事 ID 的 `meets`。「打印」页预览对这两者都重读（`reports/controller.ts` 的 `reportChanged`）：AI 改样式时，打开着的预览随之更新。`printStyles` 不带赛事 ID，只按 `meetId` 相等重读的页面不会被它唤醒，要随样式变化的页面必须自己按 `kind` 订阅（样式列表与选样式的下拉框、打印预览都订阅了）。

## 页眉页脚与预览

预览（`paginate.ts`）与打印都以报表 `<body>` 上的属性为准：纸张与边距读 `data-paper`（`宽x高`，已含方向）与 `data-margin`（`上,右,下,左`，毫米），页码格式读 `data-pagenum`，页眉左右文字读 `data-meet` / `data-title`，被 `header_left` / `header_right` 插槽替换时读 `data-head-left` / `data-head-right`，页脚读 `data-foot`（含 `{page}`、`{pages}`，替换页码格式）。没有 `data-meet` 就没有页眉（`params.header` 关闭、检录单与成绩记录表都是这样）。这些属性由 `Params.DataAttrs()` 与 `renderDoc` 写成，自定义模板不写 `<body>`。

- **纸张尺寸在两处**：`Params.CSS()` 写 CSS 变量与 `@page`，`.pv-page` 用变量排纸面；`data-paper` / `data-margin` 给 `paginate.ts` 算页容量。两处出自同一份 `Params`。自定义 CSS 改 `--paper-*`、`--m-*` 或 `@page` 的尺寸，预览的纸面与分页就对不上，打印交出的是预览的纸面，会出现内容被裁或错位，所以样式用 `params` 改纸张与边距，CSS 不改它们。
- `print` 页把预览纸面交给打印窗口时 `@page` 写成纸张尺寸、边距 0（边距在纸面自己的内边距里），见 frontend.md「打印」。
- 页眉页脚文字只取纯文本（`printstyle.PlainText`），插槽里的标签被去掉。

## 限制值

- **版面参数的数值范围只写一处**：`printstyle.NumberLimits`（键、名称、上下限、单位、是否整数）加 `MinContentMM`，`Params.Validate`、MCP 的接口说明与界面表单（经 `PrintStyles.paramRules`）都从它取。另写一份范围的地方迟早与校验不一致：表单放行的值保存时被拒，或表单拦下 Go 接受的值。`testdata/print/param-rules.json` 是它的黄金文件，前端单测与页面测试读它。

| 项 | 限制 |
|---|---|
| 样式名称 | 1–40 个字 |
| 样式数 | 至多 100 个 |
| 每个样式的版本 | 保留最近 30 个 |
| CSS | 256 KB |
| 单个模板或插槽 | 256 KB；运行 10 秒或 200 万步；输出 20 MB |
| 图片 | png / jpeg / webp，以文件头为准；单张 2 MB；每个样式或赛事至多 30 张、合计 16 MB；名称 1–40 个字、不含空白、引号、括号与 `& % ; ,` |
| 样式文件 | 32 MB |
| 前置页 | 标题 1–60 个字；正文 256 KB；每个赛事至多 20 页 |
| MCP 请求体 | 2 MB（含 base64 的图片与样式文件），所以经 MCP 传的单张图约 1.5 MB 以内 |

限制值的唯一来源是代码常量（`internal/printstyle` 的 `Max*`、`service` 的 `maxPrintStyles` 等），`get_print_style` 的 `guide` 与格式规范由它们生成或对照；改限制时同步格式规范 §3–§10。

## 代码结构

- `internal/printstyle/`：纯计算，不依赖别的包。`params.go`（`Params`、校验、CSS 变量与 `@page`）、`css.go`（CSS 清洗）、`html.go`（HTML 清洗与 CSP）、`markdown.go`（前置页 Markdown）、`template.go`（模板编译、函数、限制）、`asset.go`（图片校验与 `asset:` 解析）、`file.go`（样式文件编解码）。
- `internal/report/`：`render.go`（`Style`、`renderDoc`、各报表入口）、`source.go`（给写样式的人看的内置模板源码、数据类型描述、CSS 变量与类名清单，全部由代码与样式表生成，不另写一份）、`sample.go`（样例数据，模板保存时在它上面试运行）、`base.css` 与 `default.css`。
- `internal/service/`：`print_style.go`（样式 CRUD、版本、图片）、`print_style_file.go`（样式文件导出与导入）、`front_pages.go`（前置页）、`print_render.go`（合成 `report.Style`、渲染选项、样例渲染）、`print_data.go`（报表数据、内置模板、接口说明 `PrintGuide`）、`print_content_import.go`（交换格式里的前置页与图片）。
- `internal/store/print_styles.go` 与迁移 `0014_print_styles.sql`（→ storage.md）；`internal/bind/print_styles.go`（`PrintStyles`、`FrontPages`、`Meets.SetPrintStyle`，只加文件对话框与转调）；`internal/mcp/print_tools.go`。
- 前端：`src/features/settings/print-styles/`（设置里的「打印样式」页）、`src/features/setup/`（`PrintStylePicker`、`FrontPagesSection`、`FrontPageDialog`）、`src/features/reports/`（`ReportWarnings`、`paginate.ts`）。

**分层**：`printstyle` 不依赖任何包；`report` 与 `interchange` 只依赖 `printstyle`（`internal/layers` 的分层表把关）；`printstyle`、`report` 都是纯计算包，不碰 mygo。
