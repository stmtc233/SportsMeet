# 成绩交换格式规范 v2

把在其他系统(如计时系统)里完成的成绩写回 SportsMeet 的文件格式。一个文件只装**一个赛事里若干组的成绩**,可附带已有下一轮的晋级编排;定位用的 ID 取自该赛事导出的[交换格式](interchange-format.md)文件。

- 机器可读定义:[results-v2](schema/results-v2.schema.json)；兼容读取 [results-v1](schema/results-v1.schema.json)(JSON Schema 2020-12)
- 示例：[v2 晋级与成绩](examples/sample-results-v2.json)、[v1 成绩](examples/sample-results.json)

## 1. 文件

| 项 | 规定 |
|---|---|
| 编码 | UTF-8,无 BOM |
| 扩展名 | `.results.json` |
| 结构 | 单个 JSON 对象 |
| 未知字段 | 读取方忽略 |
| 空值 | 可选字段可省略或为 `null`,含义相同 |

## 2. 版本

```json
{ "format": "sportsmeet-results", "version": 2 }
```

- `format` 固定,不符即拒绝读取。
- 读取方支持 v1–v2，写出方写 v2；v1 忽略 `promotions`。
- `version` 为正整数,增删字段、改字段含义、改单位、改枚举取值均须 +1。与交换格式的版本号各自独立。
- 读取方遇到比自己新的版本直接拒绝,并提示升级软件。

## 3. 写出方从哪里取 ID

写出方先读取 SportsMeet 导出的交换格式文件(`.meet.json`),记下其中的 ID,写成绩文件时原样带回:

| 本格式字段 | 来源 |
|---|---|
| `meetId` | `meet.id` |
| `heats[].heatId` | `events[].rounds[].heats[].id` |
| `results[].athleteId` | `athletes[].id`(该组位置上的运动员) |
| `results[].teamId` | `events[].teams[].id`(队伍项目) |

SportsMeet 导出的交换格式文件里,这些 ID 就是它库里的主键;重新导出时保持不变。

## 4. 顶层结构

