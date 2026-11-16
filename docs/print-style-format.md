# 打印样式文件规范 v1

SportsMeet 打印样式的文件格式，以及写样式的人（通常是经 MCP 的 AI）要对照的报表数据与排版约定。样式是软件全局的，可单独导出、导入；前置页与赛事图片属于赛事，随[交换格式](interchange-format.md) §5.15 走。实现与规则见 [细则](agents/print-style.md)。

- 机器可读定义：[print-style-v1](schema/print-style-v1.schema.json)（JSON Schema 2020-12，只描述结构；校验与清洗由读取方完成）
- 示例：[examples/sample-print-style.smstyle.json](examples/sample-print-style.smstyle.json)
- 报表数据样例：[testdata/report/data-v1.json](../testdata/report/data-v1.json)（各报表样例数据，字段与测试对齐）

## 1. 文件

| 项 | 规定 |
|---|---|
| 编码 | UTF-8，无 BOM |
| 扩展名 | `.smstyle.json` |
| 结构 | 单个 JSON 对象，最大 32 MB |
| 未知字段 | 顶层未知字段忽略；`params` 里的未知字段拒绝（拼错的参数名不能悄悄不生效） |
| 空值 | `slots`、`templates`、`assets` 可省略，读作空 |

## 2. 版本

```json
{ "format": "sportsmeet-print-style", "version": 1 }
```

- `format` 固定，不符即拒绝读取。
- `version` 为正整数。增删字段、改字段含义、改取值范围均须 +1。与交换格式、成绩交换格式的版本号各自独立。
- 读取方必须能读所有已发布版本；遇到比自己新的版本直接拒绝并提示升级软件。写出方总是写最新版本。
- 报表数据另有版本 `dataVersion`（§5），与文件版本无关：文件版本管文件结构，`dataVersion` 管模板写成时对照的报表数据。

## 3. 顶层字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `format` / `version` | string / integer | ✓ | 见 §2 |
| `name` | string | ✓ | 样式名称，去掉首尾空白后 1–40 个字 |
| `params` | object | | 版面参数，见 §4；缺的字段取默认值 |
| `css` | string | | 自定义 CSS，至多 256 KB，读入时清洗（§7） |
| `slots` | object | | 插槽模板，键见 §6，值是 Go 模板 |
| `templates` | object | | 整份报表的模板，键是报表种类（`entries`、`startlist`、`results`、`team_scores`、`checkin`、`recordsheet`），值是 Go 模板 |
| `dataVersion` | integer | 有 `slots` 或 `templates` 时 ✓ | 模板写成时对照的报表数据版本，当前为 1 |
| `assets` | array | | 样式的图片，见 §8；至多 30 张 |

读取时依次检查：名称、`params`、CSS、插槽键与模板语法、报表种类与模板语法、`dataVersion`、图片。有任何问题整份拒绝，错误一行一条。

## 4. 版面参数 `params`

纸张、边距、字体、字号、表格线、页眉页码与栏目开关。缺的字段取默认值，数值超出范围或词不在可选值里整份拒绝。「默认」列即内置默认样式的值。

