# SportsMeet — Agent 指南

> 面向所有 AI 协作工具的项目事实源(Claude Code 读 `CLAUDE.md`,其内容仅 `@AGENTS.md`)。
>
> **文档分两层**:本文件 = 每次会话都加载的核心(定位、术语、结构、不变量一句话摘要);[docs/agents/](docs/agents/) = 按主题拆分的**细则**。**改动某主题的代码前,必须先读 §5 路由表指向的细则**。代码结构、术语或不变量变化时,在同一改动里同步更新本文件与对应细则(§6)。

---

## 1. 项目概览

SportsMeet 是一款开源的**单机离线**田径运动会编排软件:一台电脑完成「赛事 → 报名 → 编排 → 秩序册 → 录入成绩 → 排名 → 成绩册」全流程。

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
| **项目类型** | `EventKind` | `track` 径赛计时 / `relay` 接力 / `distance` 远度 / `height` 高度 |
| **报名** | `entry` | 运动员(接力为队伍)进入某项目 |
| **轮次** | `round` | 预赛 / 次赛 / 半决赛 / 决赛;单轮项目只有一个轮次 |
| **组** | `heat` | 轮次中的一组(径赛一组一次起跑,田赛一组一起比) |
| **位置** | `slot` | 组内的道次(径赛)或试跳顺序(田赛) |
| **成绩** | `result` / `mark` | 位置上的成绩;`mark` 为整数,径赛毫秒、田赛毫米 |
| **编排** | arrange | 把报名者排进组和位置;不用来指整个系统 |

「组」不叫「组次」;「项目」不叫「比赛项」。

## 3. 目录结构

```
SportsMeet/
├── AGENTS.md / CLAUDE.md / README.md
├── main.go                  ← mygo 入口:窗口、菜单、Bind 各服务、MCP 开关
├── go.mod
├── mygo.config.ts           ← 应用名、标识、版本、打包;bindings 输出 src/mygo.ts
├── package.json · vite.config.ts · tsconfig.json · index.html
├── resources/icon.png       ← 1024×1024 应用图标
├── internal/
│   ├── domain/              ← 实体与枚举(纯数据 + 校验,无 IO)
│   ├── store/               ← SQLite 访问(mygo sqlite 插件,库文件 sportsmeet.db 在 mygo.PathUserData)
│   │   ├── migrations/      ← NNNN_名称.sql,嵌入二进制,启动时按序执行
│   │   └── schema.sql       ← 当前完整结构快照
│   ├── arrange/             ← 自动编排(分组、排道)、组序号、组指纹、晋级判定;纯函数
│   ├── scoring/             ← 成绩解析与格式化、排名、并列判定、田赛逐次记录
│   ├── interchange/         ← 交换格式编解码与校验、与已有赛事的匹配
│   ├── report/              ← 秩序册 / 成绩册(HTML 模板);检录单、号码布待实现
│   ├── service/             ← 唯一写入口:写锁、校验、操作日志、变更通知;含表格导入、编排保存、批量排时、成绩、导入导出
│   ├── mcp/                 ← MCP 服务与监听生命周期(仅 127.0.0.1,默认关)
│   └── bind/                ← 给前端的 mygo 服务 + `dataChanged` 事件(薄层,只转调 service,另加文件对话框与打印窗口)
├── src/                     ← 前端(React)
│   ├── main.tsx · App.tsx   ← 入口;外壳(侧栏 + 页面切换)
│   ├── mygo.ts              ← `mygo generate` 生成物,勿手改
│   ├── app/                 ← nav.tsx(页面路由状态、离开前确认)、app.css(外壳与页面布局,只用 token)
│   ├── design/              ← tokens.css、components.css、`St*` 组件、icons.ts;经 index.ts 统一导出
│   ├── features/            ← meets/ units/ athletes/ events/ event/(项目工作区:报名)arrange/ results/ reports/ exchange/ settings/ about/
│   └── lib/                 ← coalesce.ts(事件刷新合并)、useData.ts、action.ts、errors.ts、labels.ts
├── .agents/skills/          ← mygo 自带的 agent 技能
├── tests/ui/                ← Playwright 页面冒烟测试(模拟 mygo 桥接)
├── playwright.config.ts      ← 宽窄窗口与浅深色页面验证
├── scripts/check-design-system.sh   ← 设计系统棘轮检查;基线 design-system-baseline.txt
├── docs/
│   ├── interchange-format.md          ← 交换格式规范(事实源)
│   ├── schema/interchange-v1.schema.json
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
go test ./...                        # Go 单测(含真实 SQLite)
bun run test:web                     # 前端纯函数单测(bun test)
bun run typecheck
bun run test:ui                      # 先 bunx playwright install chromium
scripts/check-design-system.sh
```

