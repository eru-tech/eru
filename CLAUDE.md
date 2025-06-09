# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Eru is a comprehensive enterprise microservices platform built in Go 1.22.0, consisting of 20+ specialized services that provide AI/ML capabilities, authentication, data processing, file management, and enterprise infrastructure.

## Build and Development Commands

### Building Services
```bash
# Build individual service
cd eru-{service}
go build -o app .

# Build with Docker
cd eru-{service}
docker build -t eru-{service} .

# Update all dependencies across services
find . -name go.mod -execdir go mod tidy \;

# Update all modules to latest versions
find . -name go.mod -execdir go get -u ./... \;
```

### Running Services
```bash
# Run service locally
cd eru-{service}
go run main.go

# Run with environment configuration
STORE_TYPE=POSTGRES ERUAIPORT=8088 go run main.go

# Common service ports
# eru-ai: 8088 (ERUAIPORT)
# eru-auth: 8085 (ERUAUTHPORT) 
# eru-functions: 8083 (ERUFUNCTIONSPORT)
```

## Architecture Patterns

### Service Structure
All services follow a consistent modular pattern:
- `main.go` - Service entry point with OpenTelemetry tracing setup
- `module_server/` - HTTP server setup, routing, and handlers
- `module_store/` - Data access layer with PostgreSQL/file storage abstraction
- `config/` - Environment-based configuration
- Domain-specific directories for business logic

### Shared Modules System
Services use local module replacement for shared functionality:
- `eru-server` - Common HTTP server foundation with CORS and middleware
- `eru-store` - Unified data store supporting POSTGRES/STANDALONE modes
- `eru-logs` - Centralized logging with Zap and OpenTelemetry
- `eru-crypto` - Cryptographic utilities (AES, RSA, JWT, HMAC, PKCE)
- `eru-utils` - Common utilities across services

### Inter-Service Communication
- HTTP REST APIs between services
- Environment variable service discovery (e.g., `ERUAUTH_BASEURL`, `ERUFUNCTIONS_BASEURL`)
- Event-driven architecture through `eru-events` with AWS SNS/SQS

## Key Services

### Core Infrastructure
- **eru-auth**: Multi-provider OAuth2/JWT authentication with Hydra, Kratos, Microsoft Graph
- **eru-gateway**: API gateway with routing and request management
- **eru-server**: HTTP server foundation with Gorilla Mux
- **eru-store**: Data persistence with PostgreSQL (sqlx) and file-based storage

### AI/ML Platform
- **eru-ai**: Multi-LLM support (Anthropic Claude, OpenAI, AWS Bedrock) with tool integration
- **eru-functions**: Workflow orchestration and function execution
- **eru-templates**: Go template processing system

### Data Processing
- **eru-ql**: GraphQL and SQL query engine with PostgreSQL parser (ANTLR4-generated)
- **eru-read-write**: Excel/CSV processing with data validation
- **eru-files**: Multi-cloud file storage (AWS S3, Azure Blob, GCP Storage)

### Communication & Integrations
- **eru-channels**: Email (SMTP/IMAP), Slack, WhatsApp integration
- **eru-alerts**: Alert management with multiple delivery channels

## Configuration

### Storage Modes
- `STORE_TYPE=STANDALONE` - File-based storage for development
- `STORE_TYPE=POSTGRES` - PostgreSQL with multi-tenant support for production

### Environment Variables
Services use environment-based configuration with these patterns:
- Port configuration: `ERU{SERVICE}PORT`
- Service URLs: `ERU{SERVICE}_BASEURL`
- Database: `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`
- Tracing: `TRACE_URL` for OpenTelemetry integration

## Cloud Integrations

### AWS
- Bedrock for LLM inference
- S3 for file storage
- KMS for secret management
- SNS/SQS for event processing

### Multi-Cloud Support
- GCP: Cloud Storage, Secret Manager, KMS
- Azure: Blob Storage integration

## Development Notes

### Module Dependencies
All services use local module replacement via `replace` directives in go.mod files. Changes to shared modules require rebuilding dependent services.

### Testing
The codebase includes unit tests in several modules (e.g., `eru-crypto/aes/aes_test.go`, `eru-read-write/eru_writes/excel_test.go`). Run tests with:
```bash
cd eru-{module}
go test ./...
```

### Observability
Services include OpenTelemetry tracing integration. Set `TRACE_URL` environment variable to enable distributed tracing to Jaeger/Tempo.