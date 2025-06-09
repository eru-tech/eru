# ERU-QL GraphQL Testing Framework

This comprehensive testing framework provides extensive test coverage for the eru-ql GraphQL service, supporting multiple database types with configurable database aliases (dbalias).

## Features

- **Multi-Database Support**: Tests run against PostgreSQL and MySQL with the same test cases
- **Configurable DBAlias**: Each test can specify which database to run against using the `@<dbalias>` directive
- **Comprehensive Test Coverage**: 
  - GraphQL query operations (simple queries, joins, aggregations, filtering)
  - GraphQL mutation operations (insert, update, delete, upsert, transactions)
  - Edge cases and error handling
  - Performance and load testing
- **Flexible Test Configuration**: Environment-based configuration supporting local, CI, and custom environments
- **Docker Integration**: Complete Docker setup for isolated testing
- **Test Data Management**: Automated test data generation and cleanup
- **Detailed Reporting**: Comprehensive test reports and coverage analysis

## Quick Start

### Prerequisites

- Go 1.22.0 or later
- Docker and Docker Compose (for containerized testing)
- PostgreSQL client (for local testing)
- MySQL client (for local testing)

### Running Tests

#### Local Development (with local databases)
```bash
# Run all tests against local databases
make test-local

# Run tests against specific database
make test-postgres  # PostgreSQL only
make test-mysql     # MySQL only

# Quick tests (skip slow tests)
make test-quick
```

#### Docker-based Testing (recommended)
```bash
# Run complete test suite with Docker
make test-docker

# Start databases in background for development
make test-docker-detached

# Run tests manually after starting databases
make test-local
```

#### CI/CD Environment
```bash
# Run tests in CI mode
make test-ci

# Set up and run tests in CI
make ci-setup
make ci-test
make ci-cleanup
```

## Test Structure

### Test Files

- `graphql_test.go` - Core testing framework and utilities
- `graphql_query_test.go` - GraphQL query operation tests
- `graphql_mutation_test.go` - GraphQL mutation operation tests
- `test_config.go` - Configuration management and database setup

### Test Configuration

The framework supports multiple test environments:

#### Local Environment
```go
TestConfig{
    DBAlias:    "postgres_local",
    DBType:     "postgres", 
    Host:       "localhost",
    Port:       "5432",
    Database:   "eru_test",
    Username:   "postgres",
    Password:   "password",
}
```

#### Environment Variables
Set these environment variables for custom configuration:
```bash
# PostgreSQL
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_DB=eru_test
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=password

# MySQL  
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_DB=eru_test
export MYSQL_USER=root
export MYSQL_PASSWORD=password

# Test settings
export TEST_ENV=local
export TEST_DEBUG=true
export SKIP_SLOW_TESTS=false
```

## Writing Tests

### Basic Test Structure

```go
GraphQLTestCase{
    Name:        "Test Name",
    Description: "Test description",
    Query: `query {
        users @%s {  # %s is replaced with actual dbalias
            id
            name
            email
        }
    }`,
    Variables: map[string]interface{}{
        "min_age": 25,
    },
    Setup: []string{
        "INSERT INTO users (name, email, age) VALUES ('John', 'john@example.com', 30)",
    },
    Teardown: []string{
        "DELETE FROM users WHERE email = 'john@example.com'",
    },
}
```

### Database-Specific Tests

```go
GraphQLTestCase{
    Name:    "PostgreSQL JSON Test",
    DBAlias: "postgres_test", // Only run on PostgreSQL
    Query: `query {
        users @%s {
            id
            metadata___preferences___theme
        }
    }`,
    // ... rest of test
}
```

### Mutation Tests

```go
GraphQLTestCase{
    Name: "Insert User",
    Query: `mutation($user: UserInput!) {
        insert_users(docs: [$user]) @%s {
            returning {
                id
                name
                email
            }
            error
        }
    }`,
    Variables: map[string]interface{}{
        "user": map[string]interface{}{
            "name":  "John Doe",
            "email": "john@example.com",
            "age":   30,
        },
    },
    // ... setup and teardown
}
```

### Transaction Tests

```go
GraphQLTestCase{
    Name: "Multi-operation Transaction",
    Query: `mutation @singleTxn {
        insertUser: insert_users(docs: [$user]) @%s {
            returning { id name }
            error
        }
        insertPost: insert_posts(docs: [$post]) @%s {
            returning { id title }
            error
        }
    }`,
    // ... variables and setup
}
```

## Test Categories

### Query Tests (`graphql_query_test.go`)

1. **Basic Queries**
   - Simple table queries
   - Field selection and aliases
   - WHERE clauses with variables

2. **Advanced Queries**
   - JOIN operations
   - Nested relationships
   - Aggregate functions (COUNT, SUM, AVG)
   - DISTINCT queries
   - Sorting and pagination

