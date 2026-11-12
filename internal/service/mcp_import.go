package service

import (
	"fmt"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/interchange"
)

// MCP imports reject explicit gender mixing. The general interchange reader
// keeps its warning-only behavior for files reviewed directly in the desktop UI.
func mcpImportErrors(data []byte) []string {
	f, err := interchange.Decode(data)
	if err != nil {
		return nil
	} // The normal importer reports decoding errors in full.
	genders, names := map[string]string{}, map[string]string{}
	for _, a := range f.Athletes {
		genders[a.ID] = a.Gender
		names[a.ID] = a.Name
	}
	errors := []string{}
	for _, e := range f.Events {
		if e.Kind == "relay" {
			continue
		}
		opposite := "女子"
		if e.Gender == "女" {
			opposite = "男子"
		}
		if e.Gender != "混合" && strings.Contains(e.Name, opposite) {
			errors = append(errors, fmt.Sprintf("项目「%s」的名称与性别不符", e.Name))
		}
		for _, en := range e.Entries {
			if g := genders[en.AthleteID]; g != "" && e.Gender != "混合" && g != e.Gender {
				errors = append(errors, fmt.Sprintf("运动员「%s」的性别与项目「%s」不符", names[en.AthleteID], e.Discipline))
			}
		}
		for _, r := range e.Rounds {
			for _, h := range r.Heats {
				if e.Gender != "混合" && strings.Contains(h.Name, opposite) {
					errors = append(errors, fmt.Sprintf("组「%s」的名称与项目性别不符", h.Name))
				}
			}
		}
	}
	return errors
}
