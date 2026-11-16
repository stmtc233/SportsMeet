// Package examplemeet builds the offline, fictional meet shipped with the app.
// Fixed inputs make exports reproducible; it never reads or writes user data.
package examplemeet

import (
	"fmt"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

const Name = "青禾市校园田径运动会（示例）"

var days = [...]string{"2026-10-22", "2026-10-23", "2026-10-24"}

func ptr[T any](v T) *T { return &v }

func athleteID(group, gender, unit, person int) string {
	return fmt.Sprintf("a-%d-%d-%02d-%02d", group, gender, unit, person)
}

// Build returns a fresh, complete snapshot: 1,152 athletes, 93 events
// (including 18 combined components), six combineds and 180 teams.
func Build() *interchange.File {
	rules := domain.DefaultTeamScoringRules()
	rules.Enabled = true
	f := &interchange.File{
		Format: interchange.Format, Version: interchange.Version,
		ExportedAt: "2026-10-24T18:00:00+08:00",
		Generator:  interchange.Generator{Name: "SportsMeet 示例赛事", Version: "1"},
		Meet: interchange.Meet{ID: "example-meet", Name: Name, StartDate: days[0], EndDate: ptr(days[2]),
			Location: "青禾市校园体育中心", Note: "虚构示例，人物、单位与成绩均为演示数据。",
			EntryLimits: &domain.EntryLimits{AthleteEvents: 2, UnitAthletes: 4, UnitTeams: 1}, TeamScoring: &rules},
	}
	for g, name := range []string{"初一组", "初二组", "初三组"} {
		f.Groups = append(f.Groups, interchange.Group{ID: fmt.Sprintf("g%d", g), Name: name, Ordinal: g + 1})
	}
	for u, name := range []string{"青禾一中", "青禾二中", "青禾三中", "青禾四中", "青禾五中", "青禾六中", "青禾七中", "青禾八中", "青禾九中", "青禾十中", "青禾实验中学", "青禾外国语学校"} {
		f.Units = append(f.Units, interchange.Unit{ID: fmt.Sprintf("u%d", u), Name: name, ShortName: name,
			GroupIDs: []string{"g0", "g1", "g2"}})
	}
	surnames := []rune("陈林黄张李王吴刘周徐孙赵朱郑许何高罗梁宋谢唐韩曹")
	first := []string{"子", "思", "雨", "嘉", "文", "明", "安", "若", "梓", "景", "亦", "星"}
	last := [][]string{{"轩", "辰", "泽", "远", "航", "川", "阳", "朗", "霖", "宇", "然", "宁", "诚", "哲", "睿", "清"}, {"涵", "悦", "桐", "晴", "妍", "瑶", "宁", "萱", "彤", "琳", "然", "禾", "琪", "岚", "欣", "伊"}}
	for g := range 3 {
		for sex, gender := range []string{"男", "女"} {
			for u, unit := range f.Units {
				for p := range 16 {
					n := len(f.Athletes) + 1
					f.Athletes = append(f.Athletes, interchange.Athlete{
						ID: athleteID(g, sex, u, p), Name: string(surnames[(u*7+p+g*3)%len(surnames)]) + first[(u+g*4)%len(first)] + last[sex][p],
						CompetitionGender: gender, Bib: fmt.Sprintf("%04d", n), StudentNumber: fmt.Sprintf("DEMO%06d", n),
						UnitID: ptr(unit.ID), Affiliation: unit.Name, GroupID: f.Groups[g].ID,
						Grade: []string{"初一", "初二", "初三"}[g], ClassName: fmt.Sprintf("%d班", p/4+1),
					})
				}
			}
			category := g*2 + sex
			for _, spec := range disciplines(category) {
				e := makeEvent(f, g, sex, spec)
				finishEvent(&e, spec, category)
				f.Events = append(f.Events, e)
			}
			addCombined(f, g, sex)
		}
		// Two boys and two girls, in saved relay leg order.
		spec := discipline{name: "4×100m混合接力", kind: "relay", start: "lanes", team: true, base: 53000,
			stages: []roundPlan{{day: 2, minute: 900 + g*12, duration: 4}}}
		e := makeEvent(f, g, 2, spec)
		finishEvent(&e, spec, g)
		f.Events = append(f.Events, e)
	}
	f.FrontPages = []interchange.FrontPage{
		{Title: "竞赛规程", Reports: []string{"entries", "startlist"}, BodyMD: `# 青禾市校园田径运动会

## 参赛办法

设初一、初二、初三三个组别，男子、女子分别比赛，混合接力按组别比赛。每人限报两个个人项目，全能计一项，接力与集体项目不占个人项数。每单位每项限报四人、队伍项目限报一队。

## 录取与计分

个人项目录取前八名，依次计 9、7、6、5、4、3、2、1 分。接力加倍，全能按总排名计分一次，分项不重复计分。名次并列时均分所占名次的分值。各组团体总分分别排名，同分并列。

100m设预赛、半决赛、决赛，200m、400m和男子、女子接力设预赛、决赛，其余直接决赛。晋级人数以各轮次保存的规则为准。

## 全能计分

校园三项全能依次进行100m、跳远、800m。积分采用本示例保存的分档表，仅用于演示。DNS、DNF、DQ不列总排名，NM记零分。

本文所列人物、单位与成绩均为虚构。`},
		{Title: "参赛须知", Reports: []string{"startlist"}, BodyMD: `# 参赛须知

1. 按竞赛日程到指定场地参赛，提前20分钟检录。
2. 佩戴本人号码布，接力按秩序册列出的棒次参赛。
3. 径赛在主田径场进行；跳远、跳高、铅球在对应田赛区域进行；跳绳与集体技巧在综合活动场进行。
4. 远度项目每人三次尝试；高度逐次记录试跳；计次项目取两次中较好成绩；集体技巧取两轮分数之和。
5. 现场成绩由裁判确认后公布。`},
	}
	return f
}

func makeEvent(f *interchange.File, group, sex int, spec discipline) interchange.Event {
	gender := []string{"男", "女", "混合"}[sex]
	e := interchange.Event{ID: fmt.Sprintf("e%d", len(f.Events)+1), Gender: gender, Discipline: spec.name,
		GroupID: f.Groups[group].ID, GroupLabel: f.Groups[group].Name, Kind: spec.kind,
		StartMode: spec.start, DistanceMode: spec.distance, Entries: []interchange.Entry{}}
	e.Name = domain.ComposeEventName(domain.CompetitionGender(gender), e.Discipline, e.GroupLabel)
	if spec.start != "" {
		e.LaneCount = ptr(8)
	}
	if spec.kind == "distance" {
		e.AttemptCount = ptr(3)
	}
	if spec.kind == "count" {
		e.AttemptCount = ptr(2)
		e.AttemptMode = "best"
	}
	if spec.kind == "score" {
		e.AttemptCount = ptr(2)
		e.ScoreDecimals, e.AttemptMode = 2, "sum"
	}
	if spec.team {
		e.ParticipantMode = "team"
		if spec.kind == "relay" {
			e.RelayLegCount = 4
		} else {
			e.TeamSize = 4
		}
		for u, unit := range f.Units {
			team := interchange.Team{ID: fmt.Sprintf("%s-t%d", e.ID, u), Name: unit.Name, UnitID: ptr(unit.ID)}
			for leg := range 4 {
				memberSex, person := sex, spec.from+leg
				if sex == 2 {
					memberSex, person = leg%2, 4+leg/2
				}
				team.MemberIDs = append(team.MemberIDs, athleteID(group, memberSex, u, person))
			}
			e.Teams = append(e.Teams, team)
			e.Entries = append(e.Entries, interchange.Entry{TeamID: team.ID, SeedMark: ptr(spec.base + int64(u)*370)})
		}
	} else {
		// Interleave units so a heat is not a block of one school's athletes.
		for p := spec.from; p < spec.to; p++ {
			for u := range f.Units {
				e.Entries = append(e.Entries, interchange.Entry{AthleteID: athleteID(group, sex, u, p), SeedMark: ptr(spec.base + int64(u+p)*10)})
			}
		}
	}
	return e
}
