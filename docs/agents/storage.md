# 本地数据(`internal/store/`)

## 数据库文件

- 一个 SQLite 文件,放在 `mygo.PathUserData`,经 mygo SQLite 插件的 Go API(`sqlite.Open`)打开;启动时先确保目录存在。
- 打开后启用 WAL。插件默认开启外键:级联删除由表上的 `ON DELETE CASCADE` 完成,service 不再手动逐表删除。

## 设置

- 复用 `settings(key,value)` 表,`preferences` 与 `mcp` 两个键分别存 JSON。缺失时使用默认值,已有数据库无需新增迁移。
- 普通偏好默认跟随系统外观、开启同单位分散和均衡人数、关闭项目另起一页。MCP 默认关闭、端口 18765,首次开启才生成令牌。
- 所有设置写入走 service 写锁与日志,广播 `settings` 变更;令牌只保存在本地配置中,不写操作日志。监听切换规则见 [mcp.md](mcp.md)。

## 迁移

- `internal/store/migrations/NNNN_名称.sql`,启动时按序号执行未执行过的,执行记录存在 `schema_migrations` 表。每个迁移在一个事务里完成。
- 同一改动里更新 `schema.sql` 快照(当前完整结构)。`internal/store/schema_test.go` 比对快照与迁移执行后的表、列、外键和索引,不一致即失败;`CHECK` 约束不在比对范围内,靠人工对照。
- 已发布的迁移文件不再修改,只追加新迁移:用户机器上已经跑过旧内容,改了不会重跑。

## 项目配置

- `events.start_mode`、`distance_mode`、`relay_leg_count` 独立保存起跑方式、跳跃 / 投掷与建议棒数；追加迁移 `0002_event_config.sql`，缺省为空、空、0。读取这些缺省值时按领域兼容规则解释，保留已有道次和成绩，不依据项目名批量改数据。

## 连接与并发

- mygo 每次绑定调用在独立 goroutine 执行,MCP 请求也并发到达。**只有一个写连接**,所有写入走 `Store.Write`,避免 goroutine 之间出现 `SQLITE_BUSY`。
- **插件的事务只接收一批语句**(`Transaction([]Statement)`),不能在事务里先读再决定写什么。所以 service 的写方法在持有 `Service.mu` 期间完成「读 → 校验 / 计算 → 一批写入」;所有写都经 service,持锁期间读到的数据就不会被别处改掉。不要在 service 之外调 `Store.Write`,否则这个保证失效。
- 读走单独的只读连接(`Store.Query`),大列表查询不会把写入排在后面。
- 迁移文件按「行尾分号」切成语句逐条执行,迁移里不写触发器,也不写跨行且含行尾分号的字符串。

## 接力数据

- 使用 `teams` / `team_members` 与 `entries.team_id`、`slots.team_id`,棒次为从 1 开始的连续 `leg`;个人项目只写 `athlete_id`。现有数据库使用同一结构,无需新增迁移。
- `ListTeams` 对整个查询范围读取队伍与成员,固定两次查询;运动员使用计数合并个人报名与经接力队取得的报名和位置。名单删除保护由 service 完成,见 [domain.md](domain.md)「接力」。

## 查询

- **列表查询次数不随组数增长**:项目概览、组列表一次读齐组、位置、运动员、成绩再在内存组装,不逐组查询。运动会动辄几百组,逐组查询会让每次变更后的界面刷新明显卡顿。
- 时间戳存毫秒整数;日期存 `YYYY-MM-DD` 文本,计划时间存本地 `YYYY-MM-DDTHH:mm` 文本,与交换格式一致,不做时区换算。
