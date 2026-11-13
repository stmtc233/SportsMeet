package arrange

import (
	"reflect"
	"testing"
)

// Compare exact heat membership/order against the straightforward rule, not
// merely sizes: seeding ties, unit separation and unbalanced tails must agree.
func TestDealMatchesScan(t *testing.T) {
	for _, n := range []int{1, 2, 9, 40, 127, 501, 1000} {
		for _, per := range []int{1, 3, 8, 12} {
			for _, units := range []int{0, 1, 2, 40, 2000} {
				for _, balance := range []bool{false, true} {
					for _, separate := range []bool{false, true} {
						list := entrants(n, units)
						// Interleave unknown units to exercise the plain snake within unit rows.
						for i := 0; i < n; i += 11 {
							list[i].Unit = ""
						}
						h := (n + per - 1) / per
						size := make([]int, h)
						for i := range h {
							if balance {
								size[i] = n / h
								if i < n%h {
									size[i]++
								}
							} else {
								size[i] = min(per, n-i*per)
							}
						}
						got, want := deal(list, size, separate), scanDeal(list, size, separate)
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("n=%d per=%d units=%d balance=%t separate=%t", n, per, units, balance, separate)
						}
					}
				}
			}
		}
	}
}

func scanDeal(list []Entrant, size []int, separate bool) [][]Entrant {
	h := len(size)
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
				if pick < 0 || (separate && e.Unit != "" && units[i][e.Unit] < units[pick][e.Unit]) {
					pick = i
				}
				if !separate || e.Unit == "" {
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

	return heats
}
