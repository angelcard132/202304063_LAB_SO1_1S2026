package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	Carnet  = "202304063"
	VMName  = "VM1"
	APIName = "API2"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type CallResponse struct {
	APIName    string `json:"apiname"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := HealthResponse{
		Status:    "UP",
		Message:   fmt.Sprintf("%s is Ready", APIName),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        VMName,
		Carnet:    Carnet,
	}
	json.NewEncoder(w).Encode(resp)
}

func callOtherAPI(targetAPIName string, targetVM string, healthURL string) CallResponse {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return CallResponse{
			APIName:    targetAPIName,
			Message:    fmt.Sprintf("ERROR: The %s located on the %s is not working", targetAPIName, targetVM),
			Connection: false,
			Carnet:     Carnet,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CallResponse{
			APIName:    targetAPIName,
			Message:    fmt.Sprintf("ERROR: The %s located on the %s is not working", targetAPIName, targetVM),
			Connection: false,
			Carnet:     Carnet,
		}
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil || health.Status != "UP" {
		return CallResponse{
			APIName:    targetAPIName,
			Message:    fmt.Sprintf("ERROR: The %s located on the %s is not working", targetAPIName, targetVM),
			Connection: false,
			Carnet:     Carnet,
		}
	}

	return CallResponse{
		APIName:    targetAPIName,
		Message:    fmt.Sprintf("The %s located on the %s is working", targetAPIName, targetVM),
		Connection: true,
		Carnet:     Carnet,
	}
}

func callAPI1Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	api1URL := os.Getenv("API1_URL")
	if api1URL == "" {
		api1URL = "http://localhost:8081"
	}
	res := callOtherAPI("API1", "VM1", api1URL+"/health")
	json.NewEncoder(w).Encode(res)
}

func callAPI3Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	api3URL := os.Getenv("API3_URL")
	if api3URL == "" {
		api3URL = "http://localhost:8083"
	}
	res := callOtherAPI("API3", "VM3", api3URL+"/health")
	json.NewEncoder(w).Encode(res)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc(fmt.Sprintf("/api2/%s/call-api1", Carnet), callAPI1Handler)
	http.HandleFunc(fmt.Sprintf("/api2/%s/call-api3", Carnet), callAPI3Handler)

	fmt.Printf("%s listening on port %s...\n", APIName, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