本地只需验证能否打包时用 `bunx mygo build -skip-dmg -skip-notarize -skip-installer`。

## 5. 关键不变量与「改动前必读」

### 5.0 路由表

| 改动涉及 | 先读 |
|---|---|
| `internal/domain/`、`internal/service/`、`internal/scoring/`、报名 / 组 / 成绩 / 排名 / 晋级 / 删除 / 操作日志 | [domain.md](docs/agents/domain.md) |
| `internal/arrange/`、`src/features/arrange/` | [arrange.md](docs/agents/arrange.md) + [domain.md](docs/agents/domain.md) |
| `internal/mcp/` | [mcp.md](docs/agents/mcp.md) |
| `internal/interchange/`、导入导出 | [interchange-format.md](docs/interchange-format.md) + [mcp.md](docs/agents/mcp.md)「导入」 |
| `internal/store/`、迁移、查询性能 | [storage.md](docs/agents/storage.md) |
| `src/` 代码结构、事件刷新、设置页、`internal/report/` 打印 | [frontend.md](docs/agents/frontend.md) |
| `src/design/`、新增或改写任何页面 UI、`scripts/check-design-system.sh` | [design-system.md](docs/agents/design-system.md) |
| 新增或修改任何用户可见文案(界面、打印模板、错误提示)、写提交信息 | [copywriting.md](docs/agents/copywriting.md) |

### 5.1 全局

- **所有写操作只经 `internal/service`**:前端绑定与 MCP 共用同一组方法和校验,`bind/`、`mcp/` 不直接碰 `store/`;否则会绕过校验,界面也收不到变更事件。
- **业务规则只在 Go**:成绩解析与格式化、排名、晋级、项目名称组成都在 Go,前端只调用(→ domain.md)。
- **`src/mygo.ts` 是生成物**,不手改。
- **页面只用 `src/design/` 的 `St*` 组件、token 与统一图标**;`scripts/check-design-system.sh` 棘轮把关,基线只减不增(→ design-system.md)。
- **数字一律等宽**,界面与打印模板都设 `tabular-nums`(→ design-system.md)。
- **圆角用 `superellipse(1.78)` 平滑轮廓**,胶囊、圆形与开关显式 `round`(→ design-system.md)。
- **界面文字默认不写**:删掉后用户仍能操作、看懂状态的字就不加(→ copywriting.md)。

### 5.2 领域与编排 → [domain.md](docs/agents/domain.md) · [arrange.md](docs/agents/arrange.md)

- 项目 = 一个单项,必带性别与项目名、必属于赛事;男女、不同距离不得混在一个项目。
- 运动员属于赛事,经报名进项目;排进位置即自动报名,取消报名不动已排位置;学号 / 号码布赛事内唯一。
- 成绩 `mark` 存整数(径赛毫秒、田赛毫米),并列同名次、下一名顺延。
- 位置是实际物理道次;同一轮次里一人只占一个位置;组名只存自定义名称。
- 有成绩的组锁定,不能删除、不被自动编排覆盖;判定只有 `HeatFacts.KeepReason` 一处。
- 编排页本地草稿一次保存,每组带指纹、只提交改过的组;数据变化时按指纹合并,不弹冲突框。
- 自动编排只排未分组的人、只追加新组;随机带 `seed` 可复现。
- 批量排时间只改当前轮次未比赛的组,界面与 MCP 共用预览和确认提交,数据变化须重新预览。
- 新建项目自带决赛轮次,可加一个预赛;名次与 `Q` / `q` 在每次写成绩时由 Go 重算并存储,晋级只读成绩。
- 接力项目暂不支持:不能新建,导入时跳过并警告。
- 每个写操作写操作日志(来源 `ui` / `mcp` / `system`),日志失败不影响业务。

