# 交换格式规范 v5

SportsMeet 赛事数据的导入导出格式。一个文件是**一个赛事的完整快照**:参赛单位、运动员、项目、报名、轮次、组、成绩。

- 机器可读定义：[v5](schema/interchange-v5.schema.json)；兼容读取 [v1](schema/interchange-v1.schema.json)、[v2](schema/interchange-v2.schema.json)、[v3](schema/interchange-v3.schema.json)、[v4](schema/interchange-v4.schema.json)（JSON Schema 2020-12）
- 示例:[examples/sample-meet.json](examples/sample-meet.json)

## 1. 文件

| 项 | 规定 |
|---|---|
| 编码 | UTF-8,无 BOM |
| 扩展名 | `.meet.json` |
| 结构 | 单个 JSON 对象 |
| 未知字段 | 读取方忽略(为后续版本加字段留余地) |
| 空值 | 可选字段可省略或为 `null`,含义相同;字符串字段没有值时写 `""` |

写出使用两个空格缩进、保留 HTML 字符并以换行结束；`encode.go` 按 `File` 的 JSON 标签顺序逐记录编码顶层数组，复用记录缓冲区，避免额外构建完整的缩进副本。读取不要求缩进或末尾换行。

## 2. 版本

```json
{ "format": "sportsmeet-interchange", "version": 5 }
```

- `format` 固定,不符即拒绝读取。
- `version` 为正整数。增删字段、改字段含义、改单位、改枚举取值均须 +1。
- 写出方总是写 v5；读取方支持 v1、v2、v3、v4、v5。v1 不解释起跑方式、跳跃／投掷、建议棒数；v1／v2 不解释参赛方式、计次计分与全能字段；v1–v3 不解释轮次开始时间 `rounds[].scheduledAt` 与组的预计用时 `heats[].durationMinutes`；v1–v4 不解释组的实际开赛时间 `heats[].startedAt`；这些未知字段即使使用其他 JSON 类型也忽略。兼容默认值为计时项目分道、远度跳跃、接力按队伍其余按个人参赛，不根据项目名推断。
- 读取方必须能读所有已发布版本;遇到比自己新的版本直接拒绝,并提示升级软件(不尝试猜测读取)。

## 3. 基本约定

### 3.1 标识

`id` 是字符串,在本文件内按实体类型唯一,可以是 UUID、数据库主键或任意短串。它**只用于文件内引用**,读取方不得假设它与自己库里的主键相同。

### 3.2 日期与时间

| 类型 | 格式 | 例 |
|---|---|---|
| 日期 | `YYYY-MM-DD`,本地日期,须是真实存在的日子 | `"2026-11-12"` |
| 计划时间 | `YYYY-MM-DDTHH:mm`,本地时间,精确到分钟,无时区,须是真实存在的日子与时刻 | `"2026-11-12T09:30"` |
| 导出时刻 | RFC 3339,带时区偏移 | `"2026-11-12T18:05:31+08:00"` |

### 3.3 性别

| 场合 | 取值 |
|---|---|
| 运动员 `gender` | `"男"` / `"女"` / `""`(未知) |
| 项目 `gender` | `"男"` / `"女"` / `"混合"` |

### 3.4 成绩值 `mark`

整数,单位由项目类型决定:

| 项目类型 `kind` | 单位 | 例 |
|---|---|---|
| `track`、`relay` | 毫秒 | 12.34 秒 → `12340`;1:58.20 → `118200` |
| `distance`、`height` | 毫米 | 6.52 米 → `6520`;1.85 米 → `1850` |
| `count` | 次数整数 | 125 次 → `125`，0 为有效成绩 |
| `score` | 分数乘以 `10^scoreDecimals` | 两位小数时 12.50 分 → `1250` |

手计成绩按毫秒存(12.3 → `12300`),精度由显示决定,不在文件里另记。

## 4. 顶层结构

```jsonc
{
  "format": "sportsmeet-interchange",
  "version": 4,
  "exportedAt": "2026-11-12T18:05:31+08:00",
  "generator": { "name": "SportsMeet", "version": "0.1.0" },
  "meet": { ... },
  "units": [ ... ],
  "athletes": [ ... ],
  "events": [ ... ],
  "combineds": [ ... ]
}
```

