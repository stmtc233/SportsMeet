package arrange

import (
	"fmt"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Isolate grouping from SQLite and view assembly to expose its scaling cost.
func BenchmarkPlanScale(b *testing.B) {
	for _, n := range []int{1000, 10000, 50000} {
		entrants := make([]Entrant, n)
		for i := range entrants {
			entrants[i] = Entrant{ID: fmt.Sprint(i), Unit: fmt.Sprint(i % 40)}
		}
		for _, separate := range []bool{false, true} {
			b.Run(fmt.Sprintf("n=%d/separate=%t", n, separate), func(b *testing.B) {
				opts := Options{Kind: domain.KindTrack, LaneCount: 8, PerHeat: 8, Order: OrderSeed,
					Lanes: LanesCenter, SeparateUnits: separate, Balance: true, Seed: 42}
				b.ReportAllocs()
				for b.Loop() {
					plan, err := Plan(opts, entrants)
					if err != nil || len(plan) != (n+7)/8 {
						b.Fatalf("heats=%d, error=%v", len(plan), err)
					}
				}
			})
		}
	}
}
