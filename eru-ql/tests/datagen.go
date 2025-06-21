package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// generatePostgreSQLInserts generates PostgreSQL INSERT statements from ExpectedTestData
func generatePostgreSQLInserts() string {
	var sql strings.Builder
	
	// Companies
	sql.WriteString("-- Insert companies\n")
	for _, company := range ExpectedTestData.Companies {
		metadataJSON, _ := json.Marshal(company.Metadata)
		settingsJSON, _ := json.Marshal(company.Settings)
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO companies (id, name, industry, founded_year, metadata, settings) VALUES (%d, '%s', '%s', %d, '%s'::jsonb, '%s'::jsonb);\n",
			company.ID, company.Name, company.Industry, company.FoundedYear, string(metadataJSON), string(settingsJSON),
		))
	}
	
	// Insert departments (required for user foreign keys)
	sql.WriteString("\n-- Insert departments\n")
	sql.WriteString("INSERT INTO departments (id, company_id, name, budget, head_count, location, policies) VALUES\n")
	sql.WriteString("(1, 1, 'Engineering', 5000000.00, 150, '{\"building\": \"A\", \"floor\": 3, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": true, \"flexible_hours\": true}'::jsonb),\n")
	sql.WriteString("(2, 1, 'Marketing', 2000000.00, 50, '{\"building\": \"B\", \"floor\": 1, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": false, \"flexible_hours\": true}'::jsonb),\n")
	sql.WriteString("(3, 1, 'Sales', 3000000.00, 80, '{\"building\": \"B\", \"floor\": 2, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": true, \"flexible_hours\": false}'::jsonb),\n")
	sql.WriteString("(4, 2, 'Development', 3500000.00, 120, '{\"building\": \"Main\", \"floor\": 2, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": true, \"flexible_hours\": true}'::jsonb),\n")
	sql.WriteString("(5, 2, 'QA', 1500000.00, 40, '{\"building\": \"Main\", \"floor\": 1, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": false, \"flexible_hours\": true}'::jsonb),\n")
	sql.WriteString("(6, 3, 'DevOps', 2500000.00, 60, '{\"building\": \"Cloud\", \"floor\": 4, \"country\": \"USA\"}'::jsonb, '{\"remote_work\": true, \"flexible_hours\": true}'::jsonb);\n")
	
	// Insert categories (required for product foreign keys)
	sql.WriteString("\n-- Insert categories\n")
	sql.WriteString("INSERT INTO categories (id, parent_id, name, description, level, properties) VALUES\n")
	sql.WriteString("(1, NULL, 'Electronics', 'Electronic devices and components', 1, '{\"featured\": true, \"commission_rate\": 0.05}'::jsonb),\n")
	sql.WriteString("(2, 1, 'Computers', 'Desktop and laptop computers', 2, '{\"warranty_months\": 24, \"return_policy\": \"30 days\"}'::jsonb),\n")
	sql.WriteString("(3, 1, 'Mobile Devices', 'Smartphones and tablets', 2, '{\"warranty_months\": 12, \"return_policy\": \"14 days\"}'::jsonb),\n")
	sql.WriteString("(4, 2, 'Laptops', 'Portable computers', 3, '{\"shipping_weight_limit\": 5, \"special_handling\": true}'::jsonb),\n")
	sql.WriteString("(5, 2, 'Desktops', 'Desktop computers', 3, '{\"shipping_weight_limit\": 20, \"assembly_required\": true}'::jsonb),\n")
	sql.WriteString("(6, NULL, 'Office Supplies', 'Business and office equipment', 1, '{\"featured\": false, \"commission_rate\": 0.02}'::jsonb),\n")
	sql.WriteString("(7, 6, 'Furniture', 'Office furniture and accessories', 2, '{\"assembly_required\": true, \"delivery_time\": \"5-7 days\"}'::jsonb);\n")
	
	// Users
	sql.WriteString("\n-- Insert users\n")
	for _, user := range ExpectedTestData.Users {
		// Convert tags array to PostgreSQL array format
		tagsArray := "ARRAY["
		for i, tag := range user.Tags {
			if i > 0 {
				tagsArray += ","
			}
			tagsArray += fmt.Sprintf("'%s'", tag)
		}
		tagsArray += "]"
		
		preferencesJSON, _ := json.Marshal(user.Preferences)
		metadataJSON, _ := json.Marshal(user.Metadata)
		
		deptID := "NULL"
		if user.DepartmentID != nil {
			deptID = fmt.Sprintf("%d", *user.DepartmentID)
		}
		
		salary := "NULL"
		if user.Salary != nil {
			salary = fmt.Sprintf("%.2f", *user.Salary)
		}
		
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO users (id, company_id, department_id, email, name, role, salary, is_active, tags, preferences, metadata) VALUES (%d, %d, %s, '%s', '%s', '%s', %s, %t, %s, '%s'::jsonb, '%s'::jsonb);\n",
			user.ID, user.CompanyID, deptID, user.Email, user.Name, user.Role, salary, user.IsActive, tagsArray, string(preferencesJSON), string(metadataJSON),
		))
	}
	
	// Products
	sql.WriteString("\n-- Insert products\n")
	for _, product := range ExpectedTestData.Products {
		dimensionsJSON, _ := json.Marshal(product.Dimensions)
		specsJSON, _ := json.Marshal(product.Specifications)
		
		// Convert tags array to PostgreSQL array format
		tagsArray := "ARRAY["
		for i, tag := range product.Tags {
			if i > 0 {
				tagsArray += ","
			}
			tagsArray += fmt.Sprintf("'%s'", tag)
		}
		tagsArray += "]"
		
		categoryID := "NULL"
		if product.CategoryID != nil {
			categoryID = fmt.Sprintf("%d", *product.CategoryID)
		}
		
		price := "NULL"
		if product.Price != nil {
			price = fmt.Sprintf("%.2f", *product.Price)
		}
		
		cost := "NULL"
		if product.Cost != nil {
			cost = fmt.Sprintf("%.2f", *product.Cost)
		}
		
		weight := "NULL"
		if product.Weight != nil {
			weight = fmt.Sprintf("%.3f", *product.Weight)
		}
		
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO products (id, category_id, name, description, price, cost, stock_quantity, weight, dimensions, specifications, tags, is_active) VALUES (%d, %s, '%s', '%s', %s, %s, %d, %s, '%s'::jsonb, '%s'::jsonb, %s, %t);\n",
			product.ID, categoryID, product.Name, product.Description, price, cost, product.StockQuantity, weight, string(dimensionsJSON), string(specsJSON), tagsArray, product.IsActive,
		))
	}
	
	return sql.String()
}

