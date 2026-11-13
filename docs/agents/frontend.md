# 前端约定(`src/`)

界面组件与 token 见 [design-system.md](design-system.md);文案见 [copywriting.md](copywriting.md)。

## 代码结构

- React;按 `src/features/<area>/` 拆分,单文件 ≲ 400 行。页面切换是 `src/app/nav.tsx` 里的路由状态(不用路由库),有未保存改动的页面经 `useLeaveGuard` 登记,离开前统一确认。
- 外壳用 `src/app/ErrorBoundary.tsx` 包住全部页面:某个页面渲染时抛错,窗口显示提示与「返回全部赛事」,而不是整窗空白。它只兜底渲染错误;读写失败照常经数据钩子的 `error` 与 `act()` 提示。
- 版本号只写在 `mygo.config.ts`:Go 启动时经 `mygo.App.Version()` 读入 `service.Version`,「关于」与导出文件都用它;未打包运行(单测)时为 `dev`。
- **只经生成的 `src/mygo.ts` 调用 Go**;页面组件不直接调用,读取经各 feature 的 `use*` 数据钩子(底层 `lib/useData.ts`),写入一律经 `*Controller`,由 `lib/action.ts` 的 `act()` 统一弹出结果提示(`StToast`)。
- **调用 Go 的文件只有 `src/lib/` 与各 feature 的 `controller.ts` / `*Controller.ts` / `data.ts`**,其余文件只从 `src/mygo.ts` 引入类型(`import type` 或 `type X`)。调用散在组件里,同一个写入就会出现几种提示与刷新方式。`scripts/check-design-system.sh` 的 `bindings` 规则把关:基线里列出尚未收进 `controller` 的文件,只减不增;改到其中的文件时把调用收进 `controller`,再运行 `scripts/check-design-system.sh --update-baseline`。
- `act()` 将 mygo 无返回值调用的 `null` 与 `undefined` 都视为成功,供调用方区分成功和异常;否则桌面操作已完成仍会被当作失败。
- Go 返回的错误码在 `lib/errors.ts` 一处映射为中文提示,页面不各写一份;校验错误由 service 直接给出成文的中文。
- 纯逻辑(按钮可用性、草稿合并、排序显示)写成可单测的纯函数,不埋在组件里。
- **时间**:显示用的时间文本(计划时间、日期范围、记录时间)都由 Go 算好随数据返回,规则见 [domain.md](domain.md)「时间」。表单里把计划时间拆成日期与时刻、合并回存储格式、取今天作默认值,只在 `lib/wallTime.ts`(有单测);组件不自己拆时间字符串。

## 变更事件与刷新

- service 写入成功后经 mygo 事件广播(事件声明在 `internal/bind/`),前端按事件重新读取相关数据。界面自己的写入与 MCP 的写入走同一条路径,持久化数据由 service 的查询结果驱动。编排保存用返回的完整详情重新建立草稿,确认新增组的真实 ID 与已保存状态(见 [arrange.md](arrange.md)「本地草稿,一次保存」)。
- **事件引起的重新读取按数据合并**:第一次立即执行,500ms 窗口内的后续事件合并为窗口结束时的一次。批量导入会连续发出大量事件,不合并会造成连串重读。

## 项目创建与配置

