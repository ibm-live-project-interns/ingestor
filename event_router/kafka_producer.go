package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

var kafkaProducer *kafka.Producer

func initKafka() {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:9092"
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"acks":              "all",
	})
	if err != nil {
		log.Fatalf("❌ Kafka producer init failed: %v", err)
	}

	kafkaProducer = p
	log.Println("✅ Kafka producer ready")
}

func publishToKafka(event models.RoutedEvent) error {
	topic := "ingestion-events"
	value, _ := json.Marshal(event)

	return kafkaProducer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic: &topic,
			Partition: kafka.PartitionAny,
		},
		Value: value,
	}, nil)
}
