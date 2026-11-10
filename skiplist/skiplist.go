// Package skiplist implements a leaderboard (a sorted set: member → score,
// highest score first) three ways: a skip list with rank spans, like Redis's
// sorted sets; a sorted slice; and a plain map that counts higher scores to
// find a rank.
package skiplist

import (
	"math/rand/v2"
	"sort"
)

// Board is a leaderboard. Ranks start at 1 (the highest score); ties are
// ordered by member name.
type Board interface {
	Set(member string, score float64)
	Rank(member string) int // 0 if absent
	Top(n int) []Entry
	Name() string
}

type Entry struct {
	Member string
	Score  float64
}

// before reports whether (s1, m1) ranks ahead of (s2, m2).
func before(s1 float64, m1 string, s2 float64, m2 string) bool {
	return s1 > s2 || (s1 == s2 && m1 < m2)
}

// ---- skip list ----

const (
	maxLevel = 32
	prob     = 0.25 // each level has about a quarter of the nodes below it
)

type level struct {
	next *node
	span int // how many level-0 steps this link skips: what makes Rank O(log n)
}

type node struct {
	member string
	score  float64
	lv     []level
}

// SkipList keeps members ordered across a tower of linked lists. Updates
// and rank lookups take O(log n) expected time.
type SkipList struct {
	head   *node
	level  int
	length int
	scores map[string]float64
	rnd    *rand.Rand
}

func NewSkipList() *SkipList {
	return &SkipList{
		head: &node{lv: make([]level, maxLevel)}, level: 1,
		scores: map[string]float64{}, rnd: rand.New(rand.NewPCG(1, 2)),
	}
}

func (s *SkipList) Name() string { return "skip list" }
func (s *SkipList) Len() int     { return s.length }

func (s *SkipList) randomLevel() int {
	l := 1
	for l < maxLevel && s.rnd.Float64() < prob {
		l++
	}
	return l
}

func (s *SkipList) Set(member string, score float64) {
	if old, ok := s.scores[member]; ok {
		if old == score {
			return
		}
		s.remove(member, old)
	}
	s.insert(member, score)
	s.scores[member] = score
}

func (s *SkipList) insert(member string, score float64) {
	var update [maxLevel]*node
	var rank [maxLevel]int
	x := s.head
	for i := s.level - 1; i >= 0; i-- {
		if i < s.level-1 {
			rank[i] = rank[i+1]
		}
		for x.lv[i].next != nil && before(x.lv[i].next.score, x.lv[i].next.member, score, member) {
			rank[i] += x.lv[i].span
			x = x.lv[i].next
		}
		update[i] = x
	}
	lvl := s.randomLevel()
	if lvl > s.level {
		for i := s.level; i < lvl; i++ {
			rank[i] = 0
			update[i] = s.head
			update[i].lv[i].span = s.length
		}
		s.level = lvl
	}
	n := &node{member: member, score: score, lv: make([]level, lvl)}
	for i := range lvl {
		n.lv[i].next = update[i].lv[i].next
		update[i].lv[i].next = n
		n.lv[i].span = update[i].lv[i].span - (rank[0] - rank[i])
		update[i].lv[i].span = rank[0] - rank[i] + 1
	}
	for i := lvl; i < s.level; i++ {
		update[i].lv[i].span++
	}
	s.length++
}

func (s *SkipList) remove(member string, score float64) {
	var update [maxLevel]*node
	x := s.head
	for i := s.level - 1; i >= 0; i-- {
		for x.lv[i].next != nil && before(x.lv[i].next.score, x.lv[i].next.member, score, member) {
			x = x.lv[i].next
		}
		update[i] = x
	}
	x = x.lv[0].next
	if x == nil || x.member != member {
		return
	}
	for i := range s.level {
		if update[i].lv[i].next == x {
			update[i].lv[i].span += x.lv[i].span - 1
			update[i].lv[i].next = x.lv[i].next
		} else {
			update[i].lv[i].span--
		}
	}
	for s.level > 1 && s.head.lv[s.level-1].next == nil {
		s.level--
	}
	s.length--
	delete(s.scores, member)
}

func (s *SkipList) Rank(member string) int {
	score, ok := s.scores[member]
	if !ok {
		return 0
	}
	rank, x := 0, s.head
	for i := s.level - 1; i >= 0; i-- {
		for x.lv[i].next != nil && (before(x.lv[i].next.score, x.lv[i].next.member, score, member) || x.lv[i].next.member == member) {
			rank += x.lv[i].span
			x = x.lv[i].next
		}
		if x.member == member && x != s.head {
			return rank
		}
	}
	return 0
}

func (s *SkipList) Top(n int) []Entry {
	var out []Entry
	for x := s.head.lv[0].next; x != nil && len(out) < n; x = x.lv[0].next {
		out = append(out, Entry{x.member, x.score})
	}
	return out
}

// ---- sorted slice ----

// Sorted keeps entries in one sorted slice: rank by binary search, but every
// update shifts on average half the slice.
type Sorted struct {
	entries []Entry
	scores  map[string]float64
}

func NewSorted() *Sorted       { return &Sorted{scores: map[string]float64{}} }
func (s *Sorted) Name() string { return "sorted slice" }
func (s *Sorted) find(m string, sc float64) int {
	return sort.Search(len(s.entries), func(i int) bool {
		return !before(s.entries[i].Score, s.entries[i].Member, sc, m)
	})
}

func (s *Sorted) Set(member string, score float64) {
	if old, ok := s.scores[member]; ok {
		i := s.find(member, old)
		s.entries = append(s.entries[:i], s.entries[i+1:]...)
	}
	i := s.find(member, score)
	s.entries = append(s.entries, Entry{})
	copy(s.entries[i+1:], s.entries[i:])
	s.entries[i] = Entry{member, score}
	s.scores[member] = score
}

// Load replaces the contents with entries, sorting once: building a large
// board with Set would be quadratic.
func (s *Sorted) Load(entries []Entry) {
	s.entries = append([]Entry(nil), entries...)
	s.scores = make(map[string]float64, len(entries))
	for _, e := range entries {
		s.scores[e.Member] = e.Score
	}
	sort.Slice(s.entries, func(i, j int) bool {
		return before(s.entries[i].Score, s.entries[i].Member, s.entries[j].Score, s.entries[j].Member)
	})
}

func (s *Sorted) Rank(member string) int {
	sc, ok := s.scores[member]
	if !ok {
		return 0
	}
	return s.find(member, sc) + 1
}

func (s *Sorted) Top(n int) []Entry {
	return append([]Entry(nil), s.entries[:min(n, len(s.entries))]...)
}

// ---- map + scan ----

// Scan stores scores in a map: updates are O(1), but a rank means counting
// everyone ahead, and a top-n means sorting everything.
type Scan struct{ scores map[string]float64 }

func NewScan() *Scan                             { return &Scan{scores: map[string]float64{}} }
func (s *Scan) Name() string                     { return "map + scan" }
func (s *Scan) Set(member string, score float64) { s.scores[member] = score }

func (s *Scan) Rank(member string) int {
	sc, ok := s.scores[member]
	if !ok {
		return 0
	}
	r := 1
	for m, x := range s.scores {
		if before(x, m, sc, member) {
			r++
		}
	}
	return r
}

func (s *Scan) Top(n int) []Entry {
	all := make([]Entry, 0, len(s.scores))
	for m, x := range s.scores {
		all = append(all, Entry{m, x})
	}
	sort.Slice(all, func(i, j int) bool { return before(all[i].Score, all[i].Member, all[j].Score, all[j].Member) })
	return all[:min(n, len(all))]
}
