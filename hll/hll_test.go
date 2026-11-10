package hll

import (
	"math"
	"testing"
)

func relErr(est uint64, n int) float64 { return math.Abs(float64(est)-float64(n)) / float64(n) }

func TestAccuracyWithinBounds(t *testing.T) {
	for _, p := range []uint8{10, 12, 14} {
		for _, n := range []int{100, 10_000, 1_000_000} {
			s := New(p)
			for i := range n {
				s.Add(uint64(i))
			}
			// 4 standard errors: a correct implementation essentially never fails this.
			if e := relErr(s.Count(), n); e > 4*s.StdError() {
				t.Errorf("p=%d n=%d: error %.2f%% > 4σ (%.2f%%)", p, n, 100*e, 400*s.StdError())
			}
		}
	}
}

func TestDuplicatesDontCount(t *testing.T) {
	s := New(14)
	for range 50 {
		for i := range 1000 {
			s.Add(uint64(i))
		}
	}
	if e := relErr(s.Count(), 1000); e > 0.05 {
		t.Fatalf("1,000 items added 50 times each: estimate %d", s.Count())
	}
}

func TestMergeEqualsUnion(t *testing.T) {
	a, b, both := New(14), New(14), New(14)
	for i := range 200_000 { // overlap: 100,000..199,999 in both
		a.Add(uint64(i))
		b.Add(uint64(i + 100_000))
		both.Add(uint64(i))
		both.Add(uint64(i + 100_000))
	}
	a.Merge(b)
	if a.Count() != both.Count() {
		t.Fatalf("merge %d != union %d", a.Count(), both.Count())
	}
	if e := relErr(a.Count(), 300_000); e > 4*a.StdError() {
		t.Fatalf("union estimate %d, want about 300,000", a.Count())
	}
}

func TestEmpty(t *testing.T) {
	if New(12).Count() != 0 {
		t.Fatal("empty sketch should count 0")
	}
}
