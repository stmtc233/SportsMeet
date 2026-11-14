package service

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/entryform"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func rowTexts(rows []ImportRow) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Action+":"+r.Name+":"+r.Message)
	}
	return out
}

func TestEntryForms(t *testing.T) {
	f := newFixture(t, 0, track()) // units 一班 二班; 男子100m with no group
	g1, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高一"})
	must(t, err)
	g2, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高二"})
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[0], UnitInput{Name: "一班", GroupIDs: []string{g1.ID}})
	must(t, err)
	var names []string
	for _, in := range []EventInput{
		{Gender: "女", Discipline: "100m", Kind: "track", LaneCount: 8},
		{Gender: "男", Discipline: "跳远", Kind: "distance", GroupID: g1.ID},
		{Gender: "男", Discipline: "跳远", Kind: "distance", GroupID: g2.ID},
		relay(),
	} {
		e, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", in)
		must(t, err)
		names = append(names, e.Name)
	}
	female100, jump1, jump2, relayName := names[0], names[1], names[2], names[3]

	files, err := f.svc.EntryForms(f.ctx, f.meet, []string{f.units[0]}, false)
	must(t, err)
	if len(files) != 1 || files[0].Name != "一班-报名登记表.xlsx" {
		t.Fatalf("files = %+v", files)
	}
	blank := ReadEntryForm(files[0].Name, files[0].Data)
	if blank.Problem != "" || blank.Unit != "一班" || blank.Meet != "秋季运动会" {
		t.Fatalf("blank = %+v", blank)
	}
	all, err := f.svc.EntryForms(f.ctx, f.meet, nil, false)
	must(t, err)
	if len(all) != 2 {
		t.Fatalf("all units = %d files", len(all))
	}

	form := EntryForm{File: "一班.xlsx", Form: entryform.Form{Meet: "秋季运动会", Unit: "一班",
		Athletes: []entryform.FormAthlete{
			{Line: 4, Name: "张三", Gender: "男", Events: []string{"男子100m", jump1}},
			{Line: 5, Name: "李四", Gender: "女", StudentNumber: "2001", Events: []string{female100}},
			{Line: 6, Name: "王五", Gender: "男", Events: []string{}},
		},
		Teams: []entryform.FormTeam{{Line: 2, Event: relayName, Members: []string{"张三", "王五"}}},
	}}
	p, err := f.svc.PreviewEntryForms(f.ctx, f.meet, []EntryForm{form})
	must(t, err)
	if p.Creates != 3 || p.Entries != 3 || p.Teams != 1 || p.Errors != 0 {
		t.Fatalf("preview = %+v", p)
	}
	if got := rowTexts(p.Forms[0].TeamRows); !reflect.DeepEqual(got, []string{"create:" + relayName + ":张三、王五"}) {
		t.Fatalf("team rows = %q", got)
	}
	_, err = f.svc.ImportEntryForms(f.ctx, ui, f.meet, []EntryForm{form})
	must(t, err)
	teams, _ := f.svc.store.ListTeams(f.ctx, store.Scope{MeetID: f.meet})
	if len(teams) != 1 || teams[0].Name != "一班" || len(teams[0].MemberIDs) != 2 {
		t.Fatalf("teams = %+v", teams)
	}
	athletes, _ := f.svc.store.ListAthletes(f.ctx, f.meet)
	if len(athletes) != 3 || athletes[0].GroupID != g1.ID || athletes[0].UnitID != f.units[0] {
		t.Fatalf("athletes = %+v", athletes)
	}

	// The same form again: the unit's athletes by name, nothing new.
	p, err = f.svc.PreviewEntryForms(f.ctx, f.meet, []EntryForm{form})
	must(t, err)
	if p.Creates != 0 || p.Updates != 3 || p.Entries != 0 || p.Teams != 0 || p.Errors != 0 {
		t.Fatalf("again = %+v", p)
	}
	if got := rowTexts(p.Forms[0].TeamRows); !reflect.DeepEqual(got, []string{"keep:" + relayName + ":"}) {
		t.Fatalf("team rows again = %q", got)
	}

	// Forms that cannot be imported, and rows in error.
	limit := domain.EntryLimits{UnitTeams: 1}
	must(t, f.svc.UpdateMeet(f.ctx, ui, f.meet, MeetInput{Name: "秋季运动会", StartDate: "2026-11-12", EntryLimits: &limit}))
	other := EntryForm{File: "二班.xlsx", Form: entryform.Form{Meet: "秋季运动会", Unit: "二班",
		Athletes: []entryform.FormAthlete{
			{Line: 4, Name: "赵六", Gender: "男", Events: []string{jump2}},
			{Line: 5, Name: "钱七", Gender: "男", Events: []string{}, Problem: "「男子100m」只能填 √"},
			{Line: 6, Name: "孙八", Gender: "男"},
			{Line: 7, Name: "周九", Gender: "男"},
		},
		Teams: []entryform.FormTeam{
			{Line: 2, Event: relayName, Members: []string{"赵六", "孙八"}},
			{Line: 3, Event: relayName, Members: []string{"周九", "张三"}},
		},
	}}
	p, err = f.svc.PreviewEntryForms(f.ctx, f.meet, []EntryForm{
		other,
		{File: "旧.xlsx", Form: entryform.Form{Meet: "春季运动会", Unit: "一班"}},
		{File: "三班.xlsx", Form: entryform.Form{Meet: "秋季运动会", Unit: "三班"}},
		{File: "二班副本.xlsx", Form: other.Form},
		ReadEntryForm("名单.xlsx", []byte("姓名,性别")),
	})
	must(t, err)
	var problems []string
	for _, r := range p.Forms[1:] {
		problems = append(problems, r.Problem)
	}
	if want := []string{"这是「春季运动会」的表", "没有参赛单位「三班」", "与「二班.xlsx」是同一单位", "不是报名登记表"}; !reflect.DeepEqual(problems, want) {
		t.Fatalf("problems = %q", problems)
	}
	got := rowTexts(p.Forms[0].Rows)
	want := []string{"create:赵六:未指定参赛组别", "error:钱七:「男子100m」只能填 √", "create:孙八:未指定参赛组别", "create:周九:未指定参赛组别"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q", got)
	}
	got = rowTexts(p.Forms[0].TeamRows)
	want = []string{"create:" + relayName + ":赵六、孙八", "error:" + relayName + ":本单位没有「张三」"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("team rows = %q", got)
	}
	if p.Errors != 6 {
		t.Fatalf("errors = %d", p.Errors)
	}
	_, err = f.svc.ImportEntryForms(f.ctx, ui, f.meet, []EntryForm{other})
	wantInvalid(t, err, "有误")

	// Two teams of one unit are numbered.
	other.Athletes = slices.Delete(other.Athletes, 1, 2)
	other.Teams[1].Members = []string{"周九"}
	p, err = f.svc.ImportEntryForms(f.ctx, ui, f.meet, []EntryForm{other})
	must(t, err)
	got = rowTexts(p.Forms[0].TeamRows)
	want = []string{"create:" + relayName + ":赵六、孙八；本单位报 2 队，限报 1 队", "create:" + relayName + ":周九；本单位报 2 队，限报 1 队"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("team rows over the limit = %q", got)
	}
	teams, _ = f.svc.store.ListTeams(f.ctx, store.Scope{MeetID: f.meet})
	var teamNames []string
	for _, tm := range teams {
		teamNames = append(teamNames, tm.Name)
	}
	if strings.Join(teamNames, "/") != "一班/二班 1队/二班 2队" {
		t.Fatalf("team names = %q", teamNames)
	}
}

