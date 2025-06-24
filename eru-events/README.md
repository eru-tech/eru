# Eru Events - Kafka Implementation

This directory contains the Kafka implementation for the Eru Events system, providing asynchronous event processing capabilities for the eru-functions orchestration platform.

## 🚀 Quick Start

### Prerequisites
- Docker and Docker Compose installed
- Go 1.22.0 or later
- Port 9092 (Kafka), 2181 (Zookeeper), and 8080 (Kafka UI) available

### 1. Start Kafka Infrastructure

```bash
# Start all services (Kafka, Zookeeper, Kafka UI)
docker-compose up -d

# Check service status
docker-compose ps

# View logs
docker-compose logs -f kafka
```

### 2. Access Kafka UI
Open your browser and navigate to: `http://localhost:8080`

The Kafka UI provides:
- 📊 Cluster overview and metrics
- 📂 Topic management (create, delete, configure)
- 📨 Message browser and publisher
- 👥 Consumer group monitoring
- ⚙️ Configuration management

### 3. Run Tests

```bash
# Run the comprehensive test script
go run test-kafka.go

# Run unit tests
go test ./events -v
```

## 📖 Usage

### Kafka Event Configuration

#### Basic Configuration
```json
{
  "event_type": "KAFKA",
  "event_name": "my-topic",
  "polling_interval": 5,
  "brokers": "localhost:9092",
  "topic": "my-topic",
  "group_id": "my-consumer-group",
  "partitions": 3,
  "replication_factor": 1
}
```

#### Enterprise Security Configuration

**AWS MSK with IAM Authentication:**
```json
{
  "event_type": "KAFKA",
  "brokers": "b-1.mskcluster.abc123.kafka.us-east-1.amazonaws.com:9098",
  "topic": "workflow-events",
  "group_id": "eru-functions",
  "security_protocol": "SASL_SSL",
  "sasl_mechanism": "AWS_MSK_IAM",
  "aws_region": "us-east-1",
  "aws_access_key_id": "AKIA...",
  "aws_secret_access_key": "...",
  "partitions": 12,
  "replication_factor": 3
}
```

**SCRAM-SHA-256 Authentication:**
```json
{
  "event_type": "KAFKA",
  "brokers": "kafka1.company.com:9093",
  "topic": "secure-events",
  "group_id": "eru-secure",
  "security_protocol": "SASL_SSL",
  "sasl_mechanism": "SCRAM-SHA-256",
  "sasl_username": "eru-service",
  "sasl_password": "secure-password",
  "ssl_ca_location": "/etc/ssl/certs/ca-cert.pem"
}
```

**Kerberos Authentication:**
```json
{
  "event_type": "KAFKA",
  "brokers": "kafka.secure.corp:9093",
  "topic": "kerberos-events",
  "group_id": "eru-kerberos",
  "security_protocol": "SASL_SSL",
  "sasl_mechanism": "GSSAPI",
  "sasl_kerberos_service_name": "kafka",
  "sasl_kerberos_principal": "eru-client@COMPANY.COM",
  "ssl_ca_location": "/etc/ssl/certs/ca-cert.pem"
}
```

#### Complete Configuration Parameters

<details>
<summary><strong>🔒 Security Parameters</strong></summary>

| Parameter | Description | Values | Required |
|-----------|-------------|--------|----------|
| `security_protocol` | Security protocol | `PLAINTEXT`, `SSL`, `SASL_PLAINTEXT`, `SASL_SSL` | No |
| `sasl_mechanism` | SASL mechanism | `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`, `GSSAPI`, `OAUTHBEARER`, `AWS_MSK_IAM` | No |
| `sasl_username` | SASL username | String | For PLAIN/SCRAM |
| `sasl_password` | SASL password | String | For PLAIN/SCRAM |
| `sasl_kerberos_service_name` | Kerberos service name | String (default: `kafka`) | For GSSAPI |
| `sasl_kerberos_principal` | Kerberos principal | String | For GSSAPI |
| `sasl_oauth_token` | OAuth bearer token | String | For OAUTHBEARER |
| `aws_region` | AWS region | String | For AWS_MSK_IAM |
| `aws_access_key_id` | AWS access key | String | For AWS_MSK_IAM |
| `aws_secret_access_key` | AWS secret key | String | For AWS_MSK_IAM |
| `aws_session_token` | AWS session token | String | For AWS_MSK_IAM (temp creds) |

</details>

<details>
<summary><strong>🔐 SSL/TLS Parameters</strong></summary>

