// Package cache implements three eviction policies behind one interface, to
// compare hit rates: LRU, LFU and W-TinyLFU (the policy behind Caffeine and
// Ristretto).
//
// Each cache stores keys only; Access reports a hit, and on a miss inserts the
// key (if the policy admits it). That's all a hit-rate simulation needs.
package cache

import "container/list"

// Policy is a fixed-capacity cache of uint64 keys.
type Policy interface {
	// Access looks up key, reports whether it was cached, and on a miss
	// inserts it if the policy admits it.
	Access(key uint64) bool
	Len() int
	Name() string
}

// ---- LRU ----

// lruList is a capacity-bounded list of keys, most recent at the front.
type lruList struct {
	items map[uint64]*list.Element
	order *list.List
}

func newLRUList() *lruList { return &lruList{items: map[uint64]*list.Element{}, order: list.New()} }

func (l *lruList) has(k uint64) bool { _, ok := l.items[k]; return ok }
func (l *lruList) touch(k uint64)    { l.order.MoveToFront(l.items[k]) }
func (l *lruList) push(k uint64)     { l.items[k] = l.order.PushFront(k) }
func (l *lruList) len() int          { return l.order.Len() }

func (l *lruList) remove(k uint64) {
	l.order.Remove(l.items[k])
	delete(l.items, k)
}

// back returns the least recently used key.
func (l *lruList) back() (uint64, bool) {
	e := l.order.Back()
	if e == nil {
		return 0, false
	}
	return e.Value.(uint64), true
}

func (l *lruList) popBack() (uint64, bool) {
	k, ok := l.back()
	if ok {
		l.remove(k)
	}
	return k, ok
}

// LRU evicts the least recently used key.
type LRU struct {
	cap int
	l   *lruList
}

func NewLRU(capacity int) *LRU { return &LRU{cap: capacity, l: newLRUList()} }

func (c *LRU) Name() string { return "LRU" }
func (c *LRU) Len() int     { return c.l.len() }

func (c *LRU) Access(k uint64) bool {
	if c.l.has(k) {
		c.l.touch(k)
		return true
	}
	c.l.push(k)
	if c.l.len() > c.cap {
		c.l.popBack()
	}
	return false
}

// ---- LFU ----

// LFU evicts the least frequently used key (ties: least recently used), in
// O(1) with one list per frequency. Counts never decay: that's classic LFU,
// and its weakness when popularity shifts.
type LFU struct {
	cap     int
	freq    map[uint64]int
	pos     map[uint64]*list.Element
	buckets map[int]*list.List
	minFreq int
}

func NewLFU(capacity int) *LFU {
	return &LFU{cap: capacity, freq: map[uint64]int{}, pos: map[uint64]*list.Element{}, buckets: map[int]*list.List{}}
}

func (c *LFU) Name() string { return "LFU" }
func (c *LFU) Len() int     { return len(c.freq) }

func (c *LFU) bucket(f int) *list.List {
	b := c.buckets[f]
	if b == nil {
		b = list.New()
		c.buckets[f] = b
	}
	return b
}

func (c *LFU) Access(k uint64) bool {
	if f, ok := c.freq[k]; ok {
		b := c.buckets[f]
		b.Remove(c.pos[k])
		if b.Len() == 0 {
			delete(c.buckets, f)
			if c.minFreq == f {
				c.minFreq = f + 1
			}
		}
		c.freq[k] = f + 1
		c.pos[k] = c.bucket(f + 1).PushFront(k)
		return true
	}
	if len(c.freq) >= c.cap {
		b := c.buckets[c.minFreq]
		victim := b.Remove(b.Back()).(uint64)
		if b.Len() == 0 {
			delete(c.buckets, c.minFreq)
		}
		delete(c.freq, victim)
		delete(c.pos, victim)
	}
	c.freq[k] = 1
	c.pos[k] = c.bucket(1).PushFront(k)
	c.minFreq = 1
	return false
}