### 5.3 MCP → [mcp.md](docs/agents/mcp.md)

- 默认关闭,只监听 `127.0.0.1`,独立令牌,拒绝非本机 `Origin`。
- MCP 可以写成绩(用户要求):空位直接写,覆盖已有成绩须预览确认;名次由 `scoring` 重算,不能直接指定。
- 批量改动与删除走「预览指纹 + `confirmed=true`」,提交时持 service 写锁重算比对，再事务写入;有成绩的数据 MCP 不能删。
- 不内置云模型。

### 5.4 数据与交换格式 → [storage.md](docs/agents/storage.md) · [interchange-format.md](docs/interchange-format.md)

- 单一写连接 + 只读连接;service 持写锁完成「读 → 校验 → 一批写入」,不在 service 之外写库;列表查询次数不随组数增长。
- 迁移只追加,已发布的迁移文件不再修改;同步 `schema.sql`。
- 交换格式改字段即 bump `version`;读取方能读所有已发布版本,写出总是最新版本。

### 5.5 文案与提交 → [copywriting.md](docs/agents/copywriting.md)

- 文案只写用户要知道的结果与下一步;标题 / 按钮 12 字,副标题一行 20 字,对话框 2 句 60 字,提示条 15 字。
- 中文全角标点,数字、成绩、时间保留半角;引用界面名称加「」。
- 回复用户一律用中文(用户要求)。
- 用户没确认就不提交(用户要求);提交信息 `type(scope): 摘要`,正文 3–8 条,最后一条注明同步的文档。

### 5.6 未实现与后续计划

当前已实现:基础数据(赛事、参赛单位、运动员含表格导入、项目、报名)、编排(预赛 + 决赛、自动编排、手动调整、草稿保存、批量排时间)、径赛 / 远度 / 高度成绩录入与排名晋级、秩序册与成绩册打印、交换格式导入导出、设置(通用 / 编排 / 打印 / MCP / 关于)、本机 MCP 查询与写入、宽窄窗口页面冒烟测试。验证范围见 [docs/verification.md](docs/verification.md)。以下按优先级排列,做完一项就从这里删掉,并在 §3 与对应细则里写成现状。

**待验证**
- WebView 中的手动拖放、完整现场操作流程与 800 宽原生窗口;浏览器 1280 / 800、浅色 / 深色已由自动测试覆盖。
- Windows / Linux 的运行、文件对话框、打印与打包。

**第一优先**
1. **接力**(`relay`):队伍与棒次管理、报名、编排、成绩;交换格式已支持,目前导入时跳过。
2. **检录单与号码布**打印;打印样式预设。
3. **次赛 / 半决赛**的界面入口(数据层已支持)。
4. **风速录入**:径赛整组风速、远度逐次风速(数据可随交换格式保留,界面未提供)。
5. **并列名次的人工裁定**(含最后晋级名额并列时去掉多余晋级者的操作入口)。

**第二优先**
6. 编排页:组卡片标题拖动排序;窄窗口下名单收为侧面板。
7. 团体总分与积分规则、破纪录标记(交换格式 v1 不含,需要时加字段并 bump 版本)。
8. 发布前:应用图标、签名与自动更新配置、Windows / Linux 打包。

**已知与细则不完全一致**
- `AddEntriesDialog`、`HeatResults` 等少数组件里直接调用读取钩子与 `act()`,没有全部收进独立的 `*Controller`(→ [frontend.md](docs/agents/frontend.md)「代码结构」)。

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