| 字段 | 类型 | 默认 | 范围 / 可选值 |
|---|---|---|---|
| `paper` | string | `a4` | `a4` `a3` `b5`（176×250）`16k`（195×270）`letter` `custom` |
| `paperWidth` / `paperHeight` | number（mm） | 0 | 只在 `paper=custom` 用，各 100–1000；其他纸张忽略，导出时写 0 |
| `orientation` | string | `portrait` | `portrait` `landscape`；横向时宽高对调 |
| `marginTop` `marginRight` `marginBottom` `marginLeft` | number（mm） | 15 | 各 0–60；扣掉边距后宽与高至少各剩 50 mm |
| `font` | string | `sans` | `song` 宋体、`hei` 黑体、`kai` 楷体、`fang` 仿宋、`sans` 无衬线；各带 macOS、Windows、Linux 的备选字体 |
| `fontSize` | number（pt） | 9.5 | 6–24 |
| `lineHeight` | number | 1.4 | 1–2.5（字号的倍数） |
| `titleSize` | number（pt） | 18 | 8–48，页首赛事名称 |
| `eventTitleSize` | number（pt） | 12.5 | 8–36，项目标题 |
| `tableLines` | string | `rules` | `rules` 只有横线、`grid` 全框线 |
| `lineWidth` | number（pt） | 1 | 0.1–3，粗线宽度，细线按比例跟随 |
| `header` | boolean | true | 第二页起印页眉（赛事名与报表名） |
| `pageNumber` | string | `n_of_m` | `n_of_m` 第 N 页 / 共 M 页、`dash` - N -、`none` |
| `rosterColumns` | integer | 4 | 1–8，代表队名单每行人数 |
| `showUnit` `showSeed` `showBib` | boolean | true | 表格里印「单位」「报名成绩」「号码」列 |

参数由 Go 写成一段 CSS 变量与 `@page`，排在页面规则 `base.css` 之后、默认外观 `default.css` 与自定义 CSS 之前，见 §9.2。

## 5. 报表数据

自定义模板与插槽对报表数据运行。数据是 Go 结构，模板里用 Go 字段名取值（`{{.Meet}}`），MCP 的 `get_report_data` 返回同一份数据的 JSON，键是各字段的 json 名（`meet`）。`get_report_template` 同时给出两种名字、字段类型与可调用的方法。数据结构会随版本变化：字段增删或含义变化时 `DataVersion` 加 1，模板只在 `dataVersion` 与当前版本相等时使用，否则报表退回默认版式并给出警告，样式本身保留。

### 5.1 三种数据

| 报表种类 | 数据类型 | 说明 |
|---|---|---|
| `entries` 报名表、`startlist` 秩序册、`results` 成绩册 | `Book` | 同一个类型，按各自范围填充 |
| `team_scores` 团体总分表 | `Book` | `TeamOnly=true`，与成绩册用同一份模板与数据 |
| `checkin` 检录单 | `CheckInBook` | 每组一张 `Sheets` |
| `recordsheet` 成绩记录表 | `RecordBook` | 每组一张空白 `Sheets` |

### 5.2 `Book` 要点

| 字段（Go 名 / json 名） | 说明 |
|---|---|
| `Meet` `Dates` `Location` `PrintedAt` / `meet` `dates` `location` `printedAt` | 赛事名称、日期范围、地点、打印时间（本地时间，到分钟） |
| `Title` / `title` | 报表名（报名表、秩序册、成绩册、团体总分表） |
| `Rosters` / `rosters` | 代表队名单，按参赛单位；只有秩序册填，空表示不印 |
| `Schedule` / `schedule` | 竞赛日程表，按日；只有秩序册填 |
| `Events` / `events` | 项目；每项含 `Rounds`（轮次）→ `Heats`（组）→ `Lines`（位置上的人或队） |
| `Combineds` / `combineds` | 全能：分项、统一名单、各分项成绩与积分表 |
| `TeamScores` / `teamScores` | 团体总分表，未启用或不印为空 |
| `GroupName` `GroupBooks` / `groupName` `groupBooks` | 按组别分榜时每个组别一本（成绩册、团体总分表），各自另起页 |
| `HideBib` `HideUnit` `HideSeed` `RosterColumns` | 来自 `params` 的栏目开关与名单每行人数；模板自己决定是否遵守 |
| `Compact` `NewPage` | 来自赛事设置的「秩序册紧凑排版」「每个项目另起一页」 |

`Line` 是一个人或一个队：`Position`（道次或顺序）、`Bib`、`Name`、`Unit`、`Members`（队员，已写成「1棒 张三 101」）、`Seed`（报名成绩）、`Mark`（成绩或状态文字）、`Rank`、`Qualify`（`Q` / `q`）、`Attempts`（逐次成绩，对应 `Event.TryHeads`）、`Best`（算作成绩的那一次的下标，`-1` 无）、`Bars`（高度项目的试跳记录）。成绩、名次、时间都是 Go 已经算好、格式化好的文字，模板只排版，不做成绩或时间运算。

