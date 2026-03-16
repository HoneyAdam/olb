# OpenLoadBalancer Benchmark Report

**Date:** 2026-03-16 10:17:40
**Platform:** Go 1.25

## Summary

| Test | Requests | Concurrency | RPS | Avg Latency | P99 Latency | Success |
|------|----------|-------------|-----|-------------|-------------|--------|
| Algorithm: round_robin | 1000 | 50 | 7320 | 6.265ms | 27.796ms | 100.0% |
| Algorithm: weighted_round_robin | 1000 | 50 | 8042 | 5.559ms | 20.99ms | 100.0% |
| Algorithm: least_connections | 1000 | 50 | 10119 | 4.367ms | 26.253ms | 100.0% |
| Algorithm: ip_hash | 1000 | 50 | 11062 | 3.965ms | 23.566ms | 100.0% |
| Algorithm: consistent_hash | 1000 | 50 | 8897 | 4.622ms | 26.701ms | 100.0% |
| Algorithm: maglev | 1000 | 50 | 11597 | 3.784ms | 27.916ms | 100.0% |
| Algorithm: power_of_two | 1000 | 50 | 10708 | 4.002ms | 30.048ms | 100.0% |
| Algorithm: random | 1000 | 50 | 12913 | 3.45ms | 28.313ms | 100.0% |
| Concurrency: 1 | 1000 | 1 | 6137 | 161µs | 684µs | 100.0% |
| Concurrency: 10 | 1000 | 10 | 15480 | 624µs | 5.405ms | 100.0% |
| Concurrency: 50 | 1000 | 50 | 12350 | 3.509ms | 22.283ms | 100.0% |
| Concurrency: 100 | 1000 | 100 | 11212 | 7.217ms | 46.666ms | 100.0% |
| Backend latency: 0 (instant) | 500 | 50 | 9587 | 4.47ms | 31.445ms | 100.0% |
| Backend latency: 1ms | 500 | 50 | 8023 | 5.379ms | 28.303ms | 100.0% |
| Backend latency: 5ms | 500 | 50 | 3044 | 13.439ms | 46.368ms | 100.0% |
| Backend latency: 10ms | 500 | 50 | 1815 | 22.776ms | 49.892ms | 100.0% |
| No middleware | 1000 | 50 | 11397 | 3.888ms | 37.136ms | 100.0% |
| Full middleware (rate+cors+gzip) | 1000 | 50 | 12697 | 3.391ms | 27.722ms | 100.0% |
| WAF enabled | 1000 | 50 | 11822 | 3.642ms | 26.786ms | 100.0% |
| Proxy overhead: 137µs (direct=87µs, proxy=223µs) | 200 | 1 | 4476 | 223µs | 1.015ms | 100.0% |

## Details

### Algorithm: round_robin

- **RPS:** 7320
- **Throughput:** 0.34 MB/s
- **Latency:** avg=6.265ms p50=4.966ms p95=16.962ms p99=27.796ms max=33.433ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 351 | 35.1% |
| backend-3 | 324 | 32.4% |
| backend-2 | 325 | 32.5% |

### Algorithm: weighted_round_robin

- **RPS:** 8042
- **Throughput:** 0.37 MB/s
- **Latency:** avg=5.559ms p50=3.672ms p95=15.394ms p99=20.99ms max=25.725ms

| Backend | Hits | % |
|---------|------|---|
| backend-2 | 333 | 33.3% |
| backend-1 | 334 | 33.4% |
| backend-3 | 333 | 33.3% |

### Algorithm: least_connections

- **RPS:** 10119
- **Throughput:** 0.46 MB/s
- **Latency:** avg=4.367ms p50=1.681ms p95=15.874ms p99=26.253ms max=32.891ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 333 | 33.3% |
| backend-2 | 328 | 32.8% |
| backend-3 | 339 | 33.9% |

### Algorithm: ip_hash

- **RPS:** 11062
- **Throughput:** 0.51 MB/s
- **Latency:** avg=3.965ms p50=1.556ms p95=15.516ms p99=23.566ms max=31.268ms

| Backend | Hits | % |
|---------|------|---|
| backend-2 | 129 | 12.9% |
| backend-3 | 120 | 12.0% |
| backend-1 | 751 | 75.1% |

### Algorithm: consistent_hash

- **RPS:** 8897
- **Throughput:** 0.41 MB/s
- **Latency:** avg=4.622ms p50=1.07ms p95=21.739ms p99=26.701ms max=30.375ms

| Backend | Hits | % |
|---------|------|---|
| backend-3 | 1000 | 100.0% |
| backend-1 | 0 | 0.0% |
| backend-2 | 0 | 0.0% |

### Algorithm: maglev

- **RPS:** 11597
- **Throughput:** 0.53 MB/s
- **Latency:** avg=3.784ms p50=1.025ms p95=16.656ms p99=27.916ms max=31.371ms