| Parameter | Description | Required |
|-----------|-------------|----------|
| `ssl_ca_location` | CA certificate file path | For SSL verification |
| `ssl_certificate_location` | Client certificate file path | For mutual auth |
| `ssl_key_location` | Client private key file path | For mutual auth |
| `ssl_key_password` | Private key password | If key is encrypted |
| `ssl_keystore_location` | Java keystore path | Alternative to PEM |
| `ssl_keystore_password` | Keystore password | For keystore |
| `ssl_truststore_location` | Java truststore path | Alternative to CA |
| `ssl_truststore_password` | Truststore password | For truststore |

</details>

<details>
<summary><strong>⚡ Performance Parameters</strong></summary>

| Parameter | Description | Default | Range |
|-----------|-------------|---------|-------|
| `acks` | Acknowledgment level | `all` | `0`, `1`, `all` |
| `retries` | Producer retries | `3` | `0-2147483647` |
| `batch_size` | Batch size (bytes) | `16384` | `0-2147483647` |
| `linger_ms` | Batch linger time | `0` | `0-2147483647` |
| `compression_type` | Message compression | `snappy` | `none`, `gzip`, `snappy`, `lz4`, `zstd` |
| `max_in_flight_requests` | Max parallel requests | `5` | `1-2147483647` |
| `buffer_memory` | Producer buffer size | `33554432` | `0-2147483647` |
| `request_timeout_ms` | Request timeout | `30000` | `1-2147483647` |
| `delivery_timeout_ms` | Delivery timeout | `120000` | `1-2147483647` |

</details>

<details>
<summary><strong>📥 Consumer Parameters</strong></summary>

| Parameter | Description | Default | Range |
|-----------|-------------|---------|-------|
| `auto_offset_reset` | Initial offset | `earliest` | `earliest`, `latest`, `none` |
| `enable_auto_commit` | Auto commit offsets | `true` | `true`, `false` |
| `session_timeout_ms` | Session timeout | `10000` | `1-2147483647` |
| `heartbeat_interval_ms` | Heartbeat interval | `3000` | `1-2147483647` |
| `fetch_min_bytes` | Minimum fetch size | `1` | `1-2147483647` |
| `fetch_max_wait_ms` | Fetch wait time | `500` | `0-2147483647` |
| `max_partition_fetch_bytes` | Max per partition | `1048576` | `1-2147483647` |
| `max_poll_records` | Records per poll | `500` | `1-2147483647` |
| `isolation_level` | Transaction isolation | `read_uncommitted` | `read_uncommitted`, `read_committed` |

</details>

<details>
<summary><strong>📊 Topic Configuration</strong></summary>

| Parameter | Description | Default |
|-----------|-------------|---------|
| `partitions` | Number of partitions | `1` |
| `replication_factor` | Replication factor | `1` |
| `topic_config` | Topic-level settings | `{}` |

**Common topic_config values:**
```json
{
  "retention.ms": "604800000",
  "cleanup.policy": "delete",
  "compression.type": "snappy",
  "min.insync.replicas": "2",
  "segment.ms": "86400000"
}
```

</details>

### Supported Event Operations

1. **Create Topic**: `CreateEvent(ctx)`
2. **Delete Topic**: `DeleteEvent(ctx)`
3. **Publish Message**: `Publish(ctx, msg, event)`
4. **Poll Messages**: `Poll(ctx)`
5. **Clone Event**: `Clone(ctx)`

### Security Configuration

For production environments, configure SASL/SSL:

```json
{
  "security_protocol": "SASL_SSL",
  "sasl_mechanism": "PLAIN",
  "sasl_username": "your_username",
  "sasl_password": "your_password",
  "ssl_ca_location": "/path/to/ca-cert",
  "ssl_certificate_location": "/path/to/client-cert",
  "ssl_key_location": "/path/to/client-key"
}
```

## 🛠️ Development

### Building

```bash
# Build the events module
go build ./events

# Build with dependencies
go mod tidy && go build ./events
```

### Testing

```bash
# Run all tests
go test ./events

# Run with verbose output
go test ./events -v

# Run specific test
go test ./events -run TestKafkaEventCreation
```

### Adding New Features

1. Implement methods in `events/kafka.go`
2. Add corresponding tests in `events/kafka_test.go`
3. Update this README with new configuration options
4. Test with the local Docker Compose setup

## 📋 Docker Compose Services

### Core Services

- **Zookeeper** (`:2181`) - Kafka coordination service
- **Kafka** (`:9092`) - Message broker
- **Kafka UI** (`:8080`) - Web management interface

### Optional Services (Use Profiles)

```bash
# Start with Schema Registry
docker-compose --profile with-schema-registry up -d

# Start with Kafka Connect
docker-compose --profile with-connect up -d

# Start with all optional services
docker-compose --profile with-schema-registry --profile with-connect up -d
```

- **Schema Registry** (`:8081`) - Avro schema management
- **Kafka Connect** (`:8083`) - Data integration platform

