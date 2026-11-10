// Package hashring assigns keys to nodes four ways, so they can be compared
// on what matters when nodes come and go: how many keys move, how evenly
// keys spread, and what a lookup costs.
//
//   - ModN: hash(key) % N. Simple, perfectly even, and moves almost every
//     key when N changes.
//   - Ring: consistent hashing. Each node owns the arcs of a circle before
//     its points; virtual nodes (many points per node) even out the arcs.
//   - Rendezvous: highest random weight. Every node scores the key; the
//     highest score wins. No ring to maintain, O(N) per lookup.
//   - Jump: Lamping and Veach's jump consistent hash (2014). Tiny and even,
//     but nodes are numbered 0..N-1, so only the last one can be removed.
package hashring

import (
	"hash/fnv"
	"slices"
	"strconv"
)

// Hash is FNV-1a 64 finished with a splitmix64 mixer, so that similar keys
// ("user-1", "user-2") land far apart.
func Hash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return mix(h.Sum64())
}

func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Picker maps a key to one of its nodes.
type Picker interface {
	Pick(key string) string
}

// ---- mod N ----------------------------------------------------------------

type ModN struct{ nodes []string }

func NewModN(nodes []string) *ModN { return &ModN{slices.Clone(nodes)} }

func (m *ModN) Pick(key string) string { return m.nodes[Hash(key)%uint64(len(m.nodes))] }

// ---- consistent hashing ring ----------------------------------------------

type Ring struct {
	points []uint64 // sorted
	owner  map[uint64]string
}

// NewRing places vnodes points on the ring for every node.
func NewRing(nodes []string, vnodes int) *Ring {
	r := &Ring{owner: make(map[uint64]string, len(nodes)*vnodes)}
	for _, n := range nodes {
		for i := range vnodes {
			p := Hash(n + "#" + strconv.Itoa(i))
			r.points = append(r.points, p)
			r.owner[p] = n
		}
	}
	slices.Sort(r.points)
	return r
}

// Pick returns the owner of the first point at or after the key's hash,
// wrapping around the circle.
func (r *Ring) Pick(key string) string {
	h := Hash(key)
	i, _ := slices.BinarySearch(r.points, h)
	if i == len(r.points) {
		i = 0
	}
	return r.owner[r.points[i]]
}

// Points is the ring's size: what it costs in memory.
func (r *Ring) Points() int { return len(r.points) }

// ---- rendezvous (highest random weight) -------------------------------------

type Rendezvous struct {
	nodes []string
	seeds []uint64
}

func NewRendezvous(nodes []string) *Rendezvous {
	r := &Rendezvous{nodes: slices.Clone(nodes)}
	for _, n := range nodes {
		r.seeds = append(r.seeds, Hash(n))
	}
	return r
}

func (r *Rendezvous) Pick(key string) string {
	k := Hash(key)
	best, bestScore := 0, uint64(0)
	for i, s := range r.seeds {
		if score := mix(k ^ s); score >= bestScore {
			best, bestScore = i, score
		}
	}
	return r.nodes[best]
}

// ---- jump consistent hash -----------------------------------------------------

type Jump struct{ nodes []string }

func NewJump(nodes []string) *Jump { return &Jump{slices.Clone(nodes)} }

// JumpHash is the algorithm from Lamping and Veach, "A Fast, Minimal Memory,
// Consistent Hash Algorithm" (2014).
func JumpHash(key uint64, buckets int) int {
	var b, j int64 = -1, 0
	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}
	return int(b)
}

func (j *Jump) Pick(key string) string { return j.nodes[JumpHash(Hash(key), len(j.nodes))] }
