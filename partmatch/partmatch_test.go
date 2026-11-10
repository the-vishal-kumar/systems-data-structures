package partmatch

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// Every index must return exactly what the brute-force scan returns.
func TestIndexesAgreeWithScans(t *testing.T) {
	skus, descs := Catalog(5000, 1)
	bk, sd, so, inv := NewBKTree(skus), NewSymDelete(skus), NewSorted(skus), NewInverted(descs)
	r := rand.New(rand.NewPCG(2, 2))
	for range 300 {
		i := r.IntN(len(skus))
		q := Typo(skus[i], r)
		want := Within(skus, q, 1)
		if !slices.Contains(want, i) {
			t.Fatalf("scan missed the original of %q", q)
		}
		if got, _ := bk.Within(q, 1); !slices.Equal(got, want) {
			t.Fatalf("bk-tree %q: %v, want %v", q, got, want)
		}
		if got := sd.Within1(q); !slices.Equal(got, want) {
			t.Fatalf("symdelete %q: %v, want %v", q, got, want)
		}
		want2 := Within(skus, q, 2)
		if got, _ := bk.Within(q, 2); !slices.Equal(got, want2) {
			t.Fatalf("bk-tree d=2 %q: %v, want %v", q, got, want2)
		}
		p := skus[i][:5]
		if got, want := so.Prefix(p), PrefixScan(skus, p); !slices.Equal(got, want) {
			t.Fatalf("prefix %q: %d results, want %d", p, len(got), len(want))
		}
		dq := Rephrase(descs[i], r)
		wb, ws := BestScan(descs, dq)
		if gb, gs, _ := inv.Best(dq); gb != wb || gs != ws {
			t.Fatalf("inverted %v: %d %.2f, want %d %.2f", dq, gb, gs, wb, ws)
		}
	}
}