`Book` 的方法：`RosterStyle`（代表队名单紧凑排版的宽度变量，作 `style` 属性用）、`RowsOf n`（`RosterGroup` 的方法，按每行 n 人排满行）。

### 5.3 `CheckInBook` 与 `RecordBook`

`Meet` `Dates` `Location` `PrintedAt` `Title` 之外只有 `Sheets`。检录单的一张 `CheckInSheet`：`Event` `Round` `From`（晋级来源轮次）`Heat` `Time` `CheckIn`（检录时间，没有为空）`PositionLabel` `NameLabel` `Team` `Break`（另起一页）`Lines`；`ShowCheckIn` 在簿上，提前时间为 0 时为 false。成绩记录表的一张 `RecordSheet`：除同样的标题字段外有 `Columns`（待填栏目的表头）、`ColumnsTitle`、`HeatWind`、`AttemptWind`、`MarkLabel`，不含任何成绩，方法 `HeadRows`、`LineRows` 给出表头与每人占的行数。

### 5.4 `Slots`

三种数据里都有 `Slots`（插槽的输出，已是 HTML），内置模板用 `{{with .Slots.after_events}}{{.}}{{end}}` 输出。自定义整份报表模板要自己写这一句，否则「全部内容之后」的插槽不出现。

## 6. 模板

### 6.1 写法

- 模板是 Go `html/template`，内容自动转义。**报表模板只写 `<body>` 里的内容**，不写 `<html>`、`<head>`、`<style>`、`<script>`；外观放在 `css`。
- 自定义报表模板替换整个正文：页首的赛事名称块（`<header>`）、各插槽的位置、「打印时间」页脚都要自己写。内置模板可由 `get_report_template` 取得，从它改起。
- 可以调用内置模板里的命名子模板：`{{template "competitor" .}}`（姓名与队员）、`{{template "bars" .Bars}}`（试跳记录），成绩册与团体总分表另有 `{{template "teamScores" .TeamScores}}`；各报表可用的以 `get_report_template` 的 `parts` 为准。
- 封面、前置页、页眉页脚由系统放进文档，不在报表模板里写。

### 6.2 模板函数

内置函数（`and or not eq ne lt le gt ge len index slice print println html js urlquery call`）之外只有下列函数（`printf` 换成了限制宽度的版本），不能读文件、联网或反射：

| 函数 | 作用 |
|---|---|
| `add a b` `sub a b` `mul a b` `mod a b` | 整数加减乘取余，`mod` 的除数不能为 0 |
| `seq from to` | 从 `from` 到 `to` 的整数列表，含两端，至多 10000 项 |
| `join 列表 分隔符` | 把字符串列表连起来 |
| `lower s` `upper s` | 转小写、转大写 |
| `contains s 子串` `hasPrefix s 前缀` `hasSuffix s 后缀` | 判断 |
| `repeat s n` | 重复 n 次，结果至多 64 KB |

### 6.3 限制

单个模板（报表或插槽）至多 256 KB；运行至多 10 秒或 200 万步（每进一次 `range` 循环体、每调用一次命名模板算一步）；输出至多 20 MB；`printf` 的宽度与精度至多 3 位数；超出时该次渲染出错并回退默认版式。保存与导入时模板先在各报表的样例数据上试运行，不通过就拒绝。

### 6.4 插槽

| 键 | 位置 | 说明 |
|---|---|---|
| `cover` | 封面 | 所有报表的第一页，输出放在 `<section class="cover">` 里；其后的前置页与正文各另起一页 |
| `before_rosters` | 秩序册的「代表队名单」之前 | 内置秩序册模板输出；自定义模板自己写 `{{with .Slots.before_rosters}}{{.}}{{end}}` |
| `after_events` | 全部内容之后、「打印时间」之前 | 所有报表；自定义模板自己写 |
| `header_left` | 页眉左侧文字，默认赛事名称 | 第二页起；只取纯文本，标签去掉 |
| `header_right` | 页眉右侧文字，默认报表名称 | 第二页起；只取纯文本 |
| `footer` | 页脚文字，代替页码格式 | 可用 `{page}` 与 `{pages}`（总页数）；只取纯文本 |

