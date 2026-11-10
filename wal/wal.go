// Package wal is a small write-ahead log: records appended to a file, each
// framed with its length and a CRC so a torn write at the end can be found
// and cut off on restart. It shows the two things every log-based system
// (a database's WAL, Kafka, an outbox) has to decide:
//
//   - when an append counts as durable: after fsync of every record, after
//     an fsync shared by a group of records (group commit), or never
//     (the OS writes it back when it likes);
//   - what to do with a damaged tail after a crash.
package wal

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// Sync decides when Append returns.
type Sync int

const (
	SyncEach  Sync = iota // write + fsync per record
	SyncGroup             // records waiting at the same time share one fsync
	SyncNever             // write only; durable whenever the OS flushes
)

type Log struct {
	f    *os.File
	mode Sync

	mu    sync.Mutex
	syncs int // fsyncs done, to show how much group commit saves

	// group commit
	gmu     sync.Mutex
	pending []pendingRec
	wake    chan struct{}
	done    chan struct{}
}

type pendingRec struct {
	data []byte
	errc chan error
}

const header = 8 // 4 bytes length, 4 bytes CRC-32C

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

func frame(rec []byte) []byte {
	b := make([]byte, header+len(rec))
	binary.LittleEndian.PutUint32(b[0:4], uint32(len(rec)))
	binary.LittleEndian.PutUint32(b[4:8], crc32.Checksum(rec, castagnoli))
	copy(b[header:], rec)
	return b
}

// Open opens (or creates) the log for appending. Call Recover first if the
// file may end in a torn record.
func Open(path string, mode Sync) (*Log, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	l := &Log{f: f, mode: mode}
	if mode == SyncGroup {
		l.wake, l.done = make(chan struct{}, 1), make(chan struct{})
		go l.committer()
	}
	return l, nil
}

// Append adds one record and returns when it's as durable as the mode says.
func (l *Log) Append(rec []byte) error {
	switch l.mode {
	case SyncGroup:
		errc := make(chan error, 1)
		l.gmu.Lock()
		l.pending = append(l.pending, pendingRec{rec, errc})
		l.gmu.Unlock()
		select {
		case l.wake <- struct{}{}:
		default:
		}
		return <-errc
	default:
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, err := l.f.Write(frame(rec)); err != nil {
			return err
		}
		if l.mode == SyncEach {
			l.syncs++
			return l.f.Sync()
		}
		return nil
	}
}

// committer writes everything waiting, fsyncs once, and releases all those
// writers together: the same trick PostgreSQL and MySQL use for commits.
func (l *Log) committer() {
	for {
		select {
		case <-l.wake:
		case <-l.done:
			return
		}
		for {
			l.gmu.Lock()
			batch := l.pending
			l.pending = nil
			l.gmu.Unlock()
			if len(batch) == 0 {
				break
			}
			var buf []byte
			for _, p := range batch {
				buf = append(buf, frame(p.data)...)
			}
			_, err := l.f.Write(buf)
			if err == nil {
				err = l.f.Sync()
			}
			l.mu.Lock()
			l.syncs++
			l.mu.Unlock()
			for _, p := range batch {
				p.errc <- err
			}
		}
	}
}

// Syncs is how many fsyncs the log has done.
func (l *Log) Syncs() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.syncs
}

func (l *Log) Close() error {
	if l.done != nil {
		close(l.done)
	}
	return l.f.Close()
}

// ErrCorrupt is returned by Replay when a record's checksum doesn't match.
var ErrCorrupt = errors.New("wal: corrupt record")

// Replay calls fn for every complete, valid record from the start, and
// returns the offset just after the last good one. A short or corrupt
// record ends the replay: after a crash, that's the torn tail.
func Replay(path string, fn func(rec []byte)) (good int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	h := make([]byte, header)
	for {
		if _, err := io.ReadFull(r, h); err != nil {
			return good, nil // end of file, or a torn header
		}
		n := binary.LittleEndian.Uint32(h[0:4])
		rec := make([]byte, n)
		if _, err := io.ReadFull(r, rec); err != nil {
			return good, nil // torn body
		}
		if crc32.Checksum(rec, castagnoli) != binary.LittleEndian.Uint32(h[4:8]) {
			return good, ErrCorrupt
		}
		fn(rec)
		good += int64(header) + int64(n)
	}
}

// Recover replays the log and cuts off anything after the last good record,
// so new appends don't land after garbage.
func Recover(path string, fn func(rec []byte)) (records int, cut int64, err error) {
	good, err := Replay(path, func(rec []byte) { records++; fn(rec) })
	if err != nil && !errors.Is(err, ErrCorrupt) {
		return 0, 0, err
	}
	st, statErr := os.Stat(path)
	if statErr != nil {
		return records, 0, statErr
	}
	if cut = st.Size() - good; cut > 0 {
		if err := os.Truncate(path, good); err != nil {
			return records, 0, err
		}
	}
	return records, cut, nil
}
