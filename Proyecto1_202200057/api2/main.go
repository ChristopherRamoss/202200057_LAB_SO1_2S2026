package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type CallResponse struct {
	APIName    string `json:"apiName"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

const (
	CARNET   = "202200057"
	VM_NAME  = "VM1"
	API_NAME = "API2"
	PORT     = ":8082"

	API1_URL = "http://192.168.122.87:8081/health"
	API3_URL = "http://192.168.122.56:8083/health"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := HealthResponse{
		Status:    "UP",
		Message:   fmt.Sprintf("%s is Ready", API_NAME),
		Timestamp: time.Now().Format(time.RFC3339),
		VM:        VM_NAME,
		Carnet:    CARNET,
	}
	json.NewEncoder(w).Encode(resp)
}

func callTargetAPI(targetURL string) bool {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func callAPI1Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ok := callTargetAPI(API1_URL)
	resp := CallResponse{
		APIName:    API_NAME,
		Message:    fmt.Sprintf("The %s located on the %s is working", API_NAME, VM_NAME),
		Connection: ok,
		Carnet:     CARNET,
	}
	json.NewEncoder(w).Encode(resp)
}

func callAPI3Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ok := callTargetAPI(API3_URL)
	resp := CallResponse{
		APIName:    API_NAME,
		Message:    fmt.Sprintf("The %s located on the %s is working", API_NAME, VM_NAME),
		Connection: ok,
		Carnet:     CARNET,
	}
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api2-202200057/call-api1", callAPI1Handler)
	http.HandleFunc("/api2-202200057/call-api3", callAPI3Handler)

	fmt.Printf("Servidor %s escuchando en el puerto %s...\n", API_NAME, PORT)
	if err := http.ListenAndServe(PORT, nil); err != nil {
		fmt.Printf("Error al iniciar servidor: %v\n", err)
	}
}