插槽在六种报表的样例数据上都要能运行，所以插槽模板只能取各数据类型共有的字段（`Meet`、`Title`、`Dates`、`Location`、`PrintedAt`）；`Rosters` 这类只有 `Book` 有的字段在检录单上取不到，保存时报错。插槽值为空白等于没有。`header_left`、`header_right` 只在 `params.header` 为 true 且报表有页眉时用（检录单与成绩记录表没有页眉）。

## 7. 清洗与安全

自定义内容由 AI 经 MCP 写入，所以进入报表之前一律清洗，文档另带 Content-Security-Policy。被清洗掉的内容在保存结果与渲染结果的 `warnings` 里列出。

**CSP**：`default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:`。文档不联网、不执行脚本，只能显示内嵌图片与字体。

**CSS**（`css` 字段、`<style>` 元素、`style` 属性）：

- 保留的 at 规则：`@media`、`@supports`、`@page`、`@font-face`、`@keyframes`、`@layer`、`@container`、`@counter-style`；`@import` 与其他 at 规则删除。
- `url()`、`image-set()` 等取地址的函数只允许内嵌的 png / jpeg / webp 图片（`data:image/…`）、内嵌字体（`data:font/…`，只在 `@font-face`）与 `asset:名称`；其余删除。字体用 `data:` 内嵌，不能引用网络字体。
- 删除 `expression()`、`behavior`、`-moz-binding`、`javascript:`、`vbscript:` 与会结束 `<style>` 的文字。按浏览器的方式读（先分词、解转义），`u\72l(` 同样是 `url(`。
- 超过 256 KB 整份拒绝。

**HTML**（模板输出、插槽、Markdown 渲染结果、整份文档）：

- 只保留版式元素：`header footer main section article div span p br hr h1–h6 small strong b em i u s del sub sup mark pre code blockquote ul ol li dl dt dd table thead tbody tfoot tr th td caption colgroup col figure figcaption img`。
- `script iframe form input button select textarea svg math template video audio canvas link base object embed` 等连同内容删除；未知元素（如 `<a>`、`<font>`）去掉标签、保留内容。
- 属性只留 `class` `id` `lang` `data-*`、`style`（按 CSS 规则清洗）、表格的 `colspan` `rowspan` `scope` `span`、`ol` 的 `start`、`img` 的 `src` `alt` `width` `height`。事件属性与所有外链删除；`img` 的 `src` 只能是内嵌图片或 `asset:名称`。
- 文档里只保留系统自己的 CSP `meta` 与 `charset`。

## 8. 图片 `assets[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `name` | string | ✓ | 引用名，1–40 个字，文件内唯一；不含空白、引号、括号与 `& % ; ,` |
| `mime` | string | ✓ | `image/png` `image/jpeg` `image/webp`，须与内容一致（以文件头为准） |
| `data` | string | ✓ | base64（标准字母表），解码后至多 2 MB |

一个样式或赛事至多 30 张、合计至多 16 MB。

**引用**：在模板、插槽、Markdown 与 CSS 里写 `asset:名称`：`<img src="asset:校徽">`、`![](asset:校徽)`、`url(asset:校徽)`。渲染时换成内嵌图片，清洗之后进行。赛事的同名图片优先于样式的。找不到的图片显示为一个透明像素，`warnings` 提示「找不到图片」。名称里的非 ASCII 字符被 Markdown 写成百分号转义时同样识别。

## 9. 页面与分页约定

打印的是预览排好的纸面：预览按 `params` 的纸张与边距把内容排到一页页的纸上，页眉、页脚、页码画在边距里，表格断页处画「续下页」与「（续）」，打印交出这些纸面。样式只通过下面的约定与预览对接。

