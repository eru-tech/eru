package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/eru-tech/eru/eru-events/events"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// Example demonstrates how to use Kafka events in eru-functions
// NO environment variables needed - everything comes from JSON configuration
func main() {
	logs.LogInit("eru-events-example", "eru-events-example-123")

	// Example 1: Basic Kafka Event Configuration
	fmt.Println("=== Example 1: Basic Kafka Configuration ===")
	basicKafkaExample()

	// Example 2: Secure Kafka Event Configuration
	fmt.Println("\n=== Example 2: Secure Kafka Configuration ===")
	secureKafkaExample()

	// Example 3: Production-ready Configuration
	fmt.Println("\n=== Example 3: Production Configuration ===")
	productionKafkaExample()
}

func basicKafkaExample() {
	// This is how eru-functions would configure a Kafka event
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "user-actions",
		"polling_interval": 5,
		"brokers": "localhost:9092",
		"topic": "user-actions",
		"group_id": "eru-functions-group",
		"partitions": 3,
		"replication_factor": 1,
		"auto_offset_reset": "earliest",
		"enable_auto_commit": true
	}`

	demonstrateUsage("Basic Kafka", configJSON)
}

func secureKafkaExample() {
	// Example with SASL/SSL security
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "secure-events",
		"polling_interval": 10,
		"brokers": "kafka-cluster.company.com:9093",
		"topic": "secure-events",
		"group_id": "eru-secure-group",
		"security_protocol": "SASL_SSL",
		"sasl_mechanism": "PLAIN",
		"sasl_username": "eru-service",
		"sasl_password": "secure-password",
		"ssl_ca_location": "/etc/ssl/certs/ca-cert.pem"
	}`

	demonstrateUsage("Secure Kafka", configJSON)
}

func productionKafkaExample() {
	// Production-ready configuration with multiple brokers
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "workflow-events",
		"polling_interval": 3,
		"brokers": "kafka1.prod.com:9092,kafka2.prod.com:9092,kafka3.prod.com:9092",
		"topic": "workflow-events",
		"group_id": "eru-functions-prod",
		"partitions": 12,
		"replication_factor": 3,
		"session_timeout_ms": 30000,
		"auto_offset_reset": "latest",
		"enable_auto_commit": false,
		"topic_config": {
			"retention.ms": "604800000",
			"compression.type": "snappy"
		}
	}`

	demonstrateUsage("Production Kafka", configJSON)
}

func demonstrateUsage(name string, configJSON string) {
	ctx := context.Background()

	// Convert JSON string to RawMessage (this is what eru-functions would do)
	var rawMsg json.RawMessage = []byte(configJSON)

	// Create event instance
	kafkaEvent := events.GetEvent("KAFKA")
	if kafkaEvent == nil {
		log.Printf("❌ Failed to create %s event", name)
		return
	}

	// Configure from JSON (no environment variables!)
	err := kafkaEvent.MakeFromJson(ctx, &rawMsg)
	if err != nil {
		log.Printf("❌ Failed to configure %s event: %v", name, err)
		return
	}

	// Display configuration
	fmt.Printf("✅ %s event configured successfully\n", name)

	// Show key attributes
	brokers, _ := kafkaEvent.GetAttribute("brokers")
	topic, _ := kafkaEvent.GetAttribute("topic")
	groupId, _ := kafkaEvent.GetAttribute("group_id")

	fmt.Printf("   Brokers: %v\n", brokers)
	fmt.Printf("   Topic: %v\n", topic)
	fmt.Printf("   Group ID: %v\n", groupId)

	// NOTE: We don't call Init() here since we might not have the actual brokers running
	// In eru-functions, you would call:
	// err = kafkaEvent.Init(ctx)
	// err = kafkaEvent.CreateEvent(ctx)  // if needed
	// msgId, err = kafkaEvent.Publish(ctx, message, kafkaEvent)
	// eventMsgs, err = kafkaEvent.Poll(ctx)
}