// ---- W-TinyLFU ----

// TinyLFU is W-TinyLFU: new keys enter a small LRU window (1% of capacity).
// When the window overflows, its oldest key competes with the main cache's
// eviction candidate, and only the one seen more often (by a small, decaying
// frequency sketch) stays. The main cache is a segmented LRU: probation
// (20%) and protected (80%). A one-off scan can't flush it, and decay lets
// it follow shifting popularity.
type TinyLFU struct {
	cap, winCap, protCap int
	window               *lruList
	probation, protected *lruList
	sketch               *Sketch
}

func NewTinyLFU(capacity int) *TinyLFU {
	win := max(1, capacity/100)
	main := capacity - win
	return &TinyLFU{
		cap: capacity, winCap: win, protCap: main * 8 / 10,
		window: newLRUList(), probation: newLRUList(), protected: newLRUList(),
		sketch: NewSketch(capacity),
	}
}

func (c *TinyLFU) Name() string { return "W-TinyLFU" }
func (c *TinyLFU) Len() int     { return c.window.len() + c.probation.len() + c.protected.len() }

func (c *TinyLFU) Access(k uint64) bool {
	c.sketch.Add(k)
	switch {
	case c.window.has(k):
		c.window.touch(k)
		return true
	case c.protected.has(k):
		c.protected.touch(k)
		return true
	case c.probation.has(k):
		// A second hit in probation promotes the key; protected's oldest
		// goes back to probation.
		c.probation.remove(k)
		c.protected.push(k)
		if c.protected.len() > c.protCap {
			old, _ := c.protected.popBack()
			c.probation.push(old)
		}
		return true
	}
	c.window.push(k)
	if c.window.len() > c.winCap {
		cand, _ := c.window.popBack()
		c.admit(cand)
	}
	return false
}

func (c *TinyLFU) admit(cand uint64) {
	if c.probation.len()+c.protected.len() < c.cap-c.winCap {
		c.probation.push(cand)
		return
	}
	from := c.probation
	victim, ok := from.back()
	if !ok {
		from = c.protected
		victim, _ = from.back()
	}
	if c.sketch.Estimate(cand) > c.sketch.Estimate(victim) {
		from.remove(victim)
		c.probation.push(cand)
	}
	// Otherwise the candidate is dropped: it hasn't earned a place.
}

// ---- frequency sketch ----

// Sketch is a count-min sketch of 4 rows of small counters (capped at 15)
// that halves every counter after 10 × capacity additions, so old
// popularity fades.
type Sketch struct {
	rows    [4][]uint8
	mask    uint64
	added   int
	resetAt int
}

func NewSketch(capacity int) *Sketch {
	w := 1
	for w < capacity*2 {
		w <<= 1
	}
	s := &Sketch{mask: uint64(w - 1), resetAt: 10 * capacity}
	for i := range s.rows {
		s.rows[i] = make([]uint8, w)
	}
	return s
}

var seeds = [4]uint64{0x9e3779b97f4a7c15, 0xbf58476d1ce4e5b9, 0x94d049bb133111eb, 0xd6e8feb86659fd93}

func mix(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ x>>31
}

func (s *Sketch) Add(k uint64) {
	for i := range s.rows {
		j := mix(k^seeds[i]) & s.mask
		if s.rows[i][j] < 15 {
			s.rows[i][j]++
		}
	}
	s.added++
	if s.added >= s.resetAt {
		for i := range s.rows {
			for j := range s.rows[i] {
				s.rows[i][j] >>= 1
			}
		}
		s.added /= 2
	}
}

// Estimate returns the minimum of k's counters: never below its true
// (decayed, capped) count, sometimes above it.
func (s *Sketch) Estimate(k uint64) uint8 {
	m := uint8(255)
	for i := range s.rows {
		m = min(m, s.rows[i][mix(k^seeds[i])&s.mask])
	}
	return m
}
