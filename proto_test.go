package main

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// BenchmarkProtoMarshal isolates protobuf serialization cost:
// one large ListContainersResponse vs N small StreamContainersResponses.
func BenchmarkProtoMarshal(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			name := fmt.Sprintf("containers=%d/annotations=%d", containers, annotations)

			// Build containers once, reuse across sub-benchmarks.
			cs := make([]*runtimeapi.Container, containers)
			for i := range cs {
				cs[i] = makeContainer(i, annotations)
			}

			b.Run(name+"/unary", func(b *testing.B) {
				msg := &runtimeapi.ListContainersResponse{Containers: cs}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					data, err := proto.Marshal(msg)
					if err != nil {
						b.Fatal(err)
					}
					_ = data
				}
			})

			b.Run(name+"/stream", func(b *testing.B) {
				msgs := make([]*runtimeapi.StreamContainersResponse, containers)
				for i, c := range cs {
					msgs[i] = &runtimeapi.StreamContainersResponse{Container: c}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					for _, m := range msgs {
						data, err := proto.Marshal(m)
						if err != nil {
							b.Fatal(err)
						}
						_ = data
					}
				}
			})
		}
	}
}

// BenchmarkProtoUnmarshal isolates protobuf deserialization cost.
func BenchmarkProtoUnmarshal(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			name := fmt.Sprintf("containers=%d/annotations=%d", containers, annotations)

			cs := make([]*runtimeapi.Container, containers)
			for i := range cs {
				cs[i] = makeContainer(i, annotations)
			}

			b.Run(name+"/unary", func(b *testing.B) {
				msg := &runtimeapi.ListContainersResponse{Containers: cs}
				data, err := proto.Marshal(msg)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					out := new(runtimeapi.ListContainersResponse)
					if err := proto.Unmarshal(data, out); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(name+"/stream", func(b *testing.B) {
				blobs := make([][]byte, containers)
				for i, c := range cs {
					m := &runtimeapi.StreamContainersResponse{Container: c}
					data, err := proto.Marshal(m)
					if err != nil {
						b.Fatal(err)
					}
					blobs[i] = data
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					for _, data := range blobs {
						out := new(runtimeapi.StreamContainersResponse)
						if err := proto.Unmarshal(data, out); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

// BenchmarkProtoSize reports the wire sizes for comparison.
func BenchmarkProtoSize(b *testing.B) {
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			name := fmt.Sprintf("containers=%d/annotations=%d", containers, annotations)

			cs := make([]*runtimeapi.Container, containers)
			for i := range cs {
				cs[i] = makeContainer(i, annotations)
			}

			unaryMsg := &runtimeapi.ListContainersResponse{Containers: cs}
			unaryData, _ := proto.Marshal(unaryMsg)

			var streamTotal int
			for _, c := range cs {
				m := &runtimeapi.StreamContainersResponse{Container: c}
				data, _ := proto.Marshal(m)
				streamTotal += len(data)
			}

			b.Run(name, func(b *testing.B) {
				b.ReportMetric(float64(len(unaryData)), "unary-bytes")
				b.ReportMetric(float64(streamTotal), "stream-bytes-total")
				b.ReportMetric(float64(streamTotal-len(unaryData)), "stream-overhead-bytes")
				b.ReportMetric(float64(streamTotal-len(unaryData))/float64(len(unaryData))*100, "overhead-%")
			})
		}
	}
}
