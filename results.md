# Results

Measured on 2026-11-10 on a laptop (Intel i7-8550U), `make demo`. Keys moved, balance and false-positive rates are properties of the algorithms and repeat exactly; nanoseconds depend on the machine.

## Consistent hashing (`cmd/hashdemo`)

1,000,000 keys, Go go1.27.3.

### Adding an 11th node to 10 (ideal: 1/11 = 9.1% of keys move, all to the new node)

| Algorithm | Keys moved | Moved between old nodes (needless) |
|---|---:|---:|
| mod N | 90.9% | 81.8% |
| ring, 1 point per node | 11.4% | 0.0% |
| ring, 100 points per node | 8.8% | 0.0% |
| rendezvous | 9.0% | 0.0% |
| jump hash | 9.1% | 0.0% |

### Removing one node of 10 (ideal: 10% move, only the failed node's keys)

| Algorithm | Node removed | Keys moved | Moved although their node is still up |
|---|---|---:|---:|
| mod N | cache-03 (any node) | 90.0% | 80.0% |
| ring, 1 point per node | cache-03 (any node) | 0.2% | 0.0% |
| ring, 100 points per node | cache-03 (any node) | 8.5% | 0.0% |
| rendezvous | cache-03 (any node) | 10.1% | 0.0% |
| jump hash | cache-09 (only the last one is possible) | 10.0% | 0.0% |

### Balance across 10 nodes

| Algorithm | Busiest node vs average | Std. deviation of load |
|---|---:|---:|
| mod N | 1.01× | 0.3% |
| ring, 1 point per node | 2.71× | 74.1% |
| ring, 10 points per node | 1.30× | 24.7% |
| ring, 100 points per node | 1.13× | 7.0% |
| ring, 1,000 points per node | 1.04× | 2.3% |
| rendezvous | 1.01× | 0.3% |
| jump hash | 1.00× | 0.3% |

### Lookup cost

| Algorithm | 10 nodes | 1,000 nodes | Memory (ring points, 1,000 nodes) |
|---|---:|---:|---:|
| mod N | 27 ns | 27 ns | — |
| ring, 1 point per node | 50 ns | 82 ns | 1000 |
| ring, 100 points per node | 82 ns | 158 ns | 100000 |
| rendezvous | 64 ns | 2342 ns | — |
| jump hash | 46 ns | 83 ns | — |

## Membership filters (`cmd/filterdemo`)

1000000 items in, 1000000 different items queried. Go go1.27.3.

| Structure | Memory | Bits per item | False positives | Insert | Lookup | Deletes? | Note |
|---|---:|---:|---:|---:|---:|---|---|
| exact hash set (map) | 55.9 MB | 447.4 | 0.0000% | 101 ns | 60 ns | yes | plus the strings themselves, if not already in memory |
| Bloom, sized for 1% (k = 7) | 1.2 MB | 9.6 | 1.0007% | 98 ns | 65 ns | no |  |
| Bloom, sized for 0.1% (k = 10) | 1.8 MB | 14.4 | 0.1013% | 118 ns | 70 ns | no |  |
| Bloom, sized for 0.01% (k = 13) | 2.4 MB | 19.2 | 0.0078% | 148 ns | 70 ns | no |  |
| cuckoo, 16-bit fingerprints | 2.1 MB | 16.8 | 0.0122% | 74 ns | 42 ns | yes | 95% full |

### What happens when you put in more than you sized for

| Filter | Sized for | Items added | False positives |
|---|---:|---:|---:|
| Bloom, 1% | 1000000 | 1000000 | 1.0% |
| Bloom, 1% | 1000000 | 2000000 | 15.7% |
| Bloom, 1% | 1000000 | 4000000 | 67.8% |
| cuckoo | 1048576 slots | first insert failed after 1004245 (95.8% full) | — |

## Delayed jobs in memory (`cmd/timerdemo`)

Simulated 1 ms clock, deadlines spread over 60 s, half the timers cancelled before they fire. Go go1.27.3.

| Timers | Structure | Add | Cancel | Run the clock + fire, per timer fired | Memory the structure adds | Fired late or wrongly |
|---:|---|---:|---:|---:|---:|---:|
| 100000 | min-heap | 49 ns | 55 ns | 286 ns | 1.0 MB | 0 |
| 100000 | timing wheel, 512 slots (~117 turns) | 27 ns | 15 ns | 150 ns | 1.2 MB | 0 |
| 100000 | timing wheel, 65,536 slots (one turn) | 64 ns | 21 ns | 42 ns | 0.9 MB | 0 |
| 1000000 | min-heap | 56 ns | 114 ns | 701 ns | 9.1 MB | 0 |
| 1000000 | timing wheel, 512 slots (~117 turns) | 24 ns | 20 ns | 757 ns | 9.4 MB | 0 |
| 1000000 | timing wheel, 65,536 slots (one turn) | 47 ns | 84 ns | 44 ns | 11.5 MB | 0 |

