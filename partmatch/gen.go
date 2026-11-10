package partmatch

import (
	"fmt"
	"math/rand/v2"
)

var families = []struct{ prefix, words string }{
	{"BRG", "deep groove ball bearing sealed"}, {"VLV", "hydraulic directional valve"},
	{"BLT", "hex bolt zinc plated"}, {"NUT", "hex nut stainless steel"},
	{"SEL", "oil seal nitrile rubber"}, {"FLT", "hydraulic filter element"},
	{"HOS", "hydraulic hose assembly"}, {"PMP", "gear pump cast iron"},
	{"CYL", "hydraulic cylinder double acting"}, {"FTG", "pipe fitting brass elbow"},
}

// Catalog generates n distinct normalized SKUs (like BRG6204RS) and their
// descriptions, from a fixed seed.
func Catalog(n int, seed uint64) (skus []string, descs [][]string) {
	r := rand.New(rand.NewPCG(seed, 9))
	seen := map[string]bool{}
	for len(skus) < n {
		f := families[r.IntN(len(families))]
		num := r.IntN(10000)
		sku := fmt.Sprintf("%s%04d%c%c", f.prefix, num, 'A'+rune(r.IntN(26)), 'A'+rune(r.IntN(26)))
		if seen[sku] {
			continue
		}
		seen[sku] = true
		skus = append(skus, sku)
		descs = append(descs, Tokens(fmt.Sprintf("%s %dmm series %d grade %c", f.words, 4+r.IntN(60), num, 'a'+rune(r.IntN(6)))))
	}
	return skus, descs
}

// Typo applies one random edit (substitute, delete or insert a character).
func Typo(s string, r *rand.Rand) string {
	i := r.IntN(len(s))
	c := byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[r.IntN(36)])
	switch r.IntN(3) {
	case 0:
		return s[:i] + string(c) + s[i+1:]
	case 1:
		return s[:i] + s[i+1:]
	default:
		return s[:i] + string(c) + s[i:]
	}
}

// Rephrase keeps some of a description's tokens, in another order, and adds
// a word the catalog doesn't use: how people write a part they can't name.
func Rephrase(d []string, r *rand.Rand) []string {
	out := []string{"please"}
	for _, i := range r.Perm(len(d)) {
		if r.Float64() < 0.7 {
			out = append(out, d[i])
		}
	}
	return out
}
