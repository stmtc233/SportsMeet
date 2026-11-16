# MCP 服务

`internal/mcp/` 使用官方 Go SDK 提供 Streamable HTTP,由 `main.go` 创建管理器并随应用启动 / 退出。`internal/service/mutation*.go` 与 `internal/service/schedule.go` 提供预览和确认提交,设置经本机 mygo 绑定管理。
工具只调用 `internal/service`。这是应用唯一的对外接口。

## 开关与连接

- **默认开启（用户要求）**，在独立「AI/MCP」页管理；已保存的开关选择在重启后保留，关闭时端口不监听。
- 只监听 `127.0.0.1`,端口默认 18765,设置允许 1024–65535。地址为 `http://127.0.0.1:<port>/mcp`。传输为 Streamable HTTP,用官方 Go SDK `github.com/modelcontextprotocol/go-sdk`。
- **独立的高熵 Bearer 令牌**:首次开启时生成,常量时间比较;「AI/MCP」页可显示 / 复制 / 轮换,轮换后旧客户端立即失效。
- 请求带 `Origin` 且不是本机时拒绝(403),防止网页里的脚本借用户浏览器调用本机服务。
- 请求体上限 2MB,包含未声明长度的请求;超限返回 413。同时检查 `Host`,只允许 localhost 与回环地址。传输使用无状态模式,每次请求都验证令牌。
- 「AI/MCP」页提供「复制连接信息」:由应用生成地址与令牌的通用配置片段,不区分客户端。

## 运动员字段兼容

- `save_athlete.input.competitionGender` 是男／女／空的默认参赛类别，`genderIdentity` 是男／女／非二元／其他／不愿透露／空的可选个人资料；不得互相推断。更新时这两个字段省略保持原值，空字符串明确清空。旧 `input.gender` 只兼容参赛类别，与新类别同时出现且矛盾时拒绝；其他资料仍为完整字段。
- `list_athletes` 等运动员对象分别返回 `competitionGender` 与 `genderIdentity`。项目和全能的 `gender` 兼容键不变，语义为参赛类别。运动员认同不参与 MCP 文件混放检查，原有严格检查只比较参赛类别。完整赛事交换写 v12，兼容 v1–v11，报表与成绩文件不新增个人认同。

## 工具边界

- **MCP 可以写成绩**(用户要求):录入、修改成绩与状态(`OK / DNS / DNF / DQ / NM`)、田赛逐次记录,规则见下文「写成绩」。
- **不内置任何云模型**:任意格式(Excel、Word、截图)的识别由外部 AI 完成,应用只接收统一的结构。
- 运动员写入沿用 [domain.md](domain.md) 的唯一性:学号 / 号码布与赛事内已有运动员相同即拒绝,同名只警告。

## 预览 → 确认 → 提交

批量改动(报名、导入、自动编排、排时间、晋级)与删除都分两步:

1. `preview_*` 返回完整的结果预览与 `previewHash`:规范化参数与目标数据的 SHA-256。
2. 用户在对话里确认后,AI 带同一组参数、`previewHash` 与 `confirmed=true` 调用提交工具。
3. 提交时持 service 写锁重算结果并比对指纹，再将已校验的批量语句事务写入,参数或数据变了就拒绝,要求重新预览。

预览有错误(非空 `errors`)不得提交。

**例外(用户要求)**:打印样式、前置页、图片的写入与删除直接生效,不走预览确认,见下文「打印样式」。

- **删除**:只能删没有成绩的赛事、组、项目、运动员;有成绩的(及含这类组的赛事 / 项目、排在这类组里的运动员)一律拒绝,只能在界面上删。原因:AI 不应在无人确认时抹掉比赛数据。
- **自动编排**:只追加新组,不提供「替换全部」。
- **晋级**:只新建下一轮次的组,只读来源组的成绩。

## 写成绩

