package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/eru-tech/eru/eru-events/events"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// Enterprise Kafka configuration examples for production environments
func main() {
	logs.LogInit("eru-events-enterprise-examples", "eru-events-enterprise-examples-123")

	fmt.Println("🏢 Enterprise Kafka Configuration Examples")
	fmt.Println("==========================================")

	// Example 1: AWS MSK with IAM Authentication
	fmt.Println("\n1. AWS MSK with IAM Authentication")
	awsMskExample()

	// Example 2: On-premises Kafka with SCRAM-SHA-256
	fmt.Println("\n2. Enterprise Kafka with SCRAM-SHA-256")
	scramExample()

	// Example 3: Kerberos Authentication
	fmt.Println("\n3. Kerberos Authentication")
	kerberosExample()

	// Example 4: SSL Mutual Authentication
	fmt.Println("\n4. SSL Mutual Authentication")
	sslMutualAuthExample()

	// Example 5: High-Performance Production Setup
	fmt.Println("\n5. High-Performance Production Setup")
	highPerformanceExample()

	// Example 6: Transactional Processing
	fmt.Println("\n6. Transactional Processing")
	transactionalExample()
}

func awsMskExample() {
	// AWS MSK (Managed Streaming for Kafka) with IAM authentication
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "msk-workflow-events",
		"polling_interval": 5,
		"brokers": "b-1.mskcluster.abc123.kafka.us-east-1.amazonaws.com:9098,b-2.mskcluster.abc123.kafka.us-east-1.amazonaws.com:9098",
		"topic": "workflow-events",
		"group_id": "eru-functions-prod",
		"partitions": 12,
		"replication_factor": 3,
		
		"security_protocol": "SASL_SSL",
		"sasl_mechanism": "AWS_MSK_IAM",
		"aws_region": "us-east-1",
		"aws_access_key_id": "AKIA...",
		"aws_secret_access_key": "...",
		
		"topic_config": {
			"retention.ms": "604800000",
			"cleanup.policy": "delete",
			"compression.type": "snappy",
			"min.insync.replicas": "2"
		},
		
		"acks": "all",
		"retries": 5,
		"compression_type": "snappy",
		"batch_size": 32768,
		"enable_auto_commit": false
	}`

	demonstrateConfig("AWS MSK IAM", configJSON)
}

func scramExample() {
	// Enterprise Kafka with SCRAM-SHA-256 authentication
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "enterprise-events",
		"polling_interval": 3,
		"brokers": "kafka1.company.com:9093,kafka2.company.com:9093,kafka3.company.com:9093",
		"topic": "enterprise-events",
		"group_id": "eru-enterprise-group",
		"partitions": 24,
		"replication_factor": 3,
		
		"security_protocol": "SASL_SSL",
		"sasl_mechanism": "SCRAM-SHA-256",
		"sasl_username": "eru-service-account",
		"sasl_password": "secure-password-from-vault",
		
		"ssl_ca_location": "/etc/ssl/certs/company-ca.pem",
		
		"topic_config": {
			"retention.ms": "2592000000",
			"cleanup.policy": "delete",
			"compression.type": "lz4",
			"segment.ms": "86400000"
		},
		
		"acks": "all",
		"retries": 3,
		"retry_backoff_ms": 1000,
		"compression_type": "lz4",
		"batch_size": 65536,
		"linger_ms": 10,
		"enable_auto_commit": false,
		"isolation_level": "read_committed"
	}`

	demonstrateConfig("Enterprise SCRAM", configJSON)
}

func kerberosExample() {
	// Kerberos authentication for enterprise environments
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "secure-events",
		"polling_interval": 5,
		"brokers": "kafka1.secure.corp:9093,kafka2.secure.corp:9093",
		"topic": "secure-events",
		"group_id": "eru-kerberos-group",
		"partitions": 6,
		"replication_factor": 2,
		
		"security_protocol": "SASL_SSL",
		"sasl_mechanism": "GSSAPI",
		"sasl_kerberos_service_name": "kafka",
		"sasl_kerberos_principal": "eru-client@COMPANY.COM",
		"sasl_kerberos_kinit_cmd": "/usr/bin/kinit",
		
		"ssl_ca_location": "/etc/ssl/certs/corporate-ca.pem",
		
		"acks": "all",
		"enable_auto_commit": false,
		"session_timeout_ms": 45000,
		"heartbeat_interval_ms": 15000
	}`

	demonstrateConfig("Kerberos", configJSON)
}

func sslMutualAuthExample() {
	// SSL mutual authentication without SASL
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "ssl-events",
		"polling_interval": 5,
		"brokers": "kafka-ssl.company.com:9094",
		"topic": "ssl-events",
		"group_id": "eru-ssl-group",
		"partitions": 3,
		"replication_factor": 1,
		
		"security_protocol": "SSL",
		"ssl_ca_location": "/etc/ssl/certs/ca-cert.pem",
		"ssl_certificate_location": "/etc/ssl/certs/client-cert.pem",
		"ssl_key_location": "/etc/ssl/private/client-key.pem",
		"ssl_key_password": "key-password",
		
		"acks": "1",
		"compression_type": "snappy"
	}`

	demonstrateConfig("SSL Mutual Auth", configJSON)
}

