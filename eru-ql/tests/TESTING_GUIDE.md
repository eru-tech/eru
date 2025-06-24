# ERU-QL GraphQL Testing Framework

This guide provides comprehensive testing capabilities for eru-ql, a GraphQL and SQL engine that supports multiple database backends with configurable database aliases.

## Overview

The testing framework enables extensive testing of GraphQL scenarios including:

- **Multi-Database Support**: Test the same GraphQL queries against PostgreSQL and MySQL
- **Database Aliases**: Use `@alias` directives to target specific databases
- **Comprehensive Scenarios**: Query operations, mutations, joins, aggregations, JSON field access
- **Filter Operations**: Test eq, in, like, gt, lt, and other filter operators
- **JSON Field Testing**: Test JSON field access with different syntax for PostgreSQL (`___`) and MySQL (`->`)
- **Transaction Testing**: Multi-operation transactions using `@singleTxn` directive

## Test Files

### 1. demo_test.go
Demonstrates basic concepts and validates the database alias system works correctly.

**Run demo tests:**
```bash
make test-demo
# OR
go test -v -run TestGraphQLDatabaseAliasDemo
```

### 2. integration_test.go
Universal test framework with database-agnostic test cases that work with any configured database alias.

**Key Features:**
- **Universal Test Cases**: Same GraphQL queries run against all configured databases
- **Template-Based Queries**: Use `{{.DBAlias}}` placeholders that get replaced with actual aliases
- **Database-Agnostic Validation**: Tests work with PostgreSQL, MySQL, or any supported RDBMS
- **Single Database Testing**: Target specific database alias for debugging

**Run universal tests (demo mode):**
```bash
SKIP_INTEGRATION_TESTS=true go test -v -run TestUniversalGraphQLQueries
```

**Run against specific database:**
```bash
TEST_DB_ALIAS=postgres_main go test -v -run TestSingleDatabaseAlias
```

### 3. Database Schemas
- **PostgreSQL schema** (`postgres-schema-comprehensive.sql`)
- **MySQL schema** (`mysql-schema-comprehensive.sql`)

## Universal Test Approach

The framework uses **template-based queries** that work with any database alias:

```go
// Template query with placeholder
QueryTemplate: `query {
    users @{{.DBAlias}} {
        id name email region
    }
}`

// Gets rendered as:
// For PostgreSQL: users @postgres_main { ... }
// For MySQL:      users @mysql_analytics { ... }
```

This ensures the **same exact test logic** runs against different databases.

## Usage Examples

### 1. Demo Tests (No Setup Required)
```bash
# Run demo tests - validates GraphQL structure and aliases
make test-demo
```

### 2. Universal Tests - Same Queries, All Databases
```bash
# Run same test queries against all configured databases
make test-universal
```

### 3. Single Database Testing
```bash
# Test against specific database alias
make test-single DB=postgres_main
make test-single DB=mysql_analytics
```

### 4. Adding New Database Aliases

To add a new database alias, simply update the configuration:

```go
// In NewIntegrationTestFramework()
databaseConfigs := []DatabaseConfig{
    {
        DatabaseAlias: "postgres_main",
        DatabaseType:  "postgres", 
        DSN:          "postgres://user:pass@host:port/db",
    },
    {
        DatabaseAlias: "mysql_analytics",
        DatabaseType:  "mysql",
        DSN:          "user:pass@tcp(host:port)/db",
    },
    // Add new database here
    {
        DatabaseAlias: "postgres_shard_west",
        DatabaseType:  "postgres",
        DSN:          "postgres://user:pass@west-host:port/db",
    },
}
```

The same test queries will automatically run against the new database!

## 📋 Available Make Commands

```bash
make help                    # Show all available commands
make test-demo              # Run demo tests (no databases)
make test-integration       # Run real tests against eru-ql service
make test-full              # Run both demo and integration tests
make test-docker-demo       # Run demo tests in Docker
make test-docker-db-only    # Start only databases
make test-docker-db-check   # Check database health
make test-cleanup           # Stop and remove all containers
```

