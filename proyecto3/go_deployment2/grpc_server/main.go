package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	pb "wartweets/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedWarReportServiceServer
	writerURL string
}

type WarReportJSON struct {
	Country          string `json:"country"`
	WarplanesInAir   int32  `json:"warplanes_in_air"`
	WarshipsInWater  int32  `json:"warships_in_water"`
	Timestamp        string `json:"timestamp"`
}

func countryEnumToString(c pb.Countries) string {
	switch c {
	case pb.Countries_usa:
		return "USA"
	case pb.Countries_rus:
		return "RUS"
	case pb.Countries_chn:
		return "CHN"
	case pb.Countries_esp:
		return "ESP"
	case pb.Countries_gtm:
		return "GTM"
	default:
		return "UNKNOWN"
	}
}

func (s *server) SendReport(ctx context.Context, req *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	report := WarReportJSON{
		Country:         countryEnumToString(req.GetCountry()),
		WarplanesInAir:  req.GetWarplanesInAir(),
		WarshipsInWater: req.GetWarshipsInWater(),
		Timestamp:       req.GetTimestamp(),
	}

	data, err := json.Marshal(report)
	if err != nil {
		log.Printf("[gRPC Server] Error serializando JSON: %v", err)
		return nil, fmt.Errorf("error serializando reporte")
	}

	// Reenviar al contenedor Writer en localhost dentro del mismo pod
	resp, err := http.Post(s.writerURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[gRPC Server] Error enviando al RabbitMQ Writer: %v", err)
		return nil, fmt.Errorf("error publicando en writer")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[gRPC Server] RabbitMQ Writer devolvió código: %d", resp.StatusCode)
		return nil, fmt.Errorf("falla interna en publicación de mensaje")
	}

	return &pb.WarReportResponse{
		Status: "Reporte procesado exitosamente por gRPC Server",
	}, nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	writerURL := os.Getenv("RABBITMQ_WRITER_URL")
	if writerURL == "" {
		writerURL = "http://localhost:8083/publish"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Error escuchando puerto %s: %v", port, err)
	}

	s := grpc.NewServer()
	pb.RegisterWarReportServiceServer(s, &server{writerURL: writerURL})
	reflection.Register(s)

	log.Printf("[Go gRPC Server] Escuchando en el puerto :%s (reenviando a writer %s)", port, writerURL)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error en gRPC Server: %v", err)
	}
}