| 字段 | 必填 | 说明 |
|---|:---:|---|
| `format` / `version` | ✓ | §2 |
| `exportedAt` | ✓ | 导出时刻 |
| `generator` | ✓ | 写出该文件的软件名与版本,用于排查问题 |
| `meet` | ✓ | §5.1 |
| `units` / `athletes` / `events` | ✓ | 数组,可为空 |
| `combineds` | | 全能数组，省略表示无全能，见 §5.11 |

## 5. 实体

### 5.1 赛事 `meet`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `name` | string | ✓ | 非空 |
| `startDate` | 日期 | ✓ | |
| `endDate` | 日期 | | 省略 = 与 `startDate` 同一天 |
| `location` | string | | |
| `note` | string | | |

### 5.2 参赛单位 `units[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `name` | string | ✓ | 全称,文件内唯一 |
| `shortName` | string | | 简称,用于秩序册 |

### 5.3 运动员 `athletes[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `name` | string | ✓ | 非空 |
| `gender` | string | | §3.3 |
| `bib` | string | | 号码布号 |
| `studentNumber` | string | | 学号 / 证件号 |
| `unitId` | string \| null | | 引用 `units[].id` |
| `affiliation` | string | | 单位名称文本;有 `unitId` 时写出方必须填成该单位的 `name` |
| `className` | string | | 班级 |
| `grade` | string | | 年级 |
| `birthDate` | 日期 \| null | | |
| `note` | string | | |

`bib`、`studentNumber` 非空时各自在文件内唯一。

### 5.4 项目 `events[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `name` | string | | 省略时读取方按「性别前缀 + `discipline` + 空格 + `groupLabel`」合成,如「男子100m 初中组」 |
| `gender` | string | ✓ | §3.3 |
| `discipline` | string | ✓ | 项目名,如 `100m`、`4×100m`、`跳远`、`跳高` |
| `groupLabel` | string | | 组别,如「初中组」 |
| `kind` | string | ✓ | `track` / `relay` / `distance` / `height` / `count` / `score` |
| `date` | 日期 \| null | | 省略 = 赛事开始日期 |
| `startMode` | string | | 仅计时项目：`lanes` 分道 / `group` 不分道 / `break` 分道后抢道，默认 `lanes` |
| `distanceMode` | string | | 仅远度：`jump` 跳跃 / `throw` 投掷，默认 `jump` |
| `relayLegCount` | integer | | 仅接力：非负建议棒数，默认 0（未指定），不强制名单人数 |
| `laneCount` | integer | | 径赛、接力的道数，默认 8；不分道时保留但不限制每组人数 |
| `attemptCount` | integer | | 远度项目每人尝试次数默认 3；计次／计分默认 1，多次模式按此字段录入 |
| `note` | string | | |
| `participantMode` | string | | `athlete` 个人 / `team` 队伍，接力固定为队伍，其余默认个人 |
| `teamSize` | integer | | 普通队伍的非负建议人数，0 为未指定，接力使用建议棒数 |
| `scoreDecimals` | integer | | 计分小数位 0–3，默认 0，计次固定 0 |
| `rankDirection` | string | | 计次／计分的 `desc` 多／高值优先，或 `asc` 少／低值优先，默认 `desc` |
| `attemptMode` | string | | 计次／计分的 `single` 一次 / `best` 取最好 / `sum` 求和，默认 `single` |
| `combinedId` | string | | 全能分项所属的 `combineds[].id`，普通项目为空 |
| `teams` | Team[] | | 队伍项目，§5.5 |
| `entries` | Entry[] | ✓ | §5.6 |
| `rounds` | Round[] | ✓ | §5.7,可为空(尚未编排) |

同一文件内 `combinedId + gender + discipline + groupLabel` 不得重复，普通项目 `combinedId` 为空。配置不适用于该项目类型时拒绝导入，取值与限制见 [领域细则](agents/domain.md)。

### 5.5 队伍 `teams[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | 在本项目内唯一 |
| `name` | string | ✓ | 如「一中 A 队」 |
| `unitId` | string \| null | | |
| `memberIds` | string[] | ✓ | 引用 `athletes[].id`,接力按棒次顺序，其他队伍按名单顺序；可为空(未定名单) |

同一项目、同一参赛单位内队名规范化后唯一;同一运动员在该项目只能属于一队,每队内不得重复担任棒次。

