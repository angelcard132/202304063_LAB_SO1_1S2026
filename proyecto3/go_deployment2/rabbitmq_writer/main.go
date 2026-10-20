package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	amqpURL   string
	queueName string
	ch        *amqp.Channel
	conn      *amqp.Connection
	mu        sync.Mutex
)

func initRabbitMQ() error {
	mu.Lock()
	defer mu.Unlock()

	var err error
	conn, err = amqp.Dial(amqpURL)
	if err != nil {
		return err
	}

	ch, err = conn.Channel()
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		queueName, // nombre
		true,      // durable
		false,     // autoDelete
		false,     // exclusive
		false,     // noWait
		nil,       // args
	)
	return err
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	amqpURL = os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq-service:5672/"
	}

	queueName = os.Getenv("RABBITMQ_QUEUE")
	if queueName == "" {
		queueName = "war_reports_queue"
	}

	// Intento de conexión con reintentos
	go func() {
		for {
			if err := initRabbitMQ(); err != nil {
				log.Printf("[RabbitMQ Writer] Esperando broker RabbitMQ en %s: %v", amqpURL, err)
				time.Sleep(3 * time.Second)
			} else {
				log.Printf("[RabbitMQ Writer] Conectado exitosamente a RabbitMQ (cola: %s)", queueName)
				break
			}
		}
	}()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/publish", publishHandler)

	log.Printf("[RabbitMQ Writer] Escuchando en el puerto %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error en HTTP Writer: %v", err)
	}
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
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

	mu.Lock()
	channel := ch
	mu.Unlock()

	if channel == nil {
		http.Error(w, "RabbitMQ aún no conectado", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = channel.PublishWithContext(
		ctx,
		"",        // exchange default
		queueName, // routing key = cola
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)

	if err != nil {
		log.Printf("[RabbitMQ Writer] Error publicando mensaje: %v", err)
		http.Error(w, "Fallo publicando en RabbitMQ", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"published"}`))
}
