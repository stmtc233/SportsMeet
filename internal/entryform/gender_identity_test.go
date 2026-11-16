package entryform

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestEntryFormCompetitionCategoryAndLegacyHeader(t *testing.T) {
	data, err := Build(Spec{Meet: "校运会", Unit: "一班", Events: []string{"女子100m"}})
	if err != nil {
		t.Fatal(err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	label, err := workbook.GetCellValue(SheetAthletes, "B3")
	if err != nil || label != "参赛类别" {
		t.Fatal("unexpected new header", label, err)
	}
	// Simulate an untouched published old form; identity must stay unfilled.
	for cell, value := range map[string]string{"A4": "甲", "B3": "性别", "B4": "女", "D4": "√"} {
		if err := workbook.SetCellValue(SheetAthletes, cell, value); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	form, err := Parse(encoded.Bytes())
	if err != nil || len(form.Athletes) != 1 {
		t.Fatal(form, err)
	}
	if form.Athletes[0].CompetitionGender != "女" || form.Athletes[0].GenderIdentity != "" || len(form.Athletes[0].Events) != 1 {
		t.Fatal(form.Athletes)
	}
	// An explicitly supplied identity column is data, never an event column.
	for cell, value := range map[string]string{"B3": "参赛类别", "B4": "女子", "E3": "性别认同", "E4": "非二元"} {
		if err := workbook.SetCellValue(SheetAthletes, cell, value); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err = workbook.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	form, err = Parse(encoded.Bytes())
	if err != nil || form.Athletes[0].Problem != "" || form.Athletes[0].GenderIdentity != "非二元" || len(form.Athletes[0].Events) != 1 {
		t.Fatal(form, err)
	}
}
