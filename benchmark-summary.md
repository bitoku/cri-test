## CRI gRPC Benchmark Summary: List vs Stream

### Setup
- **Benchmark tool:** `cri-bench` with a mock CRI server
- **Platform:** Linux amd64, Intel Xeon @ 2.20GHz (16 cores)
- **Method:** Fixed iteration count (N) to ensure fair wall-time comparison

### Results

#### containers=32, annotations=32 (N=1300)

| Metric | List | Stream | Delta |
|---|---|---|---|
| Wall time | 3.50s | 2.60s | **Stream 26% faster** |
| Total CPU | 4.51s | 7.07s | Stream 57% more CPU |
| CPU/wall ratio | 1.29x | 2.72x | Stream uses 2x more cores |
| Latency/op | 2.67 ms | 1.98 ms | **Stream 26% lower** |
| Bytes/op | 880 KB | 538 KB | **Stream 39% less memory** |
| Allocs/op | 14,271 | 14,563 | ~Same |

#### containers=1024, annotations=32 (N=100)

| Metric | List | Stream | Delta |
|---|---|---|---|
| Wall time | 6.78s | 5.15s | **Stream 24% faster** |
| Total CPU | 8.57s | 16.43s | Stream 92% more CPU |
| CPU/wall ratio | 1.26x | 3.19x | Stream uses 2.5x more cores |
| Latency/op | 65.6 ms | 49.8 ms | **Stream 24% lower** |
| Bytes/op | 19.3 MB | 17.1 MB | **Stream 11% less memory** |
| Allocs/op | 480,889 | 490,889 | ~Same |

### Chunk Size Impact (containers=1024, annotations=32, N=100)

#### List (perChunk has no effect — single unary response)

| Metric | List (perChunk=16) | List (perChunk=256) |
|---|---|---|
| Wall time | 6.74s | 6.81s |
| Total CPU | 8.42s (125%) | 8.55s (126%) |
| Latency/op | 65.2 ms | 65.8 ms |
| Bytes/op | 18.8 MB | 19.1 MB |
| Allocs/op | 480,862 | 480,879 |

#### Stream: perChunk=16 vs perChunk=256

| Metric | Stream (perChunk=16) | Stream (perChunk=256) | Delta |
|---|---|---|---|
| Wall time | 5.18s | 5.41s | ~Same |
| Total CPU | 11.68s (226%) | 9.32s (172%) | **256 uses 20% less CPU** |
| CPU/wall ratio | 2.26x | 1.72x | 256 is more sequential |
| Latency/op | 50.1 ms | 52.3 ms | ~Same (+4%) |
| Bytes/op | 17.0 MB | 20.7 MB | **256 uses 22% more memory** |
| Allocs/op | 482,324 | 480,980 | ~Same |

#### Stream vs List (all configurations)

| Metric | List | Stream c=16 | Stream c=256 |
|---|---|---|---|
| **Latency/op** | 65.5 ms | **50.1 ms** (-24%) | **52.3 ms** (-20%) |
| **Wall time** | 6.78s | **5.18s** (-24%) | **5.41s** (-20%) |
| **Total CPU** | 8.49s | 11.68s (+38%) | 9.32s (+10%) |
| **CPU/wall** | 1.25x | 2.26x | 1.72x |
| **Bytes/op** | 18.9 MB | **17.0 MB** (-10%) | 20.7 MB (+10%) |
| **Allocs/op** | 480,871 | 482,324 | 480,980 |

### Key Findings

1. **Stream is ~25% faster in wall time** at both scales, by parallelizing server-side sends with client-side receives.

2. **Stream uses significantly more CPU** (2.7-3.2x the wall time vs ~1.3x for list). The per-container Send/Recv cycle drives more goroutine scheduling, syscalls, and context switching.

3. **Memory savings diminish at scale.** Stream saves 39% memory at 32 containers but only 11% at 1024 containers. At scale, per-container protobuf costs dominate and the fixed overhead of the single large list response becomes proportionally smaller.

4. **Allocation counts are nearly identical** (~2% more for stream), meaning the memory savings come from smaller per-allocation sizes, not fewer allocations.

5. **CPU bottlenecks differ:**
   - **List:** dominated by `memclrNoHeapPointers` (12.6%), protobuf marshal/unmarshal (37%), and map operations.
   - **Stream:** dominated by syscalls (9.8%), GC (10.8%), goroutine scheduling (13%+), and protobuf work spread across many small messages.

6. **Chunk size is a CPU-memory trade-off for streaming:**
   - **perChunk=16** (64 messages): Best memory savings (-10% vs list), best latency (-24%), but highest CPU overhead (2.26x wall).
   - **perChunk=256** (4 messages): CPU overhead drops to 1.72x (approaching list), but memory savings disappear (+10% vs list) as each chunk is large enough to resemble list.
   - Both chunk sizes still beat list on wall-time latency.

### Trade-off

Streaming trades **CPU for latency and memory**. It's the better choice when wall-time latency matters and CPU cores are available. List is more CPU-efficient (single-threaded) and simpler, making it preferable when CPU is the bottleneck or core count is limited.

Chunk size tunes the trade-off within streaming: **small chunks** maximize memory savings and parallelism at the cost of CPU, while **large chunks** reduce CPU overhead but converge toward list's memory profile.
