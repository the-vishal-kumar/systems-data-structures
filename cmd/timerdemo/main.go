// Command timerdemo compares a min-heap with hashed timing wheels for
// scheduling a million delayed retries on a simulated 1 ms clock.
//
//	go run ./cmd/timerdemo
package main

import (
	"fmt"
	"runtime"

	"github.com/the-vishal-kumar/systems-data-structures/timers"
)

func main() {
	const horizon = 60_000 // ticks of 1 ms: retries due within the next minute
	fmt.Printf("Simulated 1 ms clock, deadlines spread over 60 s, half the timers cancelled before they fire. Go %s.\n\n", runtime.Version())
	fmt.Println("| Timers | Structure | Add | Cancel | Run the clock + fire, per timer fired | Memory the structure adds | Fired late or wrongly |")
	fmt.Println("|---:|---|---:|---:|---:|---:|---:|")
	for _, n := range []int{100_000, 1_000_000} {
		for _, c := range []struct {
			name string
			mk   func() timers.Scheduler
		}{
			{"min-heap", func() timers.Scheduler { return timers.NewHeap() }},
			{"timing wheel, 512 slots (~117 turns)", func() timers.Scheduler { return timers.NewWheel(512) }},
			{"timing wheel, 65,536 slots (one turn)", func() timers.Scheduler { return timers.NewWheel(65536) }},
		} {
			r := timers.Run(c.name, c.mk(), n, horizon, 0.5, 1)
			fmt.Printf("| %d | %s | %.0f ns | %.0f ns | %.0f ns | %.1f MB | %d |\n", n, r.Name, r.AddNs, r.CancelNs, r.FireNs,
				float64(r.MemBytes)/1e6, r.Late+r.Early)
		}
	}
}
