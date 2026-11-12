# 本地数据(`internal/store/`)

## 数据库文件

- 一个 SQLite 文件,放在 `mygo.PathUserData`,经 mygo SQLite 插件的 Go API(`sqlite.Open`)打开;启动时先确保目录存在。
- 目录取不到、数据库打不开或迁移失败时,`main.go` 弹出系统错误对话框(数据目录与原因),然后退出;应用没有数据就没有可显示的内容,静默退出会让用户以为是闪退。
- 打开后启用 WAL。插件默认开启外键:级联删除由表上的 `ON DELETE CASCADE` 完成,service 不再手动逐表删除。

## 设置

- 复用 `settings(key,value)` 表,`preferences` 与 `mcp` 两个键分别存 JSON。缺失时使用默认值,已有数据库无需新增迁移。
- 普通偏好默认跟随系统外观、开启同单位分散和均衡人数、关闭项目另起一页。MCP 默认关闭、端口 18765,首次开启才生成令牌。
- 所有设置写入走 service 写锁与日志,广播 `settings` 变更;令牌只保存在本地配置中,不写操作日志。监听切换规则见 [mcp.md](mcp.md)。

## 迁移

- `internal/store/migrations/NNNN_名称.sql`,启动时按序号执行未执行过的,执行记录存在 `schema_migrations` 表。每个迁移在一个事务里完成。
- 同一改动里更新 `schema.sql` 快照(当前完整结构)。`internal/store/schema_test.go` 比对快照与迁移执行后的表、列、外键和索引,不一致即失败;`CHECK` 约束不在比对范围内,靠人工对照。
- 已发布的迁移文件不再修改,只追加新迁移:用户机器上已经跑过旧内容,改了不会重跑。

- **迁移前备份**:数据库里已有执行记录、又有待执行的迁移时,先关闭连接,把 `sportsmeet.db` 复制为数据目录下的 `backups/sportsmeet-<年月日-时分秒>-v<迁移前序号>.db`,保留最新 5 份,再重新打开执行迁移。复制失败(目录不可写、另一个进程正占用数据库)就不迁移,`Open` 返回错误。迁移在某份真实数据上出错时,把副本放回原处即可恢复。
- 复制文件而不用 `VACUUM INTO`:插件拒绝这条语句。关闭最后一个连接会把预写日志并入主文件,所以单个文件就是完整副本;复制前若仍有非空的 `-wal` 文件,说明别的进程开着它,按失败处理。
- 新建的数据库没有可保留的数据,不备份。

## 索引与唯一性

- **外键所在的列都有索引**(`0005_fk_indexes.sql` 补齐 `slots.team_id`、`entries.team_id`、`team_members.athlete_id`、`events.combined_id`、`combined_entries.athlete_id`)。级联删除要按这些列找受影响的行,没有索引就是全表扫描。新增外键列时同一迁移里建索引。
- **号码布 / 学号的赛事内唯一由 service 校验,库里不建唯一索引**。SQLite 的唯一索引逐条语句检查、不能推迟到事务结束,一次导入里两名运动员互换号码时,第一条 `UPDATE` 就会失败,而最终状态是合法的。

## 项目配置

- `events.start_mode`、`distance_mode`、`relay_leg_count` 独立保存起跑方式、跳跃 / 投掷与建议棒数；追加迁移 `0002_event_config.sql`，缺省为空、空、0。读取这些缺省值时按领域兼容规则解释，保留已有道次和成绩，不依据项目名批量改数据。

## 连接与并发

- mygo 每次绑定调用在独立 goroutine 执行,MCP 请求也并发到达。**只有一个写连接**,所有写入走 `Store.Write`,避免 goroutine 之间出现 `SQLITE_BUSY`。
- **插件的事务只接收一批语句**(`Transaction([]Statement)`),不能在事务里先读再决定写什么。所以 service 的写方法在持有 `Service.mu` 期间完成「读 → 校验 / 计算 → 一批写入」;所有写都经 service,持锁期间读到的数据就不会被别处改掉。不要在 service 之外调 `Store.Write`,否则这个保证失效。
- 读走单独的只读连接(`Store.Query`),大列表查询不会把写入排在后面。
- 迁移文件按「行尾分号」切成语句逐条执行,迁移里不写触发器,也不写跨行且含行尾分号的字符串。

## 全能与队伍数据

- `0003_scalar_events.sql` 的 `events.config_json` 保存参赛方式、建议人数、小数位、汇总方式与排名方向；缺省 JSON 为 `{}`，按领域默认解释。
- `0004_combined.sql` 的 `combined_events`、`combined_components`、`combined_entries` 保存全能、规则与统一名单；`events.combined_id` 外键级联分项。`Store.ListCombined` 对查询范围固定读取父项、分项和报名三次，不逐个全能查询。
- 全能读取一次加载赛事项目、轮次、运动员和单位，计分与打印在内存组装，查询次数不随分项或组数增长。完整规则见 [combined.md](combined.md)。

## 队伍名单

- 使用 `teams` / `team_members` 与 `entries.team_id`、`slots.team_id`,棒次为从 1 开始的连续 `leg`;个人项目只写 `athlete_id`。普通队伍与接力共用这组外键与名单结构。
- `ListTeams` 对整个查询范围读取队伍与成员,固定两次查询;运动员使用计数合并个人报名与经队伍取得的报名和位置。名单删除保护由 service 完成,见 [domain.md](domain.md)「队伍与接力」。

## 查询

- **列表查询次数不随组数增长**:项目概览、组列表一次读齐组、位置、运动员、成绩再在内存组装,不逐组查询。运动会动辄几百组,逐组查询会让每次变更后的界面刷新明显卡顿。
- 时间戳存毫秒整数;日期存 `YYYY-MM-DD` 文本,计划时间存本地 `YYYY-MM-DDTHH:mm` 文本,与交换格式一致,不做时区换算。
