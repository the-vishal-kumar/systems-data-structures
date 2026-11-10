// Package hll implements HyperLogLog: an estimate of how many distinct items
// a stream contains, in a fixed 2^p bytes, with a typical relative error of
// 1.04/√(2^p). Two sketches merge by taking the larger register, so daily
// counts combine into weekly ones without the raw data.
package hll

import (
	"math"
	"math/bits"
)

// Sketch is a HyperLogLog with 2^p one-byte registers.
type Sketch struct {
	p   uint8
	reg []uint8
}

// New returns a sketch with precision p (4..18). Memory is 2^p bytes.
func New(p uint8) *Sketch {
	if p < 4 || p > 18 {
		panic("hll: precision must be 4..18")
	}
	return &Sketch{p: p, reg: make([]uint8, 1<<p)}
}

// Hash mixes a 64-bit value (SplitMix64's finaliser), so sequential IDs
// spread evenly. Items that are strings should be hashed first (FNV-1a, xxHash).
func Hash(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// AddHash adds an item by its 64-bit hash. The first p bits pick a
// register; the register keeps the longest run of leading zeros (+1) seen in
// the remaining bits. Long runs are rare, so they hint at many items.
func (s *Sketch) AddHash(h uint64) {
	idx := h >> (64 - s.p)
	rest := h<<s.p | 1<<(s.p-1) // a sentinel bit caps the run length
	rank := uint8(bits.LeadingZeros64(rest)) + 1
	if rank > s.reg[idx] {
		s.reg[idx] = rank
	}
}

// Add adds an integer item (an ID).
func (s *Sketch) Add(x uint64) { s.AddHash(Hash(x)) }

// Count estimates the number of distinct items added.
func (s *Sketch) Count() uint64 {
	m := float64(len(s.reg))
	sum, zeros := 0.0, 0
	for _, r := range s.reg {
		sum += 1 / float64(uint64(1)<<r)
		if r == 0 {
			zeros++
		}
	}
	alpha := 0.7213 / (1 + 1.079/m)
	switch m {
	case 16:
		alpha = 0.673
	case 32:
		alpha = 0.697
	case 64:
		alpha = 0.709
	}
	est := alpha * m * m / sum
	if est <= 2.5*m && zeros > 0 {
		// Small cardinalities: linear counting on empty registers is more accurate.
		est = m * math.Log(m/float64(zeros))
	}
	return uint64(est + 0.5)
}

// Merge folds other into s, as if every item added to other had been added
// to s. Both must have the same precision.
func (s *Sketch) Merge(other *Sketch) {
	if s.p != other.p {
		panic("hll: merging sketches of different precision")
	}
	for i, r := range other.reg {
		if r > s.reg[i] {
			s.reg[i] = r
		}
	}
}

// Bytes is the sketch's memory: one byte per register.
func (s *Sketch) Bytes() int { return len(s.reg) }

// StdError is the expected relative standard error, 1.04/√m.
func (s *Sketch) StdError() float64 { return 1.04 / math.Sqrt(float64(len(s.reg))) }
