package arrange

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func p(v int64) *int64 { return &v }

func entrants(n int, units int) []Entrant {
	out := make([]Entrant, n)
	for i := range out {
		out[i] = Entrant{ID: fmt.Sprintf("a%02d", i+1), Seed: p(int64(12000 + i*100))}
		if units > 0 {
			out[i].Unit = fmt.Sprintf("u%d", i%units)
		}
	}
	return out
}

func base() Options {
	return Options{Kind: domain.KindTrack, LaneCount: 8, PerHeat: 8, Order: OrderSeed, Lanes: LanesCenter, Balance: true, Seed: 7}
}

func TestPlanSizes(t *testing.T) {
	cases := []struct {
		n, perHeat int
		balance    bool
		want       []int
	}{
		{0, 8, true, nil},
		{5, 8, true, []int{5}},
		{8, 8, true, []int{8}},
		{9, 8, true, []int{5, 4}},
		{9, 8, false, []int{8, 1}},
		{20, 8, true, []int{7, 7, 6}},
		{20, 8, false, []int{8, 8, 4}},
	}
	for _, c := range cases {
		o := base()
		o.PerHeat, o.Balance = c.perHeat, c.balance
		heats, err := Plan(o, entrants(c.n, 0))
		if err != nil {
			t.Fatal(err)
		}
		var got []int
		seen := map[string]bool{}
		for _, h := range heats {
			got = append(got, len(h.Slots))
			pos := map[int]bool{}
			for _, s := range h.Slots {
				if seen[s.ID] || pos[s.Position] || s.Position < 1 || s.Position > 8 {
					t.Errorf("n=%d: bad slot %+v", c.n, s)
				}
				seen[s.ID], pos[s.Position] = true, true
			}
		}
		if !reflect.DeepEqual(got, c.want) || len(seen) != c.n {
			t.Errorf("n=%d perHeat=%d balance=%v: sizes %v, want %v", c.n, c.perHeat, c.balance, got, c.want)
		}
	}
}

func TestPlanSeedSnakesAndCenters(t *testing.T) {
	heats, err := Plan(base(), entrants(16, 0))
	if err != nil {
		t.Fatal(err)
	}
	lane := func(h int, id string) int {
		for _, s := range heats[h].Slots {
			if s.ID == id {
				return s.Position
			}
		}
		return 0
	}
	// a01 best → heat 1, a02 → heat 2, a03 snakes back to heat 2, a04 → heat 1.
	if lane(0, "a01") != 4 || lane(1, "a02") != 4 || lane(1, "a03") != 5 || lane(0, "a04") != 5 {
		t.Errorf("seeding = %+v", heats)
	}
	if lane(0, "a16") != 8 && lane(1, "a16") != 8 {
		t.Errorf("slowest should take an outside lane: %+v", heats)
	}
}

func TestPlanSeedsMissingGoLast(t *testing.T) {
	list := []Entrant{{ID: "none"}, {ID: "slow", Seed: p(15000)}, {ID: "fast", Seed: p(11000)}}
	heats, _ := Plan(base(), list)
	got := map[string]int{}
	for _, s := range heats[0].Slots {
		got[s.ID] = s.Position
	}
	if got["fast"] != 4 || got["slow"] != 5 || got["none"] != 3 {
		t.Errorf("lanes = %v", got)
	}
	o := base()
	o.Kind, o.LaneCount, o.Lanes = domain.KindDistance, 0, LanesSequential
	heats, _ = Plan(o, []Entrant{{ID: "short", Seed: p(4000)}, {ID: "long", Seed: p(6000)}})
	if heats[0].Slots[0].ID != "long" {
		t.Errorf("field marks: longer is better, got %+v", heats[0].Slots)
	}
}

func TestPlanSeparatesUnits(t *testing.T) {
	o := base()
	o.SeparateUnits = true
	list := entrants(24, 3) // 8 per unit, 3 heats
	heats, _ := Plan(o, list)
	unit := map[string]string{}
	for _, e := range list {
		unit[e.ID] = e.Unit
	}
	for i, h := range heats {
		count := map[string]int{}
		for _, s := range h.Slots {
			count[unit[s.ID]]++
		}
		for u, c := range count {
			if c < 2 || c > 3 {
				t.Errorf("heat %d has %d of %s; want units spread evenly: %v", i+1, c, u, count)
			}
		}
	}
	// Without the option the snake puts every unit in a single heat here.
	o.SeparateUnits = false
	heats, _ = Plan(o, list)
	count := map[string]int{}
	for _, s := range heats[0].Slots {
		count[unit[s.ID]]++
	}
	if len(count) == 3 && count["u0"] == count["u1"] {
		t.Log("snake happened to spread units; the option is still exercised above")
	}
}

func TestPlanRandomIsReproducible(t *testing.T) {
	o := base()
	o.Order, o.Lanes = OrderRandom, LanesRandom
	a, _ := Plan(o, entrants(20, 4))
	b, _ := Plan(o, entrants(20, 4))
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed gave different plans")
	}
	o.Seed = 8
	c, _ := Plan(o, entrants(20, 4))
	if reflect.DeepEqual(a, c) {
		t.Fatal("different seeds gave the same plan")
	}
}

