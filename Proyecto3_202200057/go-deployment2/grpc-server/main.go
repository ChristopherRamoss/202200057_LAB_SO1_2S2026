package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "mumnk8s/proto"
)

// server implementa el servicio definido en el .proto
type server struct {
	pb.UnimplementedWarReportServiceServer
}

// SendReport es la función remota que otros servicios invocarán
func (s *server) SendReport(ctx context.Context, req *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	log.Printf("reporte recibido: país=%s aviones=%d barcos=%d timestamp=%s",
		req.GetCountry(), req.GetWarplanesInAir(), req.GetWarshipsInWater(), req.GetTimestamp())

	// TODO: aquí se enviará el reporte al writer de RabbitMQ
	return &pb.WarReportResponse{Status: "ok"}, nil
}

func main() {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("no se pudo escuchar en el puerto %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWarReportServiceServer(grpcServer, &server{})

	log.Printf("servidor gRPC escuchando en el puerto %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
