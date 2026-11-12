# MCP 服务

`internal/mcp/` 使用官方 Go SDK 提供 Streamable HTTP,由 `main.go` 创建管理器并随应用启动 / 退出。`internal/service/mutation*.go` 与 `internal/service/schedule.go` 提供预览和确认提交,设置经本机 mygo 绑定管理。
工具只调用 `internal/service`。这是应用唯一的对外接口。

## 开关与连接

- **默认关闭**,在设置「MCP」开启;关闭时端口不监听。
- 只监听 `127.0.0.1`,端口默认 18765,设置允许 1024–65535。地址为 `http://127.0.0.1:<port>/mcp`。传输为 Streamable HTTP,用官方 Go SDK `github.com/modelcontextprotocol/go-sdk`。
- **独立的高熵 Bearer 令牌**:首次开启时生成,常量时间比较;设置里可显示 / 复制 / 轮换,轮换后旧客户端立即失效。
- 请求带 `Origin` 且不是本机时拒绝(403),防止网页里的脚本借用户浏览器调用本机服务。
- 请求体上限 2MB,包含未声明长度的请求;超限返回 413。同时检查 `Host`,只允许 localhost 与回环地址。传输使用无状态模式,每次请求都验证令牌。
- 设置页提供「复制连接信息」:由应用生成地址与令牌的通用配置片段,不区分客户端。

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

- **删除**:只能删没有成绩的赛事、组、项目、运动员;有成绩的(及含这类组的赛事 / 项目、排在这类组里的运动员)一律拒绝,只能在界面上删。原因:AI 不应在无人确认时抹掉比赛数据。
- **自动编排**:只追加新组,不提供「替换全部」。
- **晋级**:只新建下一轮次的组,只读来源组的成绩。

## 写成绩

- 与界面录入走同一个 service 方法:同样的解析、校验与排名重算,写入后同样广播变更事件,界面立即显示。
- **只填空位直接写**:目标位置还没有成绩时,`set_results` 可直接提交。
- **覆盖已有成绩走「预览 → 确认」**:任一目标位置已有成绩,必须先 `preview_results` 列出「原成绩 → 新成绩」,用户确认后带指纹与 `confirmed=true` 提交。原因:成绩识别出错时,AI 不应在无人确认时改掉裁判已录入的成绩。
- MCP 不提供清空成绩的操作,空行输入直接拒绝;删除成绩须在界面进行。
- 名次由 `scoring` 按成绩重算,MCP 不能直接指定名次;并列名次的人工裁定只在界面上做。
- 操作日志来源记为 `mcp`,详情里带原成绩与新成绩,便于追查与手动改回。
- 删除规则不变:有成绩的组、项目、运动员 MCP 仍不能删。

## 导入

- `preview_import` / `import_meet` 的 `data` 参数是完整的交换格式 JSON 文本,`meetId` 留空表示新赛事;内容遵循交换格式([interchange-format.md](../interchange-format.md))。
- 界面的「导入赛事文件」/「导入到本赛事」走同一组 service 方法(`PreviewImport` / `ImportMeet`)。
- 导入到已有赛事时:项目按全能归属 + 性别 + 项目名 + 组别复用,轮次按阶段、组按序号对应;已有组的位置与文件一致才写入成绩,不一致则跳过该组并警告;文件里的名次原样保留。v2／v3 项目配置与目标不符时拒绝整份导入，v1 保留目标配置；见交换格式 §7。队伍按项目内的规范化队名与单位复用,名单必须一致,冲突时整份拒绝。
- 一次导入只针对一个赛事(已有赛事 id,或文件里的新赛事);每个项目必带性别与项目名;运动员性别与项目不符、组名里写着另一性别等混放情形,MCP 预览即报错;这项附加校验位于 service 的 MCP 入口,普通界面文件导入仍遵循交换格式中的警告规则。
- 文件里的成绩照常导入,预览里分别列出新增与覆盖的成绩条数;有覆盖时逐条列出「原成绩 → 新成绩」。
- 导入在单个 SQLite 事务里完成,失败不留半成品。
- 运动员按交换格式 §7 识别新建 / 复用。
- 导出 `export_meet` 输出完整交换格式文件(含成绩)。

## 监听与设置一致性

- 开启或换端口时先预留本机监听,保存设置成功后才开始处理请求并切换运行状态;端口占用或保存失败时保留原设置与监听。启动时恢复保存的开关,监听失败时应用仍可使用,设置页显示停止状态与原因。
- 每个已通过认证的请求持运行管理器读锁到结束;关闭、换端口和令牌轮换等候这些请求完成,新请求随后使用当前配置。关闭监听后端口释放;退出先关闭 MCP 再关闭数据库。
- 令牌使用 `crypto/rand` 生成 32 字节,只通过本地设置绑定显示或复制。日志只记设置键,不含令牌。复制令牌和连接信息使用 mygo 的 Concealed 剪贴板选项。
- 设置、令牌、原生文件对话框和系统打印窗口均不向 MCP 开放;MCP 的导入导出传递数据,报表工具返回 HTML。

## 预览实现

- service 持真实写锁创建仅用于本次请求的计划实例,调用界面使用的业务方法;计划实例的 `write` 只收集一批 SQL、日志与通知,不执行写入、不广播。任一校验失败即返回错误,不会留下局部写入。
- 通用操作的预览指纹覆盖参数和目标赛事完整状态(包含成绩、逐次记录、名次、晋级与备注),不含导出时刻;新赛事导入覆盖输入和赛事列表。相关赛事任意数据变化都会要求重新预览。批量排时间的指纹覆盖目标项目与轮次完整状态,规则见 [arrange.md](arrange.md)「批量排时间」。
- 确认提交在同一把 service 写锁内重新生成计划、检查指纹、事务写入,成功后才广播。预览生成的新建实体 ID 不可用于后续操作,以提交返回的 ID 为准。
- 普通界面赛事导入的指纹同样包含文件内容与目标赛事状态,避免仅有备注等变化时复用确认。