### 9.1 `<body>` 属性

系统写在 `<body>` 上，预览读取；样式不要自己写 `<body>`，也不要改 `.pv-*` 类（预览内部使用）。

| 属性 | 值 |
|---|---|
| `data-paper` | `宽x高`，毫米，已含方向，如 `297x210` |
| `data-margin` | `上,右,下,左`，毫米 |
| `data-pagenum` | 页码格式：`n_of_m` `dash` `none` |
| `data-meet` `data-title` | 页眉左右文字；有 `data-meet` 才有页眉（`params.header` 为 false 或检录单、成绩记录表没有） |
| `data-head-left` `data-head-right` | `header_left` `header_right` 插槽的文字，替换上面两项 |
| `data-foot` | `footer` 插槽的文字，替换页码，含 `{page}` `{pages}` |

### 9.2 CSS 变量

`params` 设置的变量（自定义 CSS 可在 `:root` 里覆盖字体、字号、线宽这类外观变量；**不要覆盖 `--paper-*`、`--m-*`，也不要改 `@page` 的尺寸**：预览按 `data-paper` / `data-margin` 分页，两处不一致就对不上纸面，纸张与边距用 `params` 改）：`--paper-w` `--paper-h` `--m-t` `--m-r` `--m-b` `--m-l` `--font` `--fs` `--lh` `--fs-title` `--fs-event` `--rule` `--rule-event` `--rule-mid` `--rule-thin` `--cell-line` `--row-line`。完整清单与各变量的默认值由 `get_print_style` 的 `guide.cssVariables` 给出；报表类名（`guide.classes`）同样来自代码。

### 9.3 分页

- 不会被拆开跨页：`.heat`、`.unit`、`.sheet`、表格的行 `tr`；表格可以跨页，`thead` 每页重复；`h2` `h3` `h4` 不会孤立在页底。
- 强制另起一页：`.sheet.break`、第二个起的 `.report-group`（按组别分榜）、`.page-break`、`.front`（前置页）、封面之后的内容；赛事设置「每个项目另起一页」时 `.event` 也另起页。
- 模板里要另起页：给元素加 `class="page-break"`，或在 CSS 里写 `break-before: page`。预览读元素的 `break-before` 计算值，所以另起页的规则不要限定在 `@media print` 里。

## 10. 前置页 Markdown

前置页属于赛事，印在所选报表的开头，在封面之后，每页独占一页。标题 1–60 个字，正文至多 256 KB，每个赛事至多 20 页。

| 写法 | 效果 |
|---|---|
| 标题、列表、粗体、斜体 | 标准 Markdown |
| GitHub 风格表格、`~~删除线~~` | 支持 |
| 段内换行 | 即印出的换行 |
| 单独一行 `---pagebreak---` | 另起一页（代码块里不算） |
| 以 `->` 开头、`<-` 结尾的段落 | 居中，如 `->校运会<-` |
| `![说明](asset:名称)` | 插入赛事或样式的图片 |
| 原始 HTML | 不输出 |

Markdown 渲染成 HTML 后按 §7 清洗。引用了不存在的图片不是错误：保存时警告，渲染时留空。`reports` 指定印在哪些报表前（`entries` `startlist` `results` `team_scores` `checkin` `recordsheet`），空表示全部。

## 11. 导入

- 导入样式文件**总是新建一个样式**，不改动已有样式；重名时名称后加序号（如「校运会(2)」），内置「默认」也算重名；名称按应用统一的规范化文本比较，大小写与首尾空白不区分。
- 文件按保存时同样的规则检查与清洗：参数、CSS、模板在样例数据上试运行、图片类型与大小。
- `dataVersion` 与当前报表数据版本不同的模板照存，但不使用，并给出警告。
- 一个样式合计的图片超过 16 MB、样式总数超过 100 个、文件超过 32 MB 均拒绝。