func TestGroupEntryForms(t *testing.T) {
	f := newFixture(t, 0, track()) // units 一班 二班; 男子100m with no group
	_, err := f.svc.EntryForms(f.ctx, f.meet, nil, true)
	wantInvalid(t, err, "还没有组别")
	g1, err := f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高一"})
	must(t, err)
	_, err = f.svc.SaveGroup(f.ctx, ui, f.meet, "", GroupInput{Name: "高二"}) // no unit: no form
	must(t, err)
	_, err = f.svc.SaveUnit(f.ctx, ui, f.meet, f.units[0], UnitInput{Name: "一班", GroupIDs: []string{g1.ID}})
	must(t, err)
	r, err := f.svc.SaveEvent(f.ctx, ui, f.meet, "", relay())
	must(t, err)

	files, err := f.svc.EntryForms(f.ctx, f.meet, nil, true)
	must(t, err)
	if len(files) != 2 || files[0].Name != "高一-报名登记表.xlsx" || files[1].Name != "未分组-报名登记表.xlsx" {
		t.Fatalf("files = %+v", files)
	}
	blank := ReadEntryForm(files[0].Name, files[0].Data)
	if blank.Problem != "" || blank.Group != "高一" || blank.Unit != "" {
		t.Fatalf("blank = %+v", blank)
	}

	form := EntryForm{File: "高一.xlsx", Form: entryform.Form{Meet: "秋季运动会", Group: "高一",
		Athletes: []entryform.FormAthlete{
			{Line: 4, Name: "张三", Gender: "男", Unit: "一班", Events: []string{"男子100m"}},
			{Line: 5, Name: "李四", Gender: "男", Unit: "二班"},
			{Line: 6, Name: "王五", Gender: "男"},
			{Line: 7, Name: "赵六", Gender: "男", Unit: "三班"},
		},
		Teams: []entryform.FormTeam{{Line: 2, Event: r.Name, Unit: "一班", Members: []string{"张三"}}},
	}}
	p, err := f.svc.PreviewEntryForms(f.ctx, f.meet, []EntryForm{
		form,
		{File: "高一副本.xlsx", Form: form.Form},
		{File: "高三.xlsx", Form: entryform.Form{Meet: "秋季运动会", Group: "高三"}},
	})
	must(t, err)
	got := rowTexts(p.Forms[0].Rows)
	want := []string{"create:张三:", "error:李四:「二班」不属于「高一」", "error:王五:请填写参赛单位", "error:赵六:没有参赛单位「三班」"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q", got)
	}
	if u := p.Forms[0].Rows[0].Unit + "/" + p.Forms[0].TeamRows[0].Unit; u != "一班/一班" {
		t.Fatalf("row units = %q", u)
	}
	if p.Forms[1].Problem != "与「高一.xlsx」是同一组别" || p.Forms[2].Problem != "没有组别「高三」" {
		t.Fatalf("problems = %q, %q", p.Forms[1].Problem, p.Forms[2].Problem)
	}

	// The same unit's athlete in a unit's form too: one name, two rows.
	p, err = f.svc.PreviewEntryForms(f.ctx, f.meet, []EntryForm{form, {File: "一班.xlsx", Form: entryform.Form{Meet: "秋季运动会", Unit: "一班",
		Athletes: []entryform.FormAthlete{{Line: 4, Name: "张三", Gender: "男"}}}}})
	must(t, err)
	if got := rowTexts(p.Forms[1].Rows); !reflect.DeepEqual(got, []string{"error:张三:同一单位有两行同名，请填写学号"}) {
		t.Fatalf("same name = %q", got)
	}

	form.Athletes = form.Athletes[:1]
	none := EntryForm{File: "未分组.xlsx", Form: entryform.Form{Meet: "秋季运动会", Group: entryform.NoGroup,
		Athletes: []entryform.FormAthlete{{Line: 4, Name: "李四", Gender: "男", Unit: "二班", Events: []string{"男子100m"}}}}}
	p, err = f.svc.ImportEntryForms(f.ctx, ui, f.meet, []EntryForm{form, none})
	must(t, err)
	if p.Creates != 2 || p.Entries != 2 || p.Teams != 1 {
		t.Fatalf("import = %+v", p)
	}
	athletes, _ := f.svc.store.ListAthletes(f.ctx, f.meet)
	groupOf := map[string]string{}
	for _, a := range athletes {
		groupOf[a.Name] = a.GroupID
	}
	if groupOf["张三"] != g1.ID || groupOf["李四"] != "" {
		t.Fatalf("groups = %v", groupOf)
	}
}
