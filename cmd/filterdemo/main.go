// Command filterdemo compares an exact set, Bloom filters and a cuckoo
// filter on 1,000,000 items: memory, false positives and speed.
//
//	go run ./cmd/filterdemo
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/filter"
)

const n = 1_000_000

func items(prefix string, k int) []string {
	out := make([]string, k)
	for i := range out {
		out[i] = fmt.Sprintf("%s%d@example.com", prefix, i)
	}
	return out
}

func heap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

type row struct {
	name            string
	bytes           uint64
	fp              float64
	addNs, lookupNs float64
	deletes, note   string
}

func measure(name string, add func(string) bool, contains func(string) bool, size func() uint64, in, out []string, deletes, note string) row {
	t := time.Now()
	failed := 0
	for _, x := range in {
		if !add(x) {
			failed++
		}
	}
	addNs := float64(time.Since(t).Nanoseconds()) / float64(len(in))
	t = time.Now()
	fp := 0
	for _, x := range out {
		if contains(x) {
			fp++
		}
	}
	lookupNs := float64(time.Since(t).Nanoseconds()) / float64(len(out))
	if failed > 0 {
		note = fmt.Sprintf("%d inserts failed (full)", failed)
	}
	return row{name, size(), float64(fp) / float64(len(out)), addNs, lookupNs, deletes, note}
}

func main() {
	in, out := items("user", n), items("other", n)
	fmt.Printf("%d items in, %d different items queried. Go %s.\n\n", n, n, runtime.Version())
	var rows []row

	before := heap()
	s := filter.NewSet(n)
	r := measure("exact hash set (map)", func(x string) bool { s.Add(x); return true }, s.Contains,
		func() uint64 { return heap() - before }, in, out, "yes", "plus the strings themselves, if not already in memory")
	rows = append(rows, r)
	runtime.KeepAlive(s)
	s = nil

	for _, p := range []float64{0.01, 0.001, 0.0001} {
		b := filter.NewBloom(n, p)
		rows = append(rows, measure(fmt.Sprintf("Bloom, sized for %g%% (k = %d)", p*100, b.K()),
			func(x string) bool { b.Add(x); return true }, b.Contains, func() uint64 { return uint64(b.Bytes()) }, in, out, "no", ""))
	}
	// 2^18 buckets × 4 slots = 1,048,576 slots: a million items fill it to 95%.
	c := filter.NewCuckoo(n)
	r = measure("cuckoo, 16-bit fingerprints", c.Add, c.Contains, func() uint64 { return uint64(c.Bytes()) }, in, out, "yes", "")
	if r.note == "" {
		r.note = fmt.Sprintf("%.0f%% full", c.LoadFactor()*100)
	}
	rows = append(rows, r)

	fmt.Println("| Structure | Memory | Bits per item | False positives | Insert | Lookup | Deletes? | Note |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---|---|")
	for _, r := range rows {
		fmt.Printf("| %s | %.1f MB | %.1f | %.4f%% | %.0f ns | %.0f ns | %s | %s |\n", r.name, float64(r.bytes)/1e6,
			float64(r.bytes)*8/n, r.fp*100, r.addNs, r.lookupNs, r.deletes, r.note)
	}

	fmt.Print("\n### What happens when you put in more than you sized for\n\n| Filter | Sized for | Items added | False positives |\n|---|---:|---:|---:|\n")
	for _, k := range []int{n, 2 * n, 4 * n} {
		b := filter.NewBloom(n, 0.01)
		for _, x := range items("user", k) {
			b.Add(x)
		}
		fp := 0
		for _, x := range out {
			if b.Contains(x) {
				fp++
			}
		}
		fmt.Printf("| Bloom, 1%% | %d | %d | %.1f%% |\n", n, k, float64(fp)*100/float64(len(out)))
	}
	cf := filter.NewCuckoo(n)
	added := 0
	for _, x := range items("user", 2*n) {
		if !cf.Add(x) {
			break
		}
		added++
	}
	fmt.Printf("| cuckoo | %d slots | first insert failed after %d (%.1f%% full) | - |\n", 1<<20, added, cf.LoadFactor()*100)
}
