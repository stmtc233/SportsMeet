package report

// SampleBook is a small, fixed 秩序册 / 成绩册 / 报名表 data set that touches
// every part of the books: a 代表队名单, a 竞赛日程表, an individual event
// with two heats, a field event with attempts, a height event with trials, a
// relay, a 全能 and a 团体总分表. Custom templates are trial-run against it.
func SampleBook() Book {
	return Book{
		Meet:      "第一届校园运动会",
		Dates:     "2026-11-12 – 2026-11-13",
		Location:  "第一中学田径场",
		PrintedAt: "2026-11-12 09:31",
		Rosters: []UnitRoster{{
			Name: "初一（1）班", Count: 3, Bibs: "101–103",
			Groups: []RosterGroup{
				{Label: "男子", Athletes: []Line{{Bib: "101", Name: "张三"}, {Bib: "102", Name: "李四"}}},
				{Label: "女子", Athletes: []Line{{Bib: "103", Name: "王五"}}},
			},
		}},
		Schedule: []ScheduleDay{{Day: "2026-11-12 周四", Rows: []ScheduleRow{
			{No: 1, Time: "09:00", Kind: "径赛", Event: "男子100m", Round: "决赛", Heats: 2, People: 6},
			{No: 2, Time: "10:00", Kind: "田赛", Event: "男子跳远", Round: "决赛", Heats: 1, People: 3},
		}}},
		Events: []Event{
			{
				Name: "男子100m", Meta: "2026-11-12 周四 · 分道", Count: "6 人", NameLabel: "姓名", PositionLabel: "道次", Seeded: true,
				Entries: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班", Seed: "12.50"}, {Position: 2, Bib: "102", Name: "李四", Unit: "二班", Seed: "12.80"}},
				Rounds: []Round{{
					Label:   "决赛",
					Advance: "各组前 2 名晋级",
					ByHeat:  true,
					Heats: []Heat{
						{Label: "第 1 组", Time: "09:00", Count: "2 人", Started: "09:02", ResultsTime: "09:05", Lines: []Line{
							{Position: 1, Rank: "1", Bib: "101", Name: "张三", Unit: "一班", Seed: "12.50", Mark: "12.31", Qualify: "Q", Best: -1},
							{Position: 2, Rank: "2", Bib: "102", Name: "李四", Unit: "二班", Seed: "12.80", Mark: "12.90", Qualify: "Q", Best: -1},
						}},
						{Label: "第 2 组", Time: "09:10", Count: "1 人", Lines: []Line{
							{Position: 1, Rank: "1", Bib: "103", Name: "王五", Unit: "一班", Mark: "13.02", Qualify: "q", Best: -1},
						}},
					},
				}},
			},
			{
				Name: "男子跳远", Meta: "2026-11-12 周四 · 3 次", Count: "2 人", NameLabel: "姓名", PositionLabel: "顺序", Wide: true,
				TryHeads: []string{"第1次", "第2次", "第3次"},
				Entries:  []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}},
				Rounds: []Round{{
					Label:  "决赛",
					ByHeat: false,
					Heats:  []Heat{{Label: "第 1 组", Count: "2 人", Started: "10:00", Lines: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}, {Position: 2, Bib: "102", Name: "李四", Unit: "二班"}}}},
					Standings: []Line{
						{Rank: "1", Bib: "101", Name: "张三", Unit: "一班", Attempts: []string{"5.10", "5.32", "X"}, Best: 1, Mark: "5.32", Heat: "第 1 组"},
						{Rank: "2", Bib: "102", Name: "李四", Unit: "二班", Attempts: []string{"4.90", "", ""}, Best: 0, Mark: "4.90", Heat: "第 1 组"},
					},
				}},
			},
			{
				Name: "男子跳高", Meta: "2026-11-12 周四", Count: "1 人", NameLabel: "姓名", PositionLabel: "顺序", Bars: true,
				Rounds: []Round{{
					Label:  "决赛",
					ByHeat: true,
					Heats: []Heat{{Label: "第 1 组", Count: "1 人", Lines: []Line{
						{Position: 1, Rank: "1", Bib: "101", Name: "张三", Unit: "一班", Bars: []string{"1.50 O", "1.55 XO"}, Mark: "1.55", Best: -1},
					}}},
				}},
			},
			{
				Name: "男子4×100m接力", Meta: "2026-11-13 周五 · 分道", Count: "2 队", Team: true, NameLabel: "棒次", PositionLabel: "道次", Wide: true,
				Entries: []Line{{Position: 1, Name: "", Unit: "一班", Members: []string{"1棒 张三 101", "2棒 李四 102"}}},
				Rounds: []Round{{
					Label:  "决赛",
					ByHeat: true,
					Heats: []Heat{{Label: "第 1 组", Count: "1 队", Lines: []Line{
						{Position: 1, Rank: "1", Unit: "一班", Members: []string{"1棒 张三 101", "2棒 李四 102"}, Mark: "52.10", Best: -1},
					}}},
				}},
			},
		},
		Combineds: []Combined{{
			Name: "男子三项全能", State: "已完成",
			Components: []CombinedPart{{No: 1, Name: "100m", Time: "2026-11-12 08:30"}, {No: 2, Name: "跳远"}},
			Roster:     []Line{{Bib: "101", Name: "张三", Unit: "一班"}},
			Tables: []CombinedTable{{
				Headers: []string{"100m", "跳远"},
				Rows: []CombinedRow{{Name: "张三", Bib: "101", Unit: "一班", Rank: "1", Total: "1500",
					Cells: []CombinedCell{{Mark: "12.31", Points: "800"}, {Mark: "5.32", Points: "700"}}}},
			}},
		}},
		TeamScores: &TeamScoreTable{
			State: "已完成", Scope: "全部项目", Counted: 4, Total: 5, Targets: []string{"男子100m", "男子跳远"}, Mixed: true,
			Rows: []TeamScoreRow{{Rank: "1", Unit: "初一（1）班", Male: "16", Female: "9", Mixed: "2", Total: "27"}},
		},
	}
}

