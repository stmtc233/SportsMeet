# 前端约定(`src/`)

界面组件与 token 见 [design-system.md](design-system.md);文案见 [copywriting.md](copywriting.md)。

## 代码结构

- React;按 `src/features/<area>/` 拆分,单文件 ≲ 400 行。页面切换是 `src/app/nav.tsx` 里的路由状态(不用路由库),有未保存改动的页面经 `useLeaveGuard` 登记,离开前统一确认。
- **只经生成的 `src/mygo.ts` 调用 Go**;页面组件不直接调用,读取经各 feature 的 `use*` 数据钩子(底层 `lib/useData.ts`),写入一律经 `*Controller`,由 `lib/action.ts` 的 `act()` 统一弹出结果提示(`StToast`)。
- **调用 Go 的文件只有 `src/lib/` 与各 feature 的 `controller.ts` / `*Controller.ts` / `data.ts`**,其余文件只从 `src/mygo.ts` 引入类型(`import type` 或 `type X`)。调用散在组件里,同一个写入就会出现几种提示与刷新方式。`scripts/check-design-system.sh` 的 `bindings` 规则把关:基线里列出尚未收进 `controller` 的文件,只减不增;改到其中的文件时把调用收进 `controller`,再运行 `scripts/check-design-system.sh --update-baseline`。
- `act()` 将 mygo 无返回值调用的 `null` 与 `undefined` 都视为成功,供调用方区分成功和异常;否则桌面操作已完成仍会被当作失败。
- Go 返回的错误码在 `lib/errors.ts` 一处映射为中文提示,页面不各写一份;校验错误由 service 直接给出成文的中文。
- 纯逻辑(按钮可用性、草稿合并、排序显示)写成可单测的纯函数,不埋在组件里。

## 变更事件与刷新

- service 写入成功后经 mygo 事件广播(事件声明在 `internal/bind/`),前端按事件重新读取相关数据。界面自己的写入与 MCP 的写入走同一条路径,持久化数据由 service 的查询结果驱动。编排保存用返回的完整详情重新建立草稿,确认新增组的真实 ID 与已保存状态(见 [arrange.md](arrange.md)「本地草稿,一次保存」)。
- **事件引起的重新读取按数据合并**:第一次立即执行,500ms 窗口内的后续事件合并为窗口结束时的一次。批量导入会连续发出大量事件,不合并会造成连串重读。

## 接力队伍

- 项目类型包含「接力」。报名页与编排名单底部的「管理队伍」打开 `features/event/AddTeamsDialog.tsx`,显示队伍、单位、棒次、报名状态以及编辑 / 删除入口;队伍经勾选后批量报名。
- `TeamDialog.tsx` 编辑队名、参赛单位与有序成员,提供增加、替换、上移 / 下移、移除棒次;已在同项目其他队伍的成员不列入候选。已有比赛记录时禁用棒次编辑,保存失败保留输入,性别不符显示警告。所有读写经数据钩子和 `EventsController`,成功通过 `teams` / `entries` 事件刷新。
- 编排、成绩和报名表统一以 Go 返回的 `competitorId` 索引个人或接力队;接力使用道次和计时输入,报名数量使用「队」。业务约束见 [domain.md](domain.md)「接力」。

## 成绩页

- 每组的输入是本地状态,按组保存。项目详情重新读取时(本组或别的组保存、MCP 写入、运动员资料变化),`features/results/rows.ts` 的 `mergeRows` 逐位置合并:用户改过的行保留输入,其余行跟随新数据;整组直接换成新数据会清掉另一组里没保存的输入。
- 位置上换了人,该位置的输入丢弃,避免把成绩记到另一个人名下。刚保存的行跟随新数据,显示 Go 整理后的成绩文本,保存后该组不再算有改动。
- 「放弃」恢复为已读取的数据,并去掉未保存的新增高度。读写经 `results/controller.ts`。

## 编排页批量排时

- `features/arrange/ScheduleDialog.tsx` 经 `scheduleController.ts` 调用生成的 `Arrange.previewSchedule` / `Arrange.schedule` 绑定,与 MCP 共用 Go 预览和提交。入口与草稿限制见 [arrange.md](arrange.md)「批量排时间」。
- 请求期间禁用日期、时间、间隔与取消操作;过期预览或提交失败后保留输入,清除确认状态并要求重新预览。保存成功经 `heats` 事件重读,页面不自行改时间。

## 设置页

- `src/features/settings/` 按主题分类(通用 / 编排 / 打印 / MCP / 关于),每类一页;宽窗口左侧常驻分类列表,窄窗口分两级。
- 宽窄切换只增减左侧分类列表,内容区留在同一树位置,表单、焦点与未保存修改都保留。
- 开关与分段控件立即生效;需要保存的表单经底部保存栏,有修改时离开先确认。

- 通用:外观(跟随系统 / 浅色 / 深色),经根元素 `data-theme` 应用。编排:同单位分散、均衡人数,作为自动编排对话框每次打开时的默认值。打印:每个项目另起一页,Go 生成预览与打印 HTML 时读取同一设置。
- MCP:本机开关、运行状态、端口、令牌显示 / 复制 / 轮换与复制连接信息。端口单独编辑保存,失败保留输入;有端口草稿时禁用开关与轮换,避免隐式提交草稿。关于包含版本与开源许可。
- 赛事列表工具栏与赛事侧栏均提供设置入口,返回恢复进入设置前的页面。分类导航经同一 `useNav` 离开确认,宽窄变化只通过 CSS 控制分类列表的可见性。
- 读取经 `settings/controller.ts` 的钩子,写入经 `SettingsController` 与生成的 `Settings` 绑定。普通偏好与 MCP 配置分开读取,普通偏好接口不含令牌。设置写入广播 `kind=settings`,外观与打印预览按事件刷新。

## 打印

- 秩序册、成绩册由 Go `internal/report/` 生成完整 HTML 文档(检录单、号码布待实现)。「打印」页在 `sandbox` 的 `iframe` 里预览;点「打印」由 Go 另开一个窗口载入该 HTML 并调出系统打印对话框(可另存为 PDF)。不在主窗口里打印:主窗口的页面是应用界面,打出来会带上侧栏。
- 打印 HTML 同样数字等宽(`font-variant-numeric: tabular-nums`)、同样遵守文案规范。
- 打印样式预设待实现。

## 测试

- 纯函数单测(`bun test`,文件名 `*.test.ts`,不进 `tsc` 检查范围)与 Go 单测为主。
- `bun run test:ui` 使用 Playwright,覆盖宽屏 1280、窄屏 800 与浅色 / 深色,检查核心页面不出现整页横向溢出、设置输入与焦点保留、离开确认、保存与错误反馈。浏览器测试模拟 mygo 桥接,业务持久化与 HTTP MCP 使用 Go 的真实 SQLite 测试;两者不能替代 WebView 原生行为实测(见 [verification.md](../verification.md))。
