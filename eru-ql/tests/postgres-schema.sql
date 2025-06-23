-- PostgreSQL Schema for eru-ql Integration Tests
-- Complete enterprise schema with relationships, JSON fields, and comprehensive test data

-- Drop existing tables to recreate with relationships
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS user_profiles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS companies CASCADE;
DROP TABLE IF EXISTS departments CASCADE;
DROP TABLE IF EXISTS audit_log CASCADE;

-- Companies table (top level)
CREATE TABLE companies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    industry VARCHAR(100),
    founded_year INTEGER,
    metadata JSONB DEFAULT '{}',
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Departments table (belongs to company)
CREATE TABLE departments (
    id SERIAL PRIMARY KEY,
    company_id INTEGER REFERENCES companies(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    budget DECIMAL(15,2),
    head_count INTEGER DEFAULT 0,
    location JSONB DEFAULT '{}', -- JSON field for location data
    policies JSONB DEFAULT '{}', -- JSON field for department policies
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Users table (belongs to department/company)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    company_id INTEGER REFERENCES companies(id) ON DELETE CASCADE,
    department_id INTEGER REFERENCES departments(id) ON DELETE SET NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(100),
    salary DECIMAL(10,2),
    hire_date DATE,
    is_active BOOLEAN DEFAULT true,
    tags TEXT[], -- Array field
    preferences JSONB DEFAULT '{}', -- JSON field for user preferences
    metadata JSONB DEFAULT '{}', -- JSON field for additional user data
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User profiles table (one-to-one with users)
CREATE TABLE user_profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    bio TEXT,
    skills TEXT[],
    experience_years INTEGER,
    education JSONB DEFAULT '{}', -- JSON field for education history
    contacts JSONB DEFAULT '{}', -- JSON field for contact information
    achievements JSONB DEFAULT '[]', -- JSON array for achievements
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Categories table (for products)
CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    parent_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    level INTEGER DEFAULT 1,
    properties JSONB DEFAULT '{}', -- JSON field for category properties
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Products table (belongs to category)
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2),
    cost DECIMAL(10,2),
    stock_quantity INTEGER DEFAULT 0,
    weight DECIMAL(8,3),
    dimensions JSONB DEFAULT '{}', -- JSON field for dimensions
    specifications JSONB DEFAULT '{}', -- JSON field for product specs
    tags TEXT[],
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table (belongs to user)
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    order_number VARCHAR(50) UNIQUE NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(12,2),
    discount_amount DECIMAL(12,2) DEFAULT 0,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    shipping_address JSONB DEFAULT '{}', -- JSON field for address
    billing_address JSONB DEFAULT '{}', -- JSON field for address
    payment_info JSONB DEFAULT '{}', -- JSON field for payment details
    metadata JSONB DEFAULT '{}', -- JSON field for additional order data
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    shipped_date TIMESTAMP,
    delivered_date TIMESTAMP
);

-- Order items table (many-to-many between orders and products)
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price DECIMAL(10,2),
    total_price DECIMAL(12,2),
    discount_percentage DECIMAL(5,2) DEFAULT 0,
    item_metadata JSONB DEFAULT '{}', -- JSON field for item-specific data
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit log table
CREATE TABLE audit_log (
    id SERIAL PRIMARY KEY,
    entity_type VARCHAR(100),
    entity_id INTEGER,
    action VARCHAR(100),
    old_values JSONB,
    new_values JSONB,
    changed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    change_metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Test data will be generated from testdata.go

