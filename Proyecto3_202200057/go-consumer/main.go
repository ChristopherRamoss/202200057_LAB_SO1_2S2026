package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

const queueName = "war_reports"

type report struct {
	Country         string `json:"country"`
	WarplanesInAir  int    `json:"warplanes_in_air"`
	WarshipsInWater int    `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

// Un solo script Lua hace TODAS las actualizaciones de un reporte.
// Valkey ejecuta cada script de forma atómica: aunque haya varios workers
// escribiendo a la vez, nunca se mezclan a medias.
var storeScript = redis.NewScript(`
local c = ARGV[1]
local p = tonumber(ARGV[2])
local s = tonumber(ARGV[3])

-- máximos y mínimos globales
local pmax = redis.call('GET', 'planes:max')
if (not pmax) or p > tonumber(pmax) then redis.call('SET', 'planes:max', p) end
local pmin = redis.call('GET', 'planes:min')
if (not pmin) or p < tonumber(pmin) then redis.call('SET', 'planes:min', p) end
local smax = redis.call('GET', 'ships:max')
if (not smax) or s > tonumber(smax) then redis.call('SET', 'ships:max', s) end
local smin = redis.call('GET', 'ships:min')
if (not smin) or s < tonumber(smin) then redis.call('SET', 'ships:min', s) end

-- acumulado por país (para los rankings "top")
redis.call('ZINCRBY', 'top:planes', p, c)
redis.call('ZINCRBY', 'top:ships', s, c)

-- cuántas veces apareció cada valor (para calcular la moda)
redis.call('HINCRBY', 'freq:planes', p, 1)
redis.call('HINCRBY', 'freq:ships', s, 1)

-- total de reportes por país
redis.call('HINCRBY', 'reports:count', c, 1)

-- serie temporal por país, limitada a ~10 000 entradas para cuidar la RAM
redis.call('XADD', 'ts:' .. c, 'MAXLEN', '~', 10000, '*', 'planes', p, 'ships', s)
return 1
`)

func connectRabbit(url string) (*amqp.Connection, *amqp.Channel) {
	for i := 1; i <= 30; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, err := conn.Channel()
			if err == nil {
				return conn, ch
			}
		}
		log.Printf("RabbitMQ no disponible (intento %d/30)...", i)
		time.Sleep(2 * time.Second)
	}
	log.Fatal("no se pudo conectar a RabbitMQ")
	return nil, nil
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}
	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "localhost:6379"
	}
	workers := 4
	if w, err := strconv.Atoi(os.Getenv("WORKERS")); err == nil && w > 0 {
		workers = w
	}

	// Conexión a Valkey (go-redis habla el mismo protocolo)
	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	for i := 1; i <= 30; i++ {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			break
		} else if i == 30 {
			log.Fatalf("no se pudo conectar a Valkey: %v", err)
		}
		log.Printf("Valkey no disponible (intento %d/30)...", i)
		time.Sleep(2 * time.Second)
	}

	conn, ch := connectRabbit(rabbitURL)
	defer conn.Close()

	// Debe coincidir con la declaración del writer (durable = true)
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("no se pudo declarar la cola: %v", err)
	}

	// Prefetch: RabbitMQ no entrega más de N mensajes sin confirmar a la vez
	if err := ch.Qos(workers*10, 0, false); err != nil {
		log.Fatalf("error en Qos: %v", err)
	}

	// autoAck = false: confirmamos (Ack) solo cuando el dato ya está en Valkey
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("no se pudo consumir: %v", err)
	}

	log.Printf("consumer listo: %d workers, Valkey en %s", workers, valkeyAddr)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range msgs {
				process(rdb, d)
			}
		}()
	}
	wg.Wait()
}

func process(rdb *redis.Client, d amqp.Delivery) {
	var r report
	if err := json.Unmarshal(d.Body, &r); err != nil {
		log.Printf("mensaje inválido, se descarta: %v", err)
		d.Nack(false, false) // no reencolar: nunca va a poder procesarse
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := storeScript.Run(ctx, rdb, nil, r.Country, r.WarplanesInAir, r.WarshipsInWater).Err()
	if err != nil {
		log.Printf("error guardando en Valkey: %v", err)
		time.Sleep(time.Second)
		d.Nack(false, true) // reencolar: el problema puede ser temporal
		return
	}
	d.Ack(false)
}