- 与界面录入走同一个 service 方法:同样的解析、校验与排名重算,写入后同样广播变更事件,界面立即显示。
- **只填空位直接写**:目标位置还没有成绩时,`set_results` 可直接提交。
- **覆盖已有成绩走「预览 → 确认」**:任一目标位置已有成绩,必须先 `preview_results` 列出「原成绩 → 新成绩」,用户确认后带指纹与 `confirmed=true` 提交。原因:成绩识别出错时,AI 不应在无人确认时改掉裁判已录入的成绩。
- MCP 不提供清空成绩的操作,空行输入直接拒绝;删除成绩须在界面进行。
- 名次由 `scoring` 按成绩重算,MCP 不能直接指定名次;并列名次的人工裁定只在界面上做(用户要求)。`get_event` 的 `rounds[].ties` 列出各处并列与多出的晋级人数,`result.tieBreak` 是已保存的裁定。
- 高度每条输入的 `heightFinished` 默认 `false`，表示保存过程；确认该位置最终记录时传 `true`。轮次全部位置均确认后才生成晋级，覆盖过程记录以结束录入同样要预览确认；`get_event` 中 `result.heightPending=true` 表示尚在录入。试跳顺序校验与完成规则见 [domain.md](domain.md)「成绩」。
- 操作日志来源记为 `mcp`,详情里带原成绩与新成绩,便于追查与手动改回。
- 删除规则不变:有成绩的组、项目、运动员 MCP 仍不能删。

## 导入

- `preview_import` / `import_meet` 的 `data` 参数是完整的交换格式 JSON 文本,`meetId` 留空表示新赛事;内容遵循交换格式([interchange-format.md](../interchange-format.md))。
- 界面的「导入赛事文件」/「导入到本赛事」走同一组 service 方法(`PreviewImport` / `ImportMeet`)。
- 导入到已有赛事时:项目按全能归属 + 参赛类别 + 项目名 + 组别复用,轮次按阶段、组按序号对应;已有组的位置与文件一致才写入成绩,不一致则跳过该组并警告;文件里的名次原样保留。v2／v3 项目配置与目标不符时拒绝整份导入，v1 保留目标配置；见交换格式 §7。队伍按项目内的规范化队名与单位复用,名单必须一致,冲突时整份拒绝。
- 一次导入只针对一个赛事(已有赛事 id,或文件里的新赛事);每个项目必带参赛类别与项目名;运动员参赛类别与项目不符、组名里写着另一参赛类别等混放情形,MCP 预览即报错;这项附加校验位于 service 的 MCP 入口,普通界面文件导入仍遵循交换格式中的警告规则。
- 文件里的成绩照常导入,预览里分别列出新增与覆盖的成绩条数;有覆盖时逐条列出「原成绩 → 新成绩」。
- 导入在单个 SQLite 事务里完成,失败不留半成品。
- 运动员按交换格式 §7 识别新建 / 复用。
- 导出 `export_meet` 输出完整交换格式文件(含成绩)。
- `preview_import_results` / `import_results` 导入成绩交换格式文件([results-format.md](../results-format.md)):`meetId` 必填且须与文件一致,与界面「导入成绩文件」走同一组 service 方法(`PreviewResultsImport` / `ImportResults`),总是先预览、确认后带 previewHash 提交;预览列出新增、覆盖(逐条「原成绩 → 新成绩」)、未变的条数与警告。

## 监听与设置一致性

- 启动时为缺少令牌的已开启配置生成令牌，预留监听并保存配置后才开始处理请求；令牌保存失败时释放端口，页面显示原因；自动初始化的操作日志来源为 `system`。开启或换端口时先预留本机监听,保存设置成功后才开始处理请求并切换运行状态;端口占用或保存失败时保留原设置与监听。启动时恢复保存的开关,监听失败时应用仍可使用,「AI/MCP」页显示停止状态与原因。
- 每个已通过认证的请求持运行管理器读锁到结束;关闭、换端口和令牌轮换等候这些请求完成,新请求随后使用当前配置。关闭监听后端口释放;退出先关闭 MCP 再关闭数据库。
- 令牌使用 `crypto/rand` 生成 32 字节,只通过本地设置绑定显示或复制。日志只记设置键,不含令牌。复制令牌和连接信息使用 mygo 的 Concealed 剪贴板选项。
- 设置、令牌、原生文件对话框和系统打印窗口均不向 MCP 开放(赛事选用的打印样式 `set_meet_print_style` 除外);MCP 的导入导出传递数据,报表工具返回 HTML。

## 预览实现

