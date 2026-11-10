// Command topkdemo finds the top 10 keys in a skewed stream of 20 million
// events over 5 million distinct keys, exactly and with two sketches:
// memory, how many of the true top 10 each finds, and count error.
//
//	go run ./cmd/topkdemo
package main

import (
	"fmt"
	"math"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/cache"
	"github.com/the-vishal-kumar/systems-data-structures/topk"
)

const (
	events   = 20_000_000
	distinct = 5_000_000
)

func size(b int) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KiB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func main() {
	fmt.Printf("## Top-k heavy hitters (`cmd/topkdemo`)\n\n%s events over %s distinct keys.\n\n", comma(events), comma(distinct))
	for _, s := range []float64{1.1, 0.8} {
		z := cache.NewZipf(distinct, s, 3)
		stream := make([]uint64, events)
		for i := range stream {
			stream[i] = cache.Key(z.Next(), 0)
		}
		exact := topk.NewExact()
		t := time.Now()
		for _, k := range stream {
			exact.Add(k)
		}
		exactNs := float64(time.Since(t).Nanoseconds()) / events
		truth := exact.Top(10)
		inTruth := map[uint64]bool{}
		for _, it := range truth {
			inTruth[it.Key] = true
		}
		fmt.Printf("### Zipf s=%.1f (the 10th most frequent key appears %s times)\n\n", s, comma(int(truth[9].Count)))
		fmt.Println("| Method | Memory | ns per event | True top 10 found | Worst count error in its top 10 (vs the true 10th count) |\n|---|---:|---:|---:|---:|")
		fmt.Printf("| exact map | ~%s | %.0f | 10 | 0 |\n", size(exact.Bytes()), exactNs)
		for _, c := range []topk.Counter{
			topk.NewCMS(1<<12, 4, 10), topk.NewCMS(1<<16, 4, 10),
			topk.NewSpaceSaving(100), topk.NewSpaceSaving(1000),
		} {
			t := time.Now()
			for _, k := range stream {
				c.Add(k)
			}
			ns := float64(time.Since(t).Nanoseconds()) / events
			found, worst := 0, 0.0
			for _, it := range c.Top(10) {
				if inTruth[it.Key] {
					found++
				}
				tr := float64(exact.Count(it.Key))
				worst = math.Max(worst, math.Abs(float64(it.Count)-tr)/float64(truth[9].Count))
			}
			name := c.Name()
			switch c := c.(type) {
			case *topk.SpaceSaving:
				name = fmt.Sprintf("%s, %s counters", name, comma(c.Bytes()/64))
			case *topk.CMS:
				name = fmt.Sprintf("%s, width %s × 4", name, comma((c.Bytes()-640)/32))
			}
			fmt.Printf("| %s | %s | %.0f | %d | %.1f%% |\n", name, size(c.Bytes()), ns, found, 100*worst)
		}
		fmt.Println()
	}
}

func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
