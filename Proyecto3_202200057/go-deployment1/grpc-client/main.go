package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mumnk8s/proto"
)

// El JSON que llega de la API
type WarReport struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var grpcClient pb.WarReportServiceClient

func main() {
	addr := os.Getenv("GRPC_SERVER_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	// Una sola conexión gRPC, reutilizada por todas las peticiones
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("no se pudo crear el cliente gRPC: %v", err)
	}
	defer conn.Close()
	grpcClient = pb.NewWarReportServiceClient(conn)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/send", send)

	log.Printf("grpc-client escuchando en el puerto %s, servidor gRPC en %s", port, addr)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var rep WarReport
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Convertimos el texto del país ("ESP") al enum del .proto
	code, ok := pb.Countries_value[strings.ToLower(rep.Country)]
	if !ok || code == 0 {
		http.Error(w, "país no válido", http.StatusBadRequest)
		return
	}

	// Timeout para no quedarnos colgados si el servidor no responde
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := grpcClient.SendReport(ctx, &pb.WarReportRequest{
		Country:         pb.Countries(code),
		WarplanesInAir:  rep.WarplanesInAir,
		WarshipsInWater: rep.WarshipsInWater,
		Timestamp:       rep.Timestamp,
	})
	if err != nil {
		http.Error(w, "error llamando al servidor gRPC: "+err.Error(), http.StatusBadGateway)
		return
	}

	log.Printf("[grpc-client] %s aviones=%d barcos=%d -> SendReport: %s",
		rep.Country, rep.WarplanesInAir, rep.WarshipsInWater, resp.GetStatus())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": resp.GetStatus()})
}
