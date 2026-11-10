package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRecoverCutsTheTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	l, err := Open(path, SyncNever)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for i := range 1000 {
		k, v := fmt.Sprintf("k%d", i%100), fmt.Sprintf("v%d", i)
		want[k] = v
		if err := l.Append([]byte(k + "=" + v)); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()
	// A crash in the middle of writing the next record: half a frame.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.Write(frame([]byte("k1=lost"))[:6])
	f.Close()

	got := map[string]string{}
	n, cut, err := Recover(path, func(rec []byte) {
		k, v, _ := strings.Cut(string(rec), "=")
		got[k] = v
	})
	if err != nil || n != 1000 || cut != 6 {
		t.Fatalf("recover: %d records, cut %d bytes, err %v", n, cut, err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("replayed state differs at %s: %s vs %s", k, got[k], v)
		}
	}
	// New appends go after the last good record, not after the garbage.
	l, _ = Open(path, SyncEach)
	_ = l.Append([]byte("k1=after"))
	l.Close()
	if n, _, _ := Recover(path, func([]byte) {}); n != 1001 {
		t.Fatalf("after recovery and one append: %d records", n)
	}
}

func TestReplayStopsAtCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	l, _ := Open(path, SyncNever)
	for i := range 10 {
		_ = l.Append([]byte(fmt.Sprintf("record-%d", i)))
	}
	l.Close()
	b, _ := os.ReadFile(path)
	b[5*(header+8)+header+2] ^= 0xff // flip a byte inside record 5
	_ = os.WriteFile(path, b, 0o644)
	n := 0
	if _, err := Replay(path, func([]byte) { n++ }); err != ErrCorrupt || n != 5 {
		t.Fatalf("replay: %d records before %v", n, err)
	}
}

func TestGroupCommitSharesFsyncs(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "log"), SyncGroup)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if err := l.Append([]byte("x")); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if s := l.Syncs(); s >= 640 || s == 0 {
		t.Errorf("640 appends from 32 writers took %d fsyncs; group commit should share them", s)
	}
}
