package timers

import (
	"math/rand/v2"
	"runtime"
	"time"
)

// Result of scheduling n timers over horizon ticks, cancelling some, and
// running the clock until all have fired.
type Result struct {
	Name               string
	AddNs, CancelNs    float64
	FireNs             float64 // clock + firing CPU per timer fired
	Fired, Late, Early int
	MemBytes           uint64 // heap growth after adding the timers
}

func heapBytes() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// Run: n timers with deadlines uniform in [1, horizon] ticks; a share
// cancelRatio of them cancelled (the retry wasn't needed after all); then
// the clock advances one tick at a time to horizon.
func Run(name string, s Scheduler, n int, horizon int64, cancelRatio float64, seed uint64) Result {
	r := rand.New(rand.NewPCG(seed, 1))
	ts := make([]*Timer, n)
	for i := range ts {
		ts[i] = &Timer{Deadline: 1 + r.Int64N(horizon), ID: int64(i)}
	}
	before := heapBytes()
	t := time.Now()
	for _, x := range ts {
		s.Add(x)
	}
	res := Result{Name: name, AddNs: float64(time.Since(t).Nanoseconds()) / float64(n)}
	res.MemBytes = heapBytes() - before
	cancelled := make([]bool, n) // a slice, so bookkeeping costs almost nothing in the timed loops
	k := int(float64(n) * cancelRatio)
	t = time.Now()
	for _, x := range ts[:k] {
		s.Cancel(x)
		cancelled[x.ID] = true
	}
	if k > 0 {
		res.CancelNs = float64(time.Since(t).Nanoseconds()) / float64(k)
	}
	var now int64
	t = time.Now()
	fire := func(x *Timer) {
		res.Fired++
		if cancelled[x.ID] || x.Deadline > now {
			res.Early++ // a cancelled timer firing counts as wrong too
		} else if x.Deadline < now {
			res.Late++
		}
	}
	for now = 1; now <= horizon; now++ {
		s.Advance(now, fire)
	}
	if res.Fired > 0 {
		res.FireNs = float64(time.Since(t).Nanoseconds()) / float64(res.Fired)
	}
	return res
}
