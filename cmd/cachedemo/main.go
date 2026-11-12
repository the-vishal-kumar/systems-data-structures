// Command cachedemo compares LRU, LFU and W-TinyLFU hit rates on skewed
// traffic, after a one-off scan, and when popularity shifts.
//
//	go run ./cmd/cachedemo
package main

import (
	"fmt"

	"github.com/the-vishal-kumar/systems-data-structures/cache"
)

const (
	items    = 1_000_000 // distinct keys
	accesses = 4_000_000
	skew     = 0.9 // Zipf exponent; web traffic is often 0.7 to 1.0
)

func trace(z *cache.Zipf, n, epoch int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		out[i] = cache.Key(z.Next(), epoch)
	}
	return out
}

func all(capacity int) []cache.Policy {
	return []cache.Policy{cache.NewLRU(capacity), cache.NewLFU(capacity), cache.NewTinyLFU(capacity)}
}

func main() {
	fmt.Printf("## Cache eviction (`cmd/cachedemo`)\n\n%s distinct keys, Zipf s=%.1f.\n\n", comma(items), skew)

	// 1. Steady skewed traffic, at three cache sizes.
	z := cache.NewZipf(items, skew, 7)
	warm, measured := trace(z, accesses/4, 0), trace(z, accesses, 0)
	fmt.Printf("### Steady traffic: hit rate over %s accesses after warming up\n\n", comma(accesses))
	fmt.Println("| Cache size | LRU | LFU | W-TinyLFU |\n|---:|---:|---:|---:|")
	for _, size := range []int{1_000, 10_000, 100_000} {
		fmt.Printf("| %s (%.1f%% of keys) |", comma(size), 100*float64(size)/items)
		for _, p := range all(size) {
			cache.HitRate(p, warm)
			fmt.Printf(" %.1f%% |", 100*cache.HitRate(p, measured))
		}
		fmt.Println()
	}

	// 2. A one-off scan (a report, a crawler, a backfill) touches 50,000
	// keys nobody will ask for again. How much of the hot set survives?
	const size = 10_000
	scan := make([]uint64, 50_000)
	for i := range scan {
		scan[i] = cache.Key(items+i, 0)
	}
	after := trace(z, 105_000, 0)
	fmt.Printf("\n### After a one-off scan of %s keys (cache of %s)\n\n", comma(len(scan)), comma(size))
	fmt.Println("| Policy | Hit rate before | Next 5,000 accesses | Next 100,000 accesses |\n|---|---:|---:|---:|")
	for _, p := range all(size) {
		cache.HitRate(p, warm)
		before := cache.HitRate(p, measured[:200_000])
		cache.HitRate(p, scan)
		fmt.Printf("| %s | %.1f%% | %.1f%% | %.1f%% |\n", p.Name(), 100*before, 100*cache.HitRate(p, after[:5_000]), 100*cache.HitRate(p, after[5_000:]))
	}

	// 3. Popularity shifts: the same distribution, but over new keys (new
	// products, a new news cycle).
	fmt.Printf("\n### When popularity shifts to new keys (cache of %s)\n\n", comma(size))
	fmt.Println("| Policy | Before the shift | First 200,000 after | Next 1,000,000 after |\n|---|---:|---:|---:|")
	z2 := cache.NewZipf(items, skew, 11)
	next := trace(z2, 1_200_000, 1)
	for _, p := range all(size) {
		cache.HitRate(p, measured) // a long history of the old popularity
		before := cache.HitRate(p, measured[:200_000])
		fmt.Printf("| %s | %.1f%% | %.1f%% | %.1f%% |\n", p.Name(), 100*before,
			100*cache.HitRate(p, next[:200_000]), 100*cache.HitRate(p, next[200_000:]))
	}
	fmt.Println()
}

func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
