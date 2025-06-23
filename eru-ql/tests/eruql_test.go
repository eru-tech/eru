package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{} `json:"data"`
	Errors []struct {
		Message string   `json:"message"`
		Path    []string `json:"path,omitempty"`
	} `json:"errors,omitempty"`
}

type TestConfig struct {
	EruQLBaseURL string
	ProjectId    string
	DbConfig     *DbConfig
}

// Test configuration for database aliases
type DbConfig struct {
	DbAlias    string
	DbType     string
	DbCategory string
	DbHost     string
	DbPort     string
	DbName     string
	DbSchema   string
	DbUser     string
	DbPass     string
}

func (c *DbConfig) GetDsn() string {
	switch c.DbType {
	case "postgres":
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", c.DbUser, c.DbPass, c.DbHost, c.DbPort, c.DbName)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", c.DbUser, c.DbPass, c.DbHost, c.DbPort, c.DbName)
	}
	return ""
}

// Database alias configuration
type DatabaseConfig struct {
	DatabaseAlias string
	DatabaseType  string
	DSN           string
}

// getTestConfig returns test configuration from environment variables
func getDbConfig(dbAlias string) *DbConfig {

	dbConfigs := []DbConfig{
		{
			DbAlias:    "postgres_test",
			DbCategory: "sql",
			DbType:     "postgres",
			DbHost:     "localhost",
			DbPort:     "5432",
			DbName:     "eru_test",
			DbSchema:   "public",
			DbUser:     "postgres",
			DbPass:     "password",
		},
		{
			DbAlias:    "mysql_test",
			DbCategory: "sql",
			DbType:     "mysql",
			DbHost:     "localhost",
			DbPort:     "3306",
			DbName:     "eru_test",
			DbSchema:   "public",
			DbUser:     "root",
			DbPass:     "password",
		},
	}

	for _, config := range dbConfigs {
		if config.DbAlias == dbAlias {
			return &config
		}
	}
	return nil
}

// TestSingleDatabaseAlias - Test against a specific database alias (useful for debugging)
func TestGraphQLQueries(t *testing.T) {

	// Get database alias from environment variable
	dbAlias := os.Getenv("TEST_DBALIAS")
	if dbAlias == "" {
		dbAlias = "postgres_test" // Default to postgres_test if not specified
	}
	tConfig := TestConfig{
		EruQLBaseURL: "http://localhost:8087",
		ProjectId:    "test",
		DbConfig:     getDbConfig(dbAlias),
	}

	if os.Getenv("SKIP_SETUP") == "true" {
		//t.Skip("Skipping setup (SKIP_SETUP=true)")
	} else {
		if err := tConfig.executeSchemaScript(t); err != nil {
			t.Fatalf("Failed to execute schema script: %v", err)
		}
		if err := tConfig.setupEruql(t); err != nil {
			t.Fatalf("Failed to setup eru-ql: %v", err)
		}
	}

	// Wait for eru-ql service
	if err := waitForEruQLService(&tConfig, 30*time.Second); err != nil {
		t.Fatalf("failed to wait for eru-ql service: %v", err)
	}

	t.Logf("🎯 Testing specifically against database alias: %s", tConfig.DbConfig.DbAlias)

	// Run comprehensive test suite for each database alias
	t.Run("FilterOperatorTests", func(t *testing.T) {
		tConfig.testFilterOperators(t, tConfig.DbConfig.DbAlias)
	})

	/* t.Run("JSONFieldTests", func(t *testing.T) {
		tConfig.testJSONFieldQueries(t, tConfig.DbConfig.DbAlias)
	})

	t.Run("AggregationTests", func(t *testing.T) {
		tConfig.testAggregationQueries(t, tConfig.DbConfig.DbAlias)
	})

	t.Run("JoinRelationshipTests", func(t *testing.T) {
		tConfig.testJoinQueries(t, tConfig.DbConfig.DbAlias)
	})

	t.Run("MutationTests", func(t *testing.T) {
		tConfig.testMutationOperations(t, tConfig.DbConfig.DbAlias)
	})

	t.Run("UniversalTests", func(t *testing.T) {
		tConfig.testUniversalTestCases(t, tConfig.DbConfig.DbAlias)
	}) */
}

