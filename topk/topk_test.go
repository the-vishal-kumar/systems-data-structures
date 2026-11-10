package topk

import (
	"math/rand/v2"
	"testing"
)

// stream: keys 0..9 are heavy (10,000 each, decreasing), plus 200,000
// light keys seen a few times each, shuffled.
func stream() []uint64 {
	var s []uint64
	for k := range uint64(10) {
		for range 10_000 - 500*int(k) {
			s = append(s, k)
		}
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 600_000 {
		s = append(s, 100+uint64(r.IntN(200_000)))
	}
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s
}

func feed(c Counter, s []uint64) Counter {
	for _, k := range s {
		c.Add(k)
	}
	return c
}

func TestAllFindTheHeavyHitters(t *testing.T) {
	s := stream()
	for _, c := range []Counter{NewExact(), NewCMS(4096, 4, 10), NewSpaceSaving(200)} {
		top := feed(c, s).Top(10)
		for i, it := range top {
			if it.Key != uint64(i) {
				t.Fatalf("%s: rank %d is key %d, want %d (%v)", c.Name(), i+1, it.Key, i, top)
			}
		}
	}
}

func TestCMSNeverUndercounts(t *testing.T) {
	s := stream()
	c := feed(NewCMS(256, 4, 10), s).(*CMS) // deliberately narrow
	for _, it := range c.Top(10) {
		if want := uint64(10_000 - 500*int(it.Key)); it.Key < 10 && it.Count < want {
			t.Fatalf("key %d: estimate %d below true %d", it.Key, it.Count, want)
		}
	}
}

func TestSpaceSavingBounds(t *testing.T) {
	s := stream()
	ss := feed(NewSpaceSaving(200), s).(*SpaceSaving)
	for _, it := range ss.Top(10) {
		truth := uint64(10_000 - 500*int(it.Key))
		if it.Count < truth || ss.Guaranteed(it.Key) > truth {
			t.Fatalf("key %d: estimate %d, guaranteed %d, true %d", it.Key, it.Count, ss.Guaranteed(it.Key), truth)
		}
	}
}