- service 持真实写锁创建仅用于本次请求的计划实例,调用界面使用的业务方法;计划实例的 `write` 只收集一批 SQL、日志与通知,不执行写入、不广播。任一校验失败即返回错误,不会留下局部写入。
- 通用操作的预览指纹覆盖参数和目标赛事完整状态(包含成绩、逐次记录、名次、晋级与备注),不含导出时刻;新赛事导入覆盖输入和赛事列表。相关赛事任意数据变化都会要求重新预览。排时间的指纹覆盖赛事与每个目标轮次所属项目、轮次的完整状态,规则见 [arrange.md](arrange.md)「排时间」。
- 确认提交在同一把 service 写锁内重新生成计划、检查指纹、事务写入,成功后才广播。预览生成的新建实体 ID 不可用于后续操作,以提交返回的 ID 为准。
- 普通界面赛事导入的指纹同样包含文件内容与目标赛事状态,避免仅有备注等变化时复用确认。

## 工具目录

队伍与个人项目共用报名、编排、计划时间、成绩及晋级工具;队伍按队使用 ID,不将运动员直接排进接力道次。

单条保存的 `input` 表示完整字段（运动员上述两个可省略保留的字段除外）;更新前先读取,省略可选字段会使用空值。项目配置字段 `startMode`、`distanceMode`、`relayLegCount` 的取值与默认见 [domain.md](domain.md)「项目配置与预设」；`get_event.rules` 返回位置与尝试显示规则，队伍返回 `sizeWarning`。`save_athlete` 返回 `athlete` 与 `warnings`,同名仅警告。业务状态和整数成绩单位见 [domain.md](domain.md)。

| 工具 | 用途 |
|---|---|
| `get_team_scores` | 团体规则、按项目参赛类别／组别／项目集合筛选的单位排名、得分明细与计分进度；同分直接并列 |
| `preview_team_scoring` → `apply_team_scoring` | 修改赛事团体计分规则，预览原、新总分与名次，确认后提交（界面直接保存） |
| `list_meets` / `get_meet` | 赛事列表 / 详情 |
| `list_event_presets` | 读取常用项目初始 `input`，可调整后交给 `save_event` |
| `list_units` / `list_athletes` / `list_events` | 按赛事读取基础数据与进度;项目按日期与计划时间排序,见 [domain.md](domain.md)「时间」 |
| `get_event` | 队伍名单、报名、轮次(含开始时间)、组、位置、成绩、总排名与晋级;组给出单独时间 `scheduledAt`、生效时间 `effectiveAt`、`scheduleInherited`、预计用时 `durationMinutes`、结束时间 `endAt`,以及实际开赛时间 `startedAt` / `startedText` 与差值 `startDeltaText` |
| `create_meet` / `update_meet` | 创建 / 更新一个赛事,含报名限制 `entryLimits`(更新时省略即保持原值)；创建传 `input.sourceMeetId` 沿用时须先 `preview_create_meet` 并确认，规则见 [domain.md](domain.md)「沿用项目与赛制新建赛事」；更新拒绝该非空字段 |
| `preview_create_meet` → `create_meet` | 沿用已有赛事的项目与赛制；预览 `result.meet` 为新赛事资料、`result.projects` 为来源的组别／项目／轮次／全能与计分规则定义，不含参赛数据与时间；确认携带同一 `input`、`previewHash` 与 `confirmed=true`，定义或赛事列表变化后重预览，同一确认只能新建一次 |
| `save_unit` / `save_athlete` / `save_event` | 创建 / 更新单条数据;`id` 留空创建 |
| `save_team` | 创建 / 更新一个队伍及完整成员名单;规则见 [domain.md](domain.md)「队伍与接力」 |
| `set_seed` | 更新一个报名成绩;个人传 `athleteId`,接力传 `teamId` |
| `parse_table` | CSV / TSV 解析与列映射;导入只传数据行 |
| `export_meet` / `render_report` | 完整交换格式对象 / 报名表、秩序册、成绩册、团体总分表、检录单或成绩记录表的 HTML,返回 `{html, warnings}`,见「打印样式」 |
| 打印样式、前置页、图片 | 见「打印样式」一节,直接写入 |
| `preview_entries` → `add_entries` | 批量报名;每项二选一 `athleteId` / `teamId`;`result.warnings` 列出超出报名限制的人或单位,只提示不拦截 |
| `preview_remove_entries` → `remove_entries` | 取消报名;个人传 `athleteIds`,接力传 `teamIds` |
| `preview_delete_team` → `delete_team` | 传 `eventId` 与 `id`,删除未排进组的队伍及其报名 |
| `preview_athletes` → `import_athletes` | 表格运动员、单位与报名导入;各单位交回的报名登记表(或任意格式的报名表)由 AI 读出后整理成表格行,队伍再逐个 `save_team` 并报名。登记表文件导入只在界面 |
| `preview_import` → `import_meet` | 交换格式导入 |
| `preview_import_results` → `import_results` | 成绩交换格式导入 |
| `preview_arrange` → `auto_arrange` | 只追加新组,排除已保存的排位 |
| `preview_advance` → `advance` | 根据上一轮 Q / q 生成下一轮组 |
| `get_schedule` | 赛事竞赛日程(只读):按天、按生效时间的轮次与组,未安排时间的轮次,全部冲突 |
| `list_conflicts` | 赛事运动员时间冲突(只读),附所用最短休息时间 |
| `set_round_start` | 设置或清除一个轮次的开始时间;直接写入,返回 `warnings` |
| `set_heat_time` | 设置一个组的单独时间与预计用时;带组指纹,直接写入,返回新指纹、`warnings` 与该组的冲突 |
| `set_heat_started` | 修改或清除一个组的实际开赛时间;直接写入,返回该组新的开赛时间与差值 |
| `preview_schedule` → `schedule_heats` | `mode=rounds` 批量设轮次开始时间（可无组），留空或 `heats` 逐组连排并跳过已比赛组 |
| `preview_results` → `set_results` | 成绩与逐次记录,空成绩可直接提交 |
| `preview_delete_meet/unit/event/athletes/heat` → `delete_meet/unit/event/athletes/heat` | 各实体删除,按对应工具传赛事 / 项目 / 组 / 运动员 ID |

