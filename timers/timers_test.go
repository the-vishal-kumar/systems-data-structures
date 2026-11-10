package timers

import "testing"

func TestBothFireEveryTimerOnTime(t *testing.T) {
	for _, s := range []struct {
		name string
		s    Scheduler
	}{{"heap", NewHeap()}, {"wheel/256", NewWheel(256)}, {"wheel/65536", NewWheel(65536)}} {
		r := Run(s.name, s.s, 50_000, 10_000, 0.5, 3)
		if r.Fired != 25_000 || r.Late != 0 || r.Early != 0 || s.s.Len() != 0 {
			t.Errorf("%s: fired %d (want 25000), late %d, early or cancelled %d, left %d", s.name, r.Fired, r.Late, r.Early, s.s.Len())
		}
	}
}

func TestWheelPastDeadline(t *testing.T) {
	w := NewWheel(8)
	w.Advance(100, func(*Timer) {})
	x := &Timer{Deadline: 50}
	w.Add(x)
	var got *Timer
	w.Advance(101, func(t *Timer) { got = t })
	if got != x {
		t.Fatal("a timer added after its deadline should fire on the next tick")
	}
}
