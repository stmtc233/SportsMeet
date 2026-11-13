package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stmtc233/sportsmeet/internal/arrange"
	"github.com/stmtc233/sportsmeet/internal/interchange"
	"github.com/stmtc233/sportsmeet/internal/store"
)

// This opt-in workload uses the real service and mygo SQLite plugin. All writes,
// including fixture creation, use service methods and a temporary database.
type perfScale struct {
	name                       string
	athletes, events, perEvent int
}

var perfScales = []perfScale{
	{"school", 1000, 40, 80},
	{"large", 3000, 80, 120},
	{"extreme", 10000, 160, 200},
	{"wide", 10000, 40, 1000},
}

type perfFixture struct {
	svc                      *Service
	meet, event, round, heat string
	athlete                  AthleteInput
	athleteID                string
	data                     []byte
	draft                    DraftConflictRequest
}

func newPerfFixture(t *testing.T, scale perfScale) *perfFixture {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir())
	must(t, err)
	t.Cleanup(func() { st.Close() })
	s := New(st, nil)
	f := &interchange.File{Format: interchange.Format, Version: interchange.Version,
		ExportedAt: "2026-11-13T00:00:00Z", Generator: interchange.Generator{Name: "SportsMeet", Version: "perf"},
		Meet: interchange.Meet{ID: "meet", Name: "性能测试赛事", StartDate: "2026-11-12", EndDate: ptr("2026-11-15")}}
	for i := range 40 {
		f.Units = append(f.Units, interchange.Unit{ID: fmt.Sprintf("u%d", i), Name: fmt.Sprintf("单位%02d", i)})
	}
	for i := range scale.athletes {
		f.Athletes = append(f.Athletes, interchange.Athlete{ID: fmt.Sprintf("a%d", i), Name: fmt.Sprintf("运动员%05d", i),
			Gender: "男", Bib: fmt.Sprintf("%06d", i+1), StudentNumber: fmt.Sprintf("S%06d", i+1), UnitID: ptr(f.Units[i%40].ID)})
	}
	base := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	for i := range scale.events {
		e := interchange.Event{ID: fmt.Sprintf("e%d", i), Gender: "男", Discipline: "100m", GroupLabel: fmt.Sprintf("组别%d", i),
			Kind: "track", StartMode: "lanes", LaneCount: ptr(8), Entries: []interchange.Entry{}}
		r := interchange.Round{ID: fmt.Sprintf("r%d", i), Stage: "final", Ordinal: 1, Heats: []interchange.Heat{}}
		for j := 0; j < scale.perEvent; j += 8 {
			h := interchange.Heat{ID: fmt.Sprintf("h%d-%d", i, j/8), Ordinal: j/8 + 1,
				ScheduledAt: ptr(base.Add(time.Duration(i*5+j/8*3) * time.Minute).Format("2006-01-02T15:04")), DurationMinutes: ptr(3)}
			for k := j; k < min(j+8, scale.perEvent); k++ {
				id := f.Athletes[(i*scale.perEvent+k)%scale.athletes].ID
				e.Entries = append(e.Entries, interchange.Entry{AthleteID: id})
				sl := interchange.Slot{Position: k - j + 1, AthleteID: id}
				if j/8%2 == 0 {
					sl.Result = &interchange.Result{Status: "OK", Mark: ptr(int64(12000 + k*10)), Rank: ptr(k + 1)}
				}
				h.Slots = append(h.Slots, sl)
			}
			r.Heats = append(r.Heats, h)
		}
		e.Rounds = []interchange.Round{r}
		f.Events = append(f.Events, e)
	}
	data, err := json.Marshal(f)
	must(t, err)
	p, err := s.PreviewImport(ctx, data, "")
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatal(p.Errors)
	}
	p, err = s.ImportMeet(ctx, ui, data, "", p.Hash)
	must(t, err)
	events, err := s.ListEvents(ctx, p.MeetID)
	must(t, err)
	d, err := s.GetEvent(ctx, events[0].Event.ID)
	must(t, err)
	athletes, err := s.ListAthletes(ctx, p.MeetID)
	must(t, err)
	a := athletes[0].Athlete
	pf := &perfFixture{svc: s, meet: p.MeetID, event: events[0].Event.ID, round: d.Rounds[0].ID, heat: d.Rounds[0].Heats[0].ID,
		athleteID: a.ID, athlete: AthleteInput{Name: a.Name, Gender: string(a.Gender), Bib: a.Bib, StudentNumber: a.StudentNumber, UnitID: a.UnitID}, data: data}
	pf.draft.EventID = pf.event
	for _, r := range d.Rounds {
		for _, h := range r.Heats {
			v := DraftTime{Key: h.ID, RoundID: r.ID, ScheduledAt: h.ScheduledAt, DurationMinutes: h.DurationMinutes}
			for _, sl := range h.Slots {
				v.Competitors = append(v.Competitors, sl.CompetitorID)
			}
			pf.draft.Heats = append(pf.draft.Heats, v)
		}
	}
	return pf
}