### 5.6 报名 `entries[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `athleteId` | string | 二选一 | 个人项目 |
| `teamId` | string | 二选一 | 队伍项目，引用本项目 `teams[].id` |
| `seedMark` | integer \| null | | 报名成绩,用于分组排道 |

同一运动员(或队)在一个项目里只能报名一次。

### 5.7 轮次 `rounds[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `stage` | string | ✓ | `heat` 预赛 / `quarterfinal` 次赛 / `semifinal` 半决赛 / `final` 决赛 |
| `ordinal` | integer | ✓ | 本项目内从 1 开始,决赛最大 |
| `name` | string | | 省略按 `stage` 显示 |
| `scheduledAt` | 计划时间 \| null | | v4。轮次开始时间;没有 `scheduledAt` 的组沿用它 |
| `advance` | Advance \| null | | 晋级下一轮的规则,决赛为 `null` |
| `heats` | Heat[] | ✓ | §5.8 |

`advance`:`{ "byPlace": 2, "byMark": 2 }` = 各组前 2 名(大 Q)加其余成绩最好的 2 名(小 q)。

### 5.8 组 `heats[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `ordinal` | integer | ✓ | 本轮次内从 1 开始 |
| `name` | string | | 省略显示为「第 N 组」 |
| `scheduledAt` | 计划时间 \| null | | 组单独指定的时间;`null` 时沿用所在轮次的 `scheduledAt` |
| `durationMinutes` | integer \| null | | v4。预计用时,1–1440 分钟;组的预计结束时间 = 生效计划时间 + 预计用时 |
| `startedAt` | 计划时间 \| null | | v5。实际开赛时间,格式同计划时间;`null` 为未记录 |
| `wind` | integer \| null | | 径赛整组风速,单位 0.1 m/s(+1.2 → `12`) |
| `slots` | Slot[] | ✓ | §5.9 |

### 5.9 位置 `slots[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `position` | integer | ✓ | 分道与分道后抢道 = 起跑道次；不分道与田赛 = 顺序；组内唯一,从 1 开始 |
| `athleteId` / `teamId` | string | 二选一 | 必须已在本项目 `entries` 中报名 |
| `result` | Result \| null | | `null` = 尚未录入 |

### 5.10 成绩 `result`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `status` | string | ✓ | `OK` 有效 / `DNS` 未起跑(弃权)/ `DNF` 未完成 / `DQ` 犯规取消 / `NM` 无有效成绩 |
| `mark` | integer \| null | | `status = OK` 时必填;其余为 `null` |
| `rank` | integer \| null | | 组内名次,经裁判确认的值,读取方原样保留不重算 |
| `qualify` | string \| null | | `Q` 按名次晋级 / `q` 按成绩晋级 / `null` |
| `wind` | integer \| null | | 个人风速(田赛最好一跳、或覆盖整组风速),单位同 §5.8 |
| `attempts` | Attempt[] | | 远度及多次计次／计分项目的逐次成绩 |
| `bars` | Bar[] | | 高度项目逐高度记录 |
| `note` | string | | |

逐次 `attempts[]`(按次序，单位随项目类型):

| 字段 | 类型 | 说明 |
|---|---|---|
| `outcome` | string | `valid` 有效 / `foul` 犯规 / `pass` 免跳 / 免投 |
| `mark` | integer \| null | `valid` 时必填 |
| `wind` | integer \| null | |

高度 `bars[]`(按高度升序):

| 字段 | 类型 | 说明 |
|---|---|---|
| `height` | integer | 毫米 |
| `trials` | string | 1–3 个字符,由 `O`(成功)、`X`(失败)、`-`(免跳)组成,如 `"XO"`、`"XXX"`、`"-"`;在某高度失败后免跳,剩余次数带到下一高度,所以下一高度可能只有 `"X"` |

`result.mark` 是最终成绩：远度为最好一次，高度为最后成功高度，计次／计分多次模式按 `attemptMode` 汇总。犯规和免试不作为有效零值；全无有效尝试记 `NM`。计次／计分的有效最终值须与已有逐次记录一致，解析与求和检查整数溢出。

