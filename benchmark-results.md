# CRI List vs Stream Benchmark Results

Date: 2026-03-11
Platform: linux/amd64, Intel Xeon @ 2.20GHz, 16 cores
Go: 1.26.0
Branch: bitoku/kubernetes@kep-5825-cri (commit 3ba7af3bb754)
Config: Stream sends all containers in a single response (1 chunk)

## Speed (ns/op, per iteration, 3 counts)

| Containers | Annotations | List | Stream | Delta |
|---|---|---|---|---|
| 64 | 8 | 2.01 ms | 2.02 ms | +0.5% |
| 64 | 32 | 5.61 ms | 5.57 ms | -0.7% |
| 256 | 8 | 6.75 ms | 6.81 ms | +0.9% |
| 256 | 32 | 20.4 ms | 20.6 ms | +1.0% |
| 1024 | 8 | 23.9 ms | 23.8 ms | -0.4% |
| 1024 | 32 | 73.1 ms | 73.5 ms | +0.5% |

## Memory Per Iteration (B/op and allocs/op, 3 counts)

| Containers | Annotations | List B/op | Stream B/op | List allocs | Stream allocs |
|---|---|---|---|---|---|
| 64 | 8 | 315K | 315K | 9,339 | 9,342 |
| 64 | 32 | 1.75M | 1.74M | 28,362 | 28,365 |
| 256 | 8 | 1.96M | 1.89M | 36,812 | 36,815 |
| 256 | 32 | 5.43M | 5.45M | 112,887 | 112,891 |
| 1024 | 8 | 6.05M | 6.08M | 158,215 | 158,220 |
| 1024 | 32 | 19.4M | 19.6M | 480,893 | 480,922 |

## Memory Profile Per Iteration (1024 containers, 32 annotations)

| Allocation site | List | Stream |
|---|---|---|
| `reflect.mapassign_faststr0` (map entries) | 4.79 MB | 5.12 MB |
| `makeContainers` (server-side) | 3.36 MB | 3.44 MB |
| `grpc/mem.simpleBufferPool.Get` (send buffer) | 2.44 MB | 2.71 MB |
| `reflect.unsafe_New` (proto message alloc) | 2.70 MB | 2.23 MB |
| `consumeStringValueValidateUTF8` | 1.62 MB | 1.90 MB |
| `fmt.Sprintf` | 1.41 MB | 1.71 MB |
| `protoreflect.Value.Interface` | 1.24 MB | 0.96 MB |
| `stringConverter.GoValueOf` | 1.11 MB | 1.21 MB |
| **Total** | **~19.6 MB** | **~20.2 MB** |

Stream uses ~3% more memory per iteration, mainly from gRPC buffer pool overhead.

## CPU Time Per Iteration (1024 containers, 32 annotations)

Total CPU samples per iteration: List = 101.09 ms, Stream = 101.04 ms

| Category | List | Stream |
|---|---|---|
| Memory allocation (`mallocgc` + `nextFreeFast` + `writeHeapBitsSmall`) | 31.5 ms | 30.2 ms |
| Protobuf marshal/unmarshal (`consumeMap` + `appendStringValidateUTF8` + `appendMapItem`) | 30.9 ms | 34.6 ms |
| GC scanning (`scanObject` + `tryDeferToSpanScan`) | 12.6 ms | 13.5 ms |
| fmt.Sprintf (container construction) | 7.6 ms | 7.9 ms |
| Runtime (`memmove` + `memclr` + `futex`) | 7.4 ms | 9.8 ms |
| Hashing (`aeshashbody`) | 2.2 ms | 1.9 ms |

## Conclusion: List vs Stream

When all containers are sent in a single stream response, List and Stream are functionally equivalent across all dimensions:
- Speed: within noise (<1% difference)
- Memory: stream uses ~3% more from gRPC streaming buffer overhead
- CPU: identical (~101 ms/iteration), dominated by protobuf map serialization and memory allocation (~60% of CPU)
- Allocations: stream has ~3 extra allocs per call (stream setup), negligible

---

## Impact of `proto.Size()` on StreamContainers

Config: 1024 containers, 32 annotations, all in a single stream response, 5 counts

### Speed (ns/op)

| Metric | Without `proto.Size()` | With `proto.Size()` | Overhead |
|---|---|---|---|
| Speed | ~69.9 ms/op | ~78.4 ms/op | **+12.1%** |
| Memory | ~19.8 MB/op | ~21.6 MB/op | **+9.1%** |
| Allocations | ~480,923/op | ~550,705/op | **+14.5%** (+69,782 allocs) |

### CPU Per Iteration

Total CPU: without = 96.8 ms, with = 107.8 ms (**+11.3%**)

| Category | Without | With | Delta |
|---|---|---|---|
| Memory allocation (`mallocgc` + `nextFreeFast` + `writeHeapBitsSmall`) | 26.0 ms | 32.4 ms | +6.4 ms |
| Protobuf (`consumeMap`) | 25.4 ms | 24.9 ms | -0.5 ms |
| GC (`scanObject` + `tryDeferToSpanScan` + `scanObjectsSmall`) | 13.3 ms | 19.8 ms | +6.5 ms |
| `memmove` + `memclr` | 7.4 ms | 8.1 ms | +0.7 ms |
| `fmt.Sprintf` | 6.7 ms | 7.1 ms | +0.4 ms |
| Map iteration (`maps.Iter.Next`) | 2.6 ms | 4.0 ms | +1.4 ms |
| Syscalls | 2.4 ms | 2.8 ms | +0.4 ms |

The extra ~11 ms/iter CPU comes primarily from memory allocation (+6.4 ms) and GC scanning (+6.5 ms).

### Memory Per Iteration

| Allocation site | Without | With | Delta |
|---|---|---|---|
| `reflect.mapassign_faststr0` | 5.03 MB | 4.70 MB | -0.33 MB |
| `reflect.unsafe_New` | 2.53 MB | 3.84 MB | **+1.31 MB** |
| `makeContainers` | 3.48 MB | 3.01 MB | -0.47 MB |
| `grpc/mem.simpleBufferPool.Get` | 2.46 MB | 2.89 MB | +0.43 MB |
| `consumeStringValueValidateUTF8` | 1.70 MB | 1.75 MB | +0.05 MB |
| `fmt.Sprintf` | 1.58 MB | 1.69 MB | +0.11 MB |
| `protoreflect.Value.Interface` | 1.08 MB | 1.15 MB | +0.07 MB |
| `stringConverter.GoValueOf` | 1.03 MB | 1.17 MB | +0.14 MB |
| **Total** | **19.6 MB** | **20.9 MB** | **+1.3 MB (+7%)** |

### Conclusion: proto.Size() Impact

`proto.Size()` adds ~12% overhead across speed, CPU, and memory. The cost comes from
reflection-based message tree walking — `reflect.unsafe_New` jumps from 2.53 MB to
3.84 MB (+1.31 MB) as it allocates intermediate `reflect.Value` objects for every proto
message and map entry. The ~70K extra allocations per call create additional GC pressure,
accounting for most of the CPU overhead.