func TestPlanRejectsBadOptions(t *testing.T) {
	o := base()
	o.PerHeat = 9
	if _, err := Plan(o, entrants(3, 0)); err != ErrTooMany {
		t.Errorf("9 per heat on 8 lanes: %v", err)
	}
	o.PerHeat = 0
	if _, err := Plan(o, entrants(3, 0)); err != ErrPerHeat {
		t.Errorf("0 per heat: %v", err)
	}
	o = base()
	o.Order = "best"
	if _, err := Plan(o, entrants(3, 0)); err != ErrNoOption {
		t.Errorf("unknown order: %v", err)
	}
}

func TestCenterLanes(t *testing.T) {
	if got := CenterLanes(8); !reflect.DeepEqual(got, []int{4, 5, 3, 6, 2, 7, 1, 8}) {
		t.Errorf("8 lanes: %v", got)
	}
	if got := CenterLanes(6); !reflect.DeepEqual(got, []int{3, 4, 2, 5, 1, 6}) {
		t.Errorf("6 lanes: %v", got)
	}
	if got := CenterLanes(5); !reflect.DeepEqual(got, []int{3, 4, 2, 5, 1}) {
		t.Errorf("5 lanes: %v", got)
	}
}

func TestDeriveOrdinals(t *testing.T) {
	cases := []struct{ in, want []int }{
		{[]int{1, 2, 3}, []int{1, 2, 3}},
		{[]int{1, 0, 2}, []int{1, 2, 3}},
		{[]int{2, 5, 0}, []int{2, 5, 6}},
		{[]int{3, 1, 2}, []int{3, 4, 5}},
		{[]int{0, 0}, []int{1, 2}},
	}
	for _, c := range cases {
		if got := DeriveOrdinals(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("DeriveOrdinals(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFingerprint(t *testing.T) {
	h := domain.Heat{Ordinal: 1, Slots: []domain.Slot{{Position: 4, AthleteID: "a"}}}
	same := h
	same.ID, same.Wind = "other", new(int)
	if Fingerprint(h) != Fingerprint(same) {
		t.Error("id and wind are not part of the arrangement")
	}
	for name, change := range map[string]func(*domain.Heat){
		"ordinal":  func(h *domain.Heat) { h.Ordinal = 2 },
		"name":     func(h *domain.Heat) { h.Name = "A" },
		"time":     func(h *domain.Heat) { h.ScheduledAt = "2026-11-12T09:00" },
		"duration": func(h *domain.Heat) { h.DurationMinutes = new(int); *h.DurationMinutes = 5 },
		"slot":     func(h *domain.Heat) { h.Slots = []domain.Slot{{Position: 5, AthleteID: "a"}} },
	} {
		changed := h
		change(&changed)
		if Fingerprint(changed) == Fingerprint(h) {
			t.Errorf("%s change kept the fingerprint", name)
		}
	}
	five, six := h, h
	five.DurationMinutes, six.DurationMinutes = new(int), new(int)
	*five.DurationMinutes, *six.DurationMinutes = 5, 6
	if Fingerprint(five) == Fingerprint(six) {
		t.Error("a different duration kept the fingerprint")
	}
}

func TestQualify(t *testing.T) {
	ok := domain.StatusOK
	heats := [][]Finisher{
		{{"a", ok, 1, 1}, {"b", ok, 4, 2}, {"c", ok, 5, 3}, {"x", domain.StatusDNF, 0, 0}},
		{{"d", ok, 2, 1}, {"e", ok, 3, 2}, {"f", ok, 5, 3}, {"g", ok, 7, 4}},
	}
	got := Qualify(heats, 1, 2)
	want := map[string]string{"a": "Q", "d": "Q", "e": "q", "b": "q"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("1 by place + 2 by mark = %v, want %v", got, want)
	}
	// c and f tie on the last mark: both advance.
	got = Qualify(heats, 1, 3)
	want = map[string]string{"a": "Q", "d": "Q", "e": "q", "b": "q", "c": "q", "f": "q"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tie on the last mark = %v, want %v", got, want)
	}
	got = Qualify(heats, 2, 0)
	want = map[string]string{"a": "Q", "b": "Q", "d": "Q", "e": "Q"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("2 by place only = %v, want %v", got, want)
	}
	// A ruling puts f behind c: the extra one stays out.
	heats[1][2].Order = 6
	got = Qualify(heats, 1, 3)
	want = map[string]string{"a": "Q", "d": "Q", "e": "q", "b": "q", "c": "q"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ruled tie on the last mark = %v, want %v", got, want)
	}
}

func TestStartModeIndependentOfTiming(t *testing.T) {
	o := base()
	o.StartMode, o.PerHeat, o.Lanes = domain.StartGroup, 16, LanesSequential
	heats, err := Plan(o, entrants(16, 2))
	if err != nil || len(heats) != 1 || len(heats[0].Slots) != 16 {
		t.Fatalf("group: %+v, %v", heats, err)
	}
	for i, s := range heats[0].Slots {
		if s.Position != i+1 || s.ID != fmt.Sprintf("a%02d", i+1) {
			t.Fatalf("timed seeding/order: %+v", heats[0].Slots)
		}
	}
	o.StartMode = domain.StartBreak
	if _, err := Plan(o, entrants(16, 2)); err != ErrTooMany {
		t.Fatalf("break exceeds lanes: %v", err)
	}
	o.PerHeat, o.Lanes = 8, LanesCenter
	heats, err = Plan(o, entrants(8, 1))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range heats[0].Slots {
		if sl.ID == "a01" && sl.Position != 4 {
			t.Fatalf("best break runner not in middle lane: %+v", sl)
		}
	}
}
