package filter

import (
	"fmt"
	"math"
	"testing"
)

func items(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%d@example.com", prefix, i)
	}
	return out
}

func fpRate(contains func(string) bool, absent []string) float64 {
	var fp int
	for _, a := range absent {
		if contains(a) {
			fp++
		}
	}
	return float64(fp) / float64(len(absent))
}

func TestBloom(t *testing.T) {
	in, out := items("user", 200_000), items("other", 200_000)
	for _, p := range []float64{0.01, 0.001} {
		b := NewBloom(len(in), p)
		for _, x := range in {
			b.Add(x)
		}
		for _, x := range in {
			if !b.Contains(x) {
				t.Fatalf("false negative for %s", x)
			}
		}
		if got := fpRate(b.Contains, out); math.Abs(got-p) > p/2 {
			t.Errorf("p=%g: measured false positives %.4f", p, got)
		}
	}
	// Overfilled: twice the items it was sized for.
	b := NewBloom(100_000, 0.01)
	for _, x := range in {
		b.Add(x)
	}
	if got := fpRate(b.Contains, out); got < 0.1 {
		t.Errorf("an overfilled Bloom filter should degrade badly: %.3f", got)
	}
}

func TestCuckoo(t *testing.T) {
	in, out := items("user", 200_000), items("other", 200_000)
	c := NewCuckoo(int(float64(len(in)) / 0.9))
	for _, x := range in {
		if !c.Add(x) {
			t.Fatalf("insert failed at load %.2f", c.LoadFactor())
		}
	}
	if got := fpRate(c.Contains, out); got > 0.0005 {
		t.Errorf("16-bit fingerprints should give about 0.01%% false positives: %.5f", got)
	}
	for _, x := range in[:100_000] {
		if !c.Delete(x) {
			t.Fatalf("delete of an added item failed: %s", x)
		}
	}
	for _, x := range in[100_000:] {
		if !c.Contains(x) {
			t.Fatalf("deleting other items caused a false negative for %s", x)
		}
	}
	if got := fpRate(c.Contains, in[:100_000]); got > 0.001 {
		t.Errorf("deleted items should mostly be gone: %.4f still found", got)
	}
}

func TestCuckooFills(t *testing.T) {
	c := NewCuckoo(100_000)
	var n int
	for _, x := range items("fill", 200_000) {
		if !c.Add(x) {
			break
		}
		n++
	}
	if lf := c.LoadFactor(); lf < 0.9 || n >= 200_000 {
		t.Errorf("should fill to over 90%% and then refuse: load %.3f after %d", lf, n)
	}
}
