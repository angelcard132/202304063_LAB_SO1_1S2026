package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type WarReport struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var (
	ctx        = context.Background()
	rdb        *redis.Client
	userCarnet = "202304063"
	assignedP  = "RUS" // mi carnet termina en 3 -> RUS
)

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq-service:5672/"
	}

	queueName := os.Getenv("RABBITMQ_QUEUE")
	if queueName == "" {
		queueName = "war_reports_queue"
	}

	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "valkey-service:6379"
	}

	valkeyPassword := os.Getenv("VALKEY_PASSWORD")

	rdb = redis.NewClient(&redis.Options{
		Addr:     valkeyAddr,
		Password: valkeyPassword,
		DB:       0,
	})

	// Inicializar metadata fija del país asignado
	rdb.Set(ctx, "assigned_country", assignedP, 0)
	rdb.Set(ctx, "assigned_carnet", userCarnet, 0)

	log.Printf("[Go Consumer] Conectando a RabbitMQ: %s | Valkey: %s", rabbitURL, valkeyAddr)

	for {
		err := startConsumer(rabbitURL, queueName)
		if err != nil {
			log.Printf("[Go Consumer] Error en conexión RabbitMQ: %v. Reintentando en 5 segundos...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func startConsumer(rabbitURL, queueName string) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true, // auto-ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	log.Println("[Go Consumer] Listo y esperando mensajes en RabbitMQ...")

	for d := range msgs {
		var report WarReport
		if err := json.Unmarshal(d.Body, &report); err != nil {
			log.Printf("[Go Consumer] Error decodificando reporte JSON: %v", err)
			continue
		}

		processReport(report)
	}

	return fmt.Errorf("canal de mensajes cerrado")
}

func processReport(r WarReport) {
	cUpper := strings.ToUpper(strings.TrimSpace(r.Country))
	planes := float64(r.WarplanesInAir)
	ships := float64(r.WarshipsInWater)

	// 1. Contador total de reportes recibidos global
	rdb.Incr(ctx, "total_reports")

	// 2. Extremos Generales: Max / Min
	updateMinMax("max_warplanes", "min_warplanes", planes)
	updateMinMax("max_warships", "min_warships", ships)

	// 3. Frecuencia para calcular MODA
	rdb.HIncrBy(ctx, "freq_warplanes", strconv.Itoa(int(planes)), 1)
	rdb.HIncrBy(ctx, "freq_warships", strconv.Itoa(int(ships)), 1)

	// 4. Ranking de Países (Sorted Sets para Top Países con más aviones y barcos)
	rdb.ZIncrBy(ctx, "top_countries_warplanes", planes, cUpper)
	rdb.ZIncrBy(ctx, "top_countries_warships", ships, cUpper)

	// 5. Estadísticas del País Asignado (RUS)
	if cUpper == assignedP {
		rdb.Incr(ctx, "assigned_total_reports")

		// Serie temporal (guardamos los últimos registros con timestamp)
		tsData, _ := json.Marshal(map[string]interface{}{
			"timestamp":         r.Timestamp,
			"warplanes_in_air":  r.WarplanesInAir,
			"warships_in_water": r.WarshipsInWater,
		})
		rdb.RPush(ctx, "timeseries_assigned", string(tsData))
		// Limitar a los últimos 500 registros para optimización de memoria
		rdb.LTrim(ctx, "timeseries_assigned", -500, -1)
	}
}

func updateMinMax(maxKey, minKey string, val float64) {
	// Máximo
	currentMax, err := rdb.Get(ctx, maxKey).Float64()
	if err == redis.Nil || val > currentMax {
		rdb.Set(ctx, maxKey, val, 0)
	}

	// Mínimo
	currentMin, err := rdb.Get(ctx, minKey).Float64()
	if err == redis.Nil || val < currentMin {
		rdb.Set(ctx, minKey, val, 0)
	}
}