## 排时间参数

- `request.mode=rounds` 为轮次开始时间：`rounds` 按顺序指定轮次（也可只填 `eventId` / `roundId`），无需已有报名、组或晋级名单；`startAt` 是首轮时间，`intervalMinutes` 是相邻轮次开始时间间隔(1–1440 分钟)，`roundGapMinutes` 须为 0。只改轮次时间，不改组的单独时间、预计用时、位置和成绩。预览的 `roundChanges[]` 给出 `eventId`、`roundId`、`round`、`before`、`after`、`beforeText`、`afterText`；`changes` 与 `skipped` 为空。空轮次提示「名单未定，冲突待检查」，已有人员仍参与改后冲突检查；日志动作 `rounds.scheduled`。以下逐组规则适用于 `mode` 留空或 `heats`，保持旧客户端行为。
- `preview_schedule` 与 `schedule_heats` 都接受 `request: {eventId, roundId, startAt, intervalMinutes}`,排一个轮次;连排多个轮次时改传 `rounds: [{eventId, roundId}, …]`(按顺序,同一赛事)与可选的 `roundGapMinutes`(0–1440),此时不用 `eventId` / `roundId`。`startAt` 是首个未比赛组的本地时间,格式 `YYYY-MM-DDTHH:mm`;间隔单位为分钟。排序、跳过、轮次衔接与日期边界见 [arrange.md](arrange.md)「排时间」。
- 间隔同时写为每个被排组的预计用时 `durationMinutes`。
- 返回类型 `SchedulePreview`:顶层包含 `previewHash`、`errors`、`warnings`、`changes`、`roundChanges`、`skipped` 与 `conflicts`；逐组模式的 `roundChanges` 为空。每项改动与跳过都带所属 `eventId`、`roundId` 与显示名 `round`;每项逐组改动给出组 ID、组标签、原单独时间 `before`(空 = 沿用轮次)与新时间 `after`、原 / 新预计用时 `beforeMinutes` / `afterMinutes`,以及显示文本 `beforeText` / `afterText`;每项跳过给出组 ID、标签与原因。`warnings` 如「N 组不在赛事日期内」「排完后有 N 处时间冲突」,只提示不阻止提交,客户端须一并转告用户;`conflicts` 是排完后至少涉及一个被改组的冲突,格式同 `list_conflicts`。没有改动时数组为空。
- 提交还须同一组 `request`、`previewHash` 与 `confirmed=true`,客户端必须先展示预览并取得用户确认。界面与 MCP 调用同一组 `PreviewSchedule` / `ApplySchedule`,任一入口生成的指纹均可由另一入口验证;日志记录实际提交来源。

