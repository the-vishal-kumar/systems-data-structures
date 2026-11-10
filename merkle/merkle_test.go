package merkle

import (
	"fmt"
	"slices"
	"testing"
)

func replicas(n int) (Store, Store) {
	a, b := Store{}, Store{}
	for i := range n {
		k, v := fmt.Sprintf("key-%06d", i), []byte(fmt.Sprintf("value-%d", i))
		a[k], b[k] = v, v
	}
	return a, b
}

func TestFindsExactlyTheDifferences(t *testing.T) {
	a, b := replicas(10_000)
	b["key-000042"] = []byte("changed")
	delete(b, "key-000100")
	b["key-new"] = []byte("only on b")
	want := []string{"key-000042", "key-000100", "key-new"}
	for _, depth := range []int{1, 6, 12} {
		got, cost := Diff(Build(a, depth), Build(b, depth), a, b)
		if !slices.Equal(got, want) {
			t.Fatalf("depth %d: got %v, want %v", depth, got, want)
		}
		if cost.Bytes >= HashListCost(b).Bytes && depth == 12 {
			t.Fatalf("depth 12: Merkle walk sent %d bytes, no better than a hash list", cost.Bytes)
		}
	}
}

func TestIdenticalReplicasCostOneHash(t *testing.T) {
	a, b := replicas(1000)
	got, cost := Diff(Build(a, 8), Build(b, 8), a, b)
	if len(got) != 0 || cost.Bytes != HashSize || cost.RoundTrips != 1 {
		t.Fatalf("identical: diff %v, cost %+v", got, cost)
	}
}
