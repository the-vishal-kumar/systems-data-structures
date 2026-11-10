// Package filter answers "have we seen this before?" three ways:
//
//   - Set: an exact hash set. Never wrong, but stores every item.
//   - Bloom: a bit array and k hash functions. Never says "no" for an item
//     it has seen; sometimes says "yes" for one it hasn't (a false
//     positive), at a rate you choose by its size. Items can't be removed.
//   - Cuckoo: buckets of short fingerprints (Fan et al., 2014). Like Bloom,
//     but items can be removed, and it gets full instead of degrading.
package filter

import (
	"hash/fnv"
	"math"
	"math/bits"
)

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	x := h.Sum64() // splitmix64 finish, so similar strings spread out
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// ---- exact set ------------------------------------------------------------

type Set struct{ m map[string]struct{} }

func NewSet(n int) *Set                  { return &Set{make(map[string]struct{}, n)} }
func (s *Set) Add(item string)           { s.m[item] = struct{}{} }
func (s *Set) Contains(item string) bool { _, ok := s.m[item]; return ok }

// ---- Bloom filter ------------------------------------------------------------

type Bloom struct {
	bits []uint64
	m    uint64 // number of bits
	k    int    // hash functions
}

// NewBloom sizes a filter for n items at false-positive rate p:
// m = -n ln p / (ln 2)², k = (m/n) ln 2.
func NewBloom(n int, p float64) *Bloom {
	m := uint64(math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)))
	k := int(math.Round(float64(m) / float64(n) * math.Ln2))
	return &Bloom{bits: make([]uint64, (m+63)/64), m: m, k: max(k, 1)}
}

// positions uses double hashing: h1 + i·h2 stands in for k independent hashes
// (Kirsch and Mitzenmacher, 2006).
func (b *Bloom) positions(item string, f func(uint64) bool) bool {
	h := hash64(item)
	h1, h2 := h&0xffffffff, h>>32|1
	for i := range uint64(b.k) {
		if !f((h1 + i*h2) % b.m) {
			return false
		}
	}
	return true
}

func (b *Bloom) Add(item string) {
	b.positions(item, func(p uint64) bool { b.bits[p/64] |= 1 << (p % 64); return true })
}

func (b *Bloom) Contains(item string) bool {
	return b.positions(item, func(p uint64) bool { return b.bits[p/64]&(1<<(p%64)) != 0 })
}

func (b *Bloom) Bytes() int { return len(b.bits) * 8 }
func (b *Bloom) K() int     { return b.k }

// ---- cuckoo filter -----------------------------------------------------------

const slots = 4 // fingerprints per bucket

type Cuckoo struct {
	buckets [][slots]uint16 // 16-bit fingerprints; 0 means empty
	mask    uint64
	count   int
}

// NewCuckoo makes n slots (rounded up to a power-of-two number of buckets).
// With 4 slots per bucket, a cuckoo filter fills to about 95% before an
// insert fails, so size it for n/0.95 items.
func NewCuckoo(n int) *Cuckoo {
	nb := uint64(1) << bits.Len64(uint64(math.Ceil(float64(n)/slots))-1)
	return &Cuckoo{buckets: make([][slots]uint16, nb), mask: nb - 1}
}

func (c *Cuckoo) idx(item string) (fp uint16, i1, i2 uint64) {
	h := hash64(item)
	fp = uint16(h >> 48)
	if fp == 0 {
		fp = 1
	}
	i1 = h & c.mask
	return fp, i1, c.alt(i1, fp)
}

// alt is the other bucket a fingerprint may live in, computable from either
// bucket and the fingerprint alone, so entries can be moved without the item.
func (c *Cuckoo) alt(i uint64, fp uint16) uint64 {
	return (i ^ hash64(string([]byte{byte(fp), byte(fp >> 8)}))) & c.mask
}

func (c *Cuckoo) put(i uint64, fp uint16) bool {
	for s := range slots {
		if c.buckets[i][s] == 0 {
			c.buckets[i][s] = fp
			return true
		}
	}
	return false
}

// Add returns false when the filter is too full to place the item.
func (c *Cuckoo) Add(item string) bool {
	fp, i1, i2 := c.idx(item)
	if c.put(i1, fp) || c.put(i2, fp) {
		c.count++
		return true
	}
	i := i1
	for n := 0; n < 500; n++ { // kick a random resident to its other bucket
		s := n % slots
		fp, c.buckets[i][s] = c.buckets[i][s], fp
		i = c.alt(i, fp)
		if c.put(i, fp) {
			c.count++
			return true
		}
	}
	return false // a real implementation would also undo the kicks or keep a stash
}

func (c *Cuckoo) Contains(item string) bool {
	fp, i1, i2 := c.idx(item)
	for s := range slots {
		if c.buckets[i1][s] == fp || c.buckets[i2][s] == fp {
			return true
		}
	}
	return false
}

// Delete removes one copy of item's fingerprint. Only delete items that
// were added: deleting an item that merely collides removes someone else's.
func (c *Cuckoo) Delete(item string) bool {
	fp, i1, i2 := c.idx(item)
	for _, i := range []uint64{i1, i2} {
		for s := range slots {
			if c.buckets[i][s] == fp {
				c.buckets[i][s] = 0
				c.count--
				return true
			}
		}
	}
	return false
}

func (c *Cuckoo) Bytes() int { return len(c.buckets) * slots * 2 }

// LoadFactor is the share of slots in use.
func (c *Cuckoo) LoadFactor() float64 {
	return float64(c.count) / float64(len(c.buckets)*slots)
}
