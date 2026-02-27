package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type criService struct {
	runtimeapi.UnimplementedRuntimeServiceServer
	runtimeapi.UnimplementedImageServiceServer
	numContainers  int
	numAnnotations int
	numPerChunk    int
}

func makeContainers(num, numAnnotations int) []*runtimeapi.Container {
	containers := make([]*runtimeapi.Container, num)
	for i := range containers {
		annotations := make(map[string]string, numAnnotations)
		for j := 0; j < numAnnotations; j++ {
			annotations[fmt.Sprintf("io.kubernetes.annotation-%d", j)] = fmt.Sprintf("value-%d-%d", i, j)
		}
		containers[i] = &runtimeapi.Container{
			Id:           fmt.Sprintf("container-%d", i),
			PodSandboxId: fmt.Sprintf("sandbox-%d", i),
			Metadata: &runtimeapi.ContainerMetadata{
				Name:    fmt.Sprintf("name-%d", i),
				Attempt: uint32(i),
			},
			Image: &runtimeapi.ImageSpec{
				Image: fmt.Sprintf("registry.example.com/image-%d:latest", i),
			},
			ImageRef:  fmt.Sprintf("sha256:abcdef%06d", i),
			State:     runtimeapi.ContainerState_CONTAINER_RUNNING,
			CreatedAt: int64(1700000000 + i),
			Labels: map[string]string{
				"app":       fmt.Sprintf("app-%d", i),
				"component": "server",
			},
			Annotations: annotations,
			ImageId:     fmt.Sprintf("sha256:fedcba%06d", i),
		}
	}
	return containers
}

func (s *criService) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	return &runtimeapi.VersionResponse{
		Version:           "0.1.0",
		RuntimeName:       "cri-bench",
		RuntimeVersion:    "0.1.0",
		RuntimeApiVersion: "v1",
	}, nil
}

func (s *criService) ListContainers(ctx context.Context, req *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	return &runtimeapi.ListContainersResponse{Containers: makeContainers(s.numContainers, s.numAnnotations)}, nil
}

func (s *criService) StreamContainers(req *runtimeapi.StreamContainersRequest, stream grpc.ServerStreamingServer[runtimeapi.StreamContainersResponse]) error {
	containers := makeContainers(s.numContainers, s.numAnnotations)
	for i := 0; i < len(containers); i += s.numPerChunk {
		end := i + s.numPerChunk
		if end > len(containers) {
			end = len(containers)
		}
		if err := stream.Send(&runtimeapi.StreamContainersResponse{
			Containers: containers[i:end],
		}); err != nil {
			return err
		}
	}
	return nil
}
