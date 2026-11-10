// Command merkledemo compares two replicas of 1,000,000 keys that differ in
// 0 to 10,000 keys: bytes exchanged by a full copy, a hash per key, and a
// Merkle tree walk at two depths.
//
//	go run ./cmd/merkledemo
package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/the-vishal-kumar/systems-data-structures/merkle"
)

const keys = 1_000_000

func size(b int) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func main() {
	a, b := merkle.Store{}, merkle.Store{}
	for i := range keys {
		k := fmt.Sprintf("order:%07d", i)
		v := make([]byte, 200) // a 200-byte record
		for j := range v {
			v[j] = byte(i + j)
		}
		a[k], b[k] = v, v
	}
	full, list := merkle.FullCopyCost(b), merkle.HashListCost(b)
	ta := map[int]*merkle.Tree{12: merkle.Build(a, 12), 16: merkle.Build(a, 16)}

	fmt.Printf("## Finding differences between replicas (`cmd/merkledemo`)\n\n%s keys, 200-byte values. Full copy: %s; a hash per key: %s, in one round trip each.\n\n",
		comma(keys), size(full.Bytes), size(list.Bytes))
	fmt.Println("| Keys that differ | Merkle, 4,096 leaves: bytes / round trips | Merkle, 65,536 leaves: bytes / round trips | Found all? |\n|---:|---:|---:|---|")
	r := rand.New(rand.NewPCG(9, 9))
	changed := 0
	for _, target := range []int{0, 1, 10, 100, 1_000, 10_000} {
		for changed < target {
			k := fmt.Sprintf("order:%07d", r.IntN(keys))
			if string(b[k]) == string(a[k]) {
				b[k] = []byte("updated on one replica only")
				changed++
			}
		}
		row := fmt.Sprintf("| %s |", comma(target))
		ok := true
		for _, d := range []int{12, 16} {
			diff, cost := merkle.Diff(ta[d], merkle.Build(b, d), a, b)
			ok = ok && len(diff) == target
			row += fmt.Sprintf(" %s / %d |", size(cost.Bytes), cost.RoundTrips)
		}
		fmt.Printf("%s %v |\n", row, ok)
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