## Delayed jobs in PostgreSQL (`cmd/dbqueue`)

PostgreSQL 17.11, 4 workers claiming up to 100 due rows at a time with `FOR UPDATE SKIP LOCKED`; 10% of attempts fail and are retried 1 s later. "Lateness" is when a job was handed out minus when it was due.

| Jobs, due over 20 s | Poll interval when idle | Jobs/s handed out (peak second) | Lateness p50 | p99 | max |
|---:|---|---:|---:|---:|---:|
| 200,000 (10,000/s) | 50 ms | 11,280 | 1 ms | 13 ms | 51 ms |
| 200,000 (10,000/s) | 500 ms | 11,823 | 8 ms | 114 ms | 485 ms |
| 1,000,000 (50,000/s) | 50 ms | 40,400 | 6.2 s | 13.0 s | 13.1 s |
| 1,000,000 (50,000/s) | 500 ms | 49,100 | 4.5 s | 9.6 s | 9.7 s |

At 10,000 due per second the workers keep up and lateness is set by the polling interval; at 50,000 per second this setup falls behind and jobs run seconds late.

## Matching messy part numbers (`cmd/partdemo`)

Synthetic catalog of normalized SKUs like BRG6204RS. 1000 queries per row, each a real SKU with one typo (or its first 5 characters, or a reworded description). Go go1.27.3.

| Catalog | Lookup | Per query | Distances computed / items scored per query | Index build |
|---:|---|---:|---:|---:|
| 1000 | typo: linear scan | 249 µs | 1000 | — |
| 1000 | typo: BK-tree, distance ≤ 1 | 32 µs | 96 | 2 ms |
| 1000 | typo: BK-tree, distance ≤ 2 | 91 µs | 313 | 2 ms |
| 1000 | typo: symmetric-delete index (9660 keys, 9660 entries) | 0.99 µs | a few | 2 ms |
| 1000 | prefix: linear scan | 5.30 µs | 1000 | — |
| 1000 | prefix: sorted array + binary search | 0.21 µs | log₂ n + matches | 0 ms |
| 1000 | description: score every item | 162 µs | 1000 | — |
| 1000 | description: inverted index | 257 µs | 937 | 1 ms |
| 1000 | description: inverted index, rare words only (≤1% of items); best score matches the scan in 100% of queries | 81 µs | 299 | 1 ms |
| 10000 | typo: linear scan | 2.6 ms | 10000 | — |
| 10000 | typo: BK-tree, distance ≤ 1 | 136 µs | 374 | 19 ms |
| 10000 | typo: BK-tree, distance ≤ 2 | 646 µs | 1721 | 19 ms |
| 10000 | typo: symmetric-delete index (96416 keys, 96584 entries) | 1.45 µs | a few | 27 ms |
| 10000 | prefix: linear scan | 60 µs | 10000 | — |
| 10000 | prefix: sorted array + binary search | 0.75 µs | log₂ n + matches | 2 ms |
| 10000 | description: score every item | 1.4 ms | 10000 | — |
| 10000 | description: inverted index | 2.7 ms | 9332 | 5 ms |
| 10000 | description: inverted index, rare words only (≤1% of items); best score matches the scan in 100% of queries | 687 µs | 2690 | 5 ms |
| 100000 | typo: linear scan | 25.1 ms | 100000 | — |
| 100000 | typo: BK-tree, distance ≤ 1 | 608 µs | 1297 | 257 ms |
| 100000 | typo: BK-tree, distance ≤ 2 | 3.6 ms | 7886 | 257 ms |
| 100000 | typo: symmetric-delete index (949386 keys, 966195 entries) | 1.94 µs | a few | 442 ms |
| 100000 | prefix: linear scan | 563 µs | 100000 | — |
| 100000 | prefix: sorted array + binary search | 5.57 µs | log₂ n + matches | 24 ms |
| 100000 | description: score every item | 14.5 ms | 100000 | — |
| 100000 | description: inverted index | 25.6 ms | 93625 | 55 ms |
| 100000 | description: inverted index, rare words only (≤1% of items); best score matches the scan in 100% of queries | 6.5 ms | 25806 | 55 ms |

