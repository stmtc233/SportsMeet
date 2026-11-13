package arrange

import "math"

// deal preserves the snake's exact choices: the least represented unit in an
// available heat, ties in the row's direction. Frequent units use minimum
// trees; a rare unit examines at most its own occurrences before finding zero.
// A sqrt(n) threshold bounds the number of frequent units by O(sqrt(n)).
func deal(list []Entrant, size []int, separate bool) [][]Entrant {
	n, h := len(list), len(size)
	heats := make([][]Entrant, h)
	frequency := map[string]int{}
	if separate {
		for _, e := range list {
			if e.Unit != "" {
				frequency[e.Unit]++
			}
		}
	}
	trees := map[string]*unitMinimum{}
	threshold := max(2, int(math.Sqrt(float64(n))))
	for unit, count := range frequency {
		if count >= threshold {
			trees[unit] = newUnitMinimum(h, n+1)
		}
	}
	rare := make([]map[string]int, h)
	previous, next := make([]int, h), make([]int, h)
	active := make([]bool, h)
	first, last := -1, -1
	forward := false
	reset := func() {
		first, last = -1, -1
		forward = !forward
		for i := range h {
			active[i] = len(heats[i]) < size[i]
			if !active[i] {
				continue
			}
			previous[i], next[i] = last, -1
			if last >= 0 {
				next[last] = i
			} else {
				first = i
			}
			last = i
		}
		for _, tree := range trees {
			tree.reset(active)
		}
	}
	for _, e := range list {
		if first < 0 {
			reset()
		}
		pick := first
		if !forward {
			pick = last
		}
		if tree := trees[e.Unit]; tree != nil {
			pick = tree.pick(forward)
			tree.counts[pick]++
		} else if separate && e.Unit != "" {
			for i := pick; i >= 0; {
				if rare[i][e.Unit] < rare[pick][e.Unit] {
					pick = i
				}
				if rare[pick][e.Unit] == 0 {
					break
				}
				if forward {
					i = next[i]
				} else {
					i = previous[i]
				}
			}
			if rare[pick] == nil {
				rare[pick] = map[string]int{}
			}
			rare[pick][e.Unit]++
		}
		heats[pick] = append(heats[pick], e)
		active[pick] = false
		if p := previous[pick]; p >= 0 {
			next[p] = next[pick]
		} else {
			first = next[pick]
		}
		if q := next[pick]; q >= 0 {
			previous[q] = previous[pick]
		} else {
			last = previous[pick]
		}
		for _, tree := range trees {
			tree.exclude(pick)
		}
	}
	return heats
}

type unitMinimum struct {
	counts, minimum []int
	base, infinity  int
}

func newUnitMinimum(heats, infinity int) *unitMinimum {
	base := 1
	for base < heats {
		base *= 2
	}
	return &unitMinimum{counts: make([]int, heats), minimum: make([]int, base*2), base: base, infinity: infinity}
}

func (t *unitMinimum) reset(active []bool) {
	for i := range t.base {
		v := t.infinity
		if i < len(active) && active[i] {
			v = t.counts[i]
		}
		t.minimum[t.base+i] = v
	}
	for i := t.base - 1; i > 0; i-- {
		t.minimum[i] = min(t.minimum[i*2], t.minimum[i*2+1])
	}
}

func (t *unitMinimum) exclude(heat int) {
	i := t.base + heat
	t.minimum[i] = t.infinity
	for i /= 2; i > 0; i /= 2 {
		t.minimum[i] = min(t.minimum[i*2], t.minimum[i*2+1])
	}
}

func (t *unitMinimum) pick(forward bool) int {
	i := 1
	for i < t.base {
		left, right := i*2, i*2+1
		if t.minimum[left] < t.minimum[right] || forward && t.minimum[left] == t.minimum[right] {
			i = left
		} else {
			i = right
		}
	}
	return i - t.base
}