## 工具目录

队伍与个人项目共用报名、编排、计划时间、成绩及晋级工具;队伍按队使用 ID,不将运动员直接排进接力道次。

单条保存的 `input` 表示完整字段;更新前先读取,省略可选字段会使用空值。项目配置字段 `startMode`、`distanceMode`、`relayLegCount` 的取值与默认见 [domain.md](domain.md)「项目配置与预设」；`get_event.rules` 返回位置与尝试显示规则，队伍返回 `sizeWarning`。`save_athlete` 返回 `athlete` 与 `warnings`,同名仅警告。业务状态和整数成绩单位见 [domain.md](domain.md)。

| 工具 | 用途 |
|---|---|
| `list_meets` / `get_meet` | 赛事列表 / 详情 |
| `list_event_presets` | 读取常用项目初始 `input`，可调整后交给 `save_event` |
| `list_units` / `list_athletes` / `list_events` | 按赛事读取基础数据与进度 |
| `get_event` | 队伍名单、报名、轮次、组、位置、成绩、总排名与晋级 |
| `create_meet` / `update_meet` | 创建 / 更新一个赛事 |
| `save_unit` / `save_athlete` / `save_event` | 创建 / 更新单条数据;`id` 留空创建 |
| `save_team` | 创建 / 更新一个队伍及完整成员名单;规则见 [domain.md](domain.md)「队伍与接力」 |
| `set_seed` | 更新一个报名成绩;个人传 `athleteId`,接力传 `teamId` |
| `parse_table` | CSV / TSV 解析与列映射;导入只传数据行 |
| `export_meet` / `render_report` | 完整交换格式对象 / 秩序册或成绩册 HTML |
| `preview_entries` → `add_entries` | 批量报名;每项二选一 `athleteId` / `teamId` |
| `preview_remove_entries` → `remove_entries` | 取消报名;个人传 `athleteIds`,接力传 `teamIds` |
| `preview_delete_team` → `delete_team` | 传 `eventId` 与 `id`,删除未排进组的队伍及其报名 |
| `preview_athletes` → `import_athletes` | 表格运动员、单位与报名导入 |
| `preview_import` → `import_meet` | 交换格式导入 |
| `preview_arrange` → `auto_arrange` | 只追加新组,排除已保存的排位 |
| `preview_advance` → `advance` | 根据上一轮 Q / q 生成下一轮组 |
| `preview_schedule` → `schedule_heats` | 批量排当前轮次的计划时间,跳过已比赛的组 |
| `preview_results` → `set_results` | 成绩与逐次记录,空成绩可直接提交 |
| `preview_delete_meet/unit/event/athletes/heat` → `delete_meet/unit/event/athletes/heat` | 各实体删除,按对应工具传赛事 / 项目 / 组 / 运动员 ID |

## 批量排时间参数

- `preview_schedule` 与 `schedule_heats` 都接受 `request: {eventId, roundId, startAt, intervalMinutes}`。`startAt` 是首个未比赛组的本地时间,格式 `YYYY-MM-DDTHH:mm`;间隔单位为分钟。排序、跳过与日期边界见 [arrange.md](arrange.md)「批量排时间」。
- 返回类型 `SchedulePreview`:顶层包含 `previewHash`、`errors`、`changes` 与 `skipped`;每项改动给出组 ID、组标签、原时间和新时间及显示文本,每项跳过给出组 ID、标签与原因。没有改动时数组为空。
- 提交还须同一组 `request`、`previewHash` 与 `confirmed=true`,客户端必须先展示预览并取得用户确认。界面与 MCP 调用同一组 `PreviewSchedule` / `ApplySchedule`,任一入口生成的指纹均可由另一入口验证;日志记录实际提交来源。

## 客户端连接

在「设置 → MCP」开启后点「复制连接信息」,将应用生成的 `mcpServers.sportsmeet` 地址与 Authorization 请求头配置到支持 Streamable HTTP 的客户端。没有令牌的请求返回 401,非本机 Origin / Host 返回 403。应用不为客户端提供云模型或自动传输赛事数据。

## 全能与计次、计分

- `save_event` 支持 `count / score`，以及 `participantMode`、`teamSize`、`scoreDecimals`、`rankDirection`、`attemptMode`；`set_results` 使用项目配置解析、汇总和排名，多次模式须提交逐次成绩。普通队伍使用 `teamId`，规则与接力一致。
- `list_combined / get_combined / parse_point_rule` 查询；`preview_save_combined → save_combined` 保存全能及独立分项；`preview_combined_entries → set_combined_entries` 保存完整名单；`preview_delete_combined → delete_combined` 删除无成绩全能。批量操作沿用赛事快照指纹。
- `preview_combined_rules → apply_combined_rules` 接受 `request: {combinedId, rules: [{eventId, rule: {table, invalidPolicies}}]}`，必须给出所有分项；预览包含每人原、新逐项积分、总分、名次和资格，确认时携带 `previewHash`、`confirmed=true`。原始成绩不被规则修改覆盖，数据变化须重新预览。
- 全能分项不能经 `save_event / add_entries / remove_entries / delete_event` 绕过归属与统一报名；成绩仍用普通成绩工具，覆盖保护不变。完整规则见 [combined.md](combined.md)。
