package hashring

import (
	"fmt"
	"math"
	"time"
)

// Keys returns n distinct keys that look like real ones.
func Keys(n int) []string {
	k := make([]string, n)
	for i := range k {
		k[i] = fmt.Sprintf("user:%d:session", i)
	}
	return k
}

// Nodes returns n node names.
func Nodes(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("cache-%02d", i)
	}
	return out
}

// Moved is the fraction of keys whose node changed, and the fraction that
// moved although their old node still exists (unnecessary moves).
func Moved(before, after Picker, keys []string, alive map[string]bool) (moved, needless float64) {
	var m, n int
	for _, k := range keys {
		a, b := before.Pick(k), after.Pick(k)
		if a != b {
			m++
			if alive[a] {
				n++
			}
		}
	}
	return float64(m) / float64(len(keys)), float64(n) / float64(len(keys))
}

// Spread reports the busiest node's load relative to the average, and the
// standard deviation of load as a fraction of the average.
func Spread(p Picker, keys []string) (maxOverMean, stddev float64) {
	load := map[string]int{}
	for _, k := range keys {
		load[p.Pick(k)]++
	}
	mean := float64(len(keys)) / float64(len(load))
	var mx, ss float64
	for _, v := range load {
		mx = math.Max(mx, float64(v))
		ss += (float64(v) - mean) * (float64(v) - mean)
	}
	return mx / mean, math.Sqrt(ss/float64(len(load))) / mean
}

// LookupNs is the average time of one Pick.
func LookupNs(p Picker, keys []string) float64 {
	t := time.Now()
	for _, k := range keys {
		_ = p.Pick(k)
	}
	return float64(time.Since(t).Nanoseconds()) / float64(len(keys))
}
