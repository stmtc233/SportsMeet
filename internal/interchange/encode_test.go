package interchange

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeKeepsJSONFormatting(t *testing.T) {
	f := &File{Format: Format, Version: Version, Meet: Meet{Name: "<赛事> & \"名称\"", Note: strings.Repeat("{},[]:\\\"中\n", 200)},
		Units: []Unit{{ID: "u", Name: "单位"}}, Athletes: []Athlete{{ID: "a", Name: "运动员"}},
		Events: []Event{{ID: "e", Rounds: []Round{{ID: "r", Heats: []Heat{{ID: "h", Slots: []Slot{{Position: 1, AthleteID: "a"}}}}}}}}}
	for _, input := range []*File{nil, {}, {Combineds: []Combined{}}, {Combineds: []Combined{{ID: "c"}}}, f} {
		var expected bytes.Buffer
		enc := json.NewEncoder(&expected)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(input); err != nil {
			t.Fatal(err)
		}
		got, err := Encode(input)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, expected.Bytes()) {
			t.Fatal("encoded bytes differ from the standard encoder")
		}
	}
}