- `features/events/EventDialog.tsx` 的「常用项目」读取 Go 的 35 个预设，仅创建时提供；选择后带出项目名、类型、起跑方式、跳跃 / 投掷或建议棒数，可继续编辑。性别通常保留当前选择，混合接力预设选择「混合」。编辑已保存项目读取实际配置，不套用预设。
- 项目名称预览、配置显示规则、接力名单人数提示均经 `events/controller.ts` 调用 Go；自动编排与草稿保存经 `arrange/controller.ts`。不分道时道数输入置灰，已有值保留。
- `EventMeasurementFields.tsx` 共用项目配置控件，`form.ts` 保存表单初始值；计次、计分提供精度、排名方向、单次／取最好／求和，普通队伍提供建议人数。
- 「赛制」下拉与各轮次的晋级人数由 `events/format.ts` 的纯函数组织(可选组合、切换组合时保留仍在的轮次的人数),表单里的 `rounds` 原样交给 Go 校验;项目列表的「赛制」列显示 Go 的 `formatText`。
- `features/combined/` 提供列表、创建／编辑、分项工作区入口、统一报名、总成绩、计分表预览与规则重算，完整规则见 [combined.md](combined.md)。
- 成绩页位置名称和逐次表头来自 `EventDetail.rules`；投掷显示「试投」，支持 `X`、`-` 与「免投」。

## 队伍与接力

- 项目可按个人或队伍参赛，接力固定按队伍参赛。报名页与编排名单底部的「管理队伍」打开 `features/event/AddTeamsDialog.tsx`,显示队伍、单位、成员／棒次、报名状态以及编辑 / 删除入口;队伍经勾选后批量报名。
- `TeamDialog.tsx` 编辑队名、参赛单位与有序成员,提供增加、替换、上移 / 下移、移除棒次;已在同项目其他队伍的成员不列入候选。已有比赛记录时禁用名单编辑,保存失败保留输入,人数与性别警告由 Go 返回。所有读写经数据钩子和 `EventsController`,成功通过 `teams` / `entries` 事件刷新。
- 编排、成绩和报名表统一以 Go 返回的 `competitorId` 索引个人或队伍;接力使用道次和计时输入,报名数量使用「队」。业务约束见 [domain.md](domain.md)「队伍与接力」。

## 成绩页

- 每组的输入是本地状态,按组保存。项目详情重新读取时(本组或别的组保存、MCP 写入、运动员资料变化),`features/results/rows.ts` 的 `mergeRows` 逐位置合并:用户改过的行保留输入,其余行跟随新数据;整组直接换成新数据会清掉另一组里没保存的输入。
- 位置上换了人,该位置的输入丢弃,避免把成绩记到另一个人名下。刚保存的行跟随新数据,显示 Go 整理后的成绩文本,保存后该组不再算有改动。
- 成绩输入根据 Go 的 `rules.multipleAttempts` 和 `rules.markLabel` 显示单值或逐次记录；`StMarkField` 经 `lib/marks.ts` 携带完整项目配置调用 Go，计分小数位不在前端推断。
- 「放弃」恢复为已读取的数据,并去掉未保存的新增高度。读写经 `results/controller.ts`。
- 组标题经 `features/event/HeatTime.tsx` 显示生效时间 `scheduledText`:单独指定的用 `StTag`,沿用轮次(`scheduleInherited`)的用次要文字;其后经 `features/event/HeatStarted.tsx` 显示「开赛 `startedText`」与差值标签 `startDeltaText`,窄窗口下换到第二行;组内有成绩时,操作区左侧显示「修改于」与 `resultsUpdatedText`。
- 操作区的「开赛时间」图标按钮打开 `results/StartedDialog.tsx`(日期 + 时间):已有开赛时间时填入它并提供「清除」;没有时日期取生效计划日期,再取 `EventDetail.day`,时间留空(`results/started.ts` 的纯函数,有单测)。保存与清除经 `ResultsController.setStarted`(`Results.setStarted`)立即写入,成功后由 `heats` 事件重读。规则见 [domain.md](domain.md)「时间」。

## 编排页时间