func highPerformanceExample() {
	// High-performance configuration for heavy workloads
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "high-perf-events",
		"polling_interval": 1,
		"brokers": "kafka1.perf.com:9092,kafka2.perf.com:9092,kafka3.perf.com:9092",
		"topic": "high-perf-events",
		"group_id": "eru-high-perf",
		"partitions": 48,
		"replication_factor": 3,
		
		"topic_config": {
			"retention.ms": "86400000",
			"segment.ms": "3600000",
			"compression.type": "lz4",
			"min.insync.replicas": "2"
		},
		
		"acks": "1",
		"retries": 0,
		"batch_size": 131072,
		"linger_ms": 5,
		"compression_type": "lz4",
		"max_in_flight_requests": 10,
		"buffer_memory": 134217728,
		
		"fetch_min_bytes": 1024,
		"fetch_max_wait_ms": 50,
		"max_partition_fetch_bytes": 2097152,
		"max_poll_records": 1000,
		"enable_auto_commit": true,
		"session_timeout_ms": 10000,
		"heartbeat_interval_ms": 3000
	}`

	demonstrateConfig("High Performance", configJSON)
}

func transactionalExample() {
	// Transactional processing configuration
	configJSON := `{
		"event_type": "KAFKA",
		"event_name": "transactional-events",
		"polling_interval": 5,
		"brokers": "kafka1.txn.com:9092,kafka2.txn.com:9092",
		"topic": "transactional-events",
		"group_id": "eru-transactional-group",
		"partitions": 12,
		"replication_factor": 3,
		
		"topic_config": {
			"cleanup.policy": "delete",
			"min.insync.replicas": "2"
		},
		
		"acks": "all",
		"retries": 5,
		"max_in_flight_requests": 1,
		"enable_auto_commit": false,
		"isolation_level": "read_committed",
		
		"delivery_timeout_ms": 300000,
		"request_timeout_ms": 30000
	}`

	demonstrateConfig("Transactional", configJSON)
}

func demonstrateConfig(name string, configJSON string) {
	ctx := context.Background()

	// Parse configuration
	var rawMsg json.RawMessage = []byte(configJSON)

	// Create and configure event
	kafkaEvent := events.GetEvent("KAFKA")
	if kafkaEvent == nil {
		log.Printf("❌ Failed to create %s event", name)
		return
	}

	err := kafkaEvent.MakeFromJson(ctx, &rawMsg)
	if err != nil {
		log.Printf("❌ Failed to configure %s event: %v", name, err)
		return
	}

	fmt.Printf("✅ %s configuration loaded successfully\n", name)

	// Display key configuration details
	brokers, _ := kafkaEvent.GetAttribute("brokers")
	securityProtocol, _ := kafkaEvent.GetAttribute("security_protocol")
	saslMechanism, _ := kafkaEvent.GetAttribute("sasl_mechanism")
	partitions, _ := kafkaEvent.GetAttribute("partitions")
	acks, _ := kafkaEvent.GetAttribute("acks")
	compressionType, _ := kafkaEvent.GetAttribute("compression_type")

	fmt.Printf("   📡 Brokers: %v\n", brokers)
	if securityProtocol != "" {
		fmt.Printf("   🔒 Security: %v", securityProtocol)
		if saslMechanism != "" {
			fmt.Printf(" (%v)", saslMechanism)
		}
		fmt.Println()
	}
	if partitions != nil && partitions.(int32) > 0 {
		fmt.Printf("   📊 Partitions: %v\n", partitions)
	}
	if acks != "" {
		fmt.Printf("   ✅ Acknowledgments: %v\n", acks)
	}
	if compressionType != "" {
		fmt.Printf("   🗜️  Compression: %v\n", compressionType)
	}

	fmt.Println("   🎯 Ready for production use with eru-functions")
}
