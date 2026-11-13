package scoring

import (
	"math/big"
	"sort"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// ScoreValue carries the exact rational for machine consumers and a lossless
// display: terminating decimals, or a fraction when the decimal repeats.
type ScoreValue struct {
	Exact string `json:"exact"`
	Text  string `json:"text"`
}

func scoreValue(r *big.Rat) ScoreValue {
	d := new(big.Int).Set(r.Denom())
	places := 0
	for _, factor := range []int64{2, 5} {
		n := 0
		f := big.NewInt(factor)
		for new(big.Int).Mod(d, f).Sign() == 0 {
			d.Div(d, f)
			n++
		}
		places = max(places, n)
	}
	text := r.RatString()
	if d.Cmp(big.NewInt(1)) == 0 {
		text = r.FloatString(places)
		if places > 0 {
			text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
		}
	}
	return ScoreValue{Exact: r.RatString(), Text: text}
}

type TeamScoreCompetitor struct {
	CompetitorID string `json:"competitorId"`
	AthleteID    string `json:"athleteId"`
	TeamID       string `json:"teamId"`
	UnitID       string `json:"unitId"`
	Name         string `json:"name"`
	MarkText     string `json:"markText"`
	Rank         *int   `json:"rank"`
}

type TeamScoreSource struct {
	EventID     string                `json:"eventId"`
	CombinedID  string                `json:"combinedId"`
	Name        string                `json:"name"`
	Gender      domain.Gender         `json:"gender"`
	GroupLabel  string                `json:"groupLabel"`
	Complete    bool                  `json:"complete"`
	Multiplier  int64                 `json:"multiplier"`
	State       string                `json:"state"`
	Competitors []TeamScoreCompetitor `json:"-"`
}

type TeamScoreLine struct {
	TeamScoreCompetitor
	EventID    string        `json:"eventId"`
	CombinedID string        `json:"combinedId"`
	EventName  string        `json:"eventName"`
	Gender     domain.Gender `json:"gender"`
	GroupLabel string        `json:"groupLabel"`
	Base       ScoreValue    `json:"base"`
	Multiplier int64         `json:"multiplier"`
	Score      ScoreValue    `json:"score"`
}

type UnitStanding struct {
	UnitID   string          `json:"unitId"`
	UnitName string          `json:"unitName"`
	Rank     *int            `json:"rank"`
	Male     ScoreValue      `json:"male"`
	Female   ScoreValue      `json:"female"`
	Mixed    ScoreValue      `json:"mixed"`
	Total    ScoreValue      `json:"total"`
	Lines    []TeamScoreLine `json:"lines"`
}

type TeamScores struct {
	Rows       []UnitStanding    `json:"rows"`
	Sources    []TeamScoreSource `json:"sources"`
	Unassigned []TeamScoreLine   `json:"unassigned"`
	Counted    int               `json:"counted"`
	Total      int               `json:"total"`
	Complete   bool              `json:"complete"`
	State      string            `json:"state"`
}

// TeamRank consumes final rankings, including competitors without a unit.
// Those competitors occupy places and participate in ties but earn no unit points.
// Unit totals tie directly; no countback or intermediate rounding is performed.
func TeamRank(rules domain.TeamScoringRules, units []domain.Unit, sources []TeamScoreSource) TeamScores {
	out := TeamScores{Rows: []UnitStanding{}, Sources: []TeamScoreSource{}, Unassigned: []TeamScoreLine{}, State: "未启用"}
	type total struct {
		row     UnitStanding
		genders map[domain.Gender]*big.Rat
		sum     *big.Rat
	}
	rows := make([]*total, 0, len(units))
	byUnit := map[string]*total{}
	for _, u := range units {
		t := &total{row: UnitStanding{UnitID: u.ID, UnitName: u.Name, Lines: []TeamScoreLine{}}, sum: new(big.Rat), genders: map[domain.Gender]*big.Rat{domain.GenderMale: {}, domain.GenderFemale: {}, domain.GenderMixed: {}}}
		rows = append(rows, t)
		byUnit[u.ID] = t
	}
	for _, src := range sources {
		src.State = "不计分"
		if src.Multiplier > 0 && rules.Enabled {
			out.Total++
			src.State = "待计分"
			if src.Complete {
				out.Counted++
				src.State = "已计分"
			}
		}
		out.Sources = append(out.Sources, src)
		if src.State != "已计分" {
			continue
		}
		ties := map[int]int{}
		for _, c := range src.Competitors {
			if c.Rank != nil {
				ties[*c.Rank]++
			}
		}
		for _, c := range src.Competitors {
			if c.Rank == nil || *c.Rank < 1 {
				continue
			}
			place := *c.Rank
			base := new(big.Rat)
			count := 1
			if rules.TiePolicy == "average" {
				count = ties[place]
			}
			for i := 0; i < count && place-1+i < len(rules.Places); i++ {
				base.Add(base, new(big.Rat).SetInt64(rules.Places[place-1+i]))
			}
			base.Quo(base, new(big.Rat).SetInt64(int64(count)))
			points := new(big.Rat).Mul(base, new(big.Rat).SetInt64(src.Multiplier))
			line := TeamScoreLine{TeamScoreCompetitor: c, EventID: src.EventID, CombinedID: src.CombinedID, EventName: src.Name, Gender: src.Gender, GroupLabel: src.GroupLabel, Base: scoreValue(base), Multiplier: src.Multiplier, Score: scoreValue(points)}
			if row := byUnit[c.UnitID]; row != nil {
				row.sum.Add(row.sum, points)
				row.genders[src.Gender].Add(row.genders[src.Gender], points)
				row.row.Lines = append(row.row.Lines, line)
			} else {
				out.Unassigned = append(out.Unassigned, line)
			}
		}
	}
	out.Complete = rules.Enabled && out.Total > 0 && out.Counted == out.Total
	if rules.Enabled {
		out.State = "暂列排名"
	}
	if out.Complete {
		out.State = "正式排名"
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].sum.Cmp(rows[j].sum) > 0 })
	rank := 0
	for i, r := range rows {
		if i == 0 || r.sum.Cmp(rows[i-1].sum) != 0 {
			rank = i + 1
		}
		if out.Counted > 0 {
			p := rank
			r.row.Rank = &p
		}
		r.row.Male = scoreValue(r.genders[domain.GenderMale])
		r.row.Female = scoreValue(r.genders[domain.GenderFemale])
		r.row.Mixed = scoreValue(r.genders[domain.GenderMixed])
		r.row.Total = scoreValue(r.sum)
		out.Rows = append(out.Rows, r.row)
	}
	return out
}
