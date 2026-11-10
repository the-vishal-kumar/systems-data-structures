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
