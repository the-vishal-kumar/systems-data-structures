package hashring

import (
	"math"
	"testing"
)

func alive(nodes []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range nodes {
		m[n] = true
	}
	return m
}

func TestAddingANode(t *testing.T) {
	keys := Keys(200_000)
	ten, eleven := Nodes(10), Nodes(11)
	ideal := 1.0 / 11
	if moved, _ := Moved(NewModN(ten), NewModN(eleven), keys, alive(ten)); moved < 0.8 {
		t.Errorf("mod N should move almost every key: %.3f", moved)
	}
	for name, pair := range map[string][2]Picker{
		"ring/100":   {NewRing(ten, 100), NewRing(eleven, 100)},
		"rendezvous": {NewRendezvous(ten), NewRendezvous(eleven)},
		"jump":       {NewJump(ten), NewJump(eleven)},
	} {
		moved, needless := Moved(pair[0], pair[1], keys, alive(ten))
		// Every move goes to the new node, so none is "needless" here;
		// what matters is that only about 1/11 move.
		if math.Abs(moved-ideal) > 0.03 {
			t.Errorf("%s: moved %.3f, want about %.3f", name, moved, ideal)
		}
		if needless != moved {
			t.Errorf("%s: every move should go to the new node", name)
		}
	}
}

func TestRemovingANode(t *testing.T) {
	keys := Keys(200_000)
	ten := Nodes(10)
	nine := append(append([]string{}, ten[:3]...), ten[4:]...) // cache-03 fails
	for name, pair := range map[string][2]Picker{
		"ring/100":   {NewRing(ten, 100), NewRing(nine, 100)},
		"rendezvous": {NewRendezvous(ten), NewRendezvous(nine)},
	} {
		_, needless := Moved(pair[0], pair[1], keys, alive(nine))
		if needless != 0 {
			t.Errorf("%s: keys moved off nodes that are still up: %.4f", name, needless)
		}
	}
	if _, needless := Moved(NewModN(ten), NewModN(nine), keys, alive(nine)); needless < 0.7 {
		t.Errorf("mod N should reshuffle keys on surviving nodes too: %.3f", needless)
	}
}

func TestVirtualNodesEvenTheLoad(t *testing.T) {
	keys := Keys(200_000)
	one, _ := Spread(NewRing(Nodes(10), 1), keys)
	many, _ := Spread(NewRing(Nodes(10), 1000), keys)
	if one < 1.5 || many > 1.1 {
		t.Errorf("1 point per node should be lopsided (%.2f), 1000 even (%.2f)", one, many)
	}
}