```jsonc
{
  "format": "sportsmeet-results",
  "version": 2,
  "exportedAt": "2026-11-16T15:20:00+08:00",
  "generator": { "name": "SyncTrack", "version": "0.5.5" },
  "meetId": "…",
  "heats": [ … ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `format` / `version` | | ✓ | §2 |
| `exportedAt` | string | ✓ | RFC 3339,带时区偏移 |
| `generator` | object | ✓ | `{name, version}`,写出该文件的软件,用于排查问题 |
| `meetId` | string | ✓ | §3 |
| `heats` | Heat[] | ✓ | 可为空 |

### 4.1 组 `heats[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `heatId` | string | ✓ | §3,文件内唯一 |
| `label` | string | | 写出方看到的组名,如「男子100m 初中组 预赛 第1组」;只用于提示,不参与定位 |
| `startedAt` | 计划时间 \| null | | 实际开赛时间,`YYYY-MM-DDTHH:mm` 本地时间,精确到分钟 |
| `results` | Result[] | ✓ | 可为空 |

### 4.2 成绩 `results[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `position` | integer | ✓ | 写出方记录的道次或顺序,从 1 开始;只用于核对 |
| `athleteId` / `teamId` | string | 二选一 | §3 |
| `check` | object | | `{name, studentNumber, bib}`,ID 找不到人时用来在组内核对,见 §6 |
| `status` | string | ✓ | `OK` 有效 / `DNS` 未起跑(弃权)/ `DNF` 未完成 / `DQ` 犯规取消 / `NM` 无有效成绩 |
| `mark` | integer \| null | | `status = OK` 时必填,毫秒(12.34 秒 → `12340`);其余为 `null` |
| `note` | string | | 成绩备注 |

`mark` 是最终成绩:写出方已加好自己的偏移与修正,读取方不再换算;显示精度由 SportsMeet 决定。

本版只用于计时项目(交换格式里 `kind` 为 `track` 或 `relay`)。

**不在本格式里的字段**:运动员性别认同不属于成绩数据，不在此格式收集或回传；名次、晋级(`Q / q`)与并列裁定由 SportsMeet 按本赛事的规则计算;风速、逐次与逐高度记录不在本版范围。

## 5. 校验

以下任一失败即整份拒绝,并列出全部错误:

1. `format`、`version` 符合 §2。
2. 必填字段齐全,类型正确,枚举取值合法。
3. `heats[].heatId` 在文件内唯一;同一组内 `position` 唯一,同一运动员或队伍只出现一次。
4. `athleteId` 与 `teamId` 恰好填一个。
5. `status = OK` 时 `mark` 是正整数,否则 `mark` 为 `null`。
6. `startedAt` 符合计划时间格式,且是真实存在的日子与时刻。
7. `meetId` 是导入目标赛事。

## 6. 导入

导入一律先生成预览,确认后在一个事务中写入。

**定位**:对每个组,先按 `heatId` 找到组,再在组内找参赛者:

1. 按 `athleteId` / `teamId` 找到该组里的位置。位置号与 `position` 不同时照常写入,并在预览里提示,因为写出方可能调整过道次。
2. ID 不在本组时,用 `check` 在**本组内**按学号 → 号码布 → 姓名依次核对(规范化同交换格式 §7),唯一命中就写入并提示,否则跳过。
3. 组不存在、不属于目标赛事,或项目不是计时项目:整组跳过并给警告。

**写入**:

- 成绩照文件写入,与界面录入走同一套校验;写完后对涉及的每个轮次重算名次与晋级。
- 与已有成绩相同的条目不计入变化;覆盖已有成绩时,预览逐条列出「原成绩 → 新成绩」。
- 文件不能删除成绩:没写到的位置保持原样。
- 原有的并列裁定在成绩不变时保留,成绩变了即失效(同界面录入)。
- 组还没有实际开赛时间时,取文件的 `startedAt`;文件没有时按界面录入的规则取导入的时刻。已有的实际开赛时间不改。

**预览**列出新增、覆盖、未变的成绩条数,覆盖明细,以及每条警告(跳过的组与成绩、按 `check` 核对或位置不同的写入)。

## 7. v2 晋级编排 `promotions[]`

可选，省略表示只导入成绩。每项为：

```json
{"eventId":"e1","sourceRoundId":"r1","targetRoundId":"r2","count":1,
 "heats":[{"heatId":"stable-generated-id","ordinal":1,"name":"","scheduledAt":"2026-11-16T14:00",
 "slots":[{"position":4,"athleteId":"a1"}]}]}
```

- `eventId` 为原 SportsMeet 项目 ID；两个轮次须属于该项目、为紧邻的先后轮次，目标必须已经存在。仅个人计时项目；不能创建新的单项或覆盖别的轮次。`count` 为按成绩取人数，正整数、并列全进。来源规则设为 `{byPlace:0,byMark:count}`，预览列出变化；从文件成绩与本机原有成绩重算名次、晋级，本机并列裁定保持有效，名单必须与重算结果完全相同。来源未判读完成、有效人数不足、裁定与名单不符时整份拒绝。
- `heats` 为完整目标编排，非空；`heatId` 为写出方持久化的稳定全局组 ID，不能与其他轮次现有组 ID 相同。`ordinal` 为正整数且同轮唯一；`name` 为自定义名称；`scheduledAt` 可省略（沿用轮次时间）。`slots` 非空，位置为正整数、分道时不得超出道数；同轮运动员唯一且为来源晋级者，同组位置唯一。
- 目标空轮次才新建组和位置，保留轮次本机已有计划时间；目标已有完整相同的组 ID、序号与运动员位置时幂等复用，保留本机组名、计划时间、预计用时、实际开赛时间与成绩。不同编排整份拒绝，禁止增量混入或删掉原组。
- 所有来源／目标须在 `meetId` 内；文件内来源轮次、目标轮次及新组 ID 不得重复。可携带多段晋级链，先组装编排、再应用本文件成绩、重算并验证各段资格，一个事务写入，任一错误不落部分数据。
- `heats[]` 的成绩可引用本次晋级新建的组；成绩写入沿用 §6，重复文件不重复生成项目、轮次或组。
- SyncTrack 再导入 SportsMeet 赛事文件按相同轮次和组 ID 合并。晋级规则、名单与成绩不通过名字匹配。
