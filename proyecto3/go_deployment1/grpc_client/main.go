package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	pb "wartweets/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WarReport struct {
	Country          string `json:"country"`
	WarplanesInAir   int32  `json:"warplanes_in_air"`
	WarshipsInWater  int32  `json:"warships_in_water"`
	Timestamp        string `json:"timestamp"`
}

var (
	grpcServerAddress string
	grpcClient        pb.WarReportServiceClient
)

func countryToEnum(country string) pb.Countries {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "USA":
		return pb.Countries_usa
	case "RUS":
		return pb.Countries_rus
	case "CHN":
		return pb.Countries_chn
	case "ESP":
		return pb.Countries_esp
	case "GTM":
		return pb.Countries_gtm
	default:
		return pb.Countries_countries_unknown
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	grpcServerAddress = os.Getenv("GRPC_SERVER_ADDR")
	if grpcServerAddress == "" {
		grpcServerAddress = "go-deployment2-service:50051"
	}

	// Conexión gRPC persistente
	conn, err := grpc.NewClient(
		grpcServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("No se pudo conectar al servidor gRPC en %s: %v", grpcServerAddress, err)
	}
	defer conn.Close()

	grpcClient = pb.NewWarReportServiceClient(conn)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/forward", forwardHandler)

	log.Printf("[Go gRPC Client] Escuchando en el puerto %s (enviando a gRPC %s)", port, grpcServerAddress)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error en servidor gRPC Client: %v", err)
	}
}

func forwardHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	protoReq := &pb.WarReportRequest{
		Country:          countryToEnum(report.Country),
		WarplanesInAir:   report.WarplanesInAir,
		WarshipsInWater:  report.WarshipsInWater,
		Timestamp:        report.Timestamp,
	}

	resp, err := grpcClient.SendReport(ctx, protoReq)
	if err != nil {
		log.Printf("[Go gRPC Client] Error invocando SendReport: %v", err)
		http.Error(w, "Fallo enviando reporte por gRPC", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": resp.GetStatus(),
	})
}
