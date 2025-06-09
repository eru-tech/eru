# ERU-QL GraphQL Testing Framework Makefile

.PHONY: help test-demo test-integration test-universal test-single test-comprehensive test-all clean

# Default target
help:
	@echo "🚀 ERU-QL GraphQL Testing Framework"
	@echo "====================================="
	@echo ""
	@echo "Available commands:"
	@echo ""
	@echo "📋 Demo Tests (No databases required):"
	@echo "  make test-demo              # Run demo tests that validate GraphQL structure"
	@echo ""
	@echo "🔧 Integration Tests (Requires eru-ql service + databases):"
	@echo "  make test-universal         # Run same queries against all configured databases"
	@echo "  make test-single DB=<alias> # Run tests against specific database alias"
	@echo "  make test-comprehensive     # Run comprehensive framework tests"
	@echo "  make test-integration       # Run all integration tests"
	@echo ""
	@echo "🎯 Complete Test Suite:"
	@echo "  make test-all               # Run both demo and integration tests"
	@echo ""
	@echo "🗄️ Database Schema Setup:"
	@echo "  make setup-postgres         # Setup PostgreSQL test schema"
	@echo "  make setup-mysql            # Setup MySQL test schema"
	@echo "  make setup-databases        # Setup both PostgreSQL and MySQL schemas"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  make clean                  # Clean test artifacts"
	@echo ""
	@echo "📖 Examples:"
	@echo "  make test-single DB=postgres_main    # Test only PostgreSQL"
	@echo "  make test-single DB=mysql_analytics  # Test only MySQL"
	@echo ""
	@echo "⚙️  Environment Setup:"
	@echo "  # Set environment before running integration tests:"
	@echo "  export POSTGRES_HOST=localhost"
	@echo "  export POSTGRES_DB=eru_test"
	@echo "  export POSTGRES_USER=postgres"
	@echo "  export POSTGRES_PASSWORD=password"
	@echo "  export MYSQL_HOST=localhost"
	@echo "  export MYSQL_DB=eru_test"
	@echo "  export MYSQL_USER=root"
	@echo "  export MYSQL_PASSWORD=password"
	@echo "  export ERU_QL_BASE_URL=http://localhost:8087"

# Demo tests - validate GraphQL structure and database aliases (no real databases needed)
test-demo:
	@echo "🎯 Running demo tests..."
	@echo "These tests validate GraphQL query structure and database alias usage"
	@echo "No real databases or eru-ql service required"
	@echo ""
	go test -v -timeout 5m -run TestGraphQLDatabaseAliasDemo

# Universal tests - run same queries against all configured databases
test-universal:
	@echo "🌍 Running universal tests against all configured databases..."
	@echo "Same GraphQL queries executed against PostgreSQL and MySQL"
	@echo "Requires eru-ql service and databases to be running"
	@echo ""
	go test -v -timeout 10m -run TestUniversalGraphQLQueries

# Single database tests - run against specific database alias
test-single:
	@if [ -z "$(DB)" ]; then \
		echo "❌ Error: DB parameter required"; \
		echo "Usage: make test-single DB=<database_alias>"; \
		echo "Available aliases: postgres_main, mysql_analytics"; \
		exit 1; \
	fi
	@echo "🎯 Running tests against database alias: $(DB)"
	@echo "Same GraphQL queries targeted at specific database"
	@echo ""
	TEST_DB_ALIAS=$(DB) go test -v -timeout 10m -run TestSingleDatabaseAlias

# Comprehensive framework tests - all test categories
test-comprehensive:
	@echo "🔬 Running comprehensive framework tests..."
	@echo "Filter operators, JSON fields, aggregations, joins, mutations"
	@echo ""
	SKIP_INTEGRATION_TESTS=true go test -v -timeout 5m -run TestComprehensiveGraphQLWithDatabaseAliases

test-integration: ## Run integration tests against real databases and eru-ql service
	@echo "Running integration tests..."
	@echo "Checking if eru-ql service is running..."
	@if curl -f -s http://localhost:8087/health >/dev/null 2>&1; then \
		echo "✅ eru-ql service is running"; \
		go test -v -timeout 10m ./integration_test.go; \
	else \
		echo "❌ eru-ql service not running at http://localhost:8087"; \
		echo "Please start eru-ql service first: go run main.go"; \
		exit 1; \
	fi

