# 本地数据(`internal/store/`)

## 数据库文件

- 一个 SQLite 文件,放在 `mygo.PathUserData`,经 mygo SQLite 插件的 Go API(`sqlite.Open`)打开;启动时先确保目录存在。
- 目录取不到、数据库打不开或迁移失败时,`main.go` 弹出系统错误对话框(数据目录与原因),然后退出;应用没有数据就没有可显示的内容,静默退出会让用户以为是闪退。
- 打开后启用 WAL。插件默认开启外键:级联删除由表上的 `ON DELETE CASCADE` 完成,service 不再手动逐表删除。

## 设置

- 复用 `settings(key,value)` 表,`preferences` 与 `mcp` 两个键分别存 JSON。缺失时使用默认值,已有数据库无需新增迁移。
- 普通偏好默认跟随系统外观、开启同单位分散和均衡人数、关闭项目另起一页、秩序册印代表队名单与竞赛日程表、最短休息时间 0(只查重叠)、检录提前时间 20 分钟、检录单不按组分页、成绩册不印成绩时间。已存的 `preferences` JSON 缺少某个字段时,该字段按默认值读取。MCP 默认关闭、端口 18765,首次开启才生成令牌。
- 所有设置写入走 service 写锁与日志,广播 `settings` 变更;令牌只保存在本地配置中,不写操作日志。监听切换规则见 [mcp.md](mcp.md)。

## 迁移

- `internal/store/migrations/NNNN_名称.sql`,启动时按序号执行未执行过的,执行记录存在 `schema_migrations` 表。每个迁移在一个事务里完成。
- 同一改动里更新 `schema.sql` 快照(当前完整结构)。`internal/store/schema_test.go` 比对快照与迁移执行后的表、列、外键和索引,不一致即失败;`CHECK` 约束不在比对范围内,靠人工对照。
- 已发布的迁移文件不再修改,只追加新迁移:用户机器上已经跑过旧内容,改了不会重跑。

- **迁移前备份**:数据库里已有执行记录、又有待执行的迁移时,先关闭连接,把 `sportsmeet.db` 复制为数据目录下的 `backups/sportsmeet-<年月日-时分秒>-v<迁移前序号>.db`,保留最新 5 份,再重新打开执行迁移。复制失败(目录不可写、另一个进程正占用数据库)就不迁移,`Open` 返回错误。迁移在某份真实数据上出错时,把副本放回原处即可恢复。
- 复制文件而不用 `VACUUM INTO`:插件拒绝这条语句。关闭最后一个连接会把预写日志并入主文件,所以单个文件就是完整副本;复制前若仍有非空的 `-wal` 文件,说明别的进程开着它,按失败处理。
- 新建的数据库没有可保留的数据,不备份。

## 索引与唯一性

- **外键所在的列都有完整索引**，`entries.event_id` 使用 `idx_entries_event`，个人 / 队伍的条件索引不能覆盖整个项目的报名查询。级联删除要按这些列找受影响的行,没有索引就是全表扫描。新增外键列时同一迁移里建索引。
- **号码布 / 学号的赛事内唯一由 service 校验,库里不建唯一索引**。SQLite 的唯一索引逐条语句检查、不能推迟到事务结束,一次导入里两名运动员互换号码时,第一条 `UPDATE` 就会失败,而最终状态是合法的。

- `0008_query_indexes.sql` 为运动员增加 `bib_key`、`student_number_key`、`name_key` 与赛事内的非唯一索引，并为 `entries.event_id` 建完整索引。迁移在 Go 中按 `domain.NormalizeText` 计算已有运动员的检索键，与结构变更放在同一事务中；SQLite 的 `lower` / `trim` 无法代替这套 Unicode 与空白规则。`service` 的运动员新增 / 更新语句同步维护三个键，原始号码、学号与姓名保持原值；交换文件不包含这些内部检索键。

## 项目配置

- `events.start_mode`、`distance_mode`、`relay_leg_count` 独立保存起跑方式、跳跃 / 投掷与建议棒数；追加迁移 `0002_event_config.sql`，缺省为空、空、0。读取这些缺省值时按领域兼容规则解释，保留已有道次和成绩，不依据项目名批量改数据。

## 计划时间

- `0006_schedule.sql` 追加 `rounds.scheduled_at`(轮次开始时间,`TEXT NOT NULL DEFAULT ''`,空为未安排)与 `heats.duration_minutes`(预计用时,可空整数),并建操作日志索引 `idx_audit_meet(meet_id, ts)`。已有数据库升级后轮次都未安排开始时间、组都没有预计用时,组自己的 `scheduled_at` 保持原值,显示与排序不变。
- `0007_heat_started.sql` 追加 `heats.started_at`(实际开赛时间,`TEXT NOT NULL DEFAULT ''`,空为未记录);已有的组升级后都未记录。它与位置、成绩分开存,编排保存的 `UPDATE` 不写这一列,所以重排位置不会清掉它。
- `heats.scheduled_at` 只存组单独指定的时间,沿用轮次的组存空;生效时间在读取后由 domain 计算,不存储,轮次开始时间改动时不需要改写组(→ [domain.md](domain.md)「时间」)。

