// Command skiplistdemo compares leaderboards at 10,000 to 1,000,000 players:
// the cost of a score update, a rank lookup and a top-10 read.
//
//	go run ./cmd/skiplistdemo
package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/skiplist"
)

func ns(d time.Duration, n int) string {
	v := float64(d.Nanoseconds()) / float64(n)
	switch {
	case v >= 1e6:
		return fmt.Sprintf("%.1f ms", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1f µs", v/1e3)
	}
	return fmt.Sprintf("%.0f ns", v)
}

func main() {
	fmt.Println("## Leaderboards (`cmd/skiplistdemo`)")
	fmt.Println()
	fmt.Println("Each board is filled with n players, then measured on random score updates, rank lookups and top-10 reads.")
	fmt.Println()
	fmt.Println("| Players | Structure | Update | Rank of a player | Top 10 |\n|---:|---|---:|---:|---:|")
	for _, n := range []int{10_000, 100_000, 1_000_000} {
		members := make([]string, n)
		for i := range members {
			members[i] = fmt.Sprintf("player-%07d", i)
		}
		for _, b := range []skiplist.Board{skiplist.NewSkipList(), skiplist.NewSorted(), skiplist.NewScan()} {
			r := rand.New(rand.NewPCG(5, 6))
			if sb, ok := b.(*skiplist.Sorted); ok {
				entries := make([]skiplist.Entry, n)
				for i, m := range members {
					entries[i] = skiplist.Entry{Member: m, Score: float64(r.IntN(1_000_000))}
				}
				sb.Load(entries) // building it with Set would be quadratic
			} else {
				for _, m := range members {
					b.Set(m, float64(r.IntN(1_000_000)))
				}
			}
			updates, ranks, tops := 20_000, 2_000, 200
			if b.Name() == "map + scan" {
				ranks, tops = max(20, 2_000_000/n), max(5, 200_000/n)
			}
			if b.Name() == "sorted slice" && n >= 1_000_000 {
				updates = 2_000
			}
			t := time.Now()
			for range updates {
				b.Set(members[r.IntN(n)], float64(r.IntN(1_000_000)))
			}
			upd := ns(time.Since(t), updates)
			t = time.Now()
			for range ranks {
				b.Rank(members[r.IntN(n)])
			}
			rk := ns(time.Since(t), ranks)
			t = time.Now()
			for range tops {
				b.Top(10)
			}
			fmt.Printf("| %s | %s | %s | %s | %s |\n", comma(n), b.Name(), upd, rk, ns(time.Since(t), tops))
		}
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
