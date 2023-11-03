package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nsqio/go-nsq"
)

const (
	HostPostgreSQL = "postgres:5432"
	HostNSQlookupd = "nsqlookupd:4160"
	HostNSQd       = "nsqd:4150"
	HostNSQadmin   = "nsqadmin:4171"
)

var (
	serviceMap = map[string]string{
		"PostgreSQL": HostPostgreSQL,
		"NSQlookupd": HostNSQlookupd,
		"NSQd":       HostNSQd,
		"NSQadmin":   HostNSQadmin,
	}
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/check", CheckServices).Methods("GET")

	http.Handle("/", r)

	port := "8080"
	fmt.Printf("Listening on port %s...\n", port)
	go nsqHealthCheck()
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func CheckServices(w http.ResponseWriter, r *http.Request) {
	healthCheckResult := make(map[string]string)
	for name, host := range serviceMap {
		serviceHealthCheck(&healthCheckResult, name, host)
	}
	publishHealthCheck(&healthCheckResult)

	for serviceName, status := range healthCheckResult {
		fmt.Fprintf(w, "%s: %s\n", serviceName, status)
	}
}

func serviceHealthCheck(result *map[string]string, serviceName, address string) {
	fmt.Printf("Checking %s...\n", serviceName)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("%s is unreachable: %v\n", serviceName, err)
		(*result)[serviceName] = "Failed"
		return
	}
	conn.Close()
	fmt.Printf("%s is reachable\n", serviceName)
	(*result)[serviceName] = "Success"
}

func publishHealthCheck(healthCheckResult *map[string]string) {
	// Create an NSQ producer to publish a test message
	producer, err := nsq.NewProducer(HostNSQd, nsq.NewConfig())
	if err != nil {
		log.Fatalf("Failed to create NSQ producer: %v", err)
	}
	defer producer.Stop()

	// Publish a test message
	(*healthCheckResult)["PublishNSQ"] = "Success"
	messageBody := []byte("Hello, NSQ!")
	if err := producer.Publish("test-topic", messageBody); err != nil {
		(*healthCheckResult)["PublishNSQ"] = "Failed"
	}
}

func nsqHealthCheck() {
	// Create an NSQ consumer to consume the test message
	consumer, err := nsq.NewConsumer("test-topic", "test-channel", nsq.NewConfig())
	if err != nil {
		log.Fatalf("Failed to create NSQ consumer: %v", err)
	}

	// Configure the message handler for the consumer
	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		fmt.Printf("Received message: %s\n", message.Body)
		message.Finish()
		return nil
	}))

	// Connect the consumer to the NSQD server
	if err := consumer.ConnectToNSQD(HostNSQd); err != nil {
		log.Fatalf("Failed to connect to NSQD: %v", err)
	}

	// Block indefinitely to keep the health check running
	select {}
}
