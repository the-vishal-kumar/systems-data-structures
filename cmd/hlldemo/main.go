// Command hlldemo compares an exact set with HyperLogLog at several
// precisions: memory and error when counting distinct visitors.
//
//	go run ./cmd/hlldemo
package main

import (
	"fmt"
	"math"
	"runtime"

	"github.com/the-vishal-kumar/systems-data-structures/hll"
)

func heap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func size(b float64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KiB", b/(1<<10))
	}
	return fmt.Sprintf("%.0f B", b)
}

func main() {
	fmt.Println("## Counting distinct items (`cmd/hlldemo`)")
	fmt.Println()

	// Exact set memory, for reference.
	fmt.Println("### Exact set (Go map of 64-bit IDs)")
	fmt.Println()
	fmt.Println("| Distinct items | Memory |\n|---:|---:|")
	for _, n := range []int{100_000, 1_000_000, 10_000_000} {
		before := heap()
		set := make(map[uint64]struct{})
		for i := range n {
			set[hll.Hash(uint64(i))] = struct{}{}
		}
		fmt.Printf("| %s | %s |\n", comma(n), size(float64(heap()-before)))
		runtime.KeepAlive(set)
	}

	// HyperLogLog error: mean and worst absolute relative error over 20
	// independent streams of each size.
	const trials = 20
	fmt.Printf("\n### HyperLogLog: relative error over %d independent streams\n\n", trials)
	fmt.Println("| Precision | Memory | Expected σ | n = 1,000 (mean / worst) | n = 100,000 (mean / worst) | n = 10,000,000 (mean / worst) |\n|---:|---:|---:|---:|---:|---:|")
	for _, p := range []uint8{10, 12, 14, 16} {
		s := hll.New(p)
		fmt.Printf("| %d | %s | %.2f%% |", p, size(float64(s.Bytes())), 100*s.StdError())
		for _, n := range []int{1_000, 100_000, 10_000_000} {
			mean, worst := 0.0, 0.0
			for t := range trials {
				s := hll.New(p)
				base := uint64(t) << 40 // disjoint IDs per trial
				for i := range n {
					s.Add(base + uint64(i))
				}
				e := math.Abs(float64(s.Count())-float64(n)) / float64(n)
				mean += e / trials
				worst = math.Max(worst, e)
			}
			fmt.Printf(" %.2f%% / %.2f%% |", 100*mean, 100*worst)
		}
		fmt.Println()
	}

	// Merging: a week from seven daily sketches, with returning visitors.
	fmt.Println("\n### A week from seven daily sketches (p = 14, 16 KiB each)")
	fmt.Println()
	week, exact := hll.New(14), map[uint64]struct{}{}
	for day := range 7 {
		d := hll.New(14)
		// 200,000 visitors a day; half of them come back from earlier days.
		for i := range 200_000 {
			id := uint64(day*100_000 + i)
			d.Add(id)
			exact[id] = struct{}{}
		}
		week.Merge(d)
	}
	n := len(exact)
	fmt.Printf("| Exact weekly uniques | Merged estimate | Error |\n|---:|---:|---:|\n| %s | %s | %.2f%% |\n\n",
		comma(n), comma(int(week.Count())), 100*math.Abs(float64(week.Count())-float64(n))/float64(n))
}

func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