// SampleCheckInBook is a fixed 检录单 data set: an individual heat and a relay.
func SampleCheckInBook() CheckInBook {
	return CheckInBook{
		Meet: "第一届校园运动会", Dates: "2026-11-12 – 2026-11-13", Location: "第一中学田径场", PrintedAt: "2026-11-12 09:31", ShowCheckIn: true,
		Sheets: []CheckInSheet{
			{Break: false, Event: "男子100m", Round: "预赛", Heat: "第 1 组", Time: "2026-11-12 09:00", CheckIn: "08:40", PositionLabel: "道次", NameLabel: "姓名",
				From: "", Lines: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}, {Position: 2, Bib: "102", Name: "李四", Unit: "二班"}}},
			{Break: true, Event: "男子4×100m接力", Round: "决赛", Heat: "第 1 组", Time: "2026-11-13 09:00", CheckIn: "08:40", From: "预赛", PositionLabel: "道次", Team: true, NameLabel: "棒次",
				Lines: []Line{{Position: 1, Unit: "一班", Members: []string{"1棒 张三 101", "2棒 李四 102"}}}},
		},
	}
}

// SampleRecordBook is a fixed 成绩记录表 data set: a sprint with its heat's
// wind, and a jump with attempts and wind.
func SampleRecordBook() RecordBook {
	return RecordBook{
		Meet: "第一届校园运动会", Dates: "2026-11-12 – 2026-11-13", Location: "第一中学田径场", PrintedAt: "2026-11-12 09:31",
		Sheets: []RecordSheet{
			{Event: "男子100m", Round: "决赛", Heat: "第 1 组", Time: "2026-11-12 09:00", HeatWind: true, PositionLabel: "道次", NameLabel: "姓名", MarkLabel: "成绩",
				Lines: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}}},
			{Break: true, Event: "男子跳远", Round: "决赛", Heat: "第 1 组", Time: "2026-11-12 10:00", PositionLabel: "顺序", NameLabel: "姓名",
				Columns: []string{"第1次", "第2次", "第3次"}, AttemptWind: true, MarkLabel: "最好成绩",
				Lines: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}}},
			{Break: true, Event: "男子跳高", Round: "决赛", Heat: "第 1 组", PositionLabel: "顺序", NameLabel: "姓名",
				Columns: []string{"", "", ""}, ColumnsTitle: "高度", MarkLabel: "最好成绩",
				Lines: []Line{{Position: 1, Bib: "101", Name: "张三", Unit: "一班"}}},
		},
	}
}
