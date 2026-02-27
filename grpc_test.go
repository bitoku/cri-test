package main

import (
	"context"
	"fmt"
	"testing"

	internalapi "k8s.io/cri-api/pkg/apis"
)

// Separate benchmarks for list-only and stream-only so CPU/mem profiles
// are not mixed together.

func benchmarkListOnly(b *testing.B, numContainers, numAnnotations, numPerChunk int) {
	b.Helper()

	sock, stop := startTestServer(b, numContainers, numAnnotations, numPerChunk)
	defer stop()

	client := newCRIClient(b, sock, false)
	defer client.Close()

	// warm up
	_, _ = client.ListContainers(context.Background(), nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := client.ListContainers(context.Background(), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkStreamOnly(b *testing.B, numContainers, numAnnotations, numPerChunk int) {
	b.Helper()

	sock, stop := startTestServer(b, numContainers, numAnnotations, numPerChunk)
	defer stop()

	client := newCRIClient(b, sock, true)
	defer client.Close()

	// warm up
	_, _ = client.ListContainers(context.Background(), nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := client.ListContainers(context.Background(), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGRPCList(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			for _, perChunk := range chunkCounts {
				if perChunk > containers {
					continue
				}
				b.Run(fmt.Sprintf("containers=%d/annotations=%d/perChunk=%d", containers, annotations, perChunk), func(b *testing.B) {
					benchmarkListOnly(b, containers, annotations, perChunk)
				})
			}
		}
	}
}

func BenchmarkGRPCStream(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			for _, perChunk := range chunkCounts {
				if perChunk > containers {
					continue
				}
				b.Run(fmt.Sprintf("containers=%d/annotations=%d/perChunk=%d", containers, annotations, perChunk), func(b *testing.B) {
					benchmarkStreamOnly(b, containers, annotations, perChunk)
				})
			}
		}
	}
}

// BenchmarkGRPCAllocs is a paired allocation comparison at key data points.
func BenchmarkGRPCAllocs(b *testing.B) {
	type pair struct {
		containers, annotations int
	}
	pairs := []pair{{64, 8}, {256, 16}, {512, 32}, {1024, 32}}

	for _, p := range pairs {
		name := fmt.Sprintf("containers=%d/annotations=%d", p.containers, p.annotations)

		for _, streaming := range []bool{false, true} {
			label := "list"
			if streaming {
				label = "stream"
			}
			b.Run(name+"/"+label, func(b *testing.B) {
				sock, stop := startTestServer(b, p.containers, p.annotations, p.containers)
				defer stop()

				var rs internalapi.RuntimeService
				rs = newCRIClient(b, sock, streaming)
				defer rs.Close()

				_, _ = rs.ListContainers(context.Background(), nil)

				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_, err := rs.ListContainers(context.Background(), nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
