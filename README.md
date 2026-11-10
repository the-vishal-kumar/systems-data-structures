# systems-data-structures

The data structures inside caches, queues and databases, each implemented small enough to read in one sitting, and measured on the property that decides whether you'd use it. Go; standard library only, except `cmd/dbqueue`, which uses pgx and a PostgreSQL container.

```bash
make demo   # runs every cmd/*demo
make dbqueue   # needs Docker: PostgreSQL-backed delayed jobs
make test
```

| Package | Question it answers | Compared | Post |
|---|---|---|---|
| [`hashring`](hashring/) | How do you add a cache node without moving every key? | mod N vs consistent hashing (1–1,000 virtual nodes) vs rendezvous vs jump hash: keys moved, balance, lookup cost | *How do you add a cache node without moving every key?* |
| [`filter`](filter/) | How do you check "have we seen this?" for millions of items cheaply? | exact set vs Bloom filters (1%, 0.1%, 0.01%) vs a cuckoo filter: memory, false positives, deletes, overfilling | *How do you check "have we seen this?" for millions of items cheaply?* |
| [`timers`](timers/) + [`cmd/dbqueue`](cmd/dbqueue/) | How do queues schedule millions of delayed retries? | min-heap vs hashed timing wheels (simulated clock), and PostgreSQL rows with a due-time index claimed with `SKIP LOCKED` | *How do queues schedule millions of delayed retries?* |
| [`partmatch`](partmatch/) | How do you match messy part numbers to a catalog? | typo lookup: linear scan vs BK-tree vs symmetric-delete index; prefix: scan vs sorted array; description: scan vs inverted index (all words, or rare words only), at 1K–100K items | *How do you match messy part numbers to a catalog?* |
| [`wal`](wal/) | Why is there an append-only log under queues, databases and the outbox? | a write-ahead log with CRC-framed records: fsync per record vs group commit vs no fsync at 1–64 writers, torn-tail recovery and replay | *Why is there an append-only log under queues, databases and the outbox?* |
| [`cache`](cache/) | Which cache eviction policy keeps the most hits? | LRU vs LFU vs W-TinyLFU (window + count-min admission + segmented LRU): hit rate on Zipf traffic, after a one-off scan, and when popularity shifts | *Which cache eviction policy keeps the most hits?* |
| [`hll`](hll/) | How do you count unique visitors without storing them? | exact set vs HyperLogLog at precision 10–16: memory and error from 1,000 to 10,000,000 items; merging daily sketches | *How do you count unique visitors without storing them?* |
| [`topk`](topk/) | How do you find the top 10 among billions of events? | exact map vs count-min sketch + heap vs Space-Saving: memory, top-10 recall and count error on skewed and flatter traffic | *How do you find the top 10 among billions of events?* |
| [`skiplist`](skiplist/) | How does a leaderboard stay sorted under constant updates? | skip list with rank spans (as in Redis) vs sorted slice vs map + scan: update, rank and top-10 cost at 10K–1M players | *How does a leaderboard stay sorted under constant updates?* |
| [`merkle`](merkle/) | How do two replicas find what's different without sending everything? | full copy vs a hash per key vs a Merkle tree walk (4K and 64K leaves): bytes and round trips for 0–10,000 differences | *How do two replicas find what's different without sending everything?* |
| [`geo`](geo/) | How do you find what's nearby? | full scan vs geohash cells vs quadtree: points checked and time for 1–5 km queries among 1M points | *How do you find what's nearby?* |

Measured results: [results.md](results.md). The tests pin each property (keys moved within a few points of the ideal, no false negatives, false-positive rates within tolerance, deletes that don't disturb other items).

## Notes

- The implementations are for reading and measuring. In production, prefer a maintained library, or the structure built into your store: Redis Cluster's hash slots, a client library's ring, RedisBloom, or the Bloom filters already inside RocksDB and Cassandra.
- The cuckoo filter here doesn't undo its evictions when an insert fails, so after a failed insert one other item may be lost. Production implementations keep a small stash or roll back. Sizes are rounded up to a power-of-two number of buckets.

## Licence

MIT
