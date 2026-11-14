package entryform

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBuildAndParse(t *testing.T) {
	data, err := Build(Spec{Meet: "秋季运动会", Unit: "高一(2)班", Note: "每人限报 2 项",
		Events: []string{"男子100m", "男子跳远", "女子100m"},
		Teams:  []TeamSpec{{Event: "男子4×100m", Size: 4, Count: 1}, {Event: "两人三足", Size: 2, Count: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	blank, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if blank.Meet != "秋季运动会" || blank.Unit != "高一(2)班" || len(blank.Athletes) != 0 || len(blank.Teams) != 0 {
		t.Fatalf("blank form = %+v", blank)
	}

	// Fill it in the way a class would.
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for cell, v := range map[string]string{
		"A4": "张三", "B4": "男", "D4": "√", "E4": "√",
		"A6": "李四", "B6": "女", "C6": "2001", "F6": "✓",
		"A7": "王五", "B7": "男", "D7": "x",
	} {
		if err := f.SetCellValue(SheetAthletes, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	for cell, v := range map[string]string{"B2": "张三", "C2": "王五", "B4": "李四", "C4": "2001"} {
		if err := f.SetCellValue(SheetTeams, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	want := []FormAthlete{
		{Line: 4, Name: "张三", Gender: "男", Events: []string{"男子100m", "男子跳远"}},
		{Line: 6, Name: "李四", Gender: "女", StudentNumber: "2001", Events: []string{"女子100m"}},
		{Line: 7, Name: "王五", Gender: "男", Events: []string{}, Problem: "「男子100m」只能填 √"},
	}
	if !reflect.DeepEqual(got.Athletes, want) {
		t.Fatalf("athletes = %+v", got.Athletes)
	}
	wantTeams := []FormTeam{{Line: 2, Event: "男子4×100m", Members: []string{"张三", "王五"}}, {Line: 4, Event: "两人三足", Members: []string{"李四", "2001"}}}
	if !reflect.DeepEqual(got.Teams, wantTeams) {
		t.Fatalf("teams = %+v", got.Teams)
	}
}

func TestParseRejectsOtherFiles(t *testing.T) {
	if _, err := Parse([]byte("姓名,性别\n")); !errors.Is(err, ErrNotForm) {
		t.Fatalf("csv: %v", err)
	}
	f := excelize.NewFile()
	buf, _ := f.WriteToBuffer()
	if _, err := Parse(buf.Bytes()); !errors.Is(err, ErrNotForm) {
		t.Fatalf("empty workbook: %v", err)
	}
}

func TestGroupForm(t *testing.T) {
	units := []string{"高一(1)班", "高一(2)班"}
	data, err := Build(Spec{Meet: "秋季运动会", Group: "高一", Units: units,
		Events: []string{"男子100m"}, Teams: []TeamSpec{{Event: "男子4×100m", Size: 4, Count: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// Columns: 姓名 性别 学号 参赛单位 男子100m; teams: 项目 参赛单位 1..4,
	// a prefilled row per unit.
	if v, _ := f.GetCellValue(SheetTeams, "B3"); v != "高一(2)班" {
		t.Fatalf("team row unit = %q", v)
	}
	for cell, v := range map[string]string{"A4": "张三", "B4": "男", "D4": "高一(2)班", "E4": "√"} {
		if err := f.SetCellValue(SheetAthletes, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellValue(SheetTeams, "C3", "张三"); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.Group != "高一" || got.Unit != "" {
		t.Fatalf("form = %+v", got)
	}
	if want := []FormAthlete{{Line: 4, Name: "张三", Gender: "男", Unit: "高一(2)班", Events: []string{"男子100m"}}}; !reflect.DeepEqual(got.Athletes, want) {
		t.Fatalf("athletes = %+v", got.Athletes)
	}
	if want := []FormTeam{{Line: 3, Event: "男子4×100m", Unit: "高一(2)班", Members: []string{"张三"}}}; !reflect.DeepEqual(got.Teams, want) {
		t.Fatalf("teams = %+v", got.Teams)
	}

	// Units past the 255 characters a drop-down holds go to a hidden list.
	var many []string
	for i := range 40 {
		many = append(many, fmt.Sprintf("实验学校初二(%d)班", i+1))
	}
	if _, err := Build(Spec{Meet: "秋季运动会", Group: "初二", Units: many}); err != nil {
		t.Fatal(err)
	}
}
