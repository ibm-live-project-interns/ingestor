package storage

import (
	"encoding/json"
	"log"
	"os"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

func Publish(event models.RoutedEvent) error {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:9092"
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
	})
	if err != nil {
		return err
	}
	defer p.Close()

	payload, _ := json.Marshal(event)
	topic := "ingestion-events"

	p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value: payload,
	}, nil)

	log.Println("📤 Event published to Kafka")
	return nil
}