## 连接与并发

- mygo 每次绑定调用在独立 goroutine 执行,MCP 请求也并发到达。**只有一个写连接**,所有写入走 `Store.Write`,避免 goroutine 之间出现 `SQLITE_BUSY`。
- **插件的事务只接收一批语句**(`Transaction([]Statement)`),不能在事务里先读再决定写什么。所以 service 的写方法在持有 `Service.mu` 期间完成「读 → 校验 / 计算 → 一批写入」;所有写都经 service,持锁期间读到的数据就不会被别处改掉。不要在 service 之外调 `Store.Write`,否则这个保证失效。
- 读取经 `Store.Query` 使用固定 4 个只读连接，等待连接时支持取消；关闭会唤醒等待者并中断在途查询，全部连接只关闭一次。service 写锁与单一写连接保持读后校验的一致性；只读连接不用于写入。
- 迁移文件按「行尾分号」切成语句逐条执行,迁移里不写触发器,也不写跨行且含行尾分号的字符串。

## 全能与队伍数据

- 高度的 `heightPending` 与 `attempts / bars` 共用 `results.detail_json`；缺少该字段时按已确认读取，无需结构迁移。读回与写出均保留过程状态，成绩复制、重新排名和编排移动位置时一并保留。

- `0003_scalar_events.sql` 的 `events.config_json` 保存参赛方式、建议人数、小数位、汇总方式与排名方向；缺省 JSON 为 `{}`，按领域默认解释。
- `0004_combined.sql` 的 `combined_events`、`combined_components`、`combined_entries` 保存全能、规则与统一名单；`events.combined_id` 外键级联分项。`Store.ListCombined` 对查询范围固定读取父项、分项和报名三次，不逐个全能查询。
- 全能读取一次加载赛事项目、轮次、运动员和单位，计分与打印在内存组装，查询次数不随分项或组数增长。完整规则见 [combined.md](combined.md)。

## 队伍名单

- 使用 `teams` / `team_members` 与 `entries.team_id`、`slots.team_id`,棒次为从 1 开始的连续 `leg`;个人项目只写 `athlete_id`。普通队伍与接力共用这组外键与名单结构。
- `ListTeams` 对整个查询范围读取队伍与成员,固定两次查询;运动员使用计数合并个人报名与经队伍取得的报名和位置。名单删除保护由 service 完成,见 [domain.md](domain.md)「队伍与接力」。

## 查询

- **列表查询次数不随组数增长**:项目概览由 `EventOverviews` 在 SQLite 中汇总报名、组、位置、成绩计数与轮次时间，只返回轮次与项目数量的行；详情才读取位置和成绩。运动会动辄几百组,逐组查询会让每次变更后的界面刷新明显卡顿。
- 时间戳存毫秒整数;日期存 `YYYY-MM-DD` 文本,计划时间存本地 `YYYY-MM-DDTHH:mm` 文本,与交换格式一致,不做时区换算;校验与显示见 [domain.md](domain.md)「时间」。
- 组读取在成绩那次查询里一并取 `updated_at`,算出每组成绩的最后修改时间,不另查。
- 操作日志按 `ts DESC, id DESC` 读取,以「时刻 + id」接续分页,一页一次查询;不筛赛事时走 `idx_audit_ts`,按赛事筛选时走 `idx_audit_meet(meet_id, ts)`,不必扫过其他赛事的日志。
- 组读取的那次查询一并取 `duration_minutes`,轮次读取一并取 `scheduled_at`,查询次数不变。
- 日程、草稿冲突与改时间后的冲突通过 `LoadPlanRounds` 一次读取轮次、组、位置与成绩存在标记，位置查询合并成绩修改时间，不读取成绩详情；`EntryPeople` 在 SQLite 中按赛事项目汇总报名人数并展开队伍。`loadMeetPlan` 的查询次数固定。`LoadPlanRounds` 的结果只用于时间、人数与锁定状态，不用于排名、成绩显示或写回。
- 批量排时间的预览 / 提交在轻量全赛事日程之上，固定一次 `LoadRounds` 读取全部目标项目的完整轮次与成绩，供确认指纹使用；成绩或备注变化仍使确认失效。
- 项目详情、自动编排与成绩保存用 `ListParticipants` 读取该项目的报名者、位置上的运动员与队伍成员；报名、编排保存和队伍名单编辑仍读取赛事全部运动员，允许加入尚未报名者。
- 秩序册读取时间、位置与报名成绩，不加载最终成绩或计算排名；成绩册读取完整成绩。按部分项目打印时，先展开选中全能的分项，再按项目集合读取报名、组、成绩、队伍与运动员；`Scope.EventIDs` 为 nil 表示整个赛事，空集合表示不选择任何项目，集合查询使用 `json_each` 维持固定参数和查询次数。
