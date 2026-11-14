package service

import (
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestEventProblem(t *testing.T) {
	events := []domain.Event{
		{ID: "a", Name: "男子100m 初一组", Gender: domain.GenderMale, Discipline: "100m", GroupID: "g1"},
		{ID: "b", Name: "男子100m 初二组", Gender: domain.GenderMale, Discipline: "100m", GroupID: "g2"},
	}
	for _, c := range []struct {
		token, group, groupName string
		gender                  domain.Gender
		want                    string
	}{
		{"跳远", "", "", domain.GenderMale, "本赛事没有项目「跳远」"},
		{"100m", "", "", domain.GenderFemale, "「100m」没有女子项目"},
		{"100m", "g3", "初三组", domain.GenderMale, "「100m」没有初三组的项目"},
		{"100m", "", "", domain.GenderMale, "「100m」有多个项目，请填写组别"},
	} {
		_, n := findEvent(events, c.token, c.gender, c.group, true)
		if got := eventProblem(events, c.token, n, c.gender, c.group, c.groupName, true, true); got != c.want {
			t.Errorf("%s %s %s: %s", c.token, c.gender, c.groupName, got)
		}
	}
}

// Events with no group take athletes of any group.
func TestFindEventOpenToGroups(t *testing.T) {
	events := []domain.Event{
		{ID: "open", Name: "男子100m", Gender: domain.GenderMale, Discipline: "100m"},
		{ID: "g2", Name: "男子100m 高二", Gender: domain.GenderMale, Discipline: "100m", GroupID: "g2"},
		{ID: "jump", Name: "男子跳远", Gender: domain.GenderMale, Discipline: "跳远"},
	}
	for _, c := range []struct{ token, group, want string }{
		{"100m", "g1", "open"}, {"100m", "g2", "g2"}, {"跳远", "g1", "jump"}, {"100m", "", ""},
	} {
		if got := matchEvent(events, c.token, domain.GenderMale, c.group); got != c.want {
			t.Errorf("%s in %q = %q, want %q", c.token, c.group, got, c.want)
		}
	}
}
