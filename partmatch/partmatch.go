// Package partmatch finds catalog parts from messy input: a part number with
// a typo, a prefix, or a description in other words. Each question has a
// brute-force answer (compare against every item) and an index that answers
// it without looking at most of the catalog:
//
//   - typo in a SKU:      linear scan  vs  BK-tree  vs  symmetric-delete index
//   - SKU prefix:         linear scan  vs  sorted array + binary search
//   - description words:  linear scan  vs  inverted index (token -> items)
//
// The matcher in llm-document-ingestion (internal/catalog) uses the linear
// scans, which is the right call for a few thousand parts; this package
// measures where that stops being true.
package partmatch

import (
	"slices"
	"strings"
)

// Levenshtein is the edit distance (insertions, deletions, substitutions).
// It's a true metric, which the BK-tree needs.
func Levenshtein(a, b string) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// ---- typos: linear scan --------------------------------------------------------

// Within returns the indexes of keys within distance d of q, by comparing q
// with every key.
func Within(keys []string, q string, d int) []int {
	var out []int
	for i, k := range keys {
		if abs(len(k)-len(q)) <= d && Levenshtein(k, q) <= d {
			out = append(out, i)
		}
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ---- typos: BK-tree ---------------------------------------------------------------

// BKTree (Burkhard and Keller, 1973) arranges keys so that, by the triangle
// inequality, a query within d of the target only needs to visit children
// whose edge distance is within d of the query's distance to the node.
type BKTree struct {
	root *bkNode
	keys []string
}

type bkNode struct {
	idx      int
	children map[int]*bkNode
}

func NewBKTree(keys []string) *BKTree {
	t := &BKTree{keys: keys}
	for i := range keys {
		t.insert(i)
	}
	return t
}

func (t *BKTree) insert(i int) {
	if t.root == nil {
		t.root = &bkNode{idx: i}
		return
	}
	n := t.root
	for {
		d := Levenshtein(t.keys[n.idx], t.keys[i])
		if d == 0 {
			return // duplicate key
		}
		if n.children == nil {
			n.children = map[int]*bkNode{}
		}
		c, ok := n.children[d]
		if !ok {
			n.children[d] = &bkNode{idx: i}
			return
		}
		n = c
	}
}

// Within returns the indexes of keys within d of q, and how many distances
// it computed (the work a linear scan does for every key).
func (t *BKTree) Within(q string, d int) (out []int, comparisons int) {
	stack := []*bkNode{t.root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		dist := Levenshtein(t.keys[n.idx], q)
		comparisons++
		if dist <= d {
			out = append(out, n.idx)
		}
		for e, c := range n.children {
			if e >= dist-d && e <= dist+d {
				stack = append(stack, c)
			}
		}
	}
	slices.Sort(out)
	return out, comparisons
}

// ---- typos: symmetric delete ------------------------------------------------------

// SymDelete indexes every key under itself and every string made by
// deleting one character (the idea behind SymSpell). Two strings within
// edit distance 1 always share an entry, so a lookup only checks the keys
// filed under the query's own deletions, then verifies them.
type SymDelete struct {
	keys  []string
	index map[string][]int
}

func deletes1(s string) []string {
	out := []string{s}
	for i := range len(s) {
		out = append(out, s[:i]+s[i+1:])
	}
	return out
}

func NewSymDelete(keys []string) *SymDelete {
	s := &SymDelete{keys: keys, index: map[string][]int{}}
	for i, k := range keys {
		for _, v := range deletes1(k) {
			if l := s.index[v]; len(l) == 0 || l[len(l)-1] != i {
				s.index[v] = append(l, i)
			}
		}
	}
	return s
}

// Within1 returns the indexes of keys within distance 1 of q.
func (s *SymDelete) Within1(q string) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range deletes1(q) {
		for _, i := range s.index[v] {
			if !seen[i] {
				seen[i] = true
				if Levenshtein(s.keys[i], q) <= 1 {
					out = append(out, i)
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

// Entries is the size of the index: what it costs in memory.
func (s *SymDelete) Entries() (keys, postings int) {
	for _, l := range s.index {
		postings += len(l)
	}
	return len(s.index), postings
}

// ---- prefixes ---------------------------------------------------------------------

// PrefixScan returns the keys starting with p, by checking every key.
func PrefixScan(keys []string, p string) []int {
	var out []int
	for i, k := range keys {
		if strings.HasPrefix(k, p) {
			out = append(out, i)
		}
	}
	return out
}

// Sorted keeps the keys in order, so every key with a given prefix sits in
// one contiguous run found by binary search.
type Sorted struct {
	keys  []string
	order []int // order[j] = index of the j-th smallest key
}

func NewSorted(keys []string) *Sorted {
	s := &Sorted{keys: keys, order: make([]int, len(keys))}
	for i := range s.order {
		s.order[i] = i
	}
	slices.SortFunc(s.order, func(a, b int) int { return strings.Compare(keys[a], keys[b]) })
	return s
}

func (s *Sorted) Prefix(p string) []int {
	lo, _ := slices.BinarySearchFunc(s.order, p, func(i int, p string) int { return strings.Compare(s.keys[i], p) })
	var out []int
	for j := lo; j < len(s.order) && strings.HasPrefix(s.keys[s.order[j]], p); j++ {
		out = append(out, s.order[j])
	}
	slices.Sort(out)
	return out
}

// ---- descriptions -------------------------------------------------------------------

// Tokens lower-cases and splits at anything that isn't a letter or digit.
func Tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

func jaccard(a, b []string) float64 {
	inter := 0
	for _, x := range a {
		if slices.Contains(b, x) {
			inter++
		}
	}
	u := len(a) + len(b) - inter
	if u == 0 {
		return 0
	}
	return float64(inter) / float64(u)
}

// BestScan scores every item's tokens against the query and returns the best.
func BestScan(docs [][]string, q []string) (best int, score float64) {
	best = -1
	for i, d := range docs {
		if s := jaccard(q, d); s > score {
			best, score = i, s
		}
	}
	return best, score
}

// Inverted maps each token to the items containing it, so only items that
// share at least one token with the query are scored.
type Inverted struct {
	docs  [][]string
	index map[string][]int
}

func NewInverted(docs [][]string) *Inverted {
	ix := &Inverted{docs: docs, index: map[string][]int{}}
	for i, d := range docs {
		for _, t := range d {
			if l := ix.index[t]; len(l) == 0 || l[len(l)-1] != i {
				ix.index[t] = append(l, i)
			}
		}
	}
	return ix
}

// Best returns the same answer as BestScan, and how many items it scored.
// Items are visited in index order, so ties resolve the same way.
func (ix *Inverted) Best(q []string) (best int, score float64, scored int) {
	seen := map[int]bool{}
	var cands []int
	for _, t := range q {
		for _, i := range ix.index[t] {
			if !seen[i] {
				seen[i] = true
				cands = append(cands, i)
			}
		}
	}
	slices.Sort(cands)
	best = -1
	for _, i := range cands {
		if s := jaccard(q, ix.docs[i]); s > score {
			best, score = i, s
		}
	}
	return best, score, len(cands)
}

// BestSelective gathers candidates only through tokens that appear in at
// most maxDF of the items (rare words say more), then scores candidates on
// all tokens. It's faster when queries contain common words, and it can
// miss the best item when that item shares only common words with the
// query; if no token is selective, it falls back to every token.
func (ix *Inverted) BestSelective(q []string, maxDF float64) (best int, score float64, scored int) {
	limit := int(maxDF * float64(len(ix.docs)))
	var use []string
	for _, t := range q {
		if l := len(ix.index[t]); l > 0 && l <= limit {
			use = append(use, t)
		}
	}
	if len(use) == 0 {
		return ix.Best(q)
	}
	seen := map[int]bool{}
	var cands []int
	for _, t := range use {
		for _, i := range ix.index[t] {
			if !seen[i] {
				seen[i] = true
				cands = append(cands, i)
			}
		}
	}
	slices.Sort(cands)
	best = -1
	for _, i := range cands {
		if s := jaccard(q, ix.docs[i]); s > score {
			best, score = i, s
		}
	}
	return best, score, len(cands)
}
