package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func main() {
	sock := flag.String("listen", "/tmp/cri-bench.sock", "unix socket path to listen on")
	flag.Parse()

	if err := os.Remove(*sock); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	lis, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	svc := &criService{numContainers: 1000}
	runtimeapi.RegisterRuntimeServiceServer(srv, svc)
	runtimeapi.RegisterImageServiceServer(srv, svc)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nshutting down...")
		srv.GracefulStop()
	}()

	fmt.Printf("listening on unix://%s\n", *sock)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
