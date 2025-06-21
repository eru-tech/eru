-- MySQL Schema for eru-ql Integration Tests
-- Complete enterprise schema matching PostgreSQL version for universal testing
-- MySQL-specific syntax and data types (JSON, AUTO_INCREMENT, etc.)

-- Drop existing tables to recreate with relationships
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS user_analytics;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS audit_log;

-- Companies table (top level)
CREATE TABLE companies (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    industry VARCHAR(100),
    founded_year INTEGER,
    metadata JSON,
    settings JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Departments table (belongs to company)
CREATE TABLE departments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    company_id INTEGER,
    name VARCHAR(255) NOT NULL,
    budget DECIMAL(15,2),
    head_count INTEGER DEFAULT 0,
    location JSON, -- JSON field for location data
    policies JSON, -- JSON field for department policies
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (company_id) REFERENCES companies(id) ON DELETE CASCADE
);

-- Users table (belongs to department/company)
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    company_id INTEGER,
    department_id INTEGER,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(100),
    salary DECIMAL(10,2),
    hire_date DATE,
    is_active BOOLEAN DEFAULT true,
    tags JSON, -- JSON array for tags (MySQL doesn't have native array type)
    preferences JSON, -- JSON field for user preferences
    metadata JSON, -- JSON field for additional user data
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (company_id) REFERENCES companies(id) ON DELETE CASCADE,
    FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL
);

-- User profiles table (one-to-one with users)
CREATE TABLE user_profiles (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INTEGER UNIQUE,
    bio TEXT,
    skills JSON, -- JSON array for skills
    experience_years INTEGER,
    education JSON, -- JSON field for education history
    contacts JSON, -- JSON field for contact information
    achievements JSON, -- JSON array for achievements
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Categories table (for products)
CREATE TABLE categories (
    id INT AUTO_INCREMENT PRIMARY KEY,
    parent_id INTEGER,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    level INTEGER DEFAULT 1,
    properties JSON, -- JSON field for category properties
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE SET NULL
);

-- Products table (belongs to category)
CREATE TABLE products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    category_id INTEGER,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2),
    cost DECIMAL(10,2),
    stock_quantity INTEGER DEFAULT 0,
    weight DECIMAL(8,3),
    dimensions JSON, -- JSON field for dimensions
    specifications JSON, -- JSON field for product specs
    tags JSON, -- JSON array for tags
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL
);

-- Orders table (belongs to user)
CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INTEGER,
    order_number VARCHAR(50) UNIQUE NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(12,2),
    discount_amount DECIMAL(12,2) DEFAULT 0,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    shipping_address JSON, -- JSON field for address
    billing_address JSON, -- JSON field for address
    payment_info JSON, -- JSON field for payment details
    metadata JSON, -- JSON field for additional order data
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    shipped_date TIMESTAMP NULL,
    delivered_date TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Order items table (many-to-many between orders and products)
CREATE TABLE order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INTEGER,
    product_id INTEGER,
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price DECIMAL(10,2),
    total_price DECIMAL(12,2),
    discount_percentage DECIMAL(5,2) DEFAULT 0,
    item_metadata JSON, -- JSON field for item-specific data
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);

-- Audit log table
CREATE TABLE audit_log (
    id INT AUTO_INCREMENT PRIMARY KEY,
    entity_type VARCHAR(100),
    entity_id INTEGER,
    action VARCHAR(100),
    old_values JSON,
    new_values JSON,
    changed_by INTEGER,
    change_metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Insert comprehensive test data