## 🔧 Configuration

### Important: No Environment Variables Required

**The Kafka event implementation does NOT require any environment variables for runtime operation.** All Kafka configuration comes from the event configuration JSON object.

### Event Configuration Only

All Kafka settings are specified in the event configuration JSON:

```json
{
  "event_type": "KAFKA",
  "event_name": "my-topic",
  "brokers": "localhost:9092",
  "topic": "my-topic",
  "group_id": "my-consumer-group"
}
```

### Local Development Setup (.env)

The `.env` file is **ONLY** for Docker Compose infrastructure setup and contains no runtime configuration:

```bash
# Docker Compose Infrastructure Only
KAFKA_BROKER_ID=1
KAFKA_LISTENERS=INTERNAL://0.0.0.0:19092,EXTERNAL://0.0.0.0:9092
KAFKA_ADVERTISED_LISTENERS=INTERNAL://kafka:19092,EXTERNAL://localhost:9092
```

### Deployment Portability

✅ **No .env file needed in production**  
✅ **No environment variables required**  
✅ **Complete configuration via JSON**  
✅ **Works across different environments**  

This design ensures that eru-functions can pass complete Kafka configuration through JSON without any external dependencies.

### Topic Configuration

Default topic settings:
- **Partitions**: 1 (configurable)
- **Replication Factor**: 1 (configurable)
- **Retention**: 168 hours (7 days)
- **Auto Create**: Enabled
- **Delete**: Enabled

## 🐛 Troubleshooting

### Common Issues

1. **Port conflicts**
   ```bash
   # Check what's using the ports
   netstat -an | grep 9092
   netstat -an | grep 8080
   
   # Stop conflicting services
   docker-compose down
   ```

2. **Connection refused**
   ```bash
   # Check Kafka health
   docker-compose exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092
   
   # Check container logs
   docker-compose logs kafka
   ```

3. **Topic creation fails**
   ```bash
   # Manually create topic for testing
   docker-compose exec kafka kafka-topics --create --topic test-topic --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1
   ```

4. **Consumer group issues**
   ```bash
   # List consumer groups
   docker-compose exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 --list
   
   # Reset consumer group offset
   docker-compose exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 --group my-group --reset-offsets --to-earliest --all-topics --execute
   ```

### Useful Commands

```bash
# List all topics
docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Describe topic
docker-compose exec kafka kafka-topics --describe --topic my-topic --bootstrap-server localhost:9092

# Console producer (for manual testing)
docker-compose exec kafka kafka-console-producer --topic my-topic --bootstrap-server localhost:9092

# Console consumer (for manual testing)
docker-compose exec kafka kafka-console-consumer --topic my-topic --from-beginning --bootstrap-server localhost:9092

# Delete topic
docker-compose exec kafka kafka-topics --delete --topic my-topic --bootstrap-server localhost:9092
```

## 🔗 Integration with Eru Functions

The Kafka event implementation integrates seamlessly with eru-functions for:

- **Async Step Execution**: Trigger function steps asynchronously via Kafka messages
- **Event-Driven Workflows**: React to Kafka events to start/continue workflows
- **Scalable Processing**: Leverage Kafka's partitioning for parallel processing
- **Reliable Delivery**: Use Kafka's durability guarantees for critical workflows

### Zero-Configuration Integration

**No environment setup required!** eru-functions simply passes complete configuration via JSON:

```go
// eru-functions passes this configuration (no .env files needed)
kafkaConfig := `{
    "event_type": "KAFKA",
    "brokers": "your-kafka-cluster:9092",
    "topic": "workflow-events",
    "group_id": "eru-functions-group"
}`

// Create and configure event
kafkaEvent := events.GetEvent("KAFKA")
kafkaEvent.MakeFromJson(ctx, &json.RawMessage(kafkaConfig))
kafkaEvent.Init(ctx)

// Trigger async step
stepData := map[string]interface{}{
    "step_id": "process-data",
    "workflow_id": "wf-123",
    "payload": userData,
}

msgId, err := kafkaEvent.Publish(ctx, stepData, kafkaEvent)
```

See `example-usage.go` for complete configuration examples.

## 📚 Additional Resources

- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [Confluent Kafka Go Client](https://github.com/confluentinc/confluent-kafka-go)
- [Kafka UI GitHub](https://github.com/provectus/kafka-ui)
- [Eru Functions Integration Guide](../eru-functions/README.md)

## 🤝 Contributing

1. Make changes to the Kafka implementation
2. Add/update tests in `events/kafka_test.go`
3. Test with local Docker Compose setup
4. Update this README if needed
5. Submit PR with clear description

---

💡 **Tip**: Use Kafka UI at `http://localhost:8080` to monitor topics, messages, and consumer groups while developing and testing!