## 🔍 What Each Test Validates

### Demo Tests (`demo_test.go`)
- ✅ GraphQL query syntax and structure
- ✅ Database alias directive detection (`@postgres_main`, `@mysql_analytics`)
- ✅ Query type identification (query vs mutation)
- ✅ Transaction directive validation (`@singleTxn`)
- ✅ Database-specific feature keywords
- ✅ Configuration scenarios and best practices

### Integration Tests (`integration_test.go`)
- ✅ **Real database connections** to PostgreSQL and MySQL
- ✅ **Actual GraphQL execution** against eru-ql service
- ✅ **Data retrieval and validation** from real tables
- ✅ **Insert operations** with database-specific features
- ✅ **Transaction testing** across multiple operations
- ✅ **Error handling** for invalid queries
- ✅ **Variable substitution** and parameterization

## 🗄️ Database Schemas Created

### PostgreSQL Tables
```sql
users (id, name, email, region, created_at, updated_at)
audit_log (id, action, entity_type, entity_id, details, timestamp)
products (id, name, price, category, inventory_count, metadata, created_at)
```

### MySQL Tables
```sql
user_analytics (id, user_id, page_views, session_duration, device_type, browser, created_at)
events (id, event_type, user_id, event_data, timestamp, source)
orders (id, user_id, product_id, quantity, total_amount, status, order_date)
```

## 🎯 Example Integration Test Scenarios

### PostgreSQL Tests
1. **Query All Users** - `SELECT * FROM users` via GraphQL with `@postgres_main`
2. **Region-Based Filtering** - Users from specific regions using variables
3. **User Insertion** - `INSERT INTO users` with RETURNING clause
4. **Transaction Testing** - Multiple operations with `@singleTxn`

### MySQL Tests
1. **Analytics Queries** - User behavior data via `@mysql_analytics`
2. **Event Insertion** - Bulk event logging
3. **Cross-Table Queries** - Join operations across analytics tables

### Cross-Database Validation
- Same logical queries executed against both database types
- Verification that database aliases correctly route to intended databases
- Performance comparison between PostgreSQL and MySQL

## 🔧 Troubleshooting

### Database Connection Issues
```bash
# Check if databases are running
docker-compose -f docker-compose.test.yml ps

# Check database logs
docker-compose -f docker-compose.test.yml logs postgres-test mysql-test

# Test direct connections
docker exec eru-ql-postgres-test psql -U postgres -d eru_test -c "SELECT 1;"
docker exec eru-ql-mysql-test mysql -u root -ppassword eru_test -e "SELECT 1;"
```

### ERU-QL Service Issues
```bash
# Check if service is running
curl -f http://localhost:8087/health

# Check GraphQL endpoint
curl -X POST http://localhost:8087/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}'
```

### Environment Issues
```bash
# Verify environment variables
echo $POSTGRES_HOST $MYSQL_HOST
echo $STORE_TYPE $ERUQLPORT

# Re-run environment setup
. ../../set_eruconfig_db.sh db.dev.erutech.io eruconfig_devapp
```

## 🎉 Success Indicators

When everything works correctly, you should see:

### Demo Tests
```
✅ Database directive @postgres_main found in query
✅ Detected GraphQL Query operation
✅ PostgreSQL RETURNING clause detected
🎉 Test case validation completed successfully
```

### Integration Tests
```
✅ eru-ql service is running
✅ Successfully queried 5 users from PostgreSQL
✅ Successfully inserted user with ID: 6
✅ Successfully queried 7 analytics records from MySQL
✅ Transaction completed successfully
```

## 📊 Performance and Monitoring

The integration tests include:
- **Response time measurement** for each GraphQL operation
- **Data validation** ensuring correct results
- **Error handling** for malformed queries
- **Connection pooling** efficiency testing
- **Cross-database comparison** metrics

This comprehensive testing framework ensures your GraphQL implementation works correctly across multiple database types while maintaining the flexibility to target specific databases using configurable aliases.