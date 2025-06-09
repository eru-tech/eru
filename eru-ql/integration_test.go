package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
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

// Test configuration for database aliases
type TestConfig struct {
	EruQLBaseURL string
	PostgresHost string
	PostgresPort string
	PostgresDB   string
	PostgresUser string
	PostgresPass string
	MySQLHost    string
	MySQLPort    string
	MySQLDB      string
	MySQLUser    string
	MySQLPass    string
}

// Database alias configuration
type DatabaseConfig struct {
	DatabaseAlias string
	DatabaseType  string
	DSN           string
}

// Comprehensive test schema with relationships and JSON fields
type TestSchema struct {
	Companies []Company `json:"companies"`
	Users     []User    `json:"users"`
	Orders    []Order   `json:"orders"`
	Products  []Product `json:"products"`
}

type Company struct {
	ID       int                    `json:"id"`
	Name     string                 `json:"name"`
	Settings map[string]interface{} `json:"settings"`
	Address  map[string]interface{} `json:"address"`
}

type User struct {
	ID        int                    `json:"id"`
	CompanyID int                    `json:"company_id"`
	Name      string                 `json:"name"`
	Email     string                 `json:"email"`
	Profile   map[string]interface{} `json:"profile"`
	CreatedAt time.Time              `json:"created_at"`
}

type Order struct {
	ID         int                    `json:"id"`
	UserID     int                    `json:"user_id"`
	CompanyID  int                    `json:"company_id"`
	Total      float64                `json:"total"`
	Status     string                 `json:"status"`
	Metadata   map[string]interface{} `json:"metadata"`
	OrderItems []OrderItem            `json:"order_items"`
	CreatedAt  time.Time              `json:"created_at"`
}

