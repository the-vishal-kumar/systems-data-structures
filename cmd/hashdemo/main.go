// Command hashdemo compares mod-N, a consistent-hashing ring, rendezvous
// hashing and jump hash on keys moved, balance and lookup cost.
//
//	go run ./cmd/hashdemo
package main

import (
	"fmt"
	"runtime"

	"github.com/the-vishal-kumar/systems-data-structures/hashring"
)

func alive(nodes []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range nodes {
		m[n] = true
	}
	return m
}

func main() {
	keys := hashring.Keys(1_000_000)
	ten, eleven := hashring.Nodes(10), hashring.Nodes(11)
	nine := append(append([]string{}, ten[:3]...), ten[4:]...)
	nineLast := ten[:9]
	fmt.Printf("1,000,000 keys, Go %s.\n\n", runtime.Version())

	type alg struct {
		name string
		mk   func([]string) hashring.Picker
	}
	algs := []alg{
		{"mod N", func(n []string) hashring.Picker { return hashring.NewModN(n) }},
		{"ring, 1 point per node", func(n []string) hashring.Picker { return hashring.NewRing(n, 1) }},
		{"ring, 100 points per node", func(n []string) hashring.Picker { return hashring.NewRing(n, 100) }},
		{"rendezvous", func(n []string) hashring.Picker { return hashring.NewRendezvous(n) }},
		{"jump hash", func(n []string) hashring.Picker { return hashring.NewJump(n) }},
	}

	fmt.Print("### Adding an 11th node to 10 (ideal: 1/11 = 9.1% of keys move, all to the new node)\n\n")
	fmt.Println("| Algorithm | Keys moved | Moved between old nodes (needless) |\n|---|---:|---:|")
	for _, a := range algs {
		m, _ := hashring.Moved(a.mk(ten), a.mk(eleven), keys, alive(ten))
		fmt.Printf("| %s | %.1f%% | %s |\n", a.name, m*100, needlessAdd(a.mk(ten), a.mk(eleven), keys))
	}

	fmt.Print("\n### Removing one node of 10 (ideal: 10% move, only the failed node's keys)\n\n")
	fmt.Println("| Algorithm | Node removed | Keys moved | Moved although their node is still up |\n|---|---|---:|---:|")
	for _, a := range algs {
		after, removed := nine, "cache-03 (any node)"
		if a.name == "jump hash" {
			after, removed = nineLast, "cache-09 (only the last one is possible)"
		}
		m, needless := hashring.Moved(a.mk(ten), a.mk(after), keys, alive(after))
		fmt.Printf("| %s | %s | %.1f%% | %.1f%% |\n", a.name, removed, m*100, needless*100)
	}

	fmt.Print("\n### Balance across 10 nodes\n\n")
	fmt.Println("| Algorithm | Busiest node vs average | Std. deviation of load |\n|---|---:|---:|")
	bal := []alg{algs[0], algs[1],
		{"ring, 10 points per node", func(n []string) hashring.Picker { return hashring.NewRing(n, 10) }},
		algs[2],
		{"ring, 1,000 points per node", func(n []string) hashring.Picker { return hashring.NewRing(n, 1000) }},
		algs[3], algs[4]}
	for _, a := range bal {
		mx, sd := hashring.Spread(a.mk(ten), keys)
		fmt.Printf("| %s | %.2f× | %.1f%% |\n", a.name, mx, sd*100)
	}

	fmt.Print("\n### Lookup cost\n\n")
	fmt.Println("| Algorithm | 10 nodes | 1,000 nodes | Memory (ring points, 1,000 nodes) |\n|---|---:|---:|---:|")
	big := hashring.Nodes(1000)
	for _, a := range algs {
		mem := "-"
		if r, ok := a.mk(big).(*hashring.Ring); ok {
			mem = fmt.Sprintf("%d", r.Points())
		}
		fmt.Printf("| %s | %.0f ns | %.0f ns | %s |\n", a.name, hashring.LookupNs(a.mk(ten), keys), hashring.LookupNs(a.mk(big), keys[:200_000]), mem)
	}
}

// needlessAdd: keys that moved between two of the original nodes.
func needlessAdd(before, after hashring.Picker, keys []string) string {
	var n int
	newNode := "cache-10"
	for _, k := range keys {
		a, b := before.Pick(k), after.Pick(k)
		if a != b && b != newNode {
			n++
		}
	}
	return fmt.Sprintf("%.1f%%", float64(n)*100/float64(len(keys)))
}
