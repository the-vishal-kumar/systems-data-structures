// Package merkle finds the keys that differ between two replicas of a
// key-value store three ways, counting the bytes each exchanges: sending
// everything, sending a hash per key, and walking a Merkle tree (as
// Cassandra's and DynamoDB's anti-entropy repair do).
//
// The tree has a fixed shape: 2^depth leaves, a key goes to the leaf picked
// by its hash, a leaf's hash covers its keys in order, and each parent
// hashes its two children. Equal subtrees have equal hashes, so the walk
// only descends where something differs.
package merkle

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

// HashSize is the size of every hash exchanged (SHA-256).
const HashSize = 32

// Store is one replica: key → value.
type Store map[string][]byte

// Tree is a Merkle tree over a Store. nodes[1] is the root; node i has
// children 2i and 2i+1; leaves are nodes[2^depth : 2^(depth+1)].
type Tree struct {
	depth  int
	nodes  [][HashSize]byte
	leaves [][]string // keys per leaf, sorted
}

func leafOf(key string, depth int) int {
	h := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(h[:8]) >> (64 - depth))
}

// Build hashes every key once. Real systems keep the tree up to date as
// writes arrive, or rebuild it periodically.
func Build(s Store, depth int) *Tree {
	n := 1 << depth
	t := &Tree{depth: depth, nodes: make([][HashSize]byte, 2*n), leaves: make([][]string, n)}
	for k := range s {
		i := leafOf(k, depth)
		t.leaves[i] = append(t.leaves[i], k)
	}
	for i, keys := range t.leaves {
		sort.Strings(keys)
		h := sha256.New()
		for _, k := range keys {
			v := sha256.Sum256(s[k])
			h.Write([]byte(k))
			h.Write([]byte{0})
			h.Write(v[:])
		}
		copy(t.nodes[n+i][:], h.Sum(nil))
	}
	for i := n - 1; i >= 1; i-- {
		h := sha256.New()
		h.Write(t.nodes[2*i][:])
		h.Write(t.nodes[2*i+1][:])
		copy(t.nodes[i][:], h.Sum(nil))
	}
	return t
}

// Cost is what a comparison exchanged.
type Cost struct {
	Bytes      int // bytes sent between replicas
	RoundTrips int // sequential request/response exchanges
}

// Diff returns the keys whose values differ (or exist on one side only),
// found by walking both trees level by level, and the cost: two hashes per
// visited pair of children, then every key and value hash in differing
// leaves.
func Diff(a, b *Tree, sa, sb Store) ([]string, Cost) {
	if a.depth != b.depth {
		panic("merkle: trees of different depth")
	}
	cost := Cost{Bytes: HashSize, RoundTrips: 1} // compare roots
	frontier := []int{1}
	if a.nodes[1] == b.nodes[1] {
		return nil, cost
	}
	n := 1 << a.depth
	for frontier[0] < n {
		var next []int
		for _, i := range frontier {
			for _, c := range []int{2 * i, 2*i + 1} {
				cost.Bytes += HashSize
				if a.nodes[c] != b.nodes[c] {
					next = append(next, c)
				}
			}
		}
		cost.RoundTrips++
		frontier = next
	}
	// Exchange key + value hash lists for the differing leaves only.
	var diff []string
	cost.RoundTrips++
	for _, i := range frontier {
		ka, kb := a.leaves[i-n], b.leaves[i-n]
		for _, k := range kb {
			cost.Bytes += len(k) + HashSize
		}
		diff = append(diff, diffKeys(ka, kb, sa, sb)...)
	}
	sort.Strings(diff)
	return diff, cost
}

func diffKeys(ka, kb []string, sa, sb Store) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range ka {
		seen[k] = true
		vb, ok := sb[k]
		if !ok || sha256.Sum256(vb) != sha256.Sum256(sa[k]) {
			out = append(out, k)
		}
	}
	for _, k := range kb {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// FullCopyCost is sending every key and value.
func FullCopyCost(s Store) Cost {
	c := Cost{RoundTrips: 1}
	for k, v := range s {
		c.Bytes += len(k) + len(v)
	}
	return c
}

// HashListCost is sending every key with a hash of its value.
func HashListCost(s Store) Cost {
	c := Cost{RoundTrips: 1}
	for k := range s {
		c.Bytes += len(k) + HashSize
	}
	return c
}