## 日程与冲突

- `Timetable.pending` 统计所有未排好时间的轮次，包括部分组缺时间；轮次另给出独立的 `timeState`(todo / doing / done)与 `arrange`，以及 `prevLabel`、`canArrange`、`unplaced`、`peopleText`、`conflictPending`。状态与名单显示规则见 [arrange.md](arrange.md)「竞赛日程」。没有组的后续轮次仍可设时间，未知晋级者不作为实际冲突计算。
- `get_schedule {meetId}` 返回 `Timetable`:`days[]`(`day` 与当天的 `rounds[]`)、`unscheduled[]`(全部没有时间的轮次,有组的在前)、`conflicts[]` 与 `minRestMinutes`。轮次给出项目、轮次名、开始时间 `scheduledAt` / `scheduledText`、最早计划时间 `startAt` / `startText`、组数 `heatCount`、人数 `people` 与 `peopleEntered`(口径见 [arrange.md](arrange.md)「竞赛日程」);组给出 `scheduledAt`、`effectiveAt`、`scheduleInherited`、`durationMinutes`、`endAt` / `endText`、`keepReason`(非空即已比赛)、实际开赛时间 `startedAt` / `startedText` 与差值 `startDeltaText`、`people` 与该组的冲突。
- `list_conflicts {meetId}` 返回 `{minRestMinutes, conflicts}`;每项冲突给出运动员、`overlap`、`gapMinutes`(重叠时为负)、文本 `text`(「时间重叠」/「间隔 N 分钟」)与两个组 `first` / `second`(项目、轮次、组的 ID 与名称,带日期的时间 `timeText`,完整名称 `text`)。规则见 [arrange.md](arrange.md)「冲突检查」;冲突只提示,任何写入都不因它被拒绝。
- 最短休息时间只能在应用「赛事设置」修改,MCP 不改设置。

## 轮次开始时间

- `set_round_start` 接受 `input: {eventId, roundId, scheduledAt}`,`scheduledAt` 为本地 `YYYY-MM-DDTHH:mm`,空字符串清除。它只改一个轮次字段、不改组与成绩,所以直接写入、不走预览;值不变时不写日志。返回 `{warnings}`,判断与界面的保存前提示相同(→ [domain.md](domain.md)「时间」),只提示不拦截。
- 组的单独时间与预计用时由 `auto_arrange` / `advance` 新建的组为空,之后经 `schedule_heats` 批量写入,或经 `set_heat_time` 逐组修改。

## 逐组改时间

- `set_heat_time` 接受 `input: {eventId, heatId, base, scheduledAt, durationMinutes}`。`base` 是 `get_event` 里该组的指纹;组在此之后有任何变化(位置、成绩、名称、时间)返回 `E_STALE`,须重新读取。`scheduledAt` 为本地 `YYYY-MM-DDTHH:mm`,空字符串表示沿用轮次开始时间;`durationMinutes` 为 1–1440 或 `null`。
- 只改这一组的时间与预计用时,组名、位置、成绩、名次不变;已比赛的组也可改。它与界面的「编辑组」共用 service(`SetHeatTime` / `EditHeat`),经编排保存的同一路径写入,日志动作 `board.saved`、来源 `mcp`。单个字段、不动成绩,所以直接写入、不走预览。
- 返回 `{base, warnings, conflicts}`:`base` 是改后的新指纹,可直接用于下一次修改;`warnings` 如「时间不在赛事日期内」;`conflicts` 是改后该组的时间冲突(运动员、`text`、另一组 `other`)。提示与冲突都不拦截写入,客户端须转告用户。

## 实际开赛时间