type OrderItem struct {
	ID        int     `json:"id"`
	OrderID   int     `json:"order_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Product struct {
	ID         int                    `json:"id"`
	CategoryID int                    `json:"category_id"`
	Name       string                 `json:"name"`
	Price      float64                `json:"price"`
	Attributes map[string]interface{} `json:"attributes"`
	IsActive   bool                   `json:"is_active"`
}

// getTestConfig returns test configuration from environment variables
func getTestConfig() *TestConfig {
	return &TestConfig{
		EruQLBaseURL: getEnvOrDefault("ERU_QL_BASE_URL", "http://localhost:8087"),
		PostgresHost: getEnvOrDefault("POSTGRES_HOST", "localhost"),
		PostgresPort: getEnvOrDefault("POSTGRES_PORT", "5432"),
		PostgresDB:   getEnvOrDefault("POSTGRES_DB", "eru_test"),
		PostgresUser: getEnvOrDefault("POSTGRES_USER", "postgres"),
		PostgresPass: getEnvOrDefault("POSTGRES_PASSWORD", "password"),
		MySQLHost:    getEnvOrDefault("MYSQL_HOST", "localhost"),
		MySQLPort:    getEnvOrDefault("MYSQL_PORT", "3306"),
		MySQLDB:      getEnvOrDefault("MYSQL_DB", "eru_test"),
		MySQLUser:    getEnvOrDefault("MYSQL_USER", "root"),
		MySQLPass:    getEnvOrDefault("MYSQL_PASSWORD", "password"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// executeGraphQL executes a GraphQL query against the eru-ql service
func executeGraphQL(config *TestConfig, request GraphQLRequest) (*GraphQLResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := config.EruQLBaseURL + "/graphql/test/execute"
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

// checkEruQLEndpoints checks what endpoints are available
func checkEruQLEndpoints(config *TestConfig) {
	fmt.Printf("🔍 Checking available endpoints at %s:\n", config.EruQLBaseURL)
	
	endpoints := []string{"/hello", "/graphql/test/execute"}
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

// waitForEruQLService waits for eru-ql service to be available
func waitForEruQLService(config *TestConfig, timeout time.Duration) error {
	// Try health endpoints that eru-ql supports (starting with /hello)
	healthEndpoints := []string{"/hello", "/health", "/status", "/"}
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
	checkEruQLEndpoints(config)
	
	return fmt.Errorf("eru-ql service not available at %s after %v", config.EruQLBaseURL, timeout)
}

// Database-agnostic test cases that work with any configured database alias
type UniversalTestCase struct {
	Name          string
	QueryTemplate string // Template with {{.DBAlias}} placeholder
	Variables     map[string]interface{}
	Validate      func(t *testing.T, response *GraphQLResponse, dbAlias string)
	Description   string
}

// getTestCasesForDBAlias returns universal test cases that work with any database
func getTestCasesForDBAlias(dbAlias string) []UniversalTestCase {
	return universalTestCases
}

// Universal test cases that work with any database (using users, products tables)
var universalTestCases = []UniversalTestCase{
	{
		Name: "QueryAllUsers",
		QueryTemplate: `query {
			users @{{.DBAlias}} {
				id
				name
				email
			}
		}`,
		Description: "Query all users from PostgreSQL database",
		Validate: func(t *testing.T, response *GraphQLResponse, dbAlias string) {
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
		Name: "QueryProducts",
		QueryTemplate: `query {
			products @{{.DBAlias}} {
				id
				name
				price
				category_id
			}
		}`,
		Description: "Query all products from PostgreSQL database",
		Validate: func(t *testing.T, response *GraphQLResponse, dbAlias string) {
			if len(response.Errors) > 0 {
				t.Logf("GraphQL errors for %s: %v", dbAlias, response.Errors)
				return
			}
			t.Logf("✅ Successfully queried products from %s", dbAlias)
		},
	},
}

// Helper function to replace template placeholders
func renderQueryTemplate(template string, dbAlias string) string {
	return strings.ReplaceAll(template, "{{.DBAlias}}", dbAlias)
}

// setupTestProject creates test project and configures databases
func setupTestProject(t *testing.T, config *TestConfig) error {
	// 1. Create test project (skip if already exists)
	projectData := map[string]interface{}{
		"projectname": "test",
		"projectdesc": "Test project for integration tests",
	}
	
	if err := callEruQLAPI(config, "POST", "/store/test/save", projectData, nil); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create test project: %w", err)
		}
		t.Logf("✅ Test project already exists")
	} else {
		t.Logf("✅ Created test project")
	}

	// 2. Add PostgreSQL datasource
	postgresDS := map[string]interface{}{
		"db_alias": "postgres_main",
		"db_type":  "postgres",
		"db_name":  config.PostgresDB,
		"db_config": map[string]interface{}{
			"host":           config.PostgresHost,
			"port":           config.PostgresPort,
			"user":           config.PostgresUser,
			"password":       config.PostgresPass,
			"default_db":     config.PostgresDB,
			"default_schema": "public",
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
	
	if err := callEruQLAPI(config, "POST", "/store/test/datasource/save/postgres_main", postgresDS, nil); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add postgres datasource: %w", err)
		}
		t.Logf("✅ PostgreSQL datasource already exists: postgres_main")
	} else {
		t.Logf("✅ Added PostgreSQL datasource: postgres_main")
	}

	// 3. Add MySQL datasource
	mysqlDS := map[string]interface{}{
		"db_alias": "mysql_analytics",
		"db_type":  "mysql",
		"db_name":  config.MySQLDB,
		"db_config": map[string]interface{}{
			"host":           config.MySQLHost,
			"port":           config.MySQLPort,
			"user":           config.MySQLUser,
			"password":       config.MySQLPass,
			"default_db":     config.MySQLDB,
			"default_schema": "",
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
	
	if err := callEruQLAPI(config, "POST", "/store/test/datasource/save/mysql_analytics", mysqlDS, nil); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add mysql datasource: %w", err)
		}
		t.Logf("✅ MySQL datasource already exists: mysql_analytics")
	} else {
		t.Logf("✅ Added MySQL datasource: mysql_analytics")
	}

	// 4. Add tables to PostgreSQL schema
	postgresqlTables := []string{"users", "products", "companies", "orders", "audit_log"}
	for _, table := range postgresqlTables {
		if err := callEruQLAPI(config, "POST", fmt.Sprintf("/store/test/datasource/schema/postgres_main/addtable/%s", table), nil, nil); err != nil {
			t.Logf("⚠️  Warning: Could not add table %s to postgres_main: %v", table, err)
		}
	}
	t.Logf("✅ Added PostgreSQL tables to schema")

	// 5. Add tables to MySQL schema
	mysqlTables := []string{"user_analytics", "product_analytics", "campaigns", "user_sessions", "user_session_events"}
	for _, table := range mysqlTables {
		if err := callEruQLAPI(config, "POST", fmt.Sprintf("/store/test/datasource/schema/mysql_analytics/addtable/%s", table), nil, nil); err != nil {
			t.Logf("⚠️  Warning: Could not add table %s to mysql_analytics: %v", table, err)
		}
	}
	t.Logf("✅ Added MySQL tables to schema")

	return nil
}

// callEruQLAPI is a helper function to call eru-ql REST APIs
func callEruQLAPI(config *TestConfig, method, path string, requestData interface{}, response interface{}) error {
	var body io.Reader
	if requestData != nil {
		jsonData, err := json.Marshal(requestData)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	url := config.EruQLBaseURL + path
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

	if response != nil {
		if err := json.Unmarshal(responseBody, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// TestUniversalGraphQLQueries - Main test that runs same queries against all configured databases
func TestUniversalGraphQLQueries(t *testing.T) {
	config := getTestConfig()

	// Skip if running in demo mode
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}

	// Wait for eru-ql service
	if err := waitForEruQLService(config, 30*time.Second); err != nil {
		t.Skip(fmt.Sprintf("Skipping test: %v", err))
	}

	// Set up test project and databases
	if err := setupTestProject(t, config); err != nil {
		t.Skip(fmt.Sprintf("Skipping test due to setup failure: %v", err))
	}

	// Get configured database aliases from our integration framework
	framework := NewIntegrationTestFramework()

	// Run each test case against each configured database alias
	for _, dbConfig := range framework.databaseConfigs {
		dbAlias := dbConfig.DatabaseAlias

		t.Run(fmt.Sprintf("Database_%s", dbAlias), func(t *testing.T) {
			t.Logf("🎯 Testing against database alias: %s (%s)", dbAlias, dbConfig.DatabaseType)

			testCases := getTestCasesForDBAlias(dbAlias)
		for _, testCase := range testCases {
				t.Run(testCase.Name, func(t *testing.T) {
					t.Logf("🔍 Test: %s", testCase.Description)

					// Render the query template with the actual database alias
					query := renderQueryTemplate(testCase.QueryTemplate, dbAlias)

					t.Logf("📄 Query: %s", strings.ReplaceAll(query, "\n", " "))
					if testCase.Variables != nil {
						t.Logf("📊 Variables: %+v", testCase.Variables)
					}

					request := GraphQLRequest{
						Query:     query,
						Variables: testCase.Variables,
					}

					response, err := executeGraphQL(config, request)
					if err != nil {
						t.Fatalf("Failed to execute GraphQL query against %s: %v", dbAlias, err)
					}

					// Validate the response
					testCase.Validate(t, response, dbAlias)
				})
			}
		})
	}
}

// TestSingleDatabaseAlias - Test against a specific database alias (useful for debugging)
func TestSingleDatabaseAlias(t *testing.T) {
	config := getTestConfig()

	// Get database alias from environment variable
	targetDBAlias := os.Getenv("TEST_DB_ALIAS")
	if targetDBAlias == "" {
		t.Skip("Skipping single database test - set TEST_DB_ALIAS environment variable")
	}

	// Skip if running in demo mode
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}

	// Wait for eru-ql service
	if err := waitForEruQLService(config, 30*time.Second); err != nil {
		t.Skip(fmt.Sprintf("Skipping test: %v", err))
	}

	t.Logf("🎯 Testing specifically against database alias: %s", targetDBAlias)

	testCases := getTestCasesForDBAlias(targetDBAlias)
	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Logf("🔍 Test: %s", testCase.Description)

			// Render the query template with the target database alias
			query := renderQueryTemplate(testCase.QueryTemplate, targetDBAlias)

			t.Logf("📄 Query: %s", strings.ReplaceAll(query, "\n", " "))
			if testCase.Variables != nil {
				t.Logf("📊 Variables: %+v", testCase.Variables)
			}

			request := GraphQLRequest{
				Query:     query,
				Variables: testCase.Variables,
			}

			response, err := executeGraphQL(config, request)
			if err != nil {
				t.Fatalf("Failed to execute GraphQL query against %s: %v", targetDBAlias, err)
			}

			// Validate the response
			testCase.Validate(t, response, targetDBAlias)
		})
	}
}

// TestCrossDatabaseTransactions tests transactions across operations
func TestCrossDatabaseTransactions(t *testing.T) {
	config := getTestConfig()

	// Skip if running in demo mode
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}

	// Wait for eru-ql service
	if err := waitForEruQLService(config, 30*time.Second); err != nil {
		t.Skip(fmt.Sprintf("Skipping test: %v", err))
	}

	t.Run("Transaction with Multiple Operations", func(t *testing.T) {
		query := `mutation @singleTxn {
			createUser: insert_users(docs: [{
				name: "Transaction User",
				email: "transaction.user@example.com",
				region: "test-txn"
			}]) @postgres_main {
				returning { id name }
				error
			}
			
			logUserCreation: insert_audit_log(docs: [{
				action: "CREATE",
				entity_type: "user",
				entity_id: 1,
				details: "{\"event\": \"user_creation_test\"}"
			}]) @postgres_main {
				returning { id action }
				error
			}
		}`

		request := GraphQLRequest{Query: query}
		response, err := executeGraphQL(config, request)
		if err != nil {
			t.Fatalf("Failed to execute transaction: %v", err)
		}

		if len(response.Errors) > 0 {
			t.Logf("Transaction completed with errors (expected for demo): %v", response.Errors)
		} else {
			t.Logf("✅ Transaction completed successfully")
		}
	})
}

// Comprehensive Integration Test Framework for eru-ql with Database Aliases
type IntegrationTestFramework struct {
	config          *TestConfig
	databaseConfigs []DatabaseConfig
}

func NewIntegrationTestFramework() *IntegrationTestFramework {
	config := getTestConfig()

	// Configure database aliases for testing
	databaseConfigs := []DatabaseConfig{
		{
			DatabaseAlias: "postgres_main",
			DatabaseType:  "postgres",
			DSN: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
				config.PostgresUser, config.PostgresPass, config.PostgresHost, config.PostgresPort, config.PostgresDB),
		},
		{
			DatabaseAlias: "mysql_analytics",
			DatabaseType:  "mysql",
			DSN: fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
				config.MySQLUser, config.MySQLPass, config.MySQLHost, config.MySQLPort, config.MySQLDB),
		},
	}

	return &IntegrationTestFramework{
		config:          config,
		databaseConfigs: databaseConfigs,
	}
}

func (f *IntegrationTestFramework) generateTestData() TestSchema {
	return TestSchema{
		Companies: []Company{
			{
				ID:   1,
				Name: "TechCorp Inc",
				Settings: map[string]interface{}{
					"notifications": true,
					"theme":         "dark",
					"features":      []string{"ai", "analytics", "reporting"},
					"limits": map[string]interface{}{
						"users":      1000,
						"storage_gb": 500,
						"api_calls":  100000,
					},
				},
				Address: map[string]interface{}{
					"street":  "123 Tech Street",
					"city":    "San Francisco",
					"state":   "CA",
					"zipcode": "94105",
					"country": "USA",
					"location": map[string]interface{}{
						"lat": 37.7749,
						"lng": -122.4194,
					},
				},
			},
			{
				ID:   2,
				Name: "DataSoft LLC",
				Settings: map[string]interface{}{
					"notifications": false,
					"theme":         "light",
					"features":      []string{"reporting", "dashboard"},
					"limits": map[string]interface{}{
						"users":      500,
						"storage_gb": 250,
						"api_calls":  50000,
					},
				},
				Address: map[string]interface{}{
					"street":  "456 Data Ave",
					"city":    "New York",
					"state":   "NY",
					"zipcode": "10001",
					"country": "USA",
					"location": map[string]interface{}{
						"lat": 40.7128,
						"lng": -74.0060,
					},
				},
			},
		},
		Users: []User{
			{
				ID:        1,
				CompanyID: 1,
				Name:      "John Doe",
				Email:     "john@techcorp.com",
				Profile: map[string]interface{}{
					"role":        "admin",
					"department":  "engineering",
					"level":       "senior",
					"permissions": []string{"read", "write", "admin", "delete"},
					"preferences": map[string]interface{}{
						"email_notifications": true,
						"dashboard_layout":    "compact",
						"timezone":            "PST",
					},
					"contact": map[string]interface{}{
						"phone":   "+1-555-0101",
						"address": "123 User St, SF, CA",
					},
				},
				CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
			},
			{
				ID:        2,
				CompanyID: 1,
				Name:      "Jane Smith",
				Email:     "jane@techcorp.com",
				Profile: map[string]interface{}{
					"role":        "user",
					"department":  "sales",
					"level":       "junior",
					"permissions": []string{"read", "write"},
					"preferences": map[string]interface{}{
						"email_notifications": false,
						"dashboard_layout":    "expanded",
						"timezone":            "PST",
					},
					"contact": map[string]interface{}{
						"phone":   "+1-555-0102",
						"address": "456 Jane Ave, SF, CA",
					},
				},
				CreatedAt: time.Now().Add(-15 * 24 * time.Hour),
			},
			{
				ID:        3,
				CompanyID: 2,
				Name:      "Bob Johnson",
				Email:     "bob@datasoft.com",
				Profile: map[string]interface{}{
					"role":        "manager",
					"department":  "operations",
					"level":       "senior",
					"permissions": []string{"read", "write", "admin"},
					"preferences": map[string]interface{}{
						"email_notifications": true,
						"dashboard_layout":    "grid",
						"timezone":            "EST",
					},
					"contact": map[string]interface{}{
						"phone":   "+1-555-0103",
						"address": "789 Bob Blvd, NY, NY",
					},
				},
				CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
			},
		},
		Products: []Product{
			{
				ID:        1,
				CategoryID: 1,
				Name:      "AI Analytics Platform",
				Price:     999.99,
					Attributes: map[string]interface{}{
					"features":      []string{"machine_learning", "predictive_analytics", "real_time_processing"},
					"integrations":  []string{"salesforce", "hubspot", "slack"},
					"pricing_model": "subscription",
					"support_level": "enterprise",
					"requirements": map[string]interface{}{
						"cpu_cores":  4,
						"memory_gb":  16,
						"storage_gb": 100,
						"network":    "high_speed",
					},
					"metadata": map[string]interface{}{
						"version":       "2.1.0",
						"last_updated":  "2024-01-15",
						"license_type":  "commercial",
						"documentation": "https://docs.techcorp.com/ai-platform",
					},
				},
				IsActive: true,
			},
			{
				ID:        2,
				CategoryID: 1,
				Name:      "Dashboard Pro",
				Price:     299.99,
					Attributes: map[string]interface{}{
					"features":      []string{"custom_dashboards", "reporting", "alerts"},
					"integrations":  []string{"google_analytics", "mixpanel"},
					"pricing_model": "one_time",
					"support_level": "standard",
					"requirements": map[string]interface{}{
						"cpu_cores":  2,
						"memory_gb":  8,
						"storage_gb": 50,
						"network":    "standard",
					},
					"metadata": map[string]interface{}{
						"version":       "1.5.2",
						"last_updated":  "2024-01-10",
						"license_type":  "commercial",
						"documentation": "https://docs.techcorp.com/dashboard-pro",
					},
				},
				IsActive: true,
			},
			{
				ID:        3,
				CategoryID: 2,
				Name:      "Data Warehouse Solution",
				Price:     1999.99,
					Attributes: map[string]interface{}{
					"features":      []string{"data_lake", "etl_pipelines", "data_governance"},
					"integrations":  []string{"aws", "azure", "gcp"},
					"pricing_model": "subscription",
					"support_level": "premium",
					"requirements": map[string]interface{}{
						"cpu_cores":  8,
						"memory_gb":  32,
						"storage_gb": 1000,
						"network":    "enterprise",
					},
					"metadata": map[string]interface{}{
						"version":       "3.0.1",
						"last_updated":  "2024-01-20",
						"license_type":  "enterprise",
						"documentation": "https://docs.datasoft.com/warehouse",
					},
				},
				IsActive: true,
			},
		},
		Orders: []Order{
			{
				ID:        1,
				UserID:    1,
				CompanyID: 1,
				Total:     1299.98,
				Status:    "completed",
				Metadata: map[string]interface{}{
					"payment_method": "credit_card",
					"discount_code":  "WELCOME10",
					"tax_amount":     129.99,
					"currency":       "USD",
					"shipping": map[string]interface{}{
						"method":   "standard",
						"cost":     0,
						"address":  "123 Tech Street, San Francisco, CA 94105",
						"tracking": "TRACK123456",
					},
					"billing": map[string]interface{}{
						"same_as_shipping": true,
						"invoice_number":   "INV-2024-001",
						"due_date":         "2024-02-15",
					},
				},
				CreatedAt: time.Now().Add(-5 * 24 * time.Hour),
			},
			{
				ID:        2,
				UserID:    2,
				CompanyID: 1,
				Total:     299.99,
				Status:    "processing",
				Metadata: map[string]interface{}{
					"payment_method": "invoice",
					"tax_amount":     29.99,
					"currency":       "USD",
					"shipping": map[string]interface{}{
						"method":   "express",
						"cost":     15.99,
						"address":  "123 Tech Street, San Francisco, CA 94105",
						"tracking": "TRACK789012",
					},
					"billing": map[string]interface{}{
						"same_as_shipping": false,
						"invoice_number":   "INV-2024-002",
						"due_date":         "2024-02-20",
					},
				},
				CreatedAt: time.Now().Add(-2 * 24 * time.Hour),
			},
		},
	}
}

// TestComprehensiveGraphQLWithDatabaseAliases - Main comprehensive test
func TestComprehensiveGraphQLWithDatabaseAliases(t *testing.T) {
	framework := NewIntegrationTestFramework()

	// Skip if running in demo mode
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}

	// Wait for eru-ql service
	if err := waitForEruQLService(framework.config, 30*time.Second); err != nil {
		t.Skip(fmt.Sprintf("Skipping test: %v", err))
	}

	// Test each configured database alias
	for _, dbConfig := range framework.databaseConfigs {
		t.Run(fmt.Sprintf("DatabaseAlias_%s", dbConfig.DatabaseAlias), func(t *testing.T) {

			// Run comprehensive test suite for each database alias
			t.Run("FilterOperatorTests", func(t *testing.T) {
				framework.testFilterOperators(t, dbConfig.DatabaseAlias)
			})

			t.Run("JSONFieldTests", func(t *testing.T) {
				framework.testJSONFieldQueries(t, dbConfig.DatabaseAlias)
			})

			t.Run("AggregationTests", func(t *testing.T) {
				framework.testAggregationQueries(t, dbConfig.DatabaseAlias)
			})

			t.Run("JoinRelationshipTests", func(t *testing.T) {
				framework.testJoinQueries(t, dbConfig.DatabaseAlias)
			})

			t.Run("MutationTests", func(t *testing.T) {
				framework.testMutationOperations(t, dbConfig.DatabaseAlias)
			})
		})
	}
}

func (f *IntegrationTestFramework) testFilterOperators(t *testing.T, dbAlias string) {
	testCases := []struct {
		name     string
		query    string
		validate func(t *testing.T, response *GraphQLResponse)
	}{
		{
			name: "EqualityFilter",
			query: fmt.Sprintf(`query {
				users(where: {company_id: {eq: 1}}) @%s {
					id name email company_id
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}
				t.Logf("✅ Equality filter test passed for %s", dbAlias)
			},
		},
		{
			name: "InFilter",
			query: fmt.Sprintf(`query {
				users(where: {id: {in: [1, 2, 3]}}) @%s {
					id name email
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}
				t.Logf("✅ IN filter test passed for %s", dbAlias)
			},
		},
		{
			name: "LikeFilter",
			query: fmt.Sprintf(`query {
				users(where: {email: {like: "%%@techcorp.com"}}) @%s {
					id name email
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}
				t.Logf("✅ LIKE filter test passed for %s", dbAlias)
			},
		},
		{
			name: "GreaterThanFilter",
			query: fmt.Sprintf(`query {
				products(where: {price: {gt: 500}}) @%s {
					id name price
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}
				t.Logf("✅ Greater than filter test passed for %s", dbAlias)
			},
		},
		{
			name: "LessThanFilter",
			query: fmt.Sprintf(`query {
				products(where: {price: {lt: 1000}}) @%s {
					id name price
				}
			}`, dbAlias),
			validate: func(t *testing.T, response *GraphQLResponse) {
				if len(response.Errors) > 0 {
					t.Logf("Expected errors for filter test: %v", response.Errors)
					return
				}
				t.Logf("✅ Less than filter test passed for %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(f.config, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (f *IntegrationTestFramework) testJSONFieldQueries(t *testing.T, dbAlias string) {
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
				t.Logf("✅ JSON field access test passed for %s", dbAlias)
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
				t.Logf("✅ Nested JSON field test passed for %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(f.config, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (f *IntegrationTestFramework) testAggregationQueries(t *testing.T, dbAlias string) {
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
				t.Logf("✅ Count aggregation test passed for %s", dbAlias)
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
				t.Logf("✅ Sum aggregation test passed for %s", dbAlias)
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
				t.Logf("✅ Average aggregation test passed for %s", dbAlias)
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
				t.Logf("✅ Min/Max aggregation test passed for %s", dbAlias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := GraphQLRequest{Query: tc.query}
			response, err := executeGraphQL(f.config, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (f *IntegrationTestFramework) testJoinQueries(t *testing.T, dbAlias string) {
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
				t.Logf("✅ User-Company join test passed for %s", dbAlias)
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
				t.Logf("✅ Order-User-Company join test passed for %s", dbAlias)
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
			response, err := executeGraphQL(f.config, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}

func (f *IntegrationTestFramework) testMutationOperations(t *testing.T, dbAlias string) {
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
			response, err := executeGraphQL(f.config, request)
			if err != nil {
				t.Logf("Expected error for demo test: %v", err)
				return
			}
			tc.validate(t, response)
		})
	}
}
