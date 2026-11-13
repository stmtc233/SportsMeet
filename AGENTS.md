# SportsMeet — Agent 指南

> 面向所有 AI 协作工具的项目事实源(Claude Code 读 `CLAUDE.md`,其内容仅 `@AGENTS.md`)。
>
> **文档分两层**:本文件 = 每次会话都加载的核心(定位、术语、结构、不变量一句话摘要);[docs/agents/](docs/agents/) = 按主题拆分的**细则**。**改动某主题的代码前,必须先读 §5 路由表指向的细则**。代码结构、术语或不变量变化时,在同一改动里同步更新本文件与对应细则(§6)。

---

## 1. 项目概览

SportsMeet 是一款开源的**单机离线**田径与校园运动会编排软件:一台电脑完成「赛事 → 报名 → 编排 → 秩序册 → 录入成绩 → 排名 → 成绩册」全流程。

- **框架**:[mygo](https://github.com/egoist/mygo)(网页界面模式),深度使用其绑定、事件、SQLite 插件、对话框、打包与自动更新,不为替换框架做抽象层(用户要求)。
- **技术栈**:Go(版本以 `go.mod` 为准,本机较旧时 Go 自动下载对应工具链;业务全部在 Go)+ Bun + Vite + TypeScript + React 前端(跑在系统 WebView 中)。mygo 版本见 `go.mod` 与 `package.json`(`mygo-cli` / `mygo-runtime` 同版本)。
- **Go 模块**:`github.com/stmtc233/sportsmeet`;应用标识 `io.github.stmtc233.sportsmeet`。
- mygo 自带的 agent 技能在 `.agents/skills/`(`mygo install-skills` 更新),改 mygo 相关代码前可参考。
- **平台**:macOS 12+、Windows 10/11、Linux。
- **定位边界**(用户要求):只做单机;**唯一的对外接口是 MCP**(仅本机)。出网只有 mygo 自动更新。导入导出使用开放的交换格式([docs/interchange-format.md](docs/interchange-format.md))。

## 2. 术语 —— 必须严格区分,不得自创同义词

| 术语 | 代码标识 | 含义 |
|---|---|---|
| **赛事** | `meet` | 一次运动会 |
| **参赛单位** | `unit` | 学校 / 班级 / 代表队,运动员的所属 |
| **运动员** | `athlete` | 属于赛事,各项目共用 |
| **项目** | `event` | 一个单项 = 性别(男 / 女 / 混合)+ 项目名(`discipline`)+ 可选组别(`groupLabel`) |
| **项目类型** | `EventKind` | `track` 径赛计时 / `relay` 接力 / `distance` 远度 / `height` 高度 / `count` 计次 / `score` 计分 |
| **队伍** | `team` | 属于一个队伍项目；接力队成员按棒次（`leg`）顺序排列 |
| **全能** | `combined` | 统一报名、汇总多个个人分项积分的比赛 |
| **分项** | `component` | 全能拥有的一个项目，有独立编排与原始成绩 |
| **积分** | `points` | 分项成绩按计分表换算的全能积分 |
| **报名** | `entry` | 运动员或队伍进入某项目 |
| **轮次** | `round` | 预赛 / 次赛 / 半决赛 / 决赛;单轮项目只有一个轮次 |
| **组** | `heat` | 轮次中的一组(径赛一组一次起跑,田赛一组一起比) |
| **位置** | `slot` | 组内的起跑道次(分道项目)或顺序(不分道项目、田赛) |
| **成绩** | `result` / `mark` | 位置上的成绩;`mark` 为整数,计时毫秒、田赛毫米、计次次数、计分按小数位缩放 |
| **编排** | arrange | 把报名者排进组和位置;不用来指整个系统 |

「组」不叫「组次」;「项目」不叫「比赛项」。

## 3. 目录结构

```
SportsMeet/
├── AGENTS.md / CLAUDE.md / README.md / LICENSE(MIT)
├── main.go                  ← mygo 入口:窗口、Bind 各服务、MCP 开关、更新窗口;数据打不开时弹窗说明后退出
├── go.mod
├── mygo.config.ts           ← 应用名、标识、版本(唯一来源,Go 经 `mygo.App.Version()` 读取)、打包、更新来源与公钥;bindings 输出 src/mygo.ts
├── package.json · vite.config.ts · tsconfig.json · index.html
├── resources/
│   ├── icon.png             ← 1024×1024「跑道 S」应用图标
│   └── brand/               ← 品牌矢量母版、SVG / PNG / WebP / JPG / PDF / ICO / ICNS、视觉规范与交付包
├── internal/
│   ├── domain/              ← 实体、枚举、项目配置与全能规则、时间规则与项目排序(纯数据 + 校验,无 IO)
│   ├── store/               ← SQLite 访问(mygo sqlite 插件,库文件 sportsmeet.db 在 mygo.PathUserData,迁移前的副本在同目录 backups/)
│   │   ├── migrations/      ← NNNN_名称.sql,嵌入二进制,启动时按序执行
│   │   └── schema.sql       ← 当前完整结构快照
│   ├── arrange/             ← 自动编排(分组、排道)、组序号、组指纹、晋级判定、运动员时间冲突检查;纯函数
│   ├── scoring/             ← 成绩解析与格式化、逐次汇总、排名、全能积分与总排名
│   ├── interchange/         ← 交换格式编解码与校验、与已有赛事的匹配;复用 scoring 校验逐次汇总
│   ├── report/              ← 秩序册(含竞赛日程表)/ 成绩册 / 检录单 / 成绩记录表(HTML 模板);号码布待实现
│   ├── service/             ← 唯一写入口:写锁、校验、操作日志(写入与只读查询)、变更通知;含项目预设、全能与计分规则、表格导入、编排保存、批量 / 多轮次连排时间、竞赛日程与冲突、成绩与实际开赛时间、检录单、成绩记录表、导入导出
│   ├── mcp/                 ← MCP 服务与监听生命周期(仅 127.0.0.1,默认关)
│   ├── layers/              ← 分层测试:各包允许的依赖、纯计算包不碰 mygo、bind / mcp 不越过 service
│   └── bind/                ← 给前端的 mygo 服务 + `dataChanged` 事件(薄层,只转调 service,另加文件对话框与打印窗口)
├── src/                     ← 前端(React)
│   ├── main.tsx · App.tsx   ← 入口;外壳(侧栏 + 页面切换)
│   ├── mygo.ts              ← `mygo generate` 生成物,勿手改
│   ├── app/                 ← nav.tsx(页面路由状态、离开前确认)、ErrorBoundary.tsx(页面渲染出错时的退路)、app.css(外壳与页面布局,只用 token)
│   ├── design/              ← tokens.css、components.css、`St*` 组件、icons.ts;经 index.ts 统一导出
│   ├── features/            ← meets/ units/ athletes/ events/ event/(项目工作区:报名、队伍、组时间显示)combined/(全能)arrange/ schedule/(竞赛日程)results/ reports/ exchange/ settings/ about/
│   └── lib/                 ← coalesce.ts(事件刷新合并)、useData.ts、action.ts、errors.ts、labels.ts、wallTime.ts(日期 / 时刻输入拆合)
├── .agents/skills/          ← mygo 自带的 agent 技能
├── tests/ui/                ← Playwright 页面冒烟测试(模拟 mygo 桥接)
├── playwright.config.ts      ← 宽窄窗口与浅深色页面验证
├── scripts/check-design-system.sh   ← 前端棘轮检查(设计系统 + 绑定引用);基线 design-system-baseline.txt
├── scripts/generate-brand.mjs       ← 从品牌母版导出资源,同步应用与官网图标
├── scripts/website-downloads.mjs    ← 生成要发布的官网:复制 website/,把下载区换成最新 release 的链接
├── website/                 ← 静态官网(index.html + styles.css,无构建;下载区在发布时替换);assets/ 里是应用界面截图(浅色 / 深色各一张)与图标
├── .github/workflows/check.yml      ← CI:`bun run check`、绑定生成物比对、页面冒烟测试
├── .github/workflows/release.yml    ← 发布 release 后打三个系统的安装包、便携包与签名的更新文件,上传到该 release
├── .github/workflows/pages.yml      ← 发布工作流成功后或官网有改动时,把官网发布到 GitHub Pages
├── docs/
│   ├── brand-guidelines.md             ←「跑道 S」品牌设计规范与资源索引
│   ├── interchange-format.md          ← 交换格式规范(事实源)
│   ├── schema/interchange-v1.schema.json · interchange-v2.schema.json · interchange-v3.schema.json · interchange-v4.schema.json · interchange-v5.schema.json
│   ├── examples/sample-meet.json
│   └── agents/                        ← 专题细则(§5)
└── testdata/interchange/              ← 编解码黄金文件
```

分层与数据流:

```
前端 src/ ──(mygo 绑定,src/mygo.ts)──▶ internal/bind ──▶ internal/service ──▶ store / arrange / scoring
MCP 客户端 ──(HTTP 127.0.0.1)──────────▶ internal/mcp  ──▶ internal/service
service 写入成功 ──▶ mygo 事件广播 ──▶ 前端合并后重新读取
```

## 4. 构建

```sh
bun install
bun run dev                          # mygo dev,打开应用窗口并热更新
bun run build                        # mygo build,产物在 build/<平台>/
bun run generate                     # 改了 Go 绑定后更新 src/mygo.ts
bun run brand:generate               # 从矢量母版导出品牌资源与应用 / 官网图标
bun run check                        # 提交前跑这一条:go vet、Go 单测、类型检查、前端单测、前端棘轮
bun run test:ui                      # 先 bunx playwright install chromium
```

`bun run check` 里的各项也可单独跑:`go test ./...`(含真实 SQLite)、`bun run test:web`、`bun run typecheck`、`scripts/check-design-system.sh`。CI 在此之外还比对 `src/mygo.ts` 与重新生成的结果,并跑页面冒烟测试。

本地只需验证能否打包时用 `bunx mygo build -skip-dmg -skip-notarize -skip-installer`。

发布:改 `mygo.config.ts` 的 `version` 并提交,在 GitHub 上发布标签为 `v<版本>` 的 release,`release.yml` 随即打包并把文件传到该 release;标签与版本不一致时工作流报错。每个系统一份安装包、一份便携包(解压即用,数据同样在用户数据目录):

| 系统 | 安装包 | 便携包 |
|---|---|---|
| macOS(通用) | `.dmg` | `-darwin-universal-portable.zip`(内含 `.app`) |
| Windows(amd64) | `sportsmeet-setup-….exe` | `-windows-amd64-portable.zip` |
| Linux(amd64) | `.deb`、`.rpm` | `-linux-amd64.tar.gz` |

同一次打包还上传各平台的更新文件:`update-<平台>.json`、`sportsmeet-<版本>-<平台>.tar.gz`、增量包、`install.sh` 与 `SHA256SUMS`。已安装的应用从最新 release 读 `update-<平台>.json`,所以 mygo 生成的文件不改名、不从 release 里删。

测试版的版本写成 `0.1.0-beta.1` 这样并勾选预发布:已安装的应用只更新到正式版,测试版要手动下载。

更新用一对密钥签名:公钥在 `mygo.config.ts` 的 `updates.publicKey`,私钥是仓库的 Actions 密钥 `MYGO_UPDATER_PRIVATE_KEY`。换密钥后已安装的应用收不到更新,私钥要另行备份。

`resources/brand/` 是设计资料,经 `resourcesExclude` 不进安装包。

官网:`release.yml` 成功后,`pages.yml` 把 `website/` 发布到 GitHub Pages,下载区(`index.html` 里 `downloads:start` 与 `downloads:end` 之间)换成最新正式版的链接,还没有正式版时用最新的测试版;仓库里的 `index.html` 保持「即将发布」。链接按文件名匹配,改 `release.yml` 的文件名时同步改 `scripts/website-downloads.mjs`。本地预览:`gh api repos/stmtc233/SportsMeet/releases > /tmp/r.json && node scripts/website-downloads.mjs /tmp/r.json /tmp/site`。

---

## 5. 关键不变量与「改动前必读」

### 5.0 路由表

| 改动涉及 | 先读 |
|---|---|
| `internal/domain/`、`internal/service/`、`internal/scoring/`、报名 / 组 / 成绩 / 排名 / 晋级 / 删除 / 操作日志 | [domain.md](docs/agents/domain.md) |
| `internal/arrange/`、`src/features/arrange/` | [arrange.md](docs/agents/arrange.md) + [domain.md](docs/agents/domain.md) |
| 全能、分项、计分表、`src/features/combined/`、`internal/service/combined*.go` | [combined.md](docs/agents/combined.md) + [domain.md](docs/agents/domain.md) |
| `internal/mcp/` | [mcp.md](docs/agents/mcp.md) |
| `internal/interchange/`、导入导出 | [interchange-format.md](docs/interchange-format.md) + [mcp.md](docs/agents/mcp.md)「导入」 |
| `internal/store/`、迁移、查询性能 | [storage.md](docs/agents/storage.md) |
| `src/` 代码结构、事件刷新、设置页、`internal/report/` 打印 | [frontend.md](docs/agents/frontend.md) |
| `src/design/`、新增或改写任何页面 UI、`scripts/check-design-system.sh` | [design-system.md](docs/agents/design-system.md) |
| 品牌标识、应用与官网图标、`resources/brand/` | [brand-guidelines.md](docs/brand-guidelines.md) + [design-system.md](docs/agents/design-system.md) |
| 新增或修改任何用户可见文案(界面、打印模板、错误提示)、写提交信息 | [copywriting.md](docs/agents/copywriting.md) |

### 5.1 全局

- **所有写操作只经 `internal/service`**:前端绑定与 MCP 共用同一组方法和校验,`bind/`、`mcp/` 不直接碰 `store/`;否则会绕过校验,界面也收不到变更事件。
- **包的依赖方向由 `internal/layers` 的测试把关**:新增包或新增依赖时,在它的分层表里登记并同步 §3。
- **业务规则只在 Go**:成绩解析与格式化、排名、晋级、项目名称组成、时间的校验与显示都在 Go,前端只调用(→ domain.md)。
- **新版本检测与安装用 mygo 的更新窗口**:启动后与每天在后台检查 GitHub release,「关于」页可手动检查;这是应用唯一的出网(→ frontend.md「设置页」)。
- **`src/mygo.ts` 是生成物**,不手改。
- **前端只有 `src/lib/` 与各 feature 的 `controller` / `data` 文件调用 Go**,其余文件只从 `src/mygo.ts` 引入类型;棘轮把关(→ frontend.md)。
- **页面只用 `src/design/` 的 `St*` 组件、token 与统一图标**;`scripts/check-design-system.sh` 棘轮把关,基线只减不增(→ design-system.md)。
- **数字一律等宽**,界面与打印模板都设 `tabular-nums`(→ design-system.md)。
- **圆角用 `superellipse(1.78)` 平滑轮廓**,胶囊、圆形与开关显式 `round`(→ design-system.md)。
- **品牌标识为「跑道 S」**,母版在 `resources/brand/source/`,运行 `bun run brand:generate` 统一导出各格式并同步应用与官网图标(→ [品牌规范](docs/brand-guidelines.md))。
- **界面文字默认不写**:删掉后用户仍能操作、看懂状态的字就不加(→ copywriting.md)。

### 5.2 领域与编排 → [domain.md](docs/agents/domain.md) · [arrange.md](docs/agents/arrange.md)

- 项目 = 一个单项,必带性别与项目名、必属于赛事;男女、不同距离不得混在一个项目。
- 运动员属于赛事,经报名进项目;排进位置即自动报名,取消报名不动已排位置;学号 / 号码布赛事内唯一。
- 成绩 `mark` 存整数(计时毫秒、田赛毫米、计次次数、计分按精度缩放),并列同名次、下一名顺延。
- 位置在分道与分道后抢道项目中表示起跑道次,在不分道项目和田赛中表示顺序;同一轮次里一人只占一个位置,组名只存自定义名称。
- 常用项目预设只提供初始配置;起跑方式、跳跃 / 投掷与建议棒数独立保存,已有数据不按项目名推断。
- 有成绩的组锁定,不能删除、不被自动编排覆盖;判定只有 `HeatFacts.KeepReason` 一处。
- 编排页草稿一次保存,每组带指纹、只提交改过的组;数据按指纹合并,位置规则变化时重建草稿,不弹冲突框。
- 自动编排只排未分组的人、只追加新组;随机带 `seed` 可复现。
- 批量排时间只改选中轮次(一个,或按请求顺序连排的多个,顺序可调、可留轮次间隔)未比赛的组,间隔同时写为预计用时,界面与 MCP 共用预览和确认提交,数据变化须重新预览。
- 同一运动员两组时间重叠或间隔不足最短休息时间(设置项)即为冲突,只提示不拦截任何保存(用户要求);队伍展开为队员,全能分项参与,检查是 `arrange` 的纯函数;编排页按草稿(含未保存改动)经 Go 只读入口计算(→ arrange.md「冲突检查」)。
- 日程人数:有组的轮次是排进组的人数,还没有组的首个轮次是报名人数,队伍按队员计;秩序册日程表只印已安排时间的轮次(→ arrange.md「竞赛日程」)。
- 轮次有开始时间,组只存单独指定的时间与预计用时;组的生效时间 = 单独时间,没有则沿用轮次,显示、排序、打印都用生效时间,结束时间 = 生效时间 + 预计用时(→ domain.md「时间」)。
- 轮次开始时间直接写入、不进组指纹与草稿;时间不在赛事日期内只提示不拦截,判断只在 service,界面与 MCP 共用。
- 组的实际开赛时间在第一次写入成绩时自动记录,可手动修改或清除;不进组指纹、不影响锁定,清空成绩不清它(→ domain.md「时间」)。
- 检录时间 = 生效计划时间 − 检录提前时间(设置项,默认 20 分钟,0 不显示),不存储,只印在检录单上。
- 成绩记录表每组一张空白表供裁判手写,栏目由项目类型与已保存的配置决定,不印已录入的成绩(→ frontend.md「打印」)。
- 每个项目有一个决赛,之前可加预赛、次赛、半决赛各至多一个,各带晋级规则;名次与 `Q` / `q` 在每次写成绩时由 Go 重算并存储,晋级只读成绩。
- 队伍按队报名、编排和录成绩;同一项目一人只属一队,已有比赛记录的队伍锁定名单(→ domain.md)。
- 全能拥有独立分项并统一报名,积分与总排名由原始成绩和保存的计分表计算,规则修改须预览确认(→ [combined.md](docs/agents/combined.md))。
- 时间分日期、计划时间(本地到分钟、无时区,用户要求)、记录时间(毫秒,按本机时区显示)三种,校验、运算与显示只在 `domain` 的时间规则(→ domain.md「时间」)。
- 项目列表按显示日子(最早计划时间的日子,没有时间才用项目日期)→ 最早计划时间 → 创建顺序排列,全能分项保持原顺序;导出保持创建顺序。
- 每个写操作写操作日志(来源 `ui` / `mcp` / `system`),日志失败不影响业务;设置页只读查看,分页读取。

### 5.3 MCP → [mcp.md](docs/agents/mcp.md)

- 默认关闭,只监听 `127.0.0.1`,独立令牌,拒绝非本机 `Origin`。
- MCP 可以写成绩(用户要求):空位直接写,覆盖已有成绩须预览确认;名次由 `scoring` 重算,不能直接指定。
- 批量改动与删除走「预览指纹 + `confirmed=true`」,提交时持 service 写锁重算比对，再事务写入;有成绩的数据 MCP 不能删。
- 日程与冲突只读查询(`get_schedule`、`list_conflicts`);多轮次连排扩展 `preview_schedule` / `schedule_heats` 的 `rounds`,原单轮次参数照常可用;`set_heat_time` 带组指纹逐组改时间,不动位置与成绩。
- 不内置云模型。

### 5.4 数据与交换格式 → [storage.md](docs/agents/storage.md) · [interchange-format.md](docs/interchange-format.md)

- 单一写连接 + 只读连接;service 持写锁完成「读 → 校验 → 一批写入」,不在 service 之外写库;列表查询次数不随组数增长。
- 迁移只追加,已发布的迁移文件不再修改;同步 `schema.sql`。
- 已有数据的数据库在迁移前先复制到 `backups/`,复制失败就不迁移。
- 外键所在的列都建索引;号码布 / 学号的唯一性只由 service 校验,不建唯一索引。
- 交换格式改字段即 bump `version`;读取方能读所有已发布版本,写出总是最新版本。

### 5.5 文案与提交 → [copywriting.md](docs/agents/copywriting.md)

- 文案只写用户要知道的结果与下一步;标题 / 按钮 12 字,副标题一行 20 字,对话框 2 句 60 字,提示条 15 字。
- 中文全角标点,数字、成绩、时间保留半角;引用界面名称加「」。
- 回复用户一律用中文(用户要求)。
- 用户没确认就不提交(用户要求);提交信息 `type(scope): 摘要`,正文 3–8 条,最后一条注明同步的文档。

### 5.6 未实现与后续计划

当前已实现:基础数据(赛事、参赛单位、运动员含表格导入、项目含 35 个预设与自定义配置、报名)、编排(预赛 / 次赛 / 半决赛 / 决赛、自动编排、手动调整、草稿保存、批量排时间与多轮次连排)、竞赛日程页与运动员时间冲突提示、秩序册竞赛日程表、统一的时间规则(项目按时间排序、组与成绩修改时间显示、报表打印时间)、计时 / 田赛 / 计次 / 计分成绩录入与排名晋级、普通队伍项目、全能分项与积分总排名、秩序册、成绩册、检录单与成绩记录表打印、组的实际开赛时间、交换格式导入导出、设置(通用 / 编排 / 打印 / MCP / 操作日志 / 关于)、新版本检测与更新、本机 MCP 查询与写入、宽窄窗口页面冒烟测试。验证范围见 [docs/verification.md](docs/verification.md)。以下按优先级排列,做完一项就从这里删掉,并在 §3 与对应细则里写成现状。

排序依据:先让学校能装上、能办完一场校运会,再补规则细节与打磨。

**第一优先:首次发布**

学校的电脑多为 Windows,没有安装包就没有用户;现场流程没有在原生窗口里完整走过,问题只会在比赛当天暴露。

1. Windows 的运行、文件对话框、打印与打包。
2. 原生 WebView 里走完一场完整赛事:手动拖放、田赛逐次录入的键盘操作、800 宽窗口、实际纸张分页;浏览器 1280 / 800、浅色 / 深色已由自动测试覆盖。
3. 系统签名(发布的安装包未做 macOS 签名公证与 Windows 签名);发布工作流与应用内更新在 GitHub 上实际跑通一次。

**第二优先:校运会必备**

4. **团体总分**:各名次的分值(含接力加倍)、按参赛单位汇总、成绩册印团体总分表;交换格式加字段并 bump 版本。
5. **报名限制**:每人限报项数、每个参赛单位每项限报人数,报名与表格导入预览里提示超出的人。
6. **号码布**:按参赛单位分段自动编号,打印号码布。
7. **并列名次的人工裁定**(含最后晋级名额并列时去掉多余晋级者的操作入口)。
8. 秩序册的**代表队名单**:按参赛单位列出运动员与号码。

**第三优先**

9. **风速录入**:径赛整组风速、远度逐次风速(数据可随交换格式保留,成绩记录表已留栏,界面未提供)。
10. **破纪录标记**:赛事纪录表与成绩册标注(交换格式不含,需要时加字段并 bump 版本)。
11. 沿用已有赛事的项目与赛制新建赛事,不带运动员与成绩。
12. 奖状打印;打印样式预设。
13. 编排页:组卡片标题拖动排序;窄窗口下名单收为侧面板。
14. Linux 的运行、文件对话框、打印与打包。

**已知与细则不完全一致**
- `scripts/design-system-baseline.txt` 里标 `bindings` 的文件直接调用 Go 绑定,没有收进 `controller`;改到这些文件时顺手收进去并从基线删掉(→ [frontend.md](docs/agents/frontend.md)「代码结构」)。

---

## 6. 维护规则(给 AI 自己看)

同一个改动里必须同步文档的情形:

1. 增删 / 重命名包、重要文件、顶层目录、构建配置 → §3 / §4。
1. 改术语或实体层级 → §2 与 domain.md。
2. 改交换格式 → interchange-format.md、JSON Schema、示例与 `testdata/interchange/` 黄金文件。
3. 新增 / 废弃 / 修改不变量 → 在对应细则写完整规则,并在 §5 加 / 改**一句话摘要**;新主题建新细则并登记到 §5.0 路由表。

写作约定(文档与代码注释都适用):

- 本文件只放「每次会话都必须知道」的内容,§5 每条一句话;完整约束、原因与后果放细则。
- **只写现状**:当作「现在的设计本来就是这样」来写,不写演变过程 —— 不出现「原先 / 以前 / 旧版 / 曾经 / 不再 / 改为」、日期、版本沿革;已删除的代码与设置不留痕迹。改动过程只写进提交信息。
- **约束 = 做法 + 后果**:只在错误做法看起来很自然时才写「不要 X」,并写明 X 会导致什么;写不出后果的约束删掉。
- 产品取舍标明「用户要求」,与技术约束区分:前者不自行推翻;后者原因不再成立就直接改设计,同一改动里改写或删掉对应约束。
- 用户机器上已有的数据(旧数据库、旧版本交换文件)的读取兼容是现行约束,照写。
- 同一条规则只在一个细则里写全,其他地方一句话 + 链接。
- 以代码为准、文档跟随,但不允许只改代码不改文档。发现文档与代码不一致时,先告诉用户哪里不一致,再决定改哪边。
