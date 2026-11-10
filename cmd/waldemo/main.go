// Command waldemo measures what durability costs in an append-only log:
// fsync per record, group commit, or no fsync, with 1 to 64 concurrent
// writers; then crashes the log mid-record and recovers it.
//
//	go run ./cmd/waldemo
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/the-vishal-kumar/systems-data-structures/wal"
)

func ms(d time.Duration) string {
	if d < 100*time.Microsecond {
		return fmt.Sprintf("%.1f µs", float64(d.Nanoseconds())/1000)
	}
	if d < 10*time.Millisecond {
		return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.0f ms", float64(d.Microseconds())/1000)
}

func main() {
	dir, err := os.MkdirTemp("", "wal")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	const d = 3 * time.Second
	rec := []byte(strings.Repeat("x", 100))
	fmt.Printf("100-byte records, %s per row, Go %s, a laptop SSD (through Docker's overlay filesystem).\n\n", d, runtime.Version())
	fmt.Println("| Durability | Writers | Appends/s | p50 | p99 | fsyncs/s | Records per fsync | Survives power loss? |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|---|")
	for _, m := range []struct {
		name string
		mode wal.Sync
		safe string
	}{{"fsync every record", wal.SyncEach, "yes"}, {"group commit (one fsync per batch)", wal.SyncGroup, "yes"}, {"no fsync (OS writes back later)", wal.SyncNever, "**no**: the last seconds are lost"}} {
		for _, writers := range []int{1, 8, 64} {
			path := filepath.Join(dir, fmt.Sprintf("%d-%d", m.mode, writers))
			l, err := wal.Open(path, m.mode)
			if err != nil {
				panic(err)
			}
			var mu sync.Mutex
			var lat []time.Duration
			stop := time.Now().Add(d)
			var wg sync.WaitGroup
			for range writers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var mine []time.Duration
					for time.Now().Before(stop) {
						t := time.Now()
						if err := l.Append(rec); err != nil {
							panic(err)
						}
						mine = append(mine, time.Since(t))
					}
					mu.Lock()
					lat = append(lat, mine...)
					mu.Unlock()
				}()
			}
			wg.Wait()
			syncs := l.Syncs()
			l.Close()
			slices.Sort(lat)
			per := "—"
			if syncs > 0 {
				per = fmt.Sprintf("%.1f", float64(len(lat))/float64(syncs))
			}
			fmt.Printf("| %s | %d | %.0f | %s | %s | %.0f | %s | %s |\n", m.name, writers, float64(len(lat))/d.Seconds(),
				ms(lat[len(lat)/2]), ms(lat[len(lat)*99/100]), float64(syncs)/d.Seconds(), per, m.safe)
		}
	}

	// Crash recovery: 100,000 key=value records, then a crash halfway
	// through writing one more.
	path := filepath.Join(dir, "recover")
	l, _ := wal.Open(path, wal.SyncNever)
	for i := range 100_000 {
		_ = l.Append(fmt.Appendf(nil, "user:%d=%d", i%5000, i))
	}
	l.Close()
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.Write([]byte{20, 0, 0, 0, 1, 2, 3, 4, 'u', 's', 'e'})
	f.Close()
	state := map[string]string{}
	t := time.Now()
	n, cut, err := wal.Recover(path, func(r []byte) {
		k, v, _ := strings.Cut(string(r), "=")
		state[k] = v
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n### Crash recovery\n\n100,000 `key=value` records over 5,000 keys, then a crash halfway through writing the next one. Recovery replayed **%d** records into **%d** keys in %s and cut off a **%d-byte** torn tail.\n",
		n, len(state), ms(time.Since(t)), cut)
}
