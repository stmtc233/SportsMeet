package domain

import "strings"

// GenderIdentity is optional personal information and never determines eligibility.
type GenderIdentity string

const (
	GenderIdentityUnspecified GenderIdentity = ""
	GenderIdentityMale        GenderIdentity = "男"
	GenderIdentityFemale      GenderIdentity = "女"
	GenderIdentityNonbinary   GenderIdentity = "非二元"
	GenderIdentityOther       GenderIdentity = "其他"
	GenderIdentityUndisclosed GenderIdentity = "不愿透露"
)

func NormalizeGenderIdentity(s string) (GenderIdentity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "未填":
		return GenderIdentityUnspecified, true
	case "男", "男性", "male", "man":
		return GenderIdentityMale, true
	case "女", "女性", "female", "woman":
		return GenderIdentityFemale, true
	case "非二元", "非二元性别", "nonbinary", "non-binary":
		return GenderIdentityNonbinary, true
	case "其他", "other":
		return GenderIdentityOther, true
	case "不愿透露", "prefer not to say", "undisclosed":
		return GenderIdentityUndisclosed, true
	}
	return "", false
}

// CompetitionGenderText labels an athlete's category without interpreting identity.
func CompetitionGenderText(g CompetitionGender) string {
	switch g {
	case GenderMale:
		return "男子"
	case GenderFemale:
		return "女子"
	case GenderMixed:
		return "混合"
	default:
		return "未指定"
	}
}
