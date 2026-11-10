// Package topk finds the most frequent items in a stream ("heavy hitters")
// three ways: an exact count of everything, a Count-Min Sketch plus a small
// heap, and Space-Saving, which keeps only k counters.
package topk

import (
	"container/heap"
	"sort"
)

// Item is a key and its (estimated) count.
type Item struct {
	Key   uint64
	Count uint64
}

// Counter is a stream of keys in, the top n out.
type Counter interface {
	Add(key uint64)
	Top(n int) []Item
	Bytes() int // approximate memory
	Name() string
}

func sortItems(items []Item) []Item {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Key < items[j].Key
	})
	return items
}

// ---- exact ----

// Exact counts every key: perfect answers, memory grows with distinct keys.
type Exact struct{ counts map[uint64]uint64 }

func NewExact() *Exact                 { return &Exact{counts: map[uint64]uint64{}} }
func (e *Exact) Name() string          { return "exact map" }
func (e *Exact) Add(k uint64)          { e.counts[k]++ }
func (e *Exact) Count(k uint64) uint64 { return e.counts[k] }

// Bytes estimates a Go map at ~40 bytes per uint64→uint64 entry.
func (e *Exact) Bytes() int { return 40 * len(e.counts) }

func (e *Exact) Top(n int) []Item {
	all := make([]Item, 0, len(e.counts))
	for k, c := range e.counts {
		all = append(all, Item{k, c})
	}
	all = sortItems(all)
	return all[:min(n, len(all))]
}

// ---- min-heap of candidates, shared by both sketches ----

type entry struct {
	key        uint64
	count, err uint64
	idx        int
}

type minHeap []*entry

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].count < h[j].count }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].idx = i; h[j].idx = j }
func (h *minHeap) Push(x any)        { e := x.(*entry); e.idx = len(*h); *h = append(*h, e) }
func (h *minHeap) Pop() any {
	old := *h
	e := old[len(old)-1]
	*h = old[:len(old)-1]
	return e
}

// ---- Count-Min Sketch + heap ----

// CMS estimates every key's count in a fixed width × depth grid of
// counters (never undercounting; overcounting by at most ε·N with high
// probability, ε = e/width), and keeps the k keys with the highest
// estimates in a min-heap.
type CMS struct {
	width, depth, k int
	rows            [][]uint64
	heap            minHeap
	inHeap          map[uint64]*entry
}

func NewCMS(width, depth, k int) *CMS {
	c := &CMS{width: width, depth: depth, k: k, inHeap: map[uint64]*entry{}}
	c.rows = make([][]uint64, depth)
	for i := range c.rows {
		c.rows[i] = make([]uint64, width)
	}
	return c
}

func (c *CMS) Name() string { return "count-min sketch + heap" }
func (c *CMS) Bytes() int   { return 8*c.width*c.depth + 64*c.k }

func mix(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ x>>31
}

func (c *CMS) Add(k uint64) {
	est := ^uint64(0)
	for i, row := range c.rows {
		j := mix(k+uint64(i)*0x9e3779b97f4a7c15) % uint64(c.width)
		row[j]++
		est = min(est, row[j])
	}
	if e, ok := c.inHeap[k]; ok {
		e.count = est
		heap.Fix(&c.heap, e.idx)
		return
	}
	if len(c.heap) < c.k {
		e := &entry{key: k, count: est}
		heap.Push(&c.heap, e)
		c.inHeap[k] = e
		return
	}
	if est > c.heap[0].count {
		old := c.heap[0]
		delete(c.inHeap, old.key)
		old.key, old.count = k, est
		heap.Fix(&c.heap, 0)
		c.inHeap[k] = old
	}
}

func (c *CMS) Top(n int) []Item {
	out := make([]Item, 0, len(c.heap))
	for _, e := range c.heap {
		out = append(out, Item{e.key, e.count})
	}
	out = sortItems(out)
	return out[:min(n, len(out))]
}

// ---- Space-Saving ----

// SpaceSaving keeps exactly k counters. A new key takes over the smallest
// counter and inherits its count (+1), so counts are overestimates by at
// most the inherited amount, which is tracked as the error. Any key with
// more than N/k occurrences is guaranteed to be kept.
type SpaceSaving struct {
	k    int
	heap minHeap
	keys map[uint64]*entry
}

func NewSpaceSaving(k int) *SpaceSaving {
	return &SpaceSaving{k: k, keys: make(map[uint64]*entry, k)}
}

func (s *SpaceSaving) Name() string { return "Space-Saving" }
func (s *SpaceSaving) Bytes() int   { return 64 * s.k }

func (s *SpaceSaving) Add(k uint64) {
	if e, ok := s.keys[k]; ok {
		e.count++
		heap.Fix(&s.heap, e.idx)
		return
	}
	if len(s.heap) < s.k {
		e := &entry{key: k, count: 1}
		heap.Push(&s.heap, e)
		s.keys[k] = e
		return
	}
	m := s.heap[0]
	delete(s.keys, m.key)
	m.key, m.err, m.count = k, m.count, m.count+1
	heap.Fix(&s.heap, 0)
	s.keys[k] = m
}

func (s *SpaceSaving) Top(n int) []Item {
	out := make([]Item, 0, len(s.heap))
	for _, e := range s.heap {
		out = append(out, Item{e.key, e.count})
	}
	out = sortItems(out)
	return out[:min(n, len(out))]
}

// Guaranteed reports, for a key in the top list, the count it certainly
// reached (estimate minus inherited error).
func (s *SpaceSaving) Guaranteed(k uint64) uint64 {
	if e, ok := s.keys[k]; ok {
		return e.count - e.err
	}
	return 0
}