test-full: ## Run both demo and integration tests
	@echo "Running full test suite..."
	@make test-demo
	@make test-integration

test-local: ## Run tests against local databases
	@echo "Running tests in local environment..."
	TEST_ENV=local go test -v -timeout 10m ./...

test-ci: ## Run tests in CI environment
	@echo "Running tests in CI environment..."
	TEST_ENV=ci go test -v -timeout 10m ./...

test-postgres: ## Run tests only against PostgreSQL
	@echo "Running tests against PostgreSQL only..."
	POSTGRES_HOST=localhost POSTGRES_PORT=5432 POSTGRES_DB=eru_test POSTGRES_USER=postgres POSTGRES_PASSWORD=password \
	go test -v -timeout 10m -run ".*Postgres.*" ./...

test-mysql: ## Run tests only against MySQL
	@echo "Running tests against MySQL only..."
	MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_DB=eru_test MYSQL_USER=root MYSQL_PASSWORD=password \
	go test -v -timeout 10m -run ".*MySQL.*" ./...

test-quick: ## Run quick tests (skip slow tests)
	@echo "Running quick tests..."
	SKIP_SLOW_TESTS=true go test -v -timeout 5m ./...

test-full: ## Run full test suite including performance tests
	@echo "Running full test suite..."
	SKIP_SLOW_TESTS=false go test -v -timeout 15m ./...

test-parallel: ## Run tests in parallel
	@echo "Running tests in parallel..."
	PARALLEL_TESTS=true go test -v -parallel 4 -timeout 10m ./...

# Docker-based testing
test-docker: ## Run tests using Docker containers
	@echo "Starting test databases with Docker..."
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit test-runner

test-docker-detached: ## Run test databases in background
	@echo "Starting test databases in background..."
	docker-compose -f docker-compose.test.yml up -d postgres-test mysql-test

test-docker-logs: ## Show test runner logs
	docker-compose -f docker-compose.test.yml logs -f test-runner

# Database setup and management
test-setup: ## Set up test databases and sample data
	@echo "Setting up test databases..."
	@make db-create-postgres
	@make db-create-mysql
	@make db-seed

test-cleanup: ## Clean up test databases and containers
	@echo "Cleaning up test environment..."
	docker-compose -f docker-compose.test.yml down -v
	docker-compose -f docker-compose.test.yml rm -f

# Docker-based testing (simplified)
test-docker-demo: ## Run demo tests in Docker (no dependencies required)
	@echo "Running demo tests in Docker..."
	docker run --rm -v $(PWD):/app -w /app golang:1.22 go test -v -timeout 2m ./demo_test.go

test-docker-db-only: ## Start only the databases in Docker
	@echo "Starting test databases..."
	docker-compose -f docker-compose.test.yml up -d postgres-test mysql-test
	@echo "Waiting for databases to be ready..."
	@sleep 20
	@echo "Checking database health..."
	@docker-compose -f docker-compose.test.yml ps postgres-test mysql-test

test-docker-db-check: ## Check database health
	@echo "Checking database health..."
	@docker-compose -f docker-compose.test.yml ps postgres-test mysql-test

test-docker-logs: ## Show database logs
	@echo "Database logs:"
	docker-compose -f docker-compose.test.yml logs postgres-test mysql-test

db-create-postgres: ## Create PostgreSQL test database
	@echo "Creating PostgreSQL test database..."
	@if command -v psql >/dev/null 2>&1; then \
		PGPASSWORD=password psql -h localhost -U postgres -c "DROP DATABASE IF EXISTS eru_test;" || true; \
		PGPASSWORD=password psql -h localhost -U postgres -c "CREATE DATABASE eru_test;"; \
		echo "PostgreSQL test database created"; \
	else \
		echo "psql not found. Please install PostgreSQL client or use Docker."; \
	fi

db-create-mysql: ## Create MySQL test database
	@echo "Creating MySQL test database..."
	@if command -v mysql >/dev/null 2>&1; then \
		mysql -h localhost -u root -ppassword -e "DROP DATABASE IF EXISTS eru_test;" || true; \
		mysql -h localhost -u root -ppassword -e "CREATE DATABASE eru_test;"; \
		echo "MySQL test database created"; \
	else \
		echo "mysql not found. Please install MySQL client or use Docker."; \
	fi

