// Command dbqueue schedules delayed jobs the durable way: rows with a due
// time, a partial index on it, and workers claiming due rows with
// FOR UPDATE SKIP LOCKED (the pattern of a transactional outbox or a
// Postgres-backed job queue). It measures how late jobs run and how many
// the database hands out per second, for two polling intervals.
//
//	docker compose up -d --wait postgres
//	DATABASE_URL=postgres://lab:lab@localhost:55434/lab go run ./cmd/dbqueue
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	n := flag.Int("jobs", 200_000, "jobs to schedule")
	spread := flag.Duration("spread", 20*time.Second, "due times spread over this long")
	flag.Parse()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://lab:lab@localhost:55434/lab?sslmode=disable"
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	var v string
	_ = db.QueryRow(ctx, `SHOW server_version`).Scan(&v)
	fmt.Printf("PostgreSQL %s. %d jobs due over %s; 4 workers claim up to 100 due jobs at a time; 10%% of attempts fail and are retried 1 s later.\n\n", v, *n, *spread)
	fmt.Println("| Poll interval when idle | Jobs/s handed out (peak second) | Lateness p50 | p99 | max | Index size at start |")
	fmt.Println("|---|---:|---:|---:|---:|---:|")
	for _, poll := range []time.Duration{50 * time.Millisecond, 500 * time.Millisecond} {
		if err := run(ctx, db, *n, *spread, poll); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	_, _ = db.Exec(ctx, `DROP SCHEMA IF EXISTS q CASCADE`)
}

func run(ctx context.Context, db *pgxpool.Pool, n int, spread, poll time.Duration) error {
	if _, err := db.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS q CASCADE; CREATE SCHEMA q;
		CREATE TABLE q.jobs (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, due_at timestamptz NOT NULL,
		                     run_at timestamptz NOT NULL, done_at timestamptz, attempts int NOT NULL DEFAULT 0);
		INSERT INTO q.jobs (due_at, run_at)
		  SELECT t, t FROM (SELECT now() + interval '2 seconds' + random() * interval '%d milliseconds' AS t
		                    FROM generate_series(1, %d)) s;
		CREATE INDEX jobs_due ON q.jobs (run_at) WHERE done_at IS NULL;`, spread.Milliseconds(), n)); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `VACUUM ANALYZE q.jobs`); err != nil {
		return err
	}
	var idx int64
	_ = db.QueryRow(ctx, `SELECT pg_relation_size('q.jobs_due')`).Scan(&idx)

	var mu sync.Mutex
	var late []time.Duration
	perSecond := map[int64]int{}
	var remaining atomic.Int64
	remaining.Store(int64(n))
	deadline := time.Now().Add(spread + 30*time.Second)
	var wg sync.WaitGroup
	var firstErr atomic.Value
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for remaining.Load() > 0 && time.Now().Before(deadline) {
				// Claim: lease the rows 10 minutes ahead so a crashed worker's
				// batch comes back, and report when each was due.
				rows, err := db.Query(ctx, `WITH due AS (
					SELECT id FROM q.jobs WHERE done_at IS NULL AND run_at <= now()
					ORDER BY run_at LIMIT 100 FOR UPDATE SKIP LOCKED)
					UPDATE q.jobs j SET run_at = now() + interval '10 minutes', attempts = j.attempts + 1
					FROM due WHERE j.id = due.id
					RETURNING j.id, now() - j.due_at`)
				if err != nil {
					firstErr.CompareAndSwap(nil, err)
					return
				}
				var done, retry []int64
				var l []time.Duration
				for rows.Next() {
					var id int64
					var d time.Duration
					if err := rows.Scan(&id, &d); err != nil {
						firstErr.CompareAndSwap(nil, err)
						return
					}
					l = append(l, d)
					if rand.Float64() < 0.1 {
						retry = append(retry, id)
					} else {
						done = append(done, id)
					}
				}
				if len(l) == 0 {
					time.Sleep(poll)
					continue
				}
				b := &pgx.Batch{}
				b.Queue(`UPDATE q.jobs SET done_at = now() WHERE id = ANY($1)`, done)
				b.Queue(`UPDATE q.jobs SET due_at = now() + interval '1 second', run_at = now() + interval '1 second' WHERE id = ANY($1)`, retry)
				if err := db.SendBatch(ctx, b).Close(); err != nil {
					firstErr.CompareAndSwap(nil, err)
					return
				}
				remaining.Add(-int64(len(done)))
				mu.Lock()
				late = append(late, l...)
				perSecond[time.Now().Unix()] += len(l)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err, _ := firstErr.Load().(error); err != nil {
		return err
	}
	slices.Sort(late)
	peak := 0
	for _, c := range perSecond {
		peak = max(peak, c)
	}
	p := func(q float64) string {
		d := late[min(len(late)-1, int(float64(len(late))*q))]
		return fmt.Sprintf("%.0f ms", float64(d.Microseconds())/1000)
	}
	fmt.Printf("| %s | %d | %s | %s | %s | %.1f MB |\n", poll, peak, p(0.5), p(0.99), p(1), float64(idx)/1e6)
	if remaining.Load() > 0 {
		fmt.Printf("\n(%d jobs still pending at the time limit)\n", remaining.Load())
	}
	return nil
}