| Backend | Hits | % |
|---------|------|---|
| backend-2 | 24 | 2.4% |
| backend-1 | 677 | 67.7% |
| backend-3 | 299 | 29.9% |

### Algorithm: power_of_two

- **RPS:** 10708
- **Throughput:** 0.49 MB/s
- **Latency:** avg=4.002ms p50=676µs p95=20.312ms p99=30.048ms max=35.832ms

| Backend | Hits | % |
|---------|------|---|
| backend-3 | 333 | 33.3% |
| backend-1 | 342 | 34.2% |
| backend-2 | 325 | 32.5% |

### Algorithm: random

- **RPS:** 12913
- **Throughput:** 0.59 MB/s
- **Latency:** avg=3.45ms p50=641µs p95=16.158ms p99=28.313ms max=30.02ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 321 | 32.1% |
| backend-2 | 342 | 34.2% |
| backend-3 | 337 | 33.7% |

### Concurrency: 1

- **RPS:** 6137
- **Throughput:** 0.28 MB/s
- **Latency:** avg=161µs p50=0s p95=528µs p99=684µs max=3.125ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 310 | 31.0% |
| backend-2 | 325 | 32.5% |
| backend-3 | 365 | 36.5% |

### Concurrency: 10

- **RPS:** 15480
- **Throughput:** 0.71 MB/s
- **Latency:** avg=624µs p50=529µs p95=1.59ms p99=5.405ms max=9.587ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 343 | 34.3% |
| backend-2 | 320 | 32.0% |
| backend-3 | 337 | 33.7% |

### Concurrency: 50

- **RPS:** 12350
- **Throughput:** 0.57 MB/s
- **Latency:** avg=3.509ms p50=610µs p95=16.039ms p99=22.283ms max=26.281ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 334 | 33.4% |
| backend-2 | 343 | 34.3% |
| backend-3 | 323 | 32.3% |

### Concurrency: 100

- **RPS:** 11212
- **Throughput:** 0.51 MB/s
- **Latency:** avg=7.217ms p50=602µs p95=34.538ms p99=46.666ms max=49.94ms

| Backend | Hits | % |
|---------|------|---|
| backend-3 | 309 | 30.9% |
| backend-1 | 343 | 34.3% |
| backend-2 | 348 | 34.8% |

### Backend latency: 0 (instant)

- **RPS:** 9587
- **Throughput:** 0.44 MB/s
- **Latency:** avg=4.47ms p50=682µs p95=28.417ms p99=31.445ms max=33.758ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 165 | 33.0% |
| backend-3 | 164 | 32.8% |
| backend-2 | 171 | 34.2% |

### Backend latency: 1ms

- **RPS:** 8023
- **Throughput:** 0.37 MB/s
- **Latency:** avg=5.379ms p50=3.402ms p95=22.29ms p99=28.303ms max=29.736ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 163 | 32.6% |
| backend-2 | 172 | 34.4% |
| backend-3 | 165 | 33.0% |

### Backend latency: 5ms

- **RPS:** 3044
- **Throughput:** 0.14 MB/s
- **Latency:** avg=13.439ms p50=10.543ms p95=39.157ms p99=46.368ms max=49.089ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 167 | 33.4% |
| backend-2 | 158 | 31.6% |
| backend-3 | 175 | 35.0% |

### Backend latency: 10ms

- **RPS:** 1815
- **Throughput:** 0.08 MB/s
- **Latency:** avg=22.776ms p50=20.463ms p95=40.353ms p99=49.892ms max=52.542ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 151 | 30.2% |
| backend-2 | 171 | 34.2% |
| backend-3 | 178 | 35.6% |

### No middleware

- **RPS:** 11397
- **Throughput:** 0.52 MB/s
- **Latency:** avg=3.888ms p50=557µs p95=20.451ms p99=37.136ms max=39.263ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 324 | 32.4% |
| backend-3 | 340 | 34.0% |
| backend-2 | 336 | 33.6% |

### Full middleware (rate+cors+gzip)

- **RPS:** 12697
- **Throughput:** 0.58 MB/s
- **Latency:** avg=3.391ms p50=558µs p95=19.876ms p99=27.722ms max=32.748ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 333 | 33.3% |
| backend-2 | 359 | 35.9% |
| backend-3 | 308 | 30.8% |

### WAF enabled

- **RPS:** 11822
- **Throughput:** 0.54 MB/s
- **Latency:** avg=3.642ms p50=571µs p95=17.583ms p99=26.786ms max=29.531ms

| Backend | Hits | % |
|---------|------|---|
| backend-1 | 344 | 34.4% |
| backend-3 | 331 | 33.1% |
| backend-2 | 325 | 32.5% |

### Proxy overhead: 137µs (direct=87µs, proxy=223µs)

- **RPS:** 4476
- **Throughput:** 0.00 MB/s
- **Latency:** avg=223µs p50=0s p95=542µs p99=1.015ms max=4.18ms