// generateMySQLInserts generates MySQL INSERT statements from ExpectedTestData
func generateMySQLInserts() string {
	var sql strings.Builder
	
	// Companies
	sql.WriteString("-- Insert companies\n")
	for _, company := range ExpectedTestData.Companies {
		metadataJSON, _ := json.Marshal(company.Metadata)
		settingsJSON, _ := json.Marshal(company.Settings)
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO companies (id, name, industry, founded_year, metadata, settings) VALUES (%d, '%s', '%s', %d, '%s', '%s');\n",
			company.ID, company.Name, company.Industry, company.FoundedYear, string(metadataJSON), string(settingsJSON),
		))
	}
	
	// Users
	sql.WriteString("\n-- Insert users\n")
	for _, user := range ExpectedTestData.Users {
		tagsJSON, _ := json.Marshal(user.Tags)
		preferencesJSON, _ := json.Marshal(user.Preferences)
		metadataJSON, _ := json.Marshal(user.Metadata)
		
		deptID := "NULL"
		if user.DepartmentID != nil {
			deptID = fmt.Sprintf("%d", *user.DepartmentID)
		}
		
		salary := "NULL"
		if user.Salary != nil {
			salary = fmt.Sprintf("%.2f", *user.Salary)
		}
		
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO users (id, company_id, department_id, email, name, role, salary, is_active, tags, preferences, metadata) VALUES (%d, %d, %s, '%s', '%s', '%s', %s, %t, '%s', '%s', '%s');\n",
			user.ID, user.CompanyID, deptID, user.Email, user.Name, user.Role, salary, user.IsActive, string(tagsJSON), string(preferencesJSON), string(metadataJSON),
		))
	}
	
	// Products
	sql.WriteString("\n-- Insert products\n")
	for _, product := range ExpectedTestData.Products {
		dimensionsJSON, _ := json.Marshal(product.Dimensions)
		specsJSON, _ := json.Marshal(product.Specifications)
		tagsJSON, _ := json.Marshal(product.Tags)
		
		categoryID := "NULL"
		if product.CategoryID != nil {
			categoryID = fmt.Sprintf("%d", *product.CategoryID)
		}
		
		price := "NULL"
		if product.Price != nil {
			price = fmt.Sprintf("%.2f", *product.Price)
		}
		
		cost := "NULL"
		if product.Cost != nil {
			cost = fmt.Sprintf("%.2f", *product.Cost)
		}
		
		weight := "NULL"
		if product.Weight != nil {
			weight = fmt.Sprintf("%.3f", *product.Weight)
		}
		
		sql.WriteString(fmt.Sprintf(
			"INSERT INTO products (id, category_id, name, description, price, cost, stock_quantity, weight, dimensions, specifications, tags, is_active) VALUES (%d, %s, '%s', '%s', %s, %s, %d, %s, '%s', '%s', '%s', %t);\n",
			product.ID, categoryID, product.Name, product.Description, price, cost, product.StockQuantity, weight, string(dimensionsJSON), string(specsJSON), string(tagsJSON), product.IsActive,
		))
	}
	
	return sql.String()
}