3. **Complex Queries**
   - Multi-table joins
   - Subqueries
   - JSON field queries (PostgreSQL)
   - Dynamic filtering

4. **Performance Tests**
   - Large dataset queries
   - Complex multi-join queries
   - Query optimization tests

5. **Edge Cases**
   - Malformed queries
   - Non-existent tables/columns
   - Empty result sets
   - Invalid database aliases

### Mutation Tests (`graphql_mutation_test.go`)

1. **Basic Mutations**
   - INSERT operations (single and bulk)
   - UPDATE operations (single and bulk)
   - DELETE operations

2. **Advanced Mutations**
   - UPSERT operations
   - Nested object insertions
   - Conditional updates
   - Computed fields

3. **Transaction Tests**
   - Multi-operation transactions
   - Transaction rollback scenarios
   - Partial transaction control

4. **Edge Cases**
   - Constraint violations
   - Invalid data types
   - Large batch operations
   - Non-existent record operations

## Database Support

### PostgreSQL Features
- ✅ JSON/JSONB fields
- ✅ UPSERT with ON CONFLICT
- ✅ RETURNING clauses
- ✅ CTEs (Common Table Expressions)
- ✅ Window functions
- ✅ Advanced data types

### MySQL Features  
- ✅ JSON fields (MySQL 5.7+)
- ✅ UPSERT with ON DUPLICATE KEY UPDATE
- ❌ RETURNING clauses (not supported)
- ✅ CTEs (MySQL 8.0+)
- ✅ Window functions (MySQL 8.0+)

### Database-Agnostic Features
- ✅ Basic CRUD operations
- ✅ JOINs and relationships
- ✅ Aggregate functions
- ✅ Transactions
- ✅ Sorting and filtering

## Configuration Options

### Test Environment Settings

```go
GlobalTestSettings{
    TimeoutSeconds:     30,
    MaxConnections:     10,
    EnableDebugLogging: true,
    SkipSlowTests:      false,
    ParallelExecution:  true,
    TestDataPath:       "./testdata",
}
```

### Database Features Detection

The framework automatically detects database capabilities:

```go
DatabaseFeatures{
    SupportsJSON:        true,
    SupportsUpsert:      true,
    SupportsReturning:   true,
    SupportsCTE:         true,
    SupportsWindowFuncs: true,
    MaxIdentifierLength: 63,
    CaseSensitive:       false,
}
```

## Useful Commands

```bash
# Development
make test-watch          # Watch for changes and re-run tests
make test-debug          # Run with debug logging
make test-coverage       # Generate coverage report

# Database management
make test-setup          # Set up test databases
make test-cleanup        # Clean up test environment
make db-create-postgres  # Create PostgreSQL test DB
make db-create-mysql     # Create MySQL test DB

# Performance
make test-bench          # Run benchmarks
make test-bench-compare  # Compare benchmark results

# Reporting
make test-reports        # Generate test reports
make test-docs          # Generate documentation

# Validation
make test-validate       # Validate test configs
make test-lint          # Run linter

# Environment info
make test-env           # Show environment information
make help              # Show all available commands
```

## Docker Setup

The included Docker Compose configuration provides:

- PostgreSQL 15 test database
- MySQL 8.0 test database  
- Test runner container
- Health checks and dependencies
- Volume management for persistent data

```bash
# Start all services
docker-compose -f docker-compose.test.yml up

# Start only databases
docker-compose -f docker-compose.test.yml up postgres-test mysql-test

# View logs
docker-compose -f docker-compose.test.yml logs -f test-runner

# Clean up
docker-compose -f docker-compose.test.yml down -v
```

## Troubleshooting

### Common Issues

1. **Database connection failures**
   - Check if databases are running
   - Verify connection parameters
   - Ensure databases are ready (use health checks)

2. **Test timeouts**
   - Increase timeout values in configuration
   - Check database performance
   - Review query complexity

3. **Docker issues**
   - Ensure Docker daemon is running
   - Check port conflicts (5432, 3306)
   - Verify Docker Compose version compatibility

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
TEST_DEBUG=true make test-local
```

This provides:
- Detailed SQL query logs
- Database connection status
- Test execution timing
- Error stack traces

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Include appropriate setup/teardown
3. Test against both PostgreSQL and MySQL when possible
4. Add database-specific tests when features differ
5. Include edge cases and error conditions
6. Update documentation for new test categories

## Performance Considerations

- Use parallel execution for faster test runs
- Skip slow tests during development (`SKIP_SLOW_TESTS=true`)
- Use Docker for consistent test environments
- Monitor database connections and cleanup
- Optimize test data size for CI environments

## Integration with CI/CD

The framework is designed for CI/CD integration:

```yaml
# Example GitHub Actions workflow
- name: Run eru-ql tests
  run: |
    make ci-setup
    make ci-test
    make ci-cleanup
```

Environment variables and configuration files enable different test profiles for development, staging, and production validation.