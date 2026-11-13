# 设计系统:token 与 `src/design/` 组件

界面文案(标题、副标题、对话框、提示)的写法与长度见 [copywriting.md](copywriting.md)。

## 为什么有这一层

颜色、字体、间距、圆角集中成 token,页面只用 `src/design/` 的 `St*` 组件,换风格只改一处。组件内部可以借用第三方实现(无障碍、焦点管理),但不对页面暴露第三方组件的 API。

## Token(`src/design/tokens.css`)

全部写成 `:root` 上的 CSS 变量,跟随系统时由 `@media (prefers-color-scheme: dark)` 选择暗色;显式外观由根元素 `data-theme=light/dark` 覆盖,同名 token 保持一致。页面与组件只读变量,不写字面颜色值。

| 类别 | 档位 |
|---|---|
| 间距 `--st-space-*` | 2 / 4 / 8 / 12 / 16 / 24 / 32,另有 `page`(页面边距)、`section`(分组间距) |
| 圆角 `--st-radius-*` | 6 / 8 / 12 / 16 / 20 |
| 语义色 `--st-tone-{neutral,info,success,warning,danger}-{accent,container,on-container}` | 状态、徽标、提示条 |
| 层级 | `--st-bg` 页面底色(浅灰 / 近黑);`--st-surface` 卡片、对话框(白 / 深灰一级);`--st-surface-raised` 浮起层 |
| 字体 | 正文、标题若干档;成绩数字另有大号等宽档 |
| 动效 `--st-motion-{fast,medium,slow}` / `--st-ease-{out,in}` | 100 / 160 / 220ms;系统「减少动态效果」时全部为 0 |

**约定**:
- 新代码的间距 / 圆角只用 token 档位,不写 10 / 14 / 18 这类零散值。
- **数字一律等宽**:根元素设 `font-variant-numeric: tabular-nums`,成绩跳动时不左右抖,表格数字纵向对齐。打印模板(`internal/report/`)同样在 `body` 上设。不要在组件里覆盖成 `proportional-nums`。
- **语义色**经 tone 变量取,不写 `#4CAF50` 之类的硬编码。
- **对比度**：普通文字与背景至少 4.5∶1，焦点轮廓与相邻表面至少 3∶1；主按钮用 `--st-on-primary`，危险按钮用独立的 `--st-on-danger`。输入框与下拉框边界用 `--st-control-outline`，分隔线用 `--st-outline`。强制颜色模式使用系统色，勾选、开关、分段选中与表格选择通过图形或轮廓保留状态。
- **圆角统一 iOS 风格平滑轮廓**:圆角矩形用 `corner-shape: superellipse(1.78)` 作视觉近似;满圆角胶囊、真实圆形(头像、单选圆点)与开关轨道显式 `round`。WebView 不支持 `corner-shape` 时按普通圆角显示,不另写兜底。radius token 表示真实半径,页面不自行放大。

## 组件(`src/design/`)

缺组件时**先在 `src/design/` 补组件**,再在页面使用;不要在页面里临时拼原生控件或第三方组件。

