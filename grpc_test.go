package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	internalapi "k8s.io/cri-api/pkg/apis"
)

// Separate benchmarks for list-only and stream-only so CPU/mem profiles
// are not mixed together.

func benchmarkListOnly(b *testing.B, numContainers, numAnnotations int) {
	b.Helper()

	sock, stop := startTestServer(b, numContainers, numAnnotations)
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

func benchmarkStreamOnly(b *testing.B, numContainers, numAnnotations int) {
	b.Helper()

	sock, stop := startTestServer(b, numContainers, numAnnotations)
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
			b.Run(fmt.Sprintf("containers=%d/annotations=%d", containers, annotations), func(b *testing.B) {
				benchmarkListOnly(b, containers, annotations)
			})
		}
	}
}

func BenchmarkGRPCStream(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			b.Run(fmt.Sprintf("containers=%d/annotations=%d", containers, annotations), func(b *testing.B) {
				benchmarkStreamOnly(b, containers, annotations)
			})
		}
	}
}

// BenchmarkGRPCLatencyBreakdown measures client-side and server-side latency
// separately by timing the server handler directly.
func BenchmarkGRPCLatencyBreakdown(b *testing.B) {
	type mode struct {
		name      string
		streaming bool
	}
	modes := []mode{{"list", false}, {"stream", true}}

	for _, m := range modes {
		for _, containers := range []int{256, 1024} {
			for _, annotations := range []int{8, 32} {
				name := fmt.Sprintf("mode=%s/containers=%d/annotations=%d", m.name, containers, annotations)
				b.Run(name, func(b *testing.B) {
					sock, stop := startTestServer(b, containers, annotations)
					defer stop()

					client := newCRIClient(b, sock, m.streaming)
					defer client.Close()

					_, _ = client.ListContainers(context.Background(), nil)

					var total time.Duration
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						start := time.Now()
						_, err := client.ListContainers(context.Background(), nil)
						total += time.Since(start)
						if err != nil {
							b.Fatal(err)
						}
					}
					b.ReportMetric(float64(total.Microseconds())/float64(b.N), "us/req")
				})
			}
		}
	}
}

// BenchmarkGRPCServerSide isolates the server-side work by directly calling
// the service handler, bypassing gRPC transport entirely.
func BenchmarkGRPCServerSide(b *testing.B) {
	for _, containers := range []int{256, 1024} {
		for _, annotations := range []int{8, 32} {
			name := fmt.Sprintf("containers=%d/annotations=%d", containers, annotations)

			b.Run(name+"/unary", func(b *testing.B) {
				svc := &criService{numContainers: containers, numAnnotations: annotations}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_, err := svc.ListContainers(context.Background(), nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(name+"/stream-build", func(b *testing.B) {
				// Simulates streaming server work: build each container individually.
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					for i := range containers {
						c := makeContainer(i, annotations)
						_ = c
					}
				}
			})
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
				sock, stop := startTestServer(b, p.containers, p.annotations)
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