db-seed: ## Seed test databases with sample data
	@echo "Seeding test databases with sample data..."
	@if [ -f ./testdata/sql/postgres-seed.sql ]; then \
		PGPASSWORD=password psql -h localhost -U postgres -d eru_test -f ./testdata/sql/postgres-seed.sql || true; \
	fi
	@if [ -f ./testdata/sql/mysql-seed.sql ]; then \
		mysql -h localhost -u root -ppassword eru_test < ./testdata/sql/mysql-seed.sql || true; \
	fi

# Development helpers
test-watch: ## Watch for changes and re-run tests
	@echo "Watching for changes..."
	@if command -v fswatch >/dev/null 2>&1; then \
		fswatch -o ./*.go | xargs -n1 -I{} make test-quick; \
	elif command -v inotifywait >/dev/null 2>&1; then \
		while inotifywait -e modify ./*.go; do make test-quick; done; \
	else \
		echo "Install fswatch or inotify-tools to use watch mode"; \
	fi

test-debug: ## Run tests with debug logging enabled
	@echo "Running tests with debug logging..."
	TEST_DEBUG=true go test -v -timeout 10m ./...

test-coverage: ## Run tests and generate coverage report
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Test data management
test-data-generate: ## Generate test data files
	@echo "Generating test data files..."
	@mkdir -p testdata/postgres testdata/mysql
	go run test_config.go -generate-data

test-data-clean: ## Clean up test data files
	@echo "Cleaning up test data files..."
	rm -rf testdata/postgres/* testdata/mysql/*

# Reporting
test-reports: ## Generate test reports
	@echo "Generating test reports..."
	@mkdir -p test-results
	go test -v -json ./... > test-results/test-results.json
	go run test_config.go -generate-report test-results/test-report.json

# Benchmarking
test-bench: ## Run performance benchmarks
	@echo "Running performance benchmarks..."
	go test -v -bench=. -benchmem ./...

test-bench-compare: ## Compare benchmark results
	@echo "Running benchmark comparison..."
	@if [ -f benchmark-baseline.txt ]; then \
		go test -bench=. -benchmem ./... > benchmark-current.txt; \
		benchcmp benchmark-baseline.txt benchmark-current.txt; \
	else \
		go test -bench=. -benchmem ./... | tee benchmark-baseline.txt; \
		echo "Baseline benchmark saved to benchmark-baseline.txt"; \
	fi

# Validation and linting
test-validate: ## Validate test configurations and GraphQL queries
	@echo "Validating test configurations..."
	go run test_config.go -validate

test-lint: ## Run linter on test files
	@echo "Running linter on test files..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./*_test.go; \
	else \
		echo "golangci-lint not found. Please install it."; \
	fi

# CI/CD helpers
ci-setup: ## Set up CI environment
	@echo "Setting up CI environment..."
	@make test-docker-detached
	@sleep 30 # Wait for databases to be ready

ci-test: ## Run tests in CI mode
	@echo "Running CI tests..."
	@make test-ci

ci-cleanup: ## Clean up CI environment
	@echo "Cleaning up CI environment..."
	@make test-cleanup

# Documentation
test-docs: ## Generate test documentation
	@echo "Generating test documentation..."
	@mkdir -p docs
	go doc -all . > docs/test-docs.txt
	@echo "Test documentation generated in docs/test-docs.txt"

# Environment information
test-env: ## Show test environment information
	@echo "Test Environment Information:"
	@echo "GO_VERSION: $(shell go version)"
	@echo "TEST_ENV: $(TEST_ENV)"
	@echo "POSTGRES_HOST: $(POSTGRES_HOST)"
	@echo "MYSQL_HOST: $(MYSQL_HOST)"
	@echo "Current directory: $(shell pwd)"
	@echo "Docker version: $(shell docker --version 2>/dev/null || echo 'Docker not available')"
	@echo "Docker Compose version: $(shell docker-compose --version 2>/dev/null || echo 'Docker Compose not available')"

# Default environment variables
TEST_ENV ?= local
POSTGRES_HOST ?= localhost
MYSQL_HOST ?= localhost
SKIP_SLOW_TESTS ?= false
PARALLEL_TESTS ?= false
TEST_DEBUG ?= false