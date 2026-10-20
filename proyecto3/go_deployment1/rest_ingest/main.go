package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// WarReport estructura del JSON recibido desde Rust
type WarReport struct {
	Country          string `json:"country"`
	WarplanesInAir   int32  `json:"warplanes_in_air"`
	WarshipsInWater  int32  `json:"warships_in_water"`
	Timestamp        string `json:"timestamp"`
}

var grpcClientForwardURL string

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// Al compartir el pod, se comunica vía localhost con el container gRPC Client
	grpcClientForwardURL = os.Getenv("GRPC_CLIENT_URL")
	if grpcClientForwardURL == "" {
		grpcClientForwardURL = "http://localhost:8082/forward"
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/report", reportHandler)

	log.Printf("[Go Ingest REST] Escuchando en el puerto %s (reenviando a %s)", port, grpcClientForwardURL)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error en el servidor HTTP: %v", err)
	}
}

func reportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error leyendo cuerpo", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var report WarReport
	if err := json.Unmarshal(body, &report); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Reenviar al contenedor gRPC Client en localhost
	resp, err := http.Post(grpcClientForwardURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Go Ingest REST] Error contactando gRPC Client container: %v", err)
		http.Error(w, fmt.Sprintf("Error enviando al cliente gRPC: %v", err), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