### 5.11 全能 `combineds[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | 文件内唯一 |
| `name` | string | ✓ | 全能名称 |
| `gender` | string | ✓ | 男 / 女 / 混合 |
| `groupLabel` | string | | 组别 |
| `note` | string | | |
| `ruleVersion` | integer | ✓ | 正整数，赛事内规则版本 |
| `athleteIds` | string[] | ✓ | 统一报名，引用运动员，可为空，不能重复 |
| `components` | object[] | ✓ | 至少两项，按顺序排列，结构见下文 |

每个分项为 `{eventId, ordinal, rule}`：`eventId` 引用 `events[].id`，该项目的 `combinedId` 必须回指本全能；`ordinal` 从 1 连续递增。分项继承全能的性别与组别，只能个人参赛，只含一个无晋级规则的决赛轮次，报名须与全能一致。一个项目只属于一个全能，不能悬空引用。

`rule` 为 `{bands, invalidPolicies}`：`bands` 每档包含整数 `mark`（单位同分项）与非负整数 `points`；按高标准到低标准排列，阈值严格单向，积分不能随标准降低而增加。达到某档得该档积分，不插值，低于最低标准得零分。`invalidPolicies` 必须给出 `DNS / DNF / DQ / NM` 各自的 `zero`（该项零分）或 `exclude`（不列总排名）。

未录入成绩不算零分；总分与总排名由分项原始成绩和计分表计算，不另存可编辑的累计值。完整规则见 [全能细则](agents/combined.md)。

## 6. 校验

读取方导入前必须检查,任一失败即整份拒绝,并列出全部错误(不只第一条):

1. `format`、`version` 符合 §2。
2. 必填字段齐全,类型正确,枚举取值合法。
3. 所有引用(`unitId`、`athleteId`、`teamId`、`memberIds`)都能找到。
4. §5 中的唯一性约束。
5. `slots[].athleteId` / `teamId` 已在本项目报名;队伍项目的 `slots` 只能用 `teamId`,个人项目只能用 `athleteId`。
6. `status = OK` 时 `mark` 非空,否则 `mark` 为 `null`。

以下只给**警告**,不阻止导入:运动员性别与项目性别不符、`seedMark` 缺失、运动员未报任何项目。

## 7. 导入到已有赛事

导入一律先生成预览(新建 / 复用 / 冲突的数量与清单),确认后在一个事务中落库。

运动员按以下顺序识别是否为已有运动员,取第一个非空的字段:

1. `studentNumber`
2. `bib`
3. `name` + `affiliation`

找到 0 个则新建,1 个则复用,多于 1 个则报冲突、整份拒绝。比较前去掉首尾空白、合并连续空白、忽略英文大小写。

队伍在所属项目内按规范化的 `name + unitId` 识别,单位先匹配到目标赛事;成员也先匹配到目标运动员,名单顺序完全一致才复用。名单不符、匹配不唯一或成员已在该项目其他队伍时整份拒绝,避免把已有比赛记录对应到另一份名单。预览分别统计队伍新建与复用数量。

轮次按 `stage`、组按 `ordinal` 对应。新建的轮次与组带上文件里的 `scheduledAt`、`durationMinutes` 与 `startedAt`；已有的轮次与组保留目标里的开始时间、计划时间、预计用时与实际开赛时间，不按文件改写。导入的成绩不会自动记录实际开赛时间，只用文件里的值。

全能按 `gender + name + groupLabel` 识别；分项先匹配所属全能，项目再按 `combinedId + gender + discipline + groupLabel` 识别。`discipline` 宽松比较:`100 米` = `100m`,`4x100M` = `4×100m`。


同名项目的配置冲突：v2／v3 导入复用项目时，起跑方式、跳跃 / 投掷、建议棒数，以及计时项目道数、远度尝试次数须与目标一致（缺省值先按 §5.4 解释），不一致则整份拒绝，避免把顺序当道次或隐藏逐次成绩。v1 文件复用目标项目时保留目标配置。导入新赛事时使用文件配置与兼容默认值。v2／v3 的分道位置不得超过道数；v1 位置按文件保留。

v3 同赛事导入复用项目时，规范化后的参赛方式、建议人数、小数位、排名方向与汇总方式须相同；复用全能时分项及计分表须一致。计分表冲突整份拒绝，修改规则须走重算预览确认，避免文件导入绕过规则版本与操作记录。
