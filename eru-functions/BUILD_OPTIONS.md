# Eru Functions Build Options

## Overview

Eru Functions can be built with different configurations depending on your requirements for event processing capabilities.

## Build Configurations

### 1. Static Build (Default) - CGO Disabled

**Use Case**: Production deployments requiring static binaries, containerized environments, or when Kafka is not needed.

**Build Command**:
```bash
CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o app .
```

**Features**:
- ✅ Static binary (no external dependencies)
- ✅ Smaller container size
- ✅ Better security (no C libraries)
- ✅ AWS SQS/SNS events
- ✅ DB events (database-based async processing)
- ❌ Kafka events (not available)

**Event Types Available**:
- `AWS_SQS` - Amazon Simple Queue Service
- `AWS_SNS` - Amazon Simple Notification Service  
- `DB` - Database-based event processing

### 2. Dynamic Build - CGO Enabled

**Use Case**: When Kafka event processing is required.

**Build Command**:
```bash
CGO_ENABLED=1 go build -o app .
```

**Features**:
- ✅ All event types including Kafka
- ✅ Full async processing capabilities
- ❌ Larger binary size
- ❌ Requires C libraries at runtime
- ❌ More complex container setup

**Event Types Available**:
- `KAFKA` - Apache Kafka
- `AWS_SQS` - Amazon Simple Queue Service
- `AWS_SNS` - Amazon Simple Notification Service
- `DB` - Database-based event processing

## Docker Build Options

### Static Build (Recommended for Production)

```dockerfile
FROM golang:1.22.2-alpine3.19
WORKDIR /build
COPY . .
WORKDIR eru-functions
RUN GOOS=linux CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o app .
```

### Dynamic Build (For Kafka Support)

```dockerfile
FROM golang:1.22.2-alpine3.19
# Install required C libraries for Kafka
RUN apk add --no-cache librdkafka-dev pkgconfig
WORKDIR /build
COPY . .
WORKDIR eru-functions
RUN GOOS=linux CGO_ENABLED=1 go build -o app .
```

## Event Type Configuration

### Using DB Events (Recommended Alternative to Kafka)

When Kafka is not available, use the DB event type for async processing:

```json
{
  "event_type": "DB",
  "event_name": "async-processing",
  "polling_interval": 5,
  "msg_to_poll": 10
}
```

**Benefits of DB Events**:
- No external dependencies
- Works with existing database infrastructure
- Reliable and transactional
- Easy to monitor and debug

### Using AWS SQS/SNS

For cloud-native async processing:

```json
{
  "event_type": "AWS_SQS",
  "event_name": "workflow-events",
  "polling_interval": 10,
  "queue_url": "https://sqs.region.amazonaws.com/account/queue-name",
  "aws_region": "us-east-1"
}
```

## Migration Guide

### From Kafka to DB Events

1. **Update Event Configuration**:
   ```json
   // Before (Kafka)
   {
     "event_type": "KAFKA",
     "brokers": "kafka:9092",
     "topic": "workflow-events"
   }
   
   // After (DB)
   {
     "event_type": "DB", 
     "event_name": "workflow-events",
     "polling_interval": 5,
     "msg_to_poll": 10
   }
   ```

2. **Database Schema**: Ensure the required tables exist:
   ```sql
   -- These tables are automatically created by eru-functions
   CREATE TABLE IF NOT EXISTS erufunctions_async (
     event_id VARCHAR(255) PRIMARY KEY,
     func_group_name VARCHAR(255),
     func_step_name VARCHAR(255),
     event_msg TEXT,
     request_id VARCHAR(255),
     event_request TEXT,
     async_event_name VARCHAR(255)
   );
   
   CREATE TABLE IF NOT EXISTS erufunctions_async_loop (
     async_loop_id SERIAL PRIMARY KEY,
     async_id VARCHAR(255),
     event_id VARCHAR(255),
     loop_var TEXT,
     async_status VARCHAR(50) DEFAULT 'TORUN'
   );
   ```

3. **Update Function Steps**: No changes needed to function logic - the event interface remains the same.

## Performance Considerations

### DB Events vs Kafka

| Aspect | DB Events | Kafka |
|--------|-----------|-------|
| **Latency** | Higher (database round-trip) | Lower (in-memory) |
| **Throughput** | Limited by database | High (distributed) |
| **Reliability** | ACID transactions | At-least-once delivery |
| **Scalability** | Vertical scaling | Horizontal scaling |
| **Complexity** | Simple setup | Requires Kafka cluster |
| **Monitoring** | Standard DB tools | Kafka-specific tools |

### Recommendations

- **Use DB Events for**: Small to medium workloads, simple deployments, when Kafka infrastructure is not available
- **Use Kafka for**: High-throughput scenarios, distributed systems, when you need advanced features like partitioning and replay

## Troubleshooting

### Build Issues

**Error**: `undefined: kafka.NewProducer`
**Solution**: Build with `CGO_ENABLED=1` or use DB events instead

**Error**: `librdkafka not found`
**Solution**: Install librdkafka-dev package or use static build

### Runtime Issues

**Error**: `Kafka is not available when CGO is disabled`
**Solution**: Switch to DB event type or rebuild with CGO enabled

**Error**: `Failed to create Kafka producer`
**Solution**: Check Kafka cluster connectivity and configuration

## Examples

See the `example-usage.go` files in the `eru-events` module for complete configuration examples for each event type. 