- 组卡片与成绩页共用 `HeatTime` 显示草稿里的 `scheduledText` / `scheduleInherited`,另显示 `durationText`。草稿轮次带 `startAt` / `startText`,新组与没有单独时间的组按它显示沿用的时间,不在前端拼时间文本。
- 「编辑组…」以已有日期、轮次开始日期或 `EventDetail.day` 作默认日期,确定时经 `ArrangeController.heatPlan` 由 Go 校验时间与预计用时(输入原文传给 Go)并取回显示文本,一并写进草稿;校验失败留在对话框。
- 轮次开始时间经 `RoundStartDialog` 与 `ArrangeController.setRoundStart` 立即写入;两个时间对话框经 `useScheduleWarning`(调用 `Arrange.scheduleWarning`)在保存前显示 Go 给出的提示。
- `features/arrange/ScheduleDialog.tsx` 经 `scheduleController.ts` 调用生成的 `Arrange.previewSchedule` / `Arrange.schedule` 绑定,与 MCP 共用 Go 预览和提交。它接收轮次选项 `ScheduleOption[]`:编排页传当前轮次(单轮次请求),日程页以 `multi` 传全部轮次(多轮次请求,可勾选、调整先后、填轮次间隔;顺序的增删与移动是 `scheduleOrder.ts` 的纯函数,有单测);`version` 传读取选项的数据对象,它变化即清除预览。入口与草稿限制见 [arrange.md](arrange.md)「批量排时间」。
- 编排页组卡片的冲突标签经 `controller.ts` 的 `useDraftConflicts` 读取:草稿经 `draft.ts` 的 `draftTimes`(纯函数,有单测)转成请求,草稿变化与赛事数据 / 设置变化停顿 300ms 后合并为一次 `Arrange.draftConflicts` 调用,标签按组键对应。冲突列表(日程页与连排预览)共用 `arrange/ConflictList.tsx`。
- 请求期间禁用日期、时间、间隔与取消操作;过期预览或提交失败后保留输入,清除确认状态并要求重新预览。保存成功经 `heats` 事件重读,页面不自行改时间。

## 日程页

- `src/features/schedule/`:`SchedulePage.tsx` 页面;`controller.ts` 的 `useTimetable`(`Arrange.meetSchedule`,赛事任意变化与 `settings` 变化时重读)与 `ScheduleController.editHeat`;`options.ts` 把日程转成连排对话框的轮次选项(纯函数,有单测)。规则与布局见 [arrange.md](arrange.md)「竞赛日程」。
- 时间对话框复用编排页的 `RoundStartDialog`、`HeatDialog`(只取所需字段的类型)与 `ScheduleDialog`;组改动直接写入,不进草稿。
- 组表格的「时间」列在计划时间后接 `HeatStarted`(实际开赛时间与差值);列宽不够时截断,完整文本在悬停提示里。

## 设置页

- `src/features/settings/` 按主题分类(通用 / 编排 / 打印 / MCP / 操作日志 / 关于),每类一页;宽窗口左侧常驻分类列表,窄窗口分两级。
- 宽窄切换只增减左侧分类列表,内容区留在同一树位置,表单、焦点与未保存修改都保留。
- 开关与分段控件立即生效;需要保存的表单经底部保存栏,有修改时离开先确认。

