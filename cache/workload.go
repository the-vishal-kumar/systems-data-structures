package cache

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Zipf draws ranks 0..n-1 with P(rank r) proportional to 1/(r+1)^s, by
// binary search over the cumulative distribution. Unlike math/rand's Zipf,
// s may be at or below 1, which is where real web traffic usually sits.
type Zipf struct {
	cdf []float64
	rnd *rand.Rand
}

func NewZipf(n int, s float64, seed uint64) *Zipf {
	cdf := make([]float64, n)
	sum := 0.0
	for r := range n {
		sum += 1 / math.Pow(float64(r+1), s)
		cdf[r] = sum
	}
	for r := range cdf {
		cdf[r] /= sum
	}
	return &Zipf{cdf: cdf, rnd: rand.New(rand.NewPCG(seed, seed^0x5eed))}
}

func (z *Zipf) Next() int {
	return sort.SearchFloat64s(z.cdf, z.rnd.Float64())
}

// Key turns a rank into a key. Changing epoch gives every rank a new key,
// which simulates popularity moving to different items.
func Key(rank, epoch int) uint64 { return mix(uint64(rank)<<16 | uint64(epoch)) }

// HitRate runs keys through p and returns the fraction of hits.
func HitRate(p Policy, keys []uint64) float64 {
	hits := 0
	for _, k := range keys {
		if p.Access(k) {
			hits++
		}
	}
	return float64(hits) / float64(len(keys))
}
