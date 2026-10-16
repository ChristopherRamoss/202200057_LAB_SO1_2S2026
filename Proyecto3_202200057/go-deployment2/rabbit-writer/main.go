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

const queueName = "war_reports"

var (
	channel *amqp.Channel
	mu      sync.Mutex // el canal de RabbitMQ no admite publicaciones simultáneas
)

// connect reintenta hasta que RabbitMQ esté disponible (en Kubernetes
// los pods arrancan en orden impredecible)
func connect(url string) (*amqp.Connection, *amqp.Channel) {
	for i := 1; i <= 30; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, err := conn.Channel()
			if err == nil {
				return conn, ch
			}
		}
		log.Printf("RabbitMQ no disponible (intento %d/30), reintentando...", i)
		time.Sleep(2 * time.Second)
	}
	log.Fatal("no se pudo conectar a RabbitMQ")
	return nil, nil
}

func main() {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	conn, ch := connect(url)
	defer conn.Close()
	channel = ch

	// durable: la cola sobrevive a un reinicio de RabbitMQ
	if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("no se pudo declarar la cola: %v", err)
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/publish", publish)

	log.Printf("rabbit-writer escuchando en el puerto %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func publish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error leyendo el cuerpo", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	mu.Lock()
	err = channel.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	mu.Unlock()

	if err != nil {
		http.Error(w, "error publicando en RabbitMQ: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Write([]byte(`{"status":"published"}`))
}
