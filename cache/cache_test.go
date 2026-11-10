package cache

import "testing"

func policies(capacity int) []Policy {
	return []Policy{NewLRU(capacity), NewLFU(capacity), NewTinyLFU(capacity)}
}

func TestCapacityAndHits(t *testing.T) {
	for _, p := range policies(100) {
		for k := range uint64(1000) {
			p.Access(k)
			if p.Len() > 100 {
				t.Fatalf("%s holds %d keys, capacity 100", p.Name(), p.Len())
			}
		}
	}
	// A key just accessed in an otherwise idle cache is a hit next time.
	for _, p := range policies(10) {
		if p.Access(42) {
			t.Fatalf("%s: first access can't be a hit", p.Name())
		}
		if !p.Access(42) {
			t.Fatalf("%s: second access should hit", p.Name())
		}
	}
}

func TestLRUEvictsOldest(t *testing.T) {
	c := NewLRU(2)
	c.Access(1)
	c.Access(2)
	c.Access(1) // 2 is now oldest
	c.Access(3)
	if !c.Access(1) || c.Access(2) { // check 1 first: a miss on 2 would evict it
		t.Fatal("LRU evicted the wrong key")
	}
}

func TestLFUEvictsLeastFrequent(t *testing.T) {
	c := NewLFU(2)
	c.Access(1)
	c.Access(1)
	c.Access(2)
	c.Access(3) // evicts 2 (count 1), not 1 (count 2)
	if !c.Access(1) || c.Access(2) {
		t.Fatal("LFU evicted the wrong key")
	}
}

func TestSketchNeverUndercounts(t *testing.T) {
	s := NewSketch(1000)
	for k := range uint64(500) {
		for range k % 10 {
			s.Add(k)
		}
	}
	for k := range uint64(500) {
		if got := s.Estimate(k); int(got) < int(k%10) {
			t.Fatalf("key %d: estimate %d below true count %d", k, got, k%10)
		}
	}
}

// The properties the post relies on: on skewed traffic W-TinyLFU beats LRU;
// a one-off scan hurts LRU far more than W-TinyLFU.
func TestPolicyProperties(t *testing.T) {
	z := NewZipf(100_000, 0.9, 1)
	keys := make([]uint64, 400_000)
	for i := range keys {
		keys[i] = Key(z.Next(), 0)
	}
	lru, tiny := NewLRU(1000), NewTinyLFU(1000)
	lr, tr := HitRate(lru, keys), HitRate(tiny, keys)
	if tr <= lr {
		t.Fatalf("zipf: W-TinyLFU %.3f should beat LRU %.3f", tr, lr)
	}
	scan := make([]uint64, 20_000)
	for i := range scan {
		scan[i] = Key(1_000_000+i, 0)
	}
	HitRate(lru, scan)
	HitRate(tiny, scan)
	after := keys[:20_000]
	if l, w := HitRate(lru, after), HitRate(tiny, after); w-l < 0.05 {
		t.Fatalf("after a scan: W-TinyLFU %.3f should beat LRU %.3f clearly", w, l)
	}
}