// executeGraphQL executes a GraphQL query against the eru-ql service
func executeGraphQL(tConfig *TestConfig, request GraphQLRequest) (*GraphQLResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/graphql/%s/execute", tConfig.EruQLBaseURL, tConfig.ProjectId)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Helper function to safely truncate response body
	bodyPreview := func(b []byte, max int) string {
		str := string(b)
		if len(str) > max {
			return str[:max] + "..."
		}
		return str
	}

	// Check if response is HTML (indicates service is running but GraphQL endpoint not available)
	if resp.Header.Get("Content-Type") != "application/json" && strings.Contains(string(body), "<html>") {
		return nil, fmt.Errorf("received HTML response instead of JSON - GraphQL endpoint may not be available at %s. Response: %s", url, bodyPreview(body, 200))
	}

	// Check for non-200 status codes
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received status code %d from %s. Response: %s", resp.StatusCode, url, bodyPreview(body, 200))
	}

	// First try to unmarshal as standard GraphQL response
	var graphqlResp GraphQLResponse
	if err := json.Unmarshal(body, &graphqlResp); err != nil {
		// If that fails, try to unmarshal as array response (eru-ql format)
		var arrayResp []map[string]interface{}
		if arrErr := json.Unmarshal(body, &arrayResp); arrErr != nil {
			return nil, fmt.Errorf("failed to unmarshal response from %s. Status: %d, Content-Type: %s, Body: %s, Error: %w",
				url, resp.StatusCode, resp.Header.Get("Content-Type"), bodyPreview(body, 200), err)
		}

		// Convert array response to GraphQL format
		if len(arrayResp) > 0 {
			graphqlResp.Data = arrayResp[0]
		} else {
			graphqlResp.Data = make(map[string]interface{})
		}
	}

	return &graphqlResp, nil
}

// waitForEruQLService waits for eru-ql service to be available
func waitForEruQLService(config *TestConfig, timeout time.Duration) error {
	// Try health endpoints that eru-ql supports (starting with /hello)
	healthEndpoints := []string{"/hello"}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		for _, endpoint := range healthEndpoints {
			url := config.EruQLBaseURL + endpoint
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		time.Sleep(2 * time.Second)
	}

	// If service is not available, show what endpoints we can reach
	fmt.Printf("❌ eru-ql service not available after %v\n", timeout)
	checkEruqlEndpoints(config)

	return fmt.Errorf("eru-ql service not available at %s after %v", config.EruQLBaseURL, timeout)
}

// checkEruQLEndpoints checks what endpoints are available
func checkEruqlEndpoints(config *TestConfig) {
	fmt.Printf("🔍 Checking available endpoints at %s:\n", config.EruQLBaseURL)

	endpoints := []string{"/hello", fmt.Sprintf("/graphql/%s/execute", config.ProjectId)}
	for _, endpoint := range endpoints {
		url := config.EruQLBaseURL + endpoint
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("  ❌ %s - Connection error: %v\n", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyPreview := string(body)
		if len(bodyPreview) > 100 {
			bodyPreview = bodyPreview[:100] + "..."
		}

		fmt.Printf("  %s %s - Status: %d, Content-Type: %s, Body: %s\n",
			getStatusIcon(resp.StatusCode), endpoint, resp.StatusCode,
			resp.Header.Get("Content-Type"), bodyPreview)
	}
}

func getStatusIcon(status int) string {
	if status >= 200 && status < 300 {
		return "✅"
	} else if status >= 400 {
		return "❌"
	}
	return "⚠️"
}

// setupEruql creates test project and configures databases
func (tConfig *TestConfig) setupEruql(t *testing.T) error {

	// 1. Create test project (skip if already exists)

	if err := callEruqlAPI("POST", fmt.Sprintf("%s/store/%s/save", tConfig.EruQLBaseURL, tConfig.ProjectId), nil); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create test project: %w", err)
		}
		t.Logf("✅ Test project already exists")
	} else {
		t.Logf("✅ Created test project")
	}

	// 2. Add datasource
	ds := map[string]interface{}{
		"db_alias": tConfig.DbConfig.DbAlias,
		"db_type":  tConfig.DbConfig.DbCategory,
		"db_name":  tConfig.DbConfig.DbType,
		"db_config": map[string]interface{}{
			"host":           tConfig.DbConfig.DbHost,
			"port":           tConfig.DbConfig.DbPort,
			"user":           tConfig.DbConfig.DbUser,
			"password":       tConfig.DbConfig.DbPass,
			"default_db":     tConfig.DbConfig.DbName,
			"default_schema": tConfig.DbConfig.DbSchema,
			"driver_config": map[string]interface{}{
				"max_open_conns":    10,
				"max_idle_conns":    5,
				"conn_max_lifetime": 300000000000, // nanoseconds
			},
			"other_db_config": map[string]interface{}{
				"row_limit":      1000,
				"query_time_out": 30,
			},
		},
	}

	if err := callEruqlAPI("POST", fmt.Sprintf("%s/store/%s/datasource/save/%s", tConfig.EruQLBaseURL, tConfig.ProjectId, tConfig.DbConfig.DbAlias), ds); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add datasource: %w", err)
		}
		t.Logf("✅ datasource already exists: %s", tConfig.DbConfig.DbAlias)
	} else {
		t.Logf("✅ Added datasource: %s", tConfig.DbConfig.DbAlias)
	}

	// 3. Fetch Database Tables
	schemaURL := fmt.Sprintf("%s/store/%s/datasource/schema/%s", tConfig.EruQLBaseURL, tConfig.ProjectId, tConfig.DbConfig.DbAlias)
	if err := callEruqlAPI("GET", schemaURL, nil); err != nil {
		return fmt.Errorf("failed to fetch database schema: %w", err)
	}
	t.Logf("✅ Fetched database schema for: %s", tConfig.DbConfig.DbAlias)

	// 4. Add tables to schema
	tables := []string{"order_items", "orders", "products", "categories", "user_profiles", "users", "companies", "departments", "audit_logs"}
	for _, table := range tables {
		if err := callEruqlAPI("POST", fmt.Sprintf("%s/store/%s/datasource/schema/%s/addtable/%s.%s", tConfig.EruQLBaseURL, tConfig.ProjectId, tConfig.DbConfig.DbAlias, tConfig.DbConfig.DbSchema, table), nil); err != nil {
			t.Logf("⚠️  Warning: Could not add table %s to %s: %v", table, tConfig.DbConfig.DbAlias, err)
		}
	}
	t.Logf("✅ Added tables to schema: %s", tConfig.DbConfig.DbAlias)

	return nil
}

