// Package arrange builds heats and slots: grouping, seeding, lane and
// order draws, ordinals, fingerprints and advancement
// (docs/agents/arrange.md). It is pure: no I/O, and the same input and
// seed always give the same plan.
package arrange

import (
	"errors"
	"math/rand/v2"
	"sort"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Entrant is one competitor to place.
type Entrant struct {
	ID   string
	Unit string // "" = none; used to keep a unit apart
	Seed *int64 // entry mark, nil = none
}

// Order of the entrants before they are dealt into heats.
const (
	OrderSeed   = "seed"   // best entry mark first, entrants without one last
	OrderRandom = "random" // shuffled by Seed
)

// Lanes decides the positions inside a heat.
const (
	LanesCenter     = "center"     // best in the middle lanes, outwards from there
	LanesSequential = "sequential" // 1, 2, 3 … in dealing order
	LanesRandom     = "random"     // shuffled by Seed
)

// Options of an automatic arrangement.
type Options struct {
	Kind          domain.EventKind `json:"kind"`
	StartMode     domain.StartMode `json:"startMode"`
	LaneCount     int              `json:"laneCount"` // starting lanes, ignored for group starts
	PerHeat       int              `json:"perHeat"`
	Order         string           `json:"order"`
	Lanes         string           `json:"lanes"`
	SeparateUnits bool             `json:"separateUnits"`
	Balance       bool             `json:"balance"`
	Seed          int64            `json:"seed"`
}

// PlannedSlot is one position of a planned heat.
type PlannedSlot struct {
	Position int    `json:"position"`
	ID       string `json:"competitorId"`
}

// PlannedHeat is one new heat.
type PlannedHeat struct {
	Slots []PlannedSlot `json:"slots"`
}

// Errors of Plan, shown to the user as they are.
var (
	ErrPerHeat  = errors.New("每组人数至少为 1")
	ErrTooMany  = errors.New("每组人数超过道数")
	ErrNoOption = errors.New("排列顺序或位置分配不正确")
)

// Plan deals entrants into new heats.
//
// Heats: ceil(n / PerHeat) of them; with Balance the sizes differ by at
// most one, otherwise every heat is full but the last. Entrants are dealt
// in rows of one per heat, snaking (1→h, h→1, …) so that heats are equally
// strong when ordered by seed. With SeparateUnits each entrant of a row
// goes to the heat of that row holding the fewest of its unit.
func Plan(opts Options, entrants []Entrant) ([]PlannedHeat, error) {
	laned := (domain.Event{Kind: opts.Kind, StartMode: opts.StartMode}).UsesLanes()
	switch {
	case opts.PerHeat < 1:
		return nil, ErrPerHeat
	case laned && opts.LaneCount > 0 && opts.PerHeat > opts.LaneCount:
		return nil, ErrTooMany
	case opts.Order != OrderSeed && opts.Order != OrderRandom:
		return nil, ErrNoOption
	case opts.Lanes != LanesCenter && opts.Lanes != LanesSequential && opts.Lanes != LanesRandom:
		return nil, ErrNoOption
	}
	n := len(entrants)
	if n == 0 {
		return []PlannedHeat{}, nil
	}
	rng := rand.New(rand.NewPCG(uint64(opts.Seed), 0x5370_6f72_7473))

	list := append([]Entrant(nil), entrants...)
	rng.Shuffle(n, func(i, j int) { list[i], list[j] = list[j], list[i] })
	if opts.Order == OrderSeed {
		sort.SliceStable(list, func(i, j int) bool { return seedBefore(opts.Kind, list[i].Seed, list[j].Seed) })
	}

	h := (n + opts.PerHeat - 1) / opts.PerHeat
	size := make([]int, h)
	for i := range size {
		switch {
		case opts.Balance:
			size[i] = n / h
			if i < n%h {
				size[i]++
			}
		case i < h-1:
			size[i] = opts.PerHeat
		default:
			size[i] = n - opts.PerHeat*(h-1)
		}
	}

	heats := make([][]Entrant, h)
	units := make([]map[string]int, h)
	for i := range units {
		units[i] = map[string]int{}
	}
	used := make([]bool, h) // heats that already got one of this row
	forward := true
	for _, e := range list {
		pick := -1
		for range 2 {
			for k := range h {
				i := k
				if !forward {
					i = h - 1 - k
				}
				if used[i] || len(heats[i]) >= size[i] {
					continue
				}
				if pick < 0 || (opts.SeparateUnits && e.Unit != "" && units[i][e.Unit] < units[pick][e.Unit]) {
					pick = i
				}
				if !opts.SeparateUnits || e.Unit == "" {
					break
				}
			}
			if pick >= 0 {
				break
			}
			clear(used) // row done: start the next one from the other end
			forward = !forward
		}
		heats[pick] = append(heats[pick], e)
		used[pick] = true
		if e.Unit != "" {
			units[pick][e.Unit]++
		}
	}

	out := make([]PlannedHeat, h)
	for i, members := range heats {
		positions := lanes(opts, laned, len(members), rng)
		slots := make([]PlannedSlot, len(members))
		for j, e := range members {
			slots[j] = PlannedSlot{Position: positions[j], ID: e.ID}
		}
		sort.Slice(slots, func(a, b int) bool { return slots[a].Position < slots[b].Position })
		out[i] = PlannedHeat{Slots: slots}
	}
	return out, nil
}

// seedBefore orders entry marks best first; a missing mark comes last.
func seedBefore(kind domain.EventKind, a, b *int64) bool {
	switch {
	case a == nil:
		return false
	case b == nil:
		return true
	case kind.Timed():
		return *a < *b
	}
	return *a > *b
}

// lanes returns the positions of a heat's members in dealing order.
func lanes(opts Options, laned bool, n int, rng *rand.Rand) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	if laned && opts.LaneCount >= n && opts.Lanes != LanesSequential {
		copy(out, CenterLanes(opts.LaneCount)[:n]) // random draws stay in the middle lanes too
	}
	if opts.Lanes == LanesRandom {
		rng.Shuffle(n, func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	return out
}

// CenterLanes lists the lanes from the middle outwards: 4, 5, 3, 6, 2, 7,
// 1, 8 on an eight-lane track.
func CenterLanes(count int) []int {
	out := make([]int, 0, count)
	lo := (count + 1) / 2
	hi := lo + 1
	for len(out) < count {
		if lo >= 1 {
			out = append(out, lo)
			lo--
		}
		if hi <= count && len(out) < count {
			out = append(out, hi)
			hi++
		}
	}
	return out
}