## An append-only log (`cmd/waldemo`)

100-byte records, 3s per row, Go go1.27.3, a laptop SSD (through Docker's overlay filesystem).

| Durability | Writers | Appends/s | p50 | p99 | fsyncs/s | Records per fsync | Survives power loss? |
|---|---:|---:|---:|---:|---:|---:|---|
| fsync every record | 1 | 205 | 4.91 ms | 5.73 ms | 205 | 1.0 | yes |
| fsync every record | 8 | 785 | 10 ms | 14 ms | 785 | 1.0 | yes |
| fsync every record | 64 | 1103 | 57 ms | 85 ms | 1103 | 1.0 | yes |
| group commit (one fsync per batch) | 1 | 758 | 1.42 ms | 2.00 ms | 758 | 1.0 | yes |
| group commit (one fsync per batch) | 8 | 3233 | 2.48 ms | 3.78 ms | 808 | 4.0 | yes |
| group commit (one fsync per batch) | 64 | 24923 | 2.53 ms | 3.98 ms | 778 | 32.0 | yes |
| no fsync (OS writes back later) | 1 | 628629 | 1.3 µs | 3.3 µs | 0 | — | **no**: the last seconds are lost |
| no fsync (OS writes back later) | 8 | 526524 | 1.5 µs | 0.52 ms | 0 | — | **no**: the last seconds are lost |
| no fsync (OS writes back later) | 64 | 499498 | 1.6 µs | 1.63 ms | 0 | — | **no**: the last seconds are lost |

### Crash recovery

100,000 `key=value` records over 5,000 keys, then a crash halfway through writing the next one. Recovery replayed **100000** records into **5000** keys in 11 ms and cut off a **11-byte** torn tail.

## Cache eviction (`cmd/cachedemo`)

1,000,000 distinct keys, Zipf s=0.9.

### Steady traffic: hit rate over 4,000,000 accesses after warming up

| Cache size | LRU | LFU | W-TinyLFU |
|---:|---:|---:|---:|
| 1,000 (0.1% of keys) | 22.3% | 32.4% | 32.7% |
| 10,000 (1.0% of keys) | 39.5% | 48.8% | 49.4% |
| 100,000 (10.0% of keys) | 63.7% | 69.0% | 69.4% |

### After a one-off scan of 50,000 keys (cache of 10,000)

| Policy | Hit rate before | Next 5,000 accesses | Next 100,000 accesses |
|---|---:|---:|---:|
| LRU | 39.4% | 26.1% | 39.1% |
| LFU | 48.6% | 48.9% | 48.5% |
| W-TinyLFU | 49.3% | 49.6% | 49.1% |

### When popularity shifts to new keys (cache of 10,000)

| Policy | Before the shift | First 200,000 after | Next 1,000,000 after |
|---|---:|---:|---:|
| LRU | 39.4% | 39.2% | 39.5% |
| LFU | 51.1% | 12.9% | 15.5% |
| W-TinyLFU | 49.5% | 38.4% | 47.1% |

## Counting distinct items (`cmd/hlldemo`)

### Exact set (Go map of 64-bit IDs)

| Distinct items | Memory |
|---:|---:|
| 100,000 | 2.3 MiB |
| 1,000,000 | 36.0 MiB |
| 10,000,000 | 288.6 MiB |

### HyperLogLog: relative error over 20 independent streams

| Precision | Memory | Expected σ | n = 1,000 (mean / worst) | n = 100,000 (mean / worst) | n = 10,000,000 (mean / worst) |
|---:|---:|---:|---:|---:|---:|
| 10 | 1 KiB | 3.25% | 1.95% / 6.80% | 2.75% / 6.85% | 2.11% / 6.80% |
| 12 | 4 KiB | 1.62% | 0.95% / 3.00% | 1.12% / 2.72% | 1.24% / 2.99% |
| 14 | 16 KiB | 0.81% | 0.33% / 1.30% | 0.54% / 1.35% | 0.88% / 1.84% |
| 16 | 64 KiB | 0.41% | 0.20% / 0.40% | 0.32% / 0.71% | 0.31% / 0.85% |

### A week from seven daily sketches (p = 14, 16 KiB each)

| Exact weekly uniques | Merged estimate | Error |
|---:|---:|---:|
| 800,000 | 810,545 | 1.32% |

## Top-k heavy hitters (`cmd/topkdemo`)

20,000,000 events over 5,000,000 distinct keys.

### Zipf s=1.1 (the 10th most frequent key appears 188,323 times)

| Method | Memory | ns per event | True top 10 found | Worst count error in its top 10 (vs the true 10th count) |
|---|---:|---:|---:|---:|
| exact map | ~57.6 MiB | 46 | 10 | 0 |
| count-min sketch + heap, width 4,096 × 4 | 129 KiB | 61 | 10 | 0.7% |
| count-min sketch + heap, width 65,536 × 4 | 2.0 MiB | 66 | 10 | 0.0% |
| Space-Saving, 100 counters | 6 KiB | 117 | 10 | 0.0% |
| Space-Saving, 1,000 counters | 62 KiB | 112 | 10 | 0.0% |

### Zipf s=0.8 (the 10th most frequent key appears 29,935 times)

| Method | Memory | ns per event | True top 10 found | Worst count error in its top 10 (vs the true 10th count) |
|---|---:|---:|---:|---:|
| exact map | ~149.1 MiB | 116 | 10 | 0 |
| count-min sketch + heap, width 4,096 × 4 | 129 KiB | 64 | 10 | 13.6% |
| count-min sketch + heap, width 65,536 × 4 | 2.0 MiB | 79 | 10 | 0.8% |
| Space-Saving, 100 counters | 6 KiB | 186 | 2 | 668.1% |
| Space-Saving, 1,000 counters | 62 KiB | 225 | 10 | 0.0% |

## Leaderboards (`cmd/skiplistdemo`)

Each board is filled with n players, then measured on random score updates, rank lookups and top-10 reads.

| Players | Structure | Update | Rank of a player | Top 10 |
|---:|---|---:|---:|---:|
| 10,000 | skip list | 1.1 µs | 453 ns | 2.8 µs |
| 10,000 | sorted slice | 4.1 µs | 203 ns | 254 ns |
| 10,000 | map + scan | 52 ns | 174.8 µs | 2.4 ms |
| 100,000 | skip list | 2.5 µs | 1.1 µs | 577 ns |
| 100,000 | sorted slice | 279.9 µs | 517 ns | 288 ns |
| 100,000 | map + scan | 99 ns | 1.6 ms | 31.4 ms |
| 1,000,000 | skip list | 6.3 µs | 3.2 µs | 429 ns |
| 1,000,000 | sorted slice | 3.4 ms | 845 ns | 676 ns |
| 1,000,000 | map + scan | 354 ns | 21.6 ms | 390.6 ms |

## Finding differences between replicas (`cmd/merkledemo`)

1,000,000 keys, 200-byte values. Full copy: 203.1 MiB; a hash per key: 42.9 MiB, in one round trip each.

| Keys that differ | Merkle, 4,096 leaves: bytes / round trips | Merkle, 65,536 leaves: bytes / round trips | Found all? |
|---:|---:|---:|---|
| 0 | 32 B / 1 | 32 B / 1 | true |
| 1 | 11.6 KiB / 14 | 2.0 KiB / 18 | true |
| 10 | 114.7 KiB / 14 | 15.2 KiB / 18 | true |
| 100 | 1.1 MiB / 14 | 128.9 KiB / 18 | true |
| 1,000 | 9.5 MiB / 14 | 1.1 MiB / 18 | true |
| 10,000 | 39.3 MiB / 14 | 8.3 MiB / 18 | true |

## Finding what's nearby (`cmd/geodemo`)

1,000,000 points over India, half clustered around 10 cities. Build time: geohash 1376 ms (6 precisions), quadtree 335 ms. 200 queries per row (the full scan runs the first 20 of them, so its average differs slightly), half in a city centre and half anywhere.

| Radius | Method | Points checked per query (avg) | Found (avg) | Time per query |
|---:|---|---:|---:|---:|
| 1 km | full scan | 1,000,000 | 165 | 104.259ms |
| 1 km | geohash cells | 8,243 | 160 | 1.23ms |
| 1 km | quadtree | 204 | 160 | 43µs |
| 2 km | full scan | 1,000,000 | 632 | 107.238ms |
| 2 km | geohash cells | 8,285 | 639 | 1.351ms |
| 2 km | quadtree | 809 | 639 | 161µs |
| 5 km | full scan | 1,000,000 | 3707 | 131.148ms |
| 5 km | geohash cells | 29,536 | 3733 | 4.014ms |
| 5 km | quadtree | 4,635 | 3733 | 851µs |