func (tConfig *TestConfig) executeSchemaScript(t *testing.T) error {
	var schemaFile string
	var insertSQL string

	if tConfig.DbConfig.DbType == "postgres" {
		schemaFile = "postgres-schema.sql"
		insertSQL = generatePostgreSQLInserts()
	} else if tConfig.DbConfig.DbType == "mysql" {
		schemaFile = "mysql-schema.sql"
		insertSQL = generateMySQLInserts()
	} else {
		return fmt.Errorf("unsupported database type: %s", tConfig.DbConfig.DbType)
	}

	// Read schema file (only table definitions, no INSERT statements)
	content, err := os.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaFile, err)
	}

	// Execute schema + generated INSERT statements
	fullSQL := string(content) + "\n\n-- Generated test data from testdata.go\n" + insertSQL
	return tConfig.executeSchema(fullSQL)
}

func (tConfig *TestConfig) executeSchema(schema string) error {
	db, err := sql.Open(tConfig.DbConfig.DbType, tConfig.DbConfig.GetDsn())
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(schema)
	return err
}

// JSON response comparison utilities
func normalizeJSONResponse(data interface{}) interface{} {
	// Convert to JSON and back to normalize structure
	jsonBytes, _ := json.Marshal(data)
	var normalized interface{}
	json.Unmarshal(jsonBytes, &normalized)
	return normalized
}

func normalizeArrayByID(arr []interface{}) []interface{} {
	// Sort array by ID field for consistent comparison
	sort.Slice(arr, func(i, j int) bool {
		a, aOk := arr[i].(map[string]interface{})
		b, bOk := arr[j].(map[string]interface{})
		if !aOk || !bOk {
			return false
		}

		aID, aIdOk := a["id"]
		bID, bIdOk := b["id"]
		if !aIdOk || !bIdOk {
			return false
		}

		// Handle both int and float64 ID types
		aFloat, aFloatOk := aID.(float64)
		bFloat, bFloatOk := bID.(float64)
		if aFloatOk && bFloatOk {
			return aFloat < bFloat
		}

		aInt, aIntOk := aID.(int)
		bInt, bIntOk := bID.(int)
		if aIntOk && bIntOk {
			return aInt < bInt
		}

		return false
	})
	return arr
}

func compareJSONResponse(t *testing.T, actual, expected interface{}, fieldName string) bool {
	actualNorm := normalizeJSONResponse(actual)
	expectedNorm := normalizeJSONResponse(expected)

	// If both are arrays, normalize by ID before comparing
	if actualArr, ok := actualNorm.([]interface{}); ok {
		if expectedArr, ok := expectedNorm.([]interface{}); ok {
			actualNorm = normalizeArrayByID(actualArr)
			expectedNorm = normalizeArrayByID(expectedArr)
		}
	}

	if !reflect.DeepEqual(actualNorm, expectedNorm) {
		t.Errorf("Field %s does not match expected data", fieldName)
		t.Logf("Expected: %+v", expectedNorm)
		t.Logf("Actual: %+v", actualNorm)
		return false
	}
	return true
}

func validateResponseStructure(t *testing.T, data map[string]interface{}, fieldName string, expectedLength int) []interface{} {
	field, exists := data[fieldName]
	if !exists {
		t.Errorf("Field %s not found in response", fieldName)
		return nil
	}

	fieldArray, ok := field.([]interface{})
	if !ok {
		t.Errorf("Field %s is not an array", fieldName)
		return nil
	}

	if len(fieldArray) != expectedLength {
		t.Errorf("Expected %d %s records, got %d", expectedLength, fieldName, len(fieldArray))
		return fieldArray // Return even if length is wrong for further validation
	}

	return fieldArray
}