| 组件 | 用途 |
|---|---|
| `StButton`(`primary / secondary / tonal / danger / ghost` × `small / medium / large`,可 `loading`) | 所有文字按钮。同一位置会切换文字时(「全选」↔「取消全选」)传 `reserveWidthFor` 列出全部文字,宽度取最宽者,切换时旁边的控件不跳动 |
| `StLinkButton` | 按钮外观的导航链接，使用带 `href` 的原生 `<a>`，保留链接语义与键盘操作；应用外部链接经 controller 调用 mygo 在系统浏览器打开 |
| `StIconButton`(`plain / tonal / filled`) | 图标按钮,必须带无障碍标签 |
| `StConfirmButton` | 危险操作:第一次点击变红色「再按一次确认」,数秒内再点才执行 |
| `StCard` / `StSection` | 卡片 / 带标题的分组卡片 |
| `StRow` / `StStack` / `StEmpty` / `StMuted` | 一行控件(底边对齐)/ 纵向排列 / 空列表占位 / 次要文字 |
| `StNotice` / `StTag` | 提示条 / 小标签(状态、`Q` / `q` 晋级标记) |
| `StListItem` / `StSwitchItem` / `StDivider` | 设置式列表行,整行可点;`StListItem` 是 `div[role=button]`,右侧可再放按钮 |
| `StSwitch` / `StCheckbox` | 开关 / 复选;在列表行里只展示,点击由整行处理；交互复选框支持 `aria-label`，半选同步原生 `indeterminate`，全选时清除半选 |
| `StTextField` / `StSearchField` / `StTextArea` | 文本输入(多行框用于粘贴表格):标签在框**上方**(不做浮动标签),说明与错误在框下方;`prefix` / `suffix` 插槽放单位(「m」「s」)；说明、错误经 `aria-describedby` 关联，错误状态同步 `aria-invalid`，保留调用方提供的描述 |
| `StMarkField` | 成绩输入:按项目类型解析(`12.34`、`1:05.67`、`6.52`),失焦时调用 Go `scoring` 校验并整理成显示格式,前端不自己解析;`accept` 放行田赛的 `X` / `-` 等记号 |
| `StTrialsField` | 高度试跳文本格，使用文字键盘、数字右对齐；页面可传 `readOnly` 拦截按键做单键录入；`invalid` 只标红框不写文字，错误由页面在行下方显示并经 `aria-describedby` 关联；接收整组 Go 预览返回的错误，解析与动作规则见 [domain.md](domain.md)「成绩」 |
| `StDateField` / `StTimeField` | 本地日期 / 24 小时制时分,分开选择,支持 `disabled`;改日期保留时分,改时分保留日期 |
| `StSegmented` | 2–5 个互斥选项、页内 Tab；以单选组暴露语义，标签或 `aria-label` 命名，只有当前选项进入 Tab 顺序，方向键循环切换、Home / End 到首尾并跳过禁用项；选中底色是一块滑动的底板,量好位置后才开始带过渡,控件不可见(隐藏的面板)时由选中按钮自己画底色 |
| `StSelect` / `StMultiSelect` / `StMenu` | 下拉单选 / 下拉多选 / 操作菜单,见下文；`StSelect` 可传 `aria-label` 为表内无独立可见标签的选择框命名。`StMultiSelect` 勾选时菜单不关闭,不勾或全勾都表示「全部」,显示 `allLabel` |
| `StDialog` | 所有对话框(内部用原生 `<dialog>` 模态);页面不直接写 `<dialog>` 或自绘外壳;组件用 `aria-labelledby` 关联标题,供读屏与自动测试识别。对话框里的下拉菜单挂在对话框内,否则会被模态层盖住；表单提交时优先聚焦已标错的字段，异步校验返回后再检查一次 |
| `StTable` | 数据表格:表头吸顶、列宽可配置、数字列右对齐、行可选中；`rowLabel` 给勾选框命名，`onRowClick` 同时在 `actionColumn`（默认首列）生成原生按钮，该列内容须为非交互内容。填满容器且超过 200 行时按固定行高只渲染可见行，上下占位禁止 flex 收缩；总行数与行号经 ARIA 暴露，表头计为第一行；筛选时钳制渲染范围并同步真实滚动位置，观察容器高度变化。`empty` 传文字时按 `StEmpty` 排版;数据未到时传 `loading`,不显示空列表占位;`fill` 时传 `scrollKey` 记住滚动位置 |
| `StFieldFeedback` | 外壳与每个模态各有一个轻声播报区；字段离焦后或异步校验返回错误时延后 500ms 播报，修正、重新聚焦或卸载时清除该字段的待播报内容，避免打断输入与焦点播报；模态内的错误使用模态内的播报区 |
| `StPage` / `StToolbar` | 页面骨架与顶部工具栏;非 `flush` 页面传 `scrollKey` 记住正文滚动位置 |
| `StToast` | 操作结果提示,文案 ≤ 15 字 |

**下拉菜单**用自有弹出层:整块菜单从锚点一侧淡入 + 6px 位移 + 0.96→1 缩放(入 `--st-motion-medium` / 出 `--st-motion-fast`);锚点在右半边时右缘对齐、否则左对齐,下方放不下且上方更宽裕时向上弹出,超高内部滚动。不用逐项依次淡入的展开动画,看起来又慢又生硬。退出动画期间弹出层设为 `inert`、对读屏隐藏且不接收指针,避免连续选项时点到正在消失的菜单。

- 单选下拉使用 `listbox / option / aria-selected`，操作与多选使用 `menu / menuitem / menuitemcheckbox`；触发按钮声明匹配的 `aria-haspopup`，打开时通过 `aria-controls` 关联弹出层。
- Enter / 空格沿原生按钮行为打开，↑ / ↓ 也可打开；键盘打开后聚焦已选项或首个可用项，操作菜单按 ↑ 打开时聚焦末项；鼠标打开保留触发按钮焦点。弹出层内 ↑ / ↓ 循环、Home / End 到首尾，输入字符按名称前缀查找，均跳过禁用项；输入法组合期间不处理这些快捷键。
- 单选与操作执行后、Esc 关闭后焦点返回触发按钮；Esc 不关闭其所在对话框。Tab / Shift+Tab 关闭弹出层并从触发按钮继续原生顺序，焦点移到弹出层与触发按钮之外时关闭；多选勾选保留菜单和当前焦点。

