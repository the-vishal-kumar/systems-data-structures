package skiplist

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// All three boards must agree on every rank and on the top 10 after a long
// series of random inserts and updates (including ties).
func TestBoardsAgree(t *testing.T) {
	boards := []Board{NewSkipList(), NewSorted(), NewScan()}
	r := rand.New(rand.NewPCG(3, 4))
	members := make([]string, 2000)
	for i := range members {
		members[i] = fmt.Sprintf("p%04d", i)
	}
	for step := range 20_000 {
		m := members[r.IntN(len(members))]
		score := float64(r.IntN(500)) // many ties
		for _, b := range boards {
			b.Set(m, score)
		}
		if step%1000 == 0 {
			for _, m := range members[:200] {
				want := boards[2].Rank(m)
				for _, b := range boards[:2] {
					if got := b.Rank(m); got != want {
						t.Fatalf("step %d: %s rank of %s = %d, want %d", step, b.Name(), m, got, want)
					}
				}
			}
		}
	}
	want := boards[2].Top(10)
	for _, b := range boards[:2] {
		got := b.Top(10)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s top 10 = %v, want %v", b.Name(), got, want)
			}
		}
	}
	if sl := boards[0].(*SkipList); sl.Len() != len(sl.scores) {
		t.Fatalf("skip list length %d, members %d", sl.Len(), len(sl.scores))
	}
}

func TestRankOfAbsentMember(t *testing.T) {
	for _, b := range []Board{NewSkipList(), NewSorted(), NewScan()} {
		b.Set("a", 1)
		if b.Rank("nobody") != 0 || b.Rank("a") != 1 {
			t.Fatalf("%s: wrong ranks", b.Name())
		}
	}
}
