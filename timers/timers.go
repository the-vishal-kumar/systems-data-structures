// Package timers schedules many delayed jobs (retries, timeouts, expiries)
// two ways, on a simulated clock so the cost is CPU only and repeatable:
//
//   - Heap: a binary min-heap ordered by deadline. Exact; O(log n) to add,
//     cancel or fire.
//   - Wheel: a hashed timing wheel (Varghese and Lauck, 1987; the design of
//     Netty's HashedWheelTimer). Slots of one tick each; a timer goes into
//     slot deadline mod size, with a count of full turns still to wait.
//     O(1) to add and cancel; fires up to one tick late.
package timers

import "container/heap"

// Timer is one scheduled job.
type Timer struct {
	Deadline int64 // ticks
	ID       int64
	index    int // position in the heap, or in the wheel slot
	rounds   int64
	slot     int
}

// Scheduler is what both implementations offer.
type Scheduler interface {
	Add(t *Timer)
	Cancel(t *Timer)
	// Advance moves the clock to now and calls fire for every timer due.
	Advance(now int64, fire func(*Timer))
	Len() int
}

// ---- min-heap --------------------------------------------------------------

type minHeap []*Timer

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].Deadline < h[j].Deadline }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *minHeap) Push(x any)        { t := x.(*Timer); t.index = len(*h); *h = append(*h, t) }
func (h *minHeap) Pop() any {
	old := *h
	t := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	t.index = -1
	return t
}

type Heap struct{ h minHeap }

func NewHeap() *Heap            { return &Heap{} }
func (s *Heap) Add(t *Timer)    { heap.Push(&s.h, t) }
func (s *Heap) Cancel(t *Timer) { heap.Remove(&s.h, t.index) }
func (s *Heap) Len() int        { return s.h.Len() }

func (s *Heap) Advance(now int64, fire func(*Timer)) {
	for s.h.Len() > 0 && s.h[0].Deadline <= now {
		fire(heap.Pop(&s.h).(*Timer))
	}
}

// ---- hashed timing wheel ------------------------------------------------------

type Wheel struct {
	slots [][]*Timer
	now   int64 // last tick processed
	n     int
}

// NewWheel makes a wheel of size slots, one tick each.
func NewWheel(size int) *Wheel { return &Wheel{slots: make([][]*Timer, size)} }

func (w *Wheel) Len() int { return w.n }

func (w *Wheel) Add(t *Timer) {
	d := max(t.Deadline, w.now+1)
	size := int64(len(w.slots))
	t.slot = int(d % size)
	t.rounds = (d - w.now - 1) / size
	t.index = len(w.slots[t.slot])
	w.slots[t.slot] = append(w.slots[t.slot], t)
	w.n++
}

// Cancel is O(1): swap the last timer of the slot into this one's place.
func (w *Wheel) Cancel(t *Timer) {
	s := w.slots[t.slot]
	last := s[len(s)-1]
	s[t.index], last.index = last, t.index
	w.slots[t.slot] = s[:len(s)-1]
	w.n--
}

// Advance processes every tick up to now: each tick visits one slot, fires
// the timers on their last round and counts down the others.
func (w *Wheel) Advance(now int64, fire func(*Timer)) {
	for w.now < now {
		w.now++
		i := int(w.now % int64(len(w.slots)))
		s := w.slots[i]
		for j := 0; j < len(s); {
			t := s[j]
			if t.rounds > 0 {
				t.rounds--
				j++
				continue
			}
			last := s[len(s)-1]
			s[j], last.index = last, j
			s = s[:len(s)-1]
			w.n--
			fire(t)
		}
		w.slots[i] = s
	}
}