- 表格单元格里的下拉框与输入框没有可见标签，经 `aria-label` 带上对象名称区分（「运动员1的组别」）。`StCheckbox` 的 `label` 是可见文字，无文字的勾选框用 `aria-label`；表格行操作经名称单元格的原生按钮进入键盘顺序，整行点击保留。
- 字段说明与错误只在存在时建立描述关联，成绩窄格的错误保留读屏文本；解析、校验和错误文案仍由 Go 提供。对话框主要提交按钮使用 `type="submit"`，以共享表单入口处理校验后的焦点。

## 动效

只为让用户看清「发生了什么」,不为装饰;任何动画都不推迟下一次点击。

- **悬停与按下**:背景、文字、边框色用 `--st-motion-fast` 过渡;按钮按下缩到 0.97(图标按钮 0.94),给出点到了的反馈。
- **出现**:新出现的内容淡入,卡片、提示条、组卡片、分组名单再加 4px 上移;页面切换只从 0.4 透明度淡入、不位移,内容立刻可点。页内切换的 Tab / 分类内容用 `app.css` 的 `.view-enter`。
- **离开**:比进入快(`--st-motion-fast`)。下拉菜单与提示条有离场动画;**对话框只有进入动画,关闭立即消失**:关闭后对话框里的标题与正文多由已清空的状态决定,保留离场会闪出错误文字,且模态对话框淡出期间整页点不动。
- 不做逐项依次出现的列表动画,不给成绩录入格、高度试跳格加动画(录入时连续按键,动画只会拖慢);虚拟列表的行按滚动挂载,不给行加进入动画。
- 关键帧 `st-fade-in` / `st-rise-in` / `st-page-in` / `st-pop-in` / `st-pop-out` 在 `components.css`,页面样式直接引用,时长只用 token。
- 系统开启「减少动态效果」时 token 归零,动画与过渡都瞬间完成,页面不需另写判断。

## 交互约定

- 列表行整行可点,右侧开关、圆点只展示,避免只能点小控件。
- 选项 ≤ 5 个用 `StSegmented`;更多用 `StSelect` 或对话框 + `StRadioItem`。
- 每个区域最多一个 `primary` 按钮。
- **工具栏按钮不随状态移位**:右对齐,常驻按钮在没有可用操作时置灰而不是隐藏;计数之类会变宽的内容放在按钮组最左。否则连点时会点到挪过来的另一个按钮。
- **控件按状态置灰,不隐藏**:隐藏会让界面跳动。完全不该出现的入口才不渲染。
- **数据到达前不显示「还没有……」**:读取中的列表传 `StTable` 的 `loading`;否则空列表占位会先闪一下再换成内容。占一行高度的文字(赛事侧栏的日期)在读取中与没有内容时用不换行空格占住这一行,下面的入口不随之移动。
- **滚动容器设 `scrollbar-gutter: stable`**(`StPage` 正文、`fill` 表格、编排 / 成绩主区、侧栏名单、设置内容区):Windows 与 Linux 的滚动条占宽度,内容变长时出现滚动条会把整页内容往左推。
- 换筛选条件时旧结果留在原处、变淡(`.stale-aware[data-stale]`),新结果到了再替换,表格不先缩成表头再长回来。
- **窗口宽窄切换不丢输入**:宽屏多栏 / 窄屏单栏切换时,表单所在组件不得重新挂载(保持同一 key 与树位置),否则输入框失焦、未保存内容丢失。
- 输入框与按钮并排时底边对齐(标签在框上方,顶边对齐会错位)。
- 需要点保存的表单,有未保存修改时底部出现「放弃 / 保存」栏;离开页面先确认。开关与分段控件立即生效,不进保存栏。

## 图标

- 品牌标识为「跑道 S」，几何、色彩、留白与导出规则以 [品牌规范](../brand-guidelines.md) 为准；母版在 `resources/brand/source/`，运行 `bun run brand:generate` 统一导出并同步应用与官网图标。品牌标识独立于功能图标，不用 Lucide 替代，也不单独修改导出的位图。

- 全应用统一用 **Lucide**(ISC 许可,线性描边,`lucide-react`),经 `src/design/icons.ts` 改名为 `Icon*` 统一导出后使用;页面不直接引入其他图标库。
- 描边统一 `1.75`(24 视口),圆头圆角。
- Lucide 没有的图标按同一规格(24 网格、约 2 留边、圆头描边)自绘 SVG,放进 `src/design/icons/`。
- 新增依赖须同步「关于 → 开源许可」。

## 边界检查

`scripts/check-design-system.sh`(`bun run check` 与 CI 同跑)是**棘轮**:`src/design/` 之外不得引入第三方组件库、其他图标库,不得写字面颜色值(`#xxx`、`rgb(`)与零散像素圆角。这几条规则的基线为空;基线 **只减不增**。同一脚本还检查绑定引用,见 [frontend.md](frontend.md)「代码结构」。