- 组第一次写入成绩时,`set_results` 与界面录入一样自动记下当时时刻(已有就不覆盖);规则见 [domain.md](domain.md)「时间」。
- `set_heat_started` 接受 `input: {heatId, startedAt}`,`startedAt` 为本地 `YYYY-MM-DDTHH:mm`,空字符串清除。只改这一个字段,不进组指纹、不影响锁定与成绩,已比赛的组也可改,所以直接写入、不走预览;值不变时不写日志。日志动作 `heat.started`、来源 `mcp`。
- 返回 `{heatId, startedAt, startedText, startDeltaText}`,与 `get_event` 中该组的字段相同。
- 检录提前时间只在应用「赛事设置」修改,MCP 不改设置;检录单与成绩记录表的 HTML 经 `render_report` 取得。

## 客户端连接

在「AI/MCP」页点「复制连接信息」,将应用生成的 `mcpServers.sportsmeet` 地址与 Authorization 请求头配置到支持 Streamable HTTP 的客户端。没有令牌的请求返回 401,非本机 Origin / Host 返回 403。应用不为客户端提供云模型或自动传输赛事数据。

页面列出的客户端支持 Streamable HTTP 与自定义认证请求头，兼容性依据官方文档；使用与 SportsMeet 同机运行的客户端，各自按下表配置。地址与 Authorization 请求头取「复制连接信息」，第三方客户端未逐一进行实机连接验证。