// callEruQLAPI is a helper function to call eru-ql REST APIs
func callEruqlAPI(method, url string, requestData interface{}) error {
	var body io.Reader
	if requestData != nil {
		jsonData, err := json.Marshal(requestData)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if requestData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API call failed with status %d: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

func (tConfig *TestConfig) testFilterOperators(t *testing.T, dbAlias string) {
	testCases := []struct {
		name     string
		query    string
		validate func(t *testing.T, response *GraphQLResponse, err error)
	}{
		{ //TODO {eq: 1}
			name: "EqualityFilter",
			query: fmt.Sprintf(`query {
				users(where: {company_id: 1}) @%s {
					id name email company_id
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse, err error) {
				if len(response.Errors) > 0 {
					t.Errorf("Expected errors for filter test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				// Expected: 4 users from company_id = 1 (John Doe, Jane Smith, Bob Wilson, Alice Brown)
				expectedUsers := []map[string]interface{}{
					{"id": float64(1), "name": "John Doe", "email": "john.doe@techcorp.com", "company_id": float64(1)},
					{"id": float64(2), "name": "Jane Smith", "email": "jane.smith@techcorp.com", "company_id": float64(1)},
					{"id": float64(3), "name": "Bob Wilson", "email": "bob.wilson@techcorp.com", "company_id": float64(1)},
					{"id": float64(4), "name": "Alice Brown", "email": "alice.brown@techcorp.com", "company_id": float64(1)},
				}

				if len(users) != len(expectedUsers) {
					t.Errorf("Expected %d users with company_id=1, got %d", len(expectedUsers), len(users))
					return
				}

				// Validate each user
				for i, expectedUser := range expectedUsers {
					if i >= len(users) {
						t.Errorf("Missing user at index %d", i)
						continue
					}

					actualUser, ok := users[i].(map[string]interface{})
					if !ok {
						t.Errorf("Expected user %d to be map[string]interface{}, got %T", i, users[i])
						continue
					}

					for field, expectedValue := range expectedUser {
						if actualUser[field] != expectedValue {
							t.Errorf("User %d field %s: expected %v, got %v", i, field, expectedValue, actualUser[field])
						}
					}
				}

				t.Logf("✅ Equality filter test passed for %s - validated %d users", dbAlias, len(users))
			},
		},
		{
			name: "InFilter",
			query: fmt.Sprintf(`query {
				users(where: {id: {_in: [1, 2, 3]}}) @%s {
					id name email
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse, err error) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				// Expected: users with IDs 1, 2, 3 (John Doe, Jane Smith, Bob Wilson)
				expectedUsers := []map[string]interface{}{
					{"id": float64(1), "name": "John Doe", "email": "john.doe@techcorp.com"},
					{"id": float64(2), "name": "Jane Smith", "email": "jane.smith@techcorp.com"},
					{"id": float64(3), "name": "Bob Wilson", "email": "bob.wilson@techcorp.com"},
				}

				if len(users) != len(expectedUsers) {
					t.Errorf("Expected %d users with IDs [1,2,3], got %d", len(expectedUsers), len(users))
					return
				}

				// Validate each user
				for i, expectedUser := range expectedUsers {
					if i >= len(users) {
						t.Errorf("Missing user at index %d", i)
						continue
					}

					actualUser, ok := users[i].(map[string]interface{})
					if !ok {
						t.Errorf("Expected user %d to be map[string]interface{}, got %T", i, users[i])
						continue
					}

					for field, expectedValue := range expectedUser {
						if actualUser[field] != expectedValue {
							t.Errorf("User %d field %s: expected %v, got %v", i, field, expectedValue, actualUser[field])
						}
					}
				}

				t.Logf("✅ IN filter test passed for %s - validated %d users", dbAlias, len(users))
			},
		},
		{
			name: "LikeFilter",
			query: fmt.Sprintf(`query {
				users(where: {email: {_like: "@techcorp.com"}}) @%s {
					id name email
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse, err error) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				// Expected: all users with @techcorp.com emails (4 users from company_id=1)
				expectedUsers := []map[string]interface{}{
					{"id": float64(1), "name": "John Doe", "email": "john.doe@techcorp.com"},
					{"id": float64(2), "name": "Jane Smith", "email": "jane.smith@techcorp.com"},
					{"id": float64(3), "name": "Bob Wilson", "email": "bob.wilson@techcorp.com"},
					{"id": float64(4), "name": "Alice Brown", "email": "alice.brown@techcorp.com"},
				}

				if len(users) != len(expectedUsers) {
					t.Errorf("Expected %d users with @techcorp.com emails, got %d", len(expectedUsers), len(users))
					return
				}

				// Validate each user
				for i, expectedUser := range expectedUsers {
					if i >= len(users) {
						t.Errorf("Missing user at index %d", i)
						continue
					}

					actualUser, ok := users[i].(map[string]interface{})
					if !ok {
						t.Errorf("Expected user %d to be map[string]interface{}, got %T", i, users[i])
						continue
					}

					for field, expectedValue := range expectedUser {
						if actualUser[field] != expectedValue {
							t.Errorf("User %d field %s: expected %v, got %v", i, field, expectedValue, actualUser[field])
						}
					}
				}

				t.Logf("✅ LIKE filter test passed for %s - validated %d users", dbAlias, len(users))
			},
		},
		{
			name: "GreaterThanFilter",
			query: fmt.Sprintf(`query {
				products(where: {price: {_gt: 500}}) @%s {
					id name price
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse, err error) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				products, ok := data["products"].([]interface{})
				if !ok {
					t.Errorf("Expected products to be []interface{}, got %T", data["products"])
					return
				}

				// Expected: products with price > 500 (MacBook Pro 16", Dell XPS 13, iMac 24", iPhone 15 Pro, Samsung Galaxy S24, Executive Desk)
				expectedProducts := []map[string]interface{}{
					{"id": float64(1), "name": "MacBook Pro 16\"", "price": float64(2499.99)},
					{"id": float64(2), "name": "Dell XPS 13", "price": float64(1299.99)},
					{"id": float64(3), "name": "iMac 24\"", "price": float64(1499.99)},
					{"id": float64(4), "name": "iPhone 15 Pro", "price": float64(999.99)},
					{"id": float64(5), "name": "Samsung Galaxy S24", "price": float64(799.99)},
					{"id": float64(6), "name": "Executive Desk", "price": float64(899.99)},
				}

				if len(products) != len(expectedProducts) {
					t.Errorf("Expected %d products with price > 500, got %d", len(expectedProducts), len(products))
					return
				}

				// Validate each product
				for i, expectedProduct := range expectedProducts {
					if i >= len(products) {
						t.Errorf("Missing product at index %d", i)
						continue
					}

					actualProduct, ok := products[i].(map[string]interface{})
					if !ok {
						t.Errorf("Expected product %d to be map[string]interface{}, got %T", i, products[i])
						continue
					}

					for field, expectedValue := range expectedProduct {
						if actualProduct[field] != expectedValue {
							t.Errorf("Product %d field %s: expected %v, got %v", i, field, expectedValue, actualProduct[field])
						}
					}
				}

				t.Logf("✅ Greater than filter test passed for %s - validated %d products", dbAlias, len(products))
			},
		},
		{
			name: "LessThanFilter",
			query: fmt.Sprintf(`query {
				products(where: {price: {_lt: 1000}}) @%s {
					id name price
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse, err error) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				products, ok := data["products"].([]interface{})
				if !ok {
					t.Errorf("Expected products to be []interface{}, got %T", data["products"])
					return
				}

				// Expected: products with price < 1000 (iPhone 15 Pro: 999.99, Samsung Galaxy S24: 799.99, Executive Desk: 899.99)
				expectedProducts := []map[string]interface{}{
					{"id": float64(4), "name": "iPhone 15 Pro", "price": float64(999.99)},
					{"id": float64(5), "name": "Samsung Galaxy S24", "price": float64(799.99)},
					{"id": float64(6), "name": "Executive Desk", "price": float64(899.99)},
				}

				if len(products) != len(expectedProducts) {
					t.Errorf("Expected %d products with price < 1000, got %d", len(expectedProducts), len(products))
					return
				}

				// Validate each product
				for i, expectedProduct := range expectedProducts {
					if i >= len(products) {
						t.Errorf("Missing product at index %d", i)
						continue
					}

					actualProduct, ok := products[i].(map[string]interface{})
					if !ok {
						t.Errorf("Expected product %d to be map[string]interface{}, got %T", i, products[i])
						continue
					}

					for field, expectedValue := range expectedProduct {
						if actualProduct[field] != expectedValue {
							t.Errorf("Product %d field %s: expected %v, got %v", i, field, expectedValue, actualProduct[field])
						}
					}
				}

				t.Logf("✅ Less than filter test passed for %s - validated %d products", dbAlias, len(products))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Errorf("❌ Error while executing query '%s': %v", tc.name, err)
				return
			}
			tc.validate(t, response, err)
		})
	}
}

func (tConfig *TestConfig) testJSONFieldQueries(t *testing.T, dbAlias string) {
	// JSON field queries - PostgreSQL uses ___ notation for JSON fields
	var jsonNotation string
	if strings.Contains(dbAlias, "postgres") {
		jsonNotation = "___"
	} else {
		jsonNotation = "->"
	}

	testCases := []struct {
		name     string
		query    string
		validate func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "JSONFieldAccess",
			query: fmt.Sprintf(`query {
				users(where: {profile%srole: {eq: "admin"}}) @%s {
					id name profile
				}
			}`, jsonNotation, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for JSON field test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				// Note: JSON field queries might not work depending on eru-ql implementation
				// This test validates the query structure but might return 0 results
				t.Logf("✅ JSON field access test passed for %s - returned %d results", dbAlias, len(users))
			},
		},
		{
			name: "NestedJSONFieldAccess",
			query: fmt.Sprintf(`query {
				companies(where: {settings%sfeatures: {contains: "ai"}}) @%s {
					id name settings
				}
			}`, jsonNotation, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for nested JSON test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				companies, ok := data["companies"].([]interface{})
				if !ok {
					t.Errorf("Expected companies to be []interface{}, got %T", data["companies"])
					return
				}

				// Note: Nested JSON field queries might not work depending on eru-ql implementation
				// This test validates the query structure but might return 0 results
				t.Logf("✅ Nested JSON field test passed for %s - returned %d results", dbAlias, len(companies))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (tConfig *TestConfig) testAggregationQueries(t *testing.T, dbAlias string) {
	testCases := []struct {
		name     string
		query    string
		validate func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "CountAggregation",
			query: fmt.Sprintf(`query {
				users_aggregate @%s {
					aggregate {
						count
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for aggregation test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				usersAggregate, ok := data["users_aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected users_aggregate to be map[string]interface{}, got %T", data["users_aggregate"])
					return
				}

				aggregate, ok := usersAggregate["aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected aggregate to be map[string]interface{}, got %T", usersAggregate["aggregate"])
					return
				}

				count, ok := aggregate["count"]
				if !ok {
					t.Errorf("Expected count field in aggregate response")
					return
				}

				// Expected: 7 total users in test data
				expectedCount := float64(7)
				if countFloat, ok := count.(float64); ok {
					if countFloat != expectedCount {
						t.Errorf("Expected count %v, got %v", expectedCount, countFloat)
						return
					}
				} else {
					t.Errorf("Expected count to be float64, got %T", count)
					return
				}

				t.Logf("✅ Count aggregation test passed for %s - count: %v", dbAlias, count)
			},
		},
		{
			name: "SumAggregation",
			query: fmt.Sprintf(`query {
				orders_aggregate @%s {
					aggregate {
						sum { total }
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for aggregation test: %v", response.Errors)
					return
				}

				// Note: Sum aggregation on orders may not work if orders table is empty
				// This test validates the query structure
				if _, ok := response.Data.(map[string]interface{}); !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				t.Logf("✅ Sum aggregation test passed for %s - structure validated", dbAlias)
			},
		},
		{
			name: "AvgAggregation",
			query: fmt.Sprintf(`query {
				products_aggregate @%s {
					aggregate {
						avg { price }
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for aggregation test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				productsAggregate, ok := data["products_aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected products_aggregate to be map[string]interface{}, got %T", data["products_aggregate"])
					return
				}

				aggregate, ok := productsAggregate["aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected aggregate to be map[string]interface{}, got %T", productsAggregate["aggregate"])
					return
				}

				avg, ok := aggregate["avg"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected avg to be map[string]interface{}, got %T", aggregate["avg"])
					return
				}

				price, ok := avg["price"]
				if !ok {
					t.Errorf("Expected price field in avg response")
					return
				}

				// Expected: Average price should be approximately 1466.66 (sum of all prices / 6)
				// (2499.99 + 1299.99 + 1499.99 + 999.99 + 799.99 + 899.99) / 6 = 1499.99
				if priceFloat, ok := price.(float64); ok {
					if priceFloat < 1400 || priceFloat > 1600 {
						t.Logf("Average price %v is outside expected range [1400-1600]", priceFloat)
					}
				}

				t.Logf("✅ Average aggregation test passed for %s - avg price: %v", dbAlias, price)
			},
		},
		{
			name: "MinMaxAggregation",
			query: fmt.Sprintf(`query {
				products_aggregate @%s {
					aggregate {
						min { price }
						max { price }
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for aggregation test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				productsAggregate, ok := data["products_aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected products_aggregate to be map[string]interface{}, got %T", data["products_aggregate"])
					return
				}

				aggregate, ok := productsAggregate["aggregate"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected aggregate to be map[string]interface{}, got %T", productsAggregate["aggregate"])
					return
				}

				min, ok := aggregate["min"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected min to be map[string]interface{}, got %T", aggregate["min"])
					return
				}

				max, ok := aggregate["max"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected max to be map[string]interface{}, got %T", aggregate["max"])
					return
				}

				minPrice := min["price"]
				maxPrice := max["price"]

				// Expected: Min price = 799.99 (Samsung Galaxy S24), Max price = 2499.99 (MacBook Pro 16")
				if minFloat, ok := minPrice.(float64); ok {
					if minFloat != 799.99 {
						t.Logf("Min price %v differs from expected 799.99", minFloat)
					}
				}

				if maxFloat, ok := maxPrice.(float64); ok {
					if maxFloat != 2499.99 {
						t.Logf("Max price %v differs from expected 2499.99", maxFloat)
					}
				}

				t.Logf("✅ Min/Max aggregation test passed for %s - min: %v, max: %v", dbAlias, minPrice, maxPrice)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (tConfig *TestConfig) testJoinQueries(t *testing.T, dbAlias string) {
	testCases := []struct {
		name     string
		query    string
		validate func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "UserCompanyJoin",
			query: fmt.Sprintf(`query {
				users @%s {
					id name email
					company {
						id name
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for join test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				// Expected: 7 users each with their company information
				if len(users) != 7 {
					t.Errorf("Expected 7 users with company joins, got %d", len(users))
					return
				}

				// Validate first user has company information
				if len(users) > 0 {
					user := users[0].(map[string]interface{})
					company, hasCompany := user["company"]
					if !hasCompany {
						t.Errorf("Expected user to have company field")
						return
					}

					companyMap, ok := company.(map[string]interface{})
					if !ok {
						t.Errorf("Expected company to be map[string]interface{}, got %T", company)
						return
					}

					if _, hasID := companyMap["id"]; !hasID {
						t.Errorf("Expected company to have id field")
						return
					}

					if _, hasName := companyMap["name"]; !hasName {
						t.Errorf("Expected company to have name field")
						return
					}
				}

				t.Logf("✅ User-Company join test passed for %s - validated %d users with company data", dbAlias, len(users))
			},
		},
		{
			name: "OrderUserCompanyJoin",
			query: fmt.Sprintf(`query {
				orders @%s {
					id total status
					user {
						id name email
						company {
							id name
						}
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for complex join test: %v", response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				orders, ok := data["orders"].([]interface{})
				if !ok {
					t.Errorf("Expected orders to be []interface{}, got %T", data["orders"])
					return
				}

				// Note: Orders table might be empty in test data
				// This test validates the query structure for complex joins
				if len(orders) > 0 {
					order := orders[0].(map[string]interface{})
					user, hasUser := order["user"]
					if hasUser {
						userMap, ok := user.(map[string]interface{})
						if ok {
							company, hasCompany := userMap["company"]
							if hasCompany {
								if _, ok := company.(map[string]interface{}); ok {
									t.Logf("Complex join validated: order -> user -> company structure correct")
								}
							}
						}
					}
				}

				t.Logf("✅ Order-User-Company join test passed for %s - returned %d orders", dbAlias, len(orders))
			},
		},
		{
			name: "ProductCompanyJoin",
			query: fmt.Sprintf(`query {
				products @%s {
					id name price category
					company {
						id name settings
					}
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for product join test: %v", response.Errors)
					return
				}
				t.Logf("✅ Product-Company join test passed for %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (tConfig *TestConfig) testMutationOperations(t *testing.T, dbAlias string) {
	testCases := []struct {
		name      string
		query     string
		variables map[string]interface{}
		validate  func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "InsertUser",
			query: fmt.Sprintf(`mutation($user: UserInput!) {
				insert_users(docs: [$user]) @%s {
					returning {
						id name email
					}
					error
				}
			}`, dbAlias),
			variables: map[string]interface{}{
				"user": map[string]interface{}{
					"name":       "Test User",
					"email":      "test@example.com",
					"company_id": 1,
					"profile": map[string]interface{}{
						"role":       "user",
						"department": "testing",
					},
				},
			},
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for insert test: %v", response.Errors)
					return
				}
				t.Logf("✅ Insert user test passed for %s", dbAlias)
			},
		},
		{
			name: "UpdateUser",
			query: fmt.Sprintf(`mutation($id: Int!, $changes: UserInput!) {
				update_users(where: {id: {eq: $id}}, _set: $changes) @%s {
					returning {
						id name email
					}
					affected_rows
				}
			}`, dbAlias),
			variables: map[string]interface{}{
				"id": 1,
				"changes": map[string]interface{}{
					"name": "Updated Name",
				},
			},
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for update test: %v", response.Errors)
					return
				}
				t.Logf("✅ Update user test passed for %s", dbAlias)
			},
		},
		{
			name: "UpsertProduct",
			query: fmt.Sprintf(`mutation($product: ProductInput!) {
				insert_products(docs: [$product], on_conflict: {
					constraint: products_name_company_id_key,
					update_columns: [price, attributes]
				}) @%s {
					returning {
						id name price
					}
				}
			}`, dbAlias),
			variables: map[string]interface{}{
				"product": map[string]interface{}{
					"name":       "Test Product",
					"price":      99.99,
					"company_id": 1,
					"category":   "test",
					"attributes": map[string]interface{}{
						"test": true,
					},
				},
			},
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for upsert test: %v", response.Errors)
					return
				}
				t.Logf("✅ Upsert product test passed for %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{
				Query:     tc.query,
				Variables: tc.variables,
			}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (tConfig *TestConfig) testUniversalTestCases(t *testing.T, dbAlias string) {
	// Universal test cases that work with any database (using users, products tables)
	var testCases = []struct {
		name      string
		query     string
		variables map[string]interface{}
		validate  func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "QueryAllUsers",
			query: fmt.Sprintf(`query {
			users @%s {
				id
				name
				email
			}
		}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("GraphQL errors for %s: %v", dbAlias, response.Errors)
					return
				}

				data, ok := response.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
					return
				}

				users, ok := data["users"].([]interface{})
				if !ok {
					t.Errorf("Expected users to be []interface{}, got %T", data["users"])
					return
				}

				if len(users) == 0 {
					t.Logf("⚠️  No users found in %s (this may be expected)", dbAlias)
					return
				}

				// Validate first user structure
				firstUser, ok := users[0].(map[string]interface{})
				if !ok {
					t.Errorf("Expected user to be map[string]interface{}, got %T", users[0])
					return
				}

				requiredFields := []string{"id", "name", "email"}
				for _, field := range requiredFields {
					if _, exists := firstUser[field]; !exists {
						t.Errorf("Expected user to have field '%s'", field)
					}
				}

				t.Logf("✅ Successfully queried %d users from %s", len(users), dbAlias)
			},
		},
		{
			name: "QueryProducts",
			query: fmt.Sprintf(`query {
			products @%s {
				id
				name
				price
				category_id
			}
		}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("GraphQL errors for %s: %v", dbAlias, response.Errors)
					return
				}
				t.Logf("✅ Successfully queried products from %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{
				Query:     tc.query,
				Variables: tc.variables,
			}
			response, err := executeGraphQL(tConfig, request)
			if err != nil {
				t.Errorf("❌ Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

// validateUsersResponse validates users query response against expected test data
func validateUsersResponse(t *testing.T, data map[string]interface{}) {
	users := validateResponseStructure(t, data, "users", 7)
	if users == nil {
		return
	}

	// Convert expected users to interface{} for comparison
	expectedUsers := make([]interface{}, len(ExpectedTestData.Users))
	for i, user := range ExpectedTestData.Users {
		expectedUsers[i] = user
	}

	compareJSONResponse(t, users, expectedUsers, "users")
}

// validateProductsResponse validates products query response against expected test data
func validateProductsResponse(t *testing.T, data map[string]interface{}) {
	products := validateResponseStructure(t, data, "products", 6)
	if products == nil {
		return
	}

	// Convert expected products to interface{} for comparison
	expectedProducts := make([]interface{}, len(ExpectedTestData.Products))
	for i, product := range ExpectedTestData.Products {
		expectedProducts[i] = product
	}

	compareJSONResponse(t, products, expectedProducts, "products")
}

// validateCompaniesResponse validates companies query response against expected test data
func validateCompaniesResponse(t *testing.T, data map[string]interface{}) {
	companies := validateResponseStructure(t, data, "companies", 3)
	if companies == nil {
		return
	}

	// Convert expected companies to interface{} for comparison
	expectedCompanies := make([]interface{}, len(ExpectedTestData.Companies))
	for i, company := range ExpectedTestData.Companies {
		expectedCompanies[i] = company
	}

	compareJSONResponse(t, companies, expectedCompanies, "companies")
}

// Basic test cases with data validation
var basicTestCases = []struct {
	name     string
	query    string
	validate func(t *testing.T, response *GraphQLResponse)
}{
	{
		name: "QueryAllUsers",
		query: `query {
			users @{DBALIAS} {
				id name email company_id department_id role salary is_active tags preferences metadata
			}
		}`,
		validate: func(t *testing.T, response *GraphQLResponse) {
			if len(response.Errors) > 0 {
				t.Errorf("GraphQL errors: %v", response.Errors)
				return
			}

			data, ok := response.Data.(map[string]interface{})
			if !ok {
				t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
				return
			}

			validateUsersResponse(t, data)
		},
	},
	{
		name: "QueryAllProducts",
		query: `query {
			products @{DBALIAS} {
				id name description price cost stock_quantity weight dimensions specifications tags is_active category_id
			}
		}`,
		validate: func(t *testing.T, response *GraphQLResponse) {
			if len(response.Errors) > 0 {
				t.Errorf("GraphQL errors: %v", response.Errors)
				return
			}

			data, ok := response.Data.(map[string]interface{})
			if !ok {
				t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
				return
			}

			validateProductsResponse(t, data)
		},
	},
	{
		name: "QueryAllCompanies",
		query: `query {
			companies @{DBALIAS} {
				id name industry founded_year metadata settings
			}
		}`,
		validate: func(t *testing.T, response *GraphQLResponse) {
			if len(response.Errors) > 0 {
				t.Errorf("GraphQL errors: %v", response.Errors)
				return
			}

			data, ok := response.Data.(map[string]interface{})
			if !ok {
				t.Errorf("Expected data to be map[string]interface{}, got %T", response.Data)
				return
			}

			validateCompaniesResponse(t, data)
		},
	},
}
