package main

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	cri "k8s.io/cri-client/pkg"
)

func startTestServer(tb testing.TB, n int) string {
	tb.Helper()
	sock := fmt.Sprintf("%s/cri-bench-%d.sock", tb.TempDir(), time.Now().UnixNano())
	lis, err := net.Listen("unix", sock)
	if err != nil {
		tb.Fatal(err)
	}
	srv := grpc.NewServer()
	svc := &criService{numContainers: n}
	runtimev1.RegisterRuntimeServiceServer(srv, svc)
	runtimev1.RegisterImageServiceServer(srv, svc)
	go srv.Serve(lis)
	tb.Cleanup(srv.GracefulStop)
	return sock
}

func newCRIClient(tb testing.TB, sock string, useStreaming bool) internalapi.RuntimeService {
	tb.Helper()
	endpoint := "unix://" + sock
	client, err := cri.NewRemoteRuntimeService(endpoint, 10*time.Second, nil, nil, useStreaming)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { client.Close() })
	return client
}

var benchCases = []struct {
	containers int
	tries      int
}{
	{10, 10000},
	{100, 1000},
	{1000, 100},
	{10000, 100},
}

func benchRequests(b *testing.B, client internalapi.RuntimeService, expectedContainers, tries int) time.Duration {
	b.Helper()

	// warm up
	containers, err := client.ListContainers(context.Background(), nil)
	if err != nil {
		b.Fatal(err)
	}
	if len(containers) != expectedContainers {
		b.Fatalf("expected %d containers, got %d", expectedContainers, len(containers))
	}

	start := time.Now()
	for range tries {
		_, err := client.ListContainers(context.Background(), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
	return time.Since(start)
}

func BenchmarkCRIListVsStream(b *testing.B) {
	for _, bc := range benchCases {
		b.Run(fmt.Sprintf("containers=%d/tries=%d", bc.containers, bc.tries), func(b *testing.B) {
			sock := startTestServer(b, bc.containers)
			listClient := newCRIClient(b, sock, false)
			streamClient := newCRIClient(b, sock, true)

			var listTotal, streamTotal time.Duration

			for n := 0; n < b.N; n++ {
				listTotal += benchRequests(b, listClient, bc.containers, bc.tries)
				streamTotal += benchRequests(b, streamClient, bc.containers, bc.tries)
			}

			totalRequests := float64(b.N) * float64(bc.tries)
			listNsPerReq := float64(listTotal.Nanoseconds()) / totalRequests
			streamNsPerReq := float64(streamTotal.Nanoseconds()) / totalRequests

			b.ReportMetric(listNsPerReq, "list-ns/req")
			b.ReportMetric(streamNsPerReq, "stream-ns/req")

			diff := (streamNsPerReq - listNsPerReq) / listNsPerReq * 100
			if diff > 0 {
				b.ReportMetric(diff, "list-faster-%")
			} else {
				b.ReportMetric(-diff, "stream-faster-%")
			}
		})
	}
}