type perfRecord struct {
	Scale       string  `json:"scale"`
	Operation   string  `json:"operation"`
	Samples     int     `json:"samples"`
	Clients     int     `json:"clients"`
	Errors      int     `json:"errors"`
	P50MS       float64 `json:"p50_ms"`
	P95MS       float64 `json:"p95_ms"`
	MaxMS       float64 `json:"max_ms"`
	OpsSecond   float64 `json:"ops_second"`
	AllocMB     float64 `json:"alloc_mb_per_op,omitempty"`
	Allocations uint64  `json:"allocations_per_op,omitempty"`
}

func emitPerf(t *testing.T, scale, op string, clients, failures int, times []float64, wall time.Duration, before, after runtime.MemStats) {
	t.Helper()
	slices.Sort(times)
	n := len(times)
	r := perfRecord{Scale: scale, Operation: op, Samples: n, Clients: clients, Errors: failures,
		P50MS: times[int(math.Ceil(float64(n)*.5))-1], P95MS: times[int(math.Ceil(float64(n)*.95))-1], MaxMS: times[n-1],
		OpsSecond: float64(n) / wall.Seconds(), AllocMB: float64(after.TotalAlloc-before.TotalAlloc) / float64(n) / (1 << 20),
		Allocations: (after.Mallocs - before.Mallocs) / uint64(n)}
	b, err := json.Marshal(r)
	must(t, err)
	t.Log(string(b))
	if dir := os.Getenv("SPORTSMEET_PERF_OUTPUT"); dir != "" {
		must(t, os.MkdirAll(dir, 0o755))
		file, err := os.OpenFile(filepath.Join(dir, "service.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		must(t, err)
		_, err = fmt.Fprintln(file, string(b))
		must(t, err)
		must(t, file.Close())
	}
	if failures != 0 {
		t.Fatalf("%s: %d failed requests", op, failures)
	}
}

// TestPerformanceStress runs serial latency/allocation measurements and a closed
// loop mixed workload. Clients wait for completion before sending the next call.
// Profiles exclude fixture import. No latency threshold is enforced across hosts.
func TestPerformanceStress(t *testing.T) {
	if os.Getenv("SPORTSMEET_PERF") != "1" {
		t.Skip("set SPORTSMEET_PERF=1 to run the isolated SQLite load test")
	}
	n := 30
	if v := os.Getenv("SPORTSMEET_PERF_SAMPLES"); v != "" {
		var err error
		n, err = strconv.Atoi(v)
		must(t, err)
		if n < 2 {
			t.Fatal("SPORTSMEET_PERF_SAMPLES must be >= 2")
		}
	}
	for _, scale := range perfScales {
		t.Run(scale.name, func(t *testing.T) {
			f := newPerfFixture(t, scale)
			ctx := context.Background()
			s := f.svc
			t.Cleanup(func() {
				counts, err := s.CountMeet(ctx, f.meet)
				must(t, err)
				if counts.Athletes != scale.athletes || counts.Events != scale.events || counts.Heats != scale.events*((scale.perEvent+7)/8) {
					t.Fatalf("unexpected counts after load: %+v", counts)
				}
				rows, err := s.store.Query(ctx, "PRAGMA quick_check")
				must(t, err)
				if len(rows.Values) != 1 || rows.Values[0][0] != "ok" {
					t.Fatalf("quick_check: %+v", rows.Values)
				}
				rows, err = s.store.Query(ctx, "PRAGMA foreign_key_check")
				must(t, err)
				if len(rows.Values) != 0 {
					t.Fatalf("foreign_key_check: %+v", rows.Values)
				}
			})
			if dir := os.Getenv("SPORTSMEET_PERF_OUTPUT"); dir != "" {
				must(t, os.MkdirAll(dir, 0o755))
				html, err := s.RenderReport(ctx, ReportStartList, f.meet, nil)
				must(t, err)
				must(t, os.WriteFile(filepath.Join(dir, scale.name+"-startlist.html"), []byte(html), 0o644))
				file, err := os.Create(filepath.Join(dir, scale.name+".cpu.pprof"))
				must(t, err)
				must(t, pprof.StartCPUProfile(file))
				t.Cleanup(func() { pprof.StopCPUProfile(); file.Close() })
			}
			ops := []struct {
				name string
				call func(int) error
			}{
				{"ListAthletes", func(int) error { _, err := s.ListAthletes(ctx, f.meet); return err }},
				{"ListEvents", func(int) error { _, err := s.ListEvents(ctx, f.meet); return err }},
				{"GetEventJSON", func(int) error {
					v, err := s.GetEvent(ctx, f.event)
					if err == nil {
						_, err = json.Marshal(v)
					}
					return err
				}},
				{"Schedule", func(int) error { _, err := s.Schedule(ctx, f.meet); return err }},
				{"DraftConflicts", func(int) error { _, err := s.DraftConflicts(ctx, f.draft); return err }},
				{"PlanHeats", func(int) error {
					_, err := s.PlanHeats(ctx, PlanRequest{EventID: f.event, RoundID: f.round, PerHeat: 8, Order: arrange.OrderSeed, Lanes: arrange.LanesCenter, SeparateUnits: true, Balance: true, Seed: 42})
					return err
				}},
				{"PreviewSchedule", func(int) error {
					_, err := s.PreviewSchedule(ctx, ScheduleRequest{EventID: f.event, RoundID: f.round, StartAt: "2026-11-12T09:00", IntervalMinutes: 3})
					return err
				}},
				{"ReportAll", func(int) error { _, err := s.RenderReport(ctx, ReportStartList, f.meet, nil); return err }},
				{"ReportOne", func(int) error { _, err := s.RenderReport(ctx, ReportStartList, f.meet, []string{f.event}); return err }},
				{"ExportJSON", func(int) error {
					v, err := s.ExportMeet(ctx, f.meet)
					if err == nil {
						_, err = interchange.Encode(v)
					}
					return err
				}},
				{"PreviewImport", func(int) error { _, err := s.PreviewImport(ctx, f.data, ""); return err }},
				{"SaveAthlete", func(i int) error {
					in := f.athlete
					in.Note = fmt.Sprint(i % 2)
					_, err := s.SaveAthlete(ctx, ui, f.meet, f.athleteID, in)
					return err
				}},
				{"SaveResults", func(i int) error {
					return s.SaveResults(ctx, ui, f.heat, []ResultInput{{Position: 1, Mark: fmt.Sprintf("%d.00", 11+i%2)}})
				}},
			}
			for _, op := range ops {
				if filter := os.Getenv("SPORTSMEET_PERF_OP"); filter != "" && !strings.Contains(op.name, filter) {
					continue
				}
				t.Run(op.name, func(t *testing.T) {
					must(t, op.call(-1))
					runtime.GC()
					var before, after runtime.MemStats
					times := make([]float64, n)
					runtime.ReadMemStats(&before)
					start := time.Now()
					for i := range n {
						at := time.Now()
						must(t, op.call(i))
						times[i] = float64(time.Since(at).Nanoseconds()) / 1e6
					}
					wall := time.Since(start)
					runtime.ReadMemStats(&after)
					emitPerf(t, scale.name, op.name, 1, 0, times, wall, before, after)
				})
			}
			if os.Getenv("SPORTSMEET_PERF_OP") != "" {
				return
			}
			for _, clients := range []int{4, 16} {
				t.Run(fmt.Sprintf("Mixed%d", clients), func(t *testing.T) {
					var wg sync.WaitGroup
					times := make([]float64, clients*n)
					errs := make([]error, len(times))
					ready := make(chan struct{})
					var before, after runtime.MemStats
					runtime.GC()
					runtime.ReadMemStats(&before)
					for client := range clients {
						wg.Go(func() {
							<-ready
							for i := range n {
								at := time.Now()
								var err error
								switch (client + i) % 5 {
								case 0:
									_, err = s.ListEvents(ctx, f.meet)
								case 1:
									_, err = s.GetEvent(ctx, f.event)
								case 2:
									_, err = s.Schedule(ctx, f.meet)
								case 3:
									err = s.SaveResults(ctx, ui, f.heat, []ResultInput{{Position: 1, Mark: fmt.Sprintf("%d.00", 11+i%2)}})
								case 4:
									_, err = s.ListAthletes(ctx, f.meet)
								}
								times[client*n+i], errs[client*n+i] = float64(time.Since(at).Nanoseconds())/1e6, err
							}
						})
					}
					start := time.Now()
					close(ready)
					wg.Wait()
					wall := time.Since(start)
					runtime.ReadMemStats(&after)
					failures := 0
					for _, err := range errs {
						if err != nil {
							failures++
							t.Log(err)
						}
					}
					emitPerf(t, scale.name, "Mixed", clients, failures, times, wall, before, after)
				})
			}
		})
	}
}
