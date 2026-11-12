# 交换格式规范 v1

SportsMeet 赛事数据的导入导出格式。一个文件是**一个赛事的完整快照**:参赛单位、运动员、项目、报名、轮次、组、成绩。

- 机器可读定义:[schema/interchange-v1.schema.json](schema/interchange-v1.schema.json)(JSON Schema 2020-12)
- 示例:[examples/sample-meet.json](examples/sample-meet.json)

## 1. 文件

| 项 | 规定 |
|---|---|
| 编码 | UTF-8,无 BOM |
| 扩展名 | `.meet.json` |
| 结构 | 单个 JSON 对象 |
| 未知字段 | 读取方忽略(为后续版本加字段留余地) |
| 空值 | 可选字段可省略或为 `null`,含义相同;字符串字段没有值时写 `""` |

## 2. 版本

```json
{ "format": "sportsmeet-interchange", "version": 1 }
```

- `format` 固定,不符即拒绝读取。
- `version` 为正整数。删字段、改字段含义、改单位、改枚举取值都要 +1;只**新增可选字段**不 +1。
- 写出方总是写当前最新版本。
- 读取方必须能读所有已发布版本;遇到比自己新的版本直接拒绝,并提示升级软件(不尝试猜测读取)。

## 3. 基本约定

### 3.1 标识

`id` 是字符串,在本文件内按实体类型唯一,可以是 UUID、数据库主键或任意短串。它**只用于文件内引用**,读取方不得假设它与自己库里的主键相同。

### 3.2 日期与时间

| 类型 | 格式 | 例 |
|---|---|---|
| 日期 | `YYYY-MM-DD`,本地日期 | `"2026-11-12"` |
| 计划时间 | `YYYY-MM-DDTHH:mm`,本地时间,精确到分钟,无时区 | `"2026-11-12T09:30"` |
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

手计成绩按毫秒存(12.3 → `12300`),精度由显示决定,不在文件里另记。

## 4. 顶层结构

```jsonc
{
  "format": "sportsmeet-interchange",
  "version": 1,
  "exportedAt": "2026-11-12T18:05:31+08:00",
  "generator": { "name": "SportsMeet", "version": "0.1.0" },
  "meet": { ... },
  "units": [ ... ],
  "athletes": [ ... ],
  "events": [ ... ]
}
```

| 字段 | 必填 | 说明 |
|---|:---:|---|
| `format` / `version` | ✓ | §2 |
| `exportedAt` | ✓ | 导出时刻 |
| `generator` | ✓ | 写出该文件的软件名与版本,用于排查问题 |
| `meet` | ✓ | §5.1 |
| `units` / `athletes` / `events` | ✓ | 数组,可为空 |

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
| `kind` | string | ✓ | `track` / `relay` / `distance` / `height` |
| `date` | 日期 \| null | | 省略 = 赛事开始日期 |
| `laneCount` | integer | | 径赛、接力的道数,默认 8 |
| `attemptCount` | integer | | 远度项目每人试跳 / 试掷次数,默认 3 |
| `note` | string | | |
| `teams` | Team[] | | 仅 `relay`,§5.5 |
| `entries` | Entry[] | ✓ | §5.6 |
| `rounds` | Round[] | ✓ | §5.7,可为空(尚未编排) |

同一文件内 `gender + discipline + groupLabel` 不得重复。

### 5.5 接力队 `teams[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | 在本项目内唯一 |
| `name` | string | ✓ | 如「一中 A 队」 |
| `unitId` | string \| null | | |
| `memberIds` | string[] | ✓ | 引用 `athletes[].id`,按棒次顺序;可为空(未定名单) |

### 5.6 报名 `entries[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `athleteId` | string | 二选一 | 个人项目 |
| `teamId` | string | 二选一 | 接力项目,引用本项目 `teams[].id` |
| `seedMark` | integer \| null | | 报名成绩,用于分组排道 |

同一运动员(或队)在一个项目里只能报名一次。

### 5.7 轮次 `rounds[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `stage` | string | ✓ | `heat` 预赛 / `quarterfinal` 次赛 / `semifinal` 半决赛 / `final` 决赛 |
| `ordinal` | integer | ✓ | 本项目内从 1 开始,决赛最大 |
| `name` | string | | 省略按 `stage` 显示 |
| `advance` | Advance \| null | | 晋级下一轮的规则,决赛为 `null` |
| `heats` | Heat[] | ✓ | §5.8 |

