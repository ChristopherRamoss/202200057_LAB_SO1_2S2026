package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mumnk8s/proto"
)

type report struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

type server struct {
	pb.UnimplementedWarReportServiceServer
	writerURL  string
	httpClient *http.Client
}

func (s *server) SendReport(ctx context.Context, req *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	rep := report{
		Country:         strings.ToUpper(req.GetCountry().String()),
		WarplanesInAir:  req.GetWarplanesInAir(),
		WarshipsInWater: req.GetWarshipsInWater(),
		Timestamp:       req.GetTimestamp(),
	}
	body, _ := json.Marshal(rep)

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.writerURL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "writer no disponible: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, status.Errorf(codes.Internal, "writer respondió %d", resp.StatusCode)
	}

	return &pb.WarReportResponse{Status: "ok"}, nil
}

func main() {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	writerURL := os.Getenv("WRITER_URL")
	if writerURL == "" {
		writerURL = "http://localhost:8081/publish"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en el puerto %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWarReportServiceServer(grpcServer, &server{
		writerURL:  writerURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	})

	log.Printf("servidor gRPC escuchando en el puerto %s, writer en %s", port, writerURL)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
