package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type WarReport struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var (
	clientURL  string
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func main() {
	clientURL = os.Getenv("GRPC_CLIENT_URL")
	if clientURL == "" {
		clientURL = "http://localhost:9090/send"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/report", report)

	log.Printf("api escuchando en el puerto %s, reenvía a %s", port, clientURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var rep WarReport
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	if len(rep.Country) != 3 {
		http.Error(w, "country debe tener 3 letras", http.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(rep)
	resp, err := httpClient.Post(clientURL, "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "no se pudo contactar al grpc-client: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Devolvemos tal cual lo que respondió el grpc-client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