`advance`:`{ "byPlace": 2, "byMark": 2 }` = 各组前 2 名(大 Q)加其余成绩最好的 2 名(小 q)。

### 5.8 组 `heats[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `id` | string | ✓ | |
| `ordinal` | integer | ✓ | 本轮次内从 1 开始 |
| `name` | string | | 省略显示为「第 N 组」 |
| `scheduledAt` | 计划时间 \| null | | |
| `wind` | integer \| null | | 径赛整组风速,单位 0.1 m/s(+1.2 → `12`) |
| `slots` | Slot[] | ✓ | §5.9 |

### 5.9 位置 `slots[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `position` | integer | ✓ | 径赛 = 道次;田赛 = 试跳顺序;组内唯一,从 1 开始 |
| `athleteId` / `teamId` | string | 二选一 | 必须已在本项目 `entries` 中报名 |
| `result` | Result \| null | | `null` = 尚未录入 |

### 5.10 成绩 `result`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `status` | string | ✓ | `OK` 有效 / `DNS` 未起跑(弃权)/ `DNF` 未完成 / `DQ` 犯规取消 / `NM` 无有效成绩(田赛) |
| `mark` | integer \| null | | `status = OK` 时必填;其余为 `null` |
| `rank` | integer \| null | | 组内名次,经裁判确认的值,读取方原样保留不重算 |
| `qualify` | string \| null | | `Q` 按名次晋级 / `q` 按成绩晋级 / `null` |
| `wind` | integer \| null | | 个人风速(田赛最好一跳、或覆盖整组风速),单位同 §5.8 |
| `attempts` | Attempt[] | | 远度项目逐次成绩 |
| `bars` | Bar[] | | 高度项目逐高度记录 |
| `note` | string | | |

远度 `attempts[]`(按次序):

| 字段 | 类型 | 说明 |
|---|---|---|
| `outcome` | string | `valid` 有效 / `foul` 犯规 / `pass` 免跳 |
| `mark` | integer \| null | `valid` 时必填 |
| `wind` | integer \| null | |

高度 `bars[]`(按高度升序):

| 字段 | 类型 | 说明 |
|---|---|---|
| `height` | integer | 毫米 |
| `trials` | string | 1–3 个字符,由 `O`(成功)、`X`(失败)、`-`(免跳)组成,如 `"XO"`、`"XXX"`、`"-"`;在某高度失败后免跳,剩余次数带到下一高度,所以下一高度可能只有 `"X"` |

`result.mark` 是最终成绩(远度为最好一次,高度为最后成功高度),写出方必须与逐次记录一致,读取方以 `mark` 为准。

## 6. 校验

读取方导入前必须检查,任一失败即整份拒绝,并列出全部错误(不只第一条):

1. `format`、`version` 符合 §2。
2. 必填字段齐全,类型正确,枚举取值合法。
3. 所有引用(`unitId`、`athleteId`、`teamId`、`memberIds`)都能找到。
4. §5 中的唯一性约束。
5. `slots[].athleteId` / `teamId` 已在本项目报名;接力的 `slots` 只能用 `teamId`,个人项目只能用 `athleteId`。
6. `status = OK` 时 `mark` 非空,否则 `mark` 为 `null`。

以下只给**警告**,不阻止导入:运动员性别与项目性别不符、`seedMark` 缺失、运动员未报任何项目。

## 7. 导入到已有赛事

导入一律先生成预览(新建 / 复用 / 冲突的数量与清单),确认后在一个事务中落库。

运动员按以下顺序识别是否为已有运动员,取第一个非空的字段:

1. `studentNumber`
2. `bib`
3. `name` + `affiliation`

找到 0 个则新建,1 个则复用,多于 1 个则报冲突、整份拒绝。比较前去掉首尾空白、合并连续空白、忽略英文大小写。

项目按 `gender + discipline + groupLabel` 识别。`discipline` 宽松比较:`100 米` = `100m`,`4x100M` = `4×100m`。

