package events

import (
	"context"
	"encoding/json"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestKafkaEventCreation(t *testing.T) {
	event := GetEvent("KAFKA")
	if event == nil {
		t.Fatal("Failed to create Kafka event")
	}

	kafkaEvent, ok := event.(*Kafka_Event)
	if !ok {
		t.Fatal("Event is not a Kafka_Event")
	}

	if kafkaEvent == nil {
		t.Fatal("Kafka event is nil")
	}
}

func TestKafkaEventMakeFromJson(t *testing.T) {
	logs.LogInit("eru-events-test", "eru-events-test-123")

	jsonData := `{
		"event_type": "KAFKA",
		"event_name": "test-topic",
		"polling_interval": 5,
		"brokers": "localhost:9092",
		"topic": "test-topic",
		"group_id": "test-group",
		"partitions": 1,
		"replication_factor": 1,
		"auto_offset_reset": "earliest",
		"enable_auto_commit": true
	}`

	var rawMsg json.RawMessage = []byte(jsonData)
	event := GetEvent("KAFKA")

	err := event.MakeFromJson(context.Background(), &rawMsg)
	if err != nil {
		t.Fatalf("Failed to make from JSON: %v", err)
	}

	kafkaEvent := event.(*Kafka_Event)
	if kafkaEvent.EventType != "KAFKA" {
		t.Errorf("Expected event type KAFKA, got %s", kafkaEvent.EventType)
	}
	if kafkaEvent.EventName != "test-topic" {
		t.Errorf("Expected event name test-topic, got %s", kafkaEvent.EventName)
	}
	if kafkaEvent.Brokers != "localhost:9092" {
		t.Errorf("Expected brokers localhost:9092, got %s", kafkaEvent.Brokers)
	}
	if kafkaEvent.Topic != "test-topic" {
		t.Errorf("Expected topic test-topic, got %s", kafkaEvent.Topic)
	}
	if kafkaEvent.GroupId != "test-group" {
		t.Errorf("Expected group ID test-group, got %s", kafkaEvent.GroupId)
	}
}

func TestKafkaEventGetAttribute(t *testing.T) {
	kafkaEvent := &Kafka_Event{
		Event: Event{
			EventType:       "KAFKA",
			EventName:       "test-topic",
			PollingInterval: 5,
		},
		Brokers: "localhost:9092",
		Topic:   "test-topic",
		GroupId: "test-group",
	}

	tests := []struct {
		attribute string
		expected  interface{}
	}{
		{"event_type", "KAFKA"},
		{"event_name", "test-topic"},
		{"polling_interval", int32(5)},
		{"brokers", "localhost:9092"},
		{"topic", "test-topic"},
		{"group_id", "test-group"},
	}

	for _, test := range tests {
		value, err := kafkaEvent.GetAttribute(test.attribute)
		if err != nil {
			t.Errorf("Failed to get attribute %s: %v", test.attribute, err)
		}
		if value != test.expected {
			t.Errorf("Expected %s to be %v, got %v", test.attribute, test.expected, value)
		}
	}
}

func TestKafkaEventClone(t *testing.T) {
	logs.LogInit("eru-events-test", "eru-events-test-123")

	kafkaEvent := &Kafka_Event{
		Event: Event{
			EventType:       "KAFKA",
			EventName:       "test-topic",
			PollingInterval: 5,
		},
		Brokers: "localhost:9092",
		Topic:   "test-topic",
		GroupId: "test-group",
	}

	cloned, err := kafkaEvent.Clone(context.Background())
	if err != nil {
		t.Fatalf("Failed to clone Kafka event: %v", err)
	}

	clonedKafka, ok := cloned.(*Kafka_Event)
	if !ok {
		t.Fatal("Cloned event is not a Kafka_Event")
	}

	if clonedKafka.EventName != kafkaEvent.EventName {
		t.Errorf("Cloned event name mismatch: expected %s, got %s", kafkaEvent.EventName, clonedKafka.EventName)
	}
	if clonedKafka.Brokers != kafkaEvent.Brokers {
		t.Errorf("Cloned brokers mismatch: expected %s, got %s", kafkaEvent.Brokers, clonedKafka.Brokers)
	}
}

func TestKafkaDefaultValues(t *testing.T) {
	kafkaEvent := &Kafka_Event{}

	if kafkaEvent.getPartitions() != 1 {
		t.Errorf("Expected default partitions to be 1, got %d", kafkaEvent.getPartitions())
	}

	if kafkaEvent.getReplicationFactor() != 1 {
		t.Errorf("Expected default replication factor to be 1, got %d", kafkaEvent.getReplicationFactor())
	}

	if kafkaEvent.getAutoOffsetReset() != "earliest" {
		t.Errorf("Expected default auto offset reset to be earliest, got %s", kafkaEvent.getAutoOffsetReset())
	}
}
