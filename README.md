# systems-data-structures

The data structures inside caches, queues and databases, each implemented small enough to read in one sitting, and measured on the property that decides whether you'd use it. Go; standard library only, except `cmd/dbqueue`, which uses pgx and a PostgreSQL container.

```bash
make demo   # or: go run ./cmd/hashdemo && go run ./cmd/filterdemo && go run ./cmd/timerdemo
make dbqueue   # needs Docker: PostgreSQL-backed delayed jobs
make test
```

| Package | Question it answers | Compared | Post |
|---|---|---|---|
| [`hashring`](hashring/) | How do you add a cache node without moving every key? | mod N vs consistent hashing (1–1,000 virtual nodes) vs rendezvous vs jump hash: keys moved, balance, lookup cost | *How do you add a cache node without moving every key?* |
| [`filter`](filter/) | How do you check "have we seen this?" for millions of items cheaply? | exact set vs Bloom filters (1%, 0.1%, 0.01%) vs a cuckoo filter: memory, false positives, deletes, overfilling | *How do you check "have we seen this?" for millions of items cheaply?* |
| [`timers`](timers/) + [`cmd/dbqueue`](cmd/dbqueue/) | How do queues schedule millions of delayed retries? | min-heap vs hashed timing wheels (simulated clock), and PostgreSQL rows with a due-time index claimed with `SKIP LOCKED` | *How do queues schedule millions of delayed retries?* |

Measured results: [results.md](results.md). The tests pin each property (keys moved within a few points of the ideal, no false negatives, false-positive rates within tolerance, deletes that don't disturb other items).

## Notes

- The implementations are for reading and measuring. In production, prefer a maintained library, or the structure built into your store: Redis Cluster's hash slots, a client library's ring, RedisBloom, or the Bloom filters already inside RocksDB and Cassandra.
- The cuckoo filter here doesn't undo its evictions when an insert fails, so after a failed insert one other item may be lost. Production implementations keep a small stash or roll back. Sizes are rounded up to a power-of-two number of buckets.

## Licence

MIT