- 通用:外观(跟随系统 / 浅色 / 深色),经根元素 `data-theme` 应用。编排:同单位分散、均衡人数,作为自动编排对话框每次打开时的默认值;「时间冲突」的最短休息时间(`StSelect`:只查重叠 / 5–60 分钟,选择即生效),冲突检查经 service 读取;「检录」的检录提前时间(`StSelect`:不显示 / 10–240 分钟,默认 20)。分钟选项由 `settings/minutes.ts` 生成,已存的值不在列表里时追加显示(有单测)。打印:秩序册是否印竞赛日程表、成绩册是否印「成绩时间」、检录单与成绩记录表各自的「分页」(`StSelect`:不分页 / 每个轮次一页 / 每组一页,默认每个轮次一页)、每个项目另起一页,Go 生成预览与打印 HTML 时读取同一设置。
- 操作日志:只读表格,按赛事(`StSelect`)与来源(`StSegmented`:全部 / 界面 / MCP / 系统)筛选,每页 50 条,「加载更多」接续上一页,没有更多时置灰。任一数据变化重读最新一页并与已加载的页按 id 合并(`settings/auditLog.ts`,有单测);新记录多于一页、接不上已加载部分时从最新一页重新开始,不显示断档。读取经 `settings/controller.ts` 的 `useAuditLog` 与生成的 `Settings.auditLog`。
- MCP:本机开关、运行状态、端口、令牌显示 / 复制 / 轮换与复制连接信息。端口单独编辑保存,失败保留输入;有端口草稿时禁用开关与轮换,避免隐式提交草稿。关于包含版本、「检查更新」与开源许可。
- 新版本检测用 mygo 的更新插件(`main.go` 里 `mygo.Use`),窗口固定简体中文:启动 10 秒后与每天各在后台检查一次,有新版本时弹出更新窗口,可安装、稍后提醒或跳过;「检查更新」经 `bind.App.CheckForUpdates` 立即打开同一窗口。开发运行与 deb / rpm 安装的应用不能自更新,不做后台检查,手动检查时窗口说明原因。
- 赛事列表工具栏与赛事侧栏均提供设置入口,返回恢复进入设置前的页面。分类导航经同一 `useNav` 离开确认,宽窄变化只通过 CSS 控制分类列表的可见性。
- 读取经 `settings/controller.ts` 的钩子,写入经 `SettingsController` 与生成的 `Settings` 绑定。普通偏好与 MCP 配置分开读取,普通偏好接口不含令牌。设置写入广播 `kind=settings`,外观与打印预览按事件刷新。

## 打印

- 打印选择项由 `Reports.targets` 提供普通项目和全能，选择全能展开全部分项与汇总表；读取、生成与打印经 `reports/controller.ts`。
- 秩序册按起跑方式显示「道次」或「顺序」，两类报表显示起跑方式或尝试配置；成绩册包含远度逐次试跳 / 试投记录，接力显示建议棒数与成员棒次。
- 项目按项目列表的顺序排列;组的时间与项目同日只印时刻;两类报表末尾印「打印时间」(见 [domain.md](domain.md)「时间」)。秩序册开头的「竞赛日程表」见 [domain.md](domain.md)「时间」;每个项目另起一页时,日程表之后的第一个项目也另起一页。
- 秩序册、成绩册、检录单、成绩记录表由 Go `internal/report/` 生成完整 HTML 文档(号码布待实现)。「打印」页在 `iframe` 里预览(见下条);点「打印」由 Go 另开一个窗口载入该 HTML 并调出系统打印对话框(可另存为 PDF)。不在主窗口里打印:主窗口的页面是应用界面,打出来会带上侧栏。
- **预览按纸张分页**:`reports/ReportPreview.tsx` 把 Go 的 HTML 载入 `sandbox="allow-same-origin"` 的 `iframe`(不放行脚本,只让页面能读写它的文档),载入后由 `reports/paginate.ts` 把内容搬到 A4 纸面上,预览上方显示页数与「比例」(`StSelect`:适合宽度 / 50%–150%,默认适合宽度,随窗口宽度重算)。Go 的报表是一份连续的文档,只有 WebView 打印时才分页,直接显示看不出每页印什么。
  - 分页规则:放不下的容器(`section` / `div` / `table` / `tbody`)在下一页接着排,表格重复表头;`.heat` / `.sheet` / `.line` 能放进一页就不拆;标题与表头不单独留在页尾,也不算作页面内容;强制换页读元素的 `break-before` 计算值,所以 Go 把「另起一页」写成不限打印媒体的规则。纸面尺寸与边距(`.pv-*`)写在 `internal/report/style.css` 里 `@page` 旁边,改纸张时两处一起改。
  - 预览的分页接近打印结果,不保证逐行一致:实际分页由 WebView 的打印排版决定。分页在主线程一次算完,上百页的报表会有可感知的停顿。
  - 没有可预览的内容时不显示空白的预览,用 `StEmpty` 说明原因(还没有项目 / 还没有组 / 当天没有组 / 未选择项目 / 未选择轮次),此时「打印」置灰;读取失败显示错误。新报表载入期间保留上一份预览。
  - 每份报表用一个新的 `iframe`(以 HTML 为 `key`),不改已有 `iframe` 的 `srcdoc`;纸面四周的底色由页面按预览区的背景色写进 `iframe` 文档(主题变化时重写),不靠透明:报表文档是浅色方案,深色主题下透明的 `iframe` 会被 WebView 涂成白底。不改 `srcdoc` 是因为WKWebView 在同一个 `iframe` 的 `srcdoc` 短时间内连改两次时会丢掉第二次,预览停在空白。