| 客户端与官方说明 | HTTP 连接配置 |
|---|---|
| [TRAE](https://docs.trae.ai/ide/add-mcp-servers) | HTTP 服务使用 `url` 与 `headers`，支持 Streamable HTTP |
| [Qoder](https://docs.qoder.com/cli/mcp-reference) | CLI 使用 `type: "http"`、`url` 与 `headers`；[IDE](https://docs.qoder.com/user-guide/chat/model-context-protocol) 自动识别 Streamable HTTP |
| [CodeBuddy](https://www.codebuddy.ai/docs/cli/mcp) | CodeBuddy Code 使用 `type: "http"`、`url` 与 `headers` |
| [WorkBuddy](https://open.workbuddy.cn/en/docs/connector) | Streamable HTTP 使用 `type: "streamableHttp"`、`url` 与 `headers` |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/developers/tools/mcp-server/) | 使用 `httpUrl` 与 `headers`，地址取复制信息中的 `url` |
| [Kimi Code](https://github.com/MoonshotAI/kimi-code/blob/main/docs/zh/customization/mcp.md) | 使用 `url` 与 `headers`，HTTP 为默认传输 |
| [Claude Code](https://code.claude.com/docs/en/mcp) | HTTP 连接可通过 `--header` 设置 Authorization |
| [Codex](https://developers.openai.com/codex/mcp/) | `config.toml` 的 `[mcp_servers.sportsmeet]` 使用 `url` 与 `http_headers` |
| [Cursor](https://cursor.com/docs/mcp) | `mcpServers` 下配置 `url` 与 `headers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | 使用 `httpUrl` 与 `headers`，地址取复制信息中的 `url` |
| [Google Antigravity](https://antigravity.google/docs/mcp) | 使用 `serverUrl` 与 `headers` |
| [Cline](https://docs.cline.bot/mcp/mcp-overview) | 使用 `type: "streamableHttp"`、`url` 与 `headers` |

## 全能与计次、计分

- `save_event` 的 `rounds` 列出决赛之前的轮次(`stage` 为 `heat / quarterfinal / semifinal`,各带 `advanceByPlace / advanceByMark`),留空即直接决赛;更新时未列出的轮次被删除,已有组的轮次不能删除(→ [domain.md](domain.md)「层级」)。
- `save_event` 支持 `count / score`，以及 `participantMode`、`teamSize`、`scoreDecimals`、`rankDirection`、`attemptMode`；`set_results` 使用项目配置解析、汇总和排名，多次模式须提交逐次成绩。普通队伍使用 `teamId`，规则与接力一致。
- `list_combined / get_combined / parse_point_rule` 查询；`preview_save_combined → save_combined` 保存全能及独立分项；`preview_combined_entries → set_combined_entries` 保存完整名单，预览结果的 `warnings` 列出超出报名限制的人或单位；`preview_delete_combined → delete_combined` 删除无成绩全能。批量操作沿用赛事快照指纹。
- `preview_combined_rules → apply_combined_rules` 接受 `request: {combinedId, rules: [{eventId, rule: {table, invalidPolicies}}]}`，必须给出所有分项；预览包含每人原、新逐项积分、总分、名次和资格，确认时携带 `previewHash`、`confirmed=true`。原始成绩不被规则修改覆盖，数据变化须重新预览。
- 全能分项不能经 `save_event / add_entries / remove_entries / delete_event` 绕过归属与统一报名；成绩仍用普通成绩工具，覆盖保护不变。完整规则见 [combined.md](combined.md)。

## 团体总分

- `get_team_scores` 接受 `{meetId, gender?, groupLabel?, targetIds?}`；`groupLabel` 省略为全部、空字符串为无组别；`targetIds` 为普通项目或全能 ID。查询结果包含规则、排名、单位明细、无单位明细与计分进度，`ScoreValue.exact` 为精确分数，`text` 为显示值。
- `preview_team_scoring` 与 `apply_team_scoring` 接受 `request: {meetId, rules: {enabled, places, tiePolicy, relayMultiplier, overrides}}`，确认提交携带 `previewHash` 与 `confirmed=true`。规则、完成口径、指纹范围、导入兼容与同分并列要求见 [team-scoring.md](team-scoring.md)。
- `render_report` 的 `kind=team_scores` 返回团体总分表 HTML(`{html, warnings}`)，`eventIds` 沿用普通项目与全能的选择；不调用系统打印窗口。

## 赛事组别

- `list_groups` / `save_group` 管理赛事目录与改名；`suggest_groups` 只读归属建议。单位输入支持 `groupIds`，运动员、项目与全能输入支持 `groupId`，所有校验共用 service；运动员的组别随单位组别联动，`preview_group_assignments` 的变化与 `follows` 列出联动结果，完整归属规则见 [groups.md](groups.md)。
- `preview_group_change → apply_group_change` 接受 `request={meetId,groupId,eventIds,combinedIds}`，确认调整已有项目或全能；`preview_group_assignments → apply_group_assignments` 接受 `request={meetId,assignments:[{kind,id,groupIds}]}`，`kind` 为 `unit` 或 `athlete`。提交须 `previewHash` 与 `confirmed=true`，预览内容变化返回 `E_STALE`。界面不经预览直接调整。
- `preview_delete_group → delete_group` 与 `preview_reorder_groups → reorder_groups` 使用通用预览；删除组别时引用它的项目、全能与单位移出该组别，运动员随单位剩下的组别调整，改后项目身份重复时预览 `errors` 列出原因，不能提交。排序 `ids` 必须完整包含赛事全部组别。
- `get_team_score_boards` 返回各组独立排名，`groupId` 省略表示全部组别分别出榜，空字符串表示未分组；`get_team_scores` 的全赛事汇总接口保留兼容调用。
- `render_report` 的成绩册与团体表支持 `groupIds`，省略或空数组为全部，数组中的空字符串选择未分组；`eventIds` 限定所选项目，按组分别汇总并另起页。

## 打印样式

打印样式(版面参数、自定义 CSS、插槽、报表模板、图片)全局保存,赛事选用其一;前置页属于赛事。内置「默认」样式不存储、不能改,新样式从它复制或起步。清洗、模板、样式文件与报表数据的实现见 `internal/printstyle`、`internal/report`。自定义内容由 AI 经 MCP 写,普通用户不接触。

**写入与删除直接生效,不走预览 → 确认(用户要求,「预览 → 确认 → 提交」的例外)**:样式、前置页与图片不是比赛记录,改坏不丢数据;样式每次保存留一个版本(最近 30 个),`restore_print_style_version` 可回退,前置页和图片由 `list_*` 与 `save_*` 修正。这些工具没有 `previewHash` / `confirmed` 参数;日志来源 `mcp`。

| 工具 | 用途 |
|---|---|
| `list_print_styles` | 内置默认样式与自定义样式,含版本、被几个赛事使用、是否有自定义 CSS 与模板 |
| `get_print_style` | 一个样式的 `params`、`css`、`slots`、`templates`、图片与使用它的赛事,外加接口说明 `guide`(`noGuide=true` 时省略) |
| `save_print_style` | `id` 省略为新建;`input` 各字段省略即不改:`name`、`params`(完整)或 `paramsPatch`(部分,对象或其 JSON 文本,二选一)、`css`(整体替换)、`slots` 与 `templates`(按键修改,值空白即删除该键)。返回 `{style, version, changed, warnings}`,`changed=false` 不产生版本 |
| `copy_print_style` / `rename_print_style` / `delete_print_style` | 复制(含图片,内置默认样式也可复制,同名报错)/ 改名 / 删除(使用它的赛事读作默认样式);内置默认样式不能改名与删除 |
| `list_print_style_versions` / `restore_print_style_version` | 保留的版本(新的在前,含来源与原因)/ 恢复参数、CSS、插槽与模板为新的一次保存,名称与图片不变 |
| `export_print_style` / `import_print_style` | 返回样式文件 JSON 对象(含图片 base64)/ 接受同一对象或其 JSON 文本,总是新建一个样式,不碰文件系统 |
| `get_report_template` | 某报表种类的内置模板源码、子模板 `parts`、数据类型 `dataTypes`(每个字段的 Go 名与 JSON 键名、可调用的方法)、模板函数;`withSample` 附样例数据 |
| `get_report_data` | 某赛事某报表在范围内的数据 JSON 与 `dataVersion`,就是自定义模板收到的 `Book` / `CheckInBook` / `RecordBook`;只读 |
| `render_report` | 见下 |
| `render_style_sample` | 不依赖赛事,用内置样例数据渲染 `kind`;`styleId` 省略为内置默认样式,`draft` 试写改动不保存;返回 `{html, warnings}` |
| `set_meet_print_style` | 赛事选用样式(`default` 为内置默认样式),直接生效 |
| `list_front_pages` / `save_front_page` / `delete_front_page` / `reorder_front_pages` | 赛事前置页(Markdown);`save_front_page` 的 `id` 省略为新增到末尾,返回 `{page, warnings}`(找不到的图片);`reorder_front_pages` 的 `ids` 须恰好包含全部前置页 |
| `list_assets` / `save_asset` / `delete_asset` | `scope` 为 `style` 或 `meet`,`ownerId` 是样式或赛事 ID;`save_asset` 传 `dataBase64`(也接受 `data:` URI),`mime` 可省;列表不返回图片数据 |

- **`render_report`**:参数 `meetId`、`kind`(`entries`、`startlist`、`results`、`team_scores`、`checkin`、`recordsheet`)、`eventIds`、`groupIds`(成绩册与团体表)、`roundIds` 与 `day`(检录单与成绩记录表,选择同界面)、`styleId`(覆盖赛事选用,`default` 为内置)、`draft`(同 `save_print_style` 的 `input`,不含 `name`,叠在样式上渲染、不保存、按保存同样的规则检查与清洗)。返回 `{html, warnings}`;`warnings` 是:被清洗的内容、找不到的图片、出错而回退默认版式的模板、旧数据版本而失效的模板。`styleId` 不存在、草稿不合格(参数越界、模板语法或运行出错)返回错误。与界面共用 `RenderReportRequest`,不调用系统打印窗口。
- **接口说明 `guide`** 由 `service.PrintGuide` 生成:参数、CSS 变量、类名、插槽、报表种类与限制来自代码和样式表,取值范围由测试对照 `Params.Validate`;分页约定、模板、前置页 Markdown、图片引用与清洗规则是文字说明。改版面规则时同步这些文字与测试。
- 模板与插槽用 Go 字段名取数(`{{.Meet}}`),`get_report_data` 的 JSON 用各字段的 json 名;自定义报表模板只写 `<body>` 内容并替换整个正文,封面、前置页、页眉页脚由系统放入。模板针对报表数据版本 `dataVersion` 写成,版本变化后不再使用并给出警告。
- 保存时模板与插槽在样例数据上试运行,CSS、HTML、Markdown 清洗后写入,清洗掉的内容在 `warnings` 里列出;文档另带 Content-Security-Policy,不联网。图片只收 png / jpeg / webp、单张 2 MB;MCP 请求体上限 2MB(含 base64 与样式文件的图片),所以经 MCP 传的单张图约 1.5 MB 以内。
