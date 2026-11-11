// Command partdemo measures how part-number and description lookups scale
// with catalog size: brute-force scans vs a BK-tree, a symmetric-delete
// index, a sorted array and an inverted index.
//
//	go run ./cmd/partdemo
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/partmatch"
)

func us(d time.Duration, n int) string {
	v := float64(d.Nanoseconds()) / float64(n) / 1000
	if v < 10 {
		return fmt.Sprintf("%.2f µs", v)
	}
	if v >= 1000 {
		return fmt.Sprintf("%.1f ms", v/1000)
	}
	return fmt.Sprintf("%.0f µs", v)
}

func main() {
	const queries = 1000
	fmt.Printf("Synthetic catalog of normalized SKUs like BRG6204RS. %d queries per row, each a real SKU with one typo (or its first 5 characters, or a reworded description). Go %s.\n\n", queries, runtime.Version())
	fmt.Println("| Catalog | Lookup | Per query | Distances computed / items scored per query | Index build |")
	fmt.Println("|---:|---|---:|---:|---:|")
	for _, n := range []int{1_000, 10_000, 100_000} {
		skus, descs := partmatch.Catalog(n, 1)
		r := rand.New(rand.NewPCG(7, 7))
		typo, pre, desc := make([]string, queries), make([]string, queries), make([][]string, queries)
		for i := range queries {
			j := r.IntN(n)
			typo[i], pre[i], desc[i] = partmatch.Typo(skus[j], r), skus[j][:5], partmatch.Rephrase(descs[j], r)
		}
		row := func(what string, d time.Duration, work string, build time.Duration) {
			b := "—"
			if build > 0 {
				b = fmt.Sprintf("%.0f ms", float64(build.Microseconds())/1000)
			}
			fmt.Printf("| %d | %s | %s | %s | %s |\n", n, what, us(d, queries), work, b)
		}

		t := time.Now()
		for _, q := range typo {
			partmatch.Within(skus, q, 1)
		}
		row("typo: linear scan", time.Since(t), fmt.Sprintf("%d", n), 0)

		t = time.Now()
		bk := partmatch.NewBKTree(skus)
		build := time.Since(t)
		for _, d := range []int{1, 2} {
			comps := 0
			t = time.Now()
			for _, q := range typo {
				_, c := bk.Within(q, d)
				comps += c
			}
			row(fmt.Sprintf("typo: BK-tree, distance ≤ %d", d), time.Since(t), fmt.Sprintf("%d", comps/queries), build)
		}

		t = time.Now()
		sd := partmatch.NewSymDelete(skus)
		build = time.Since(t)
		keys, postings := sd.Entries()
		t = time.Now()
		for _, q := range typo {
			sd.Within1(q)
		}
		row(fmt.Sprintf("typo: symmetric-delete index (%d keys, %d entries)", keys, postings), time.Since(t), "a few", build)

		t = time.Now()
		for _, p := range pre {
			partmatch.PrefixScan(skus, p)
		}
		row("prefix: linear scan", time.Since(t), fmt.Sprintf("%d", n), 0)
		t = time.Now()
		so := partmatch.NewSorted(skus)
		build = time.Since(t)
		t = time.Now()
		for _, p := range pre {
			so.Prefix(p)
		}
		row("prefix: sorted array + binary search", time.Since(t), "log₂ n + matches", build)

		t = time.Now()
		for _, q := range desc {
			partmatch.BestScan(descs, q)
		}
		row("description: score every item", time.Since(t), fmt.Sprintf("%d", n), 0)
		t = time.Now()
		inv := partmatch.NewInverted(descs)
		build = time.Since(t)
		scored := 0
		t = time.Now()
		for _, q := range desc {
			_, _, s := inv.Best(q)
			scored += s
		}
		row("description: inverted index", time.Since(t), fmt.Sprintf("%d", scored/queries), build)
		scored, same := 0, 0
		for _, q := range desc {
			_, sc, s := inv.BestSelective(q, 0.01)
			scored += s
			if _, want := partmatch.BestScan(descs, q); sc == want {
				same++
			}
		}
		t = time.Now() // timed separately, without the comparison against the scan
		for _, q := range desc {
			inv.BestSelective(q, 0.01)
		}
		row(fmt.Sprintf("description: inverted index, rare words only (≤1%% of items); best score matches the scan in %d%% of queries", same*100/queries),
			time.Since(t), fmt.Sprintf("%d", scored/queries), build)
	}
}