- 检录单(`Reports.checkInTargets` / `renderCheckIn` / `printCheckIn`,service `RenderCheckIn`):「打印」页分段控件的第三项。侧栏上方「日期」(`StSelect`:全部日期 + 有计划组的日子)筛选,下方按轮次勾选(项目只有一个轮次时只写项目名);选了日期只列当天有组的轮次。全部勾选时请求不带轮次(等于全部),否则带勾选的轮次;选择逻辑是 `reports/checkin.ts` 的纯函数(有单测)。
  - 内容:每组一张,印赛事、项目、轮次、组、计划时间、检录时间(提前时间为 0 时整格不印)、道次 / 顺序、号码、姓名、单位与「到」「缺」方框,末尾「裁判签名」;队伍项目印队名与成员 / 棒次(含号码)。没有计划时间的组照印,时间留空。
  - 顺序:不选日期时按项目列表顺序、轮次、组序号;选了日期只印当天的组,按生效计划时间排。分页按「设置 → 打印 → 检录单 → 分页」(service `sheetBreaks`):每个轮次一页(默认)时同一轮次的各组接着排在同一张纸上,排满才换页,轮次变了另起一页,按日期打印时相邻两组轮次不同也另起;每组一页时每张另起;不分页时只在排满时换页。一组能放进一页就不拆开。已保存的设置里旧的「每组另起一页」开关打开时读作每组一页。
- 成绩记录表(`Reports.renderRecordSheet` / `printRecordSheet`,service `RenderRecordSheet`):「打印」页分段控件的第四项,场地裁判手写成绩用。选择方式、请求(`CheckInRequest`)与顺序同检录单,按组选取与名单是 service 里两者共用的一段;分页同检录单,用自己的设置项。
  - 每张印赛事、项目、轮次、组、计划时间、待填的「实际开赛」、道次 / 顺序、号码、姓名(队伍印队名与成员 / 棒次)、单位,之后是待填的栏目、名次与「裁判签名」。只留空格:已录入的成绩不印,免得手写稿与系统里的数对不上时分不清哪个是现场记录。
  - 栏目只看项目类型与已保存的配置(`recordColumns`),不按项目名推断:径赛与接力填「成绩」,分道起跑的个人径赛另留整组「风速」;远度按尝试次数每次一列加「最好成绩」,跳跃每人多一行逐次「风速」,投掷没有;高度留 10 列,表头空着由裁判写高度(项目不保存起跳高度),加「最好成绩」;计次与计分填「次数」/「分数」,多次尝试时前面按尝试次数每次一列。
- 打印 HTML 同样数字等宽(`font-variant-numeric: tabular-nums`)、同样遵守文案规范。
- 打印样式预设待实现。

## 测试

- 纯函数单测(`bun test`,文件名 `*.test.ts`,不进 `tsc` 检查范围)与 Go 单测为主。
- `bun run test:ui` 使用 Playwright,覆盖宽屏 1280、窄屏 800 与浅色 / 深色,检查核心页面不出现整页横向溢出、设置输入与焦点保留、离开确认、保存与错误反馈。浏览器测试模拟 mygo 桥接,业务持久化与 HTTP MCP 使用 Go 的真实 SQLite 测试;两者不能替代 WebView 原生行为实测(见 [verification.md](../verification.md))。
