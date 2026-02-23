package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	cri "k8s.io/cri-client/pkg"
)

func startTestServer(tb testing.TB, numContainers, numAnnotations int) (string, func()) {
	tb.Helper()
	sock := fmt.Sprintf("/tmp/cri-%d.sock", time.Now().UnixNano())
	tb.Cleanup(func() { os.Remove(sock) })
	lis, err := net.Listen("unix", sock)
	if err != nil {
		tb.Fatal(err)
	}
	srv := grpc.NewServer()
	svc := &criService{numContainers: numContainers, numAnnotations: numAnnotations}
	runtimev1.RegisterRuntimeServiceServer(srv, svc)
	runtimev1.RegisterImageServiceServer(srv, svc)
	go srv.Serve(lis)
	return sock, srv.GracefulStop
}

func newCRIClient(tb testing.TB, sock string, useStreaming bool) internalapi.RuntimeService {
	tb.Helper()
	endpoint := "unix://" + sock
	client, err := cri.NewRemoteRuntimeService(endpoint, 10*time.Second, nil, nil, useStreaming)
	if err != nil {
		tb.Fatal(err)
	}
	return client
}

var (
	containerCounts  = []int{4, 8, 16, 32, 64, 128, 256, 512, 1024}
	annotationCounts = []int{1, 2, 4, 8, 16, 32}
	tries            = 100
)

func benchRequests(b *testing.B, useStreaming bool, numContainers, numAnnotations, tries int) time.Duration {
	b.Helper()

	sock, stop := startTestServer(b, numContainers, numAnnotations)
	defer stop()

	client := newCRIClient(b, sock, useStreaming)
	defer client.Close()

	// warm up
	_, err := client.ListContainers(context.Background(), nil)
	if err != nil {
		b.Fatal(err)
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
	for _, containers := range containerCounts {
		for _, annotations := range annotationCounts {
			b.Run(fmt.Sprintf("containers=%d/annotations=%d", containers, annotations), func(b *testing.B) {
				var listTotal, streamTotal time.Duration

				for n := 0; n < b.N; n++ {
					listTotal += benchRequests(b, false, containers, annotations, tries)
					streamTotal += benchRequests(b, true, containers, annotations, tries)
				}

				totalRequests := float64(b.N) * float64(tries)
				listMsPerReq := float64(listTotal.Milliseconds()) / totalRequests
				streamMsPerReq := float64(streamTotal.Milliseconds()) / totalRequests

				b.ReportMetric(listMsPerReq, "list-ms/req")
				b.ReportMetric(streamMsPerReq, "stream-ms/req")
			})
		}
	}
}
