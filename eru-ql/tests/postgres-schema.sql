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
DROP TABLE IF EXISTS user_analytics CASCADE;
DROP TABLE IF EXISTS events CASCADE;
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

-- Insert comprehensive test data
INSERT INTO companies (name, industry, founded_year, metadata, settings) VALUES 
    ('TechCorp Inc', 'Technology', 2010, '{"headquarters": "San Francisco", "employees": 5000}', '{"timezone": "PST", "fiscal_year_start": "January"}'),
    ('DataSoft LLC', 'Software', 2015, '{"headquarters": "Austin", "employees": 1200}', '{"timezone": "CST", "fiscal_year_start": "April"}'),
    ('CloudNet Systems', 'Cloud Services', 2018, '{"headquarters": "Seattle", "employees": 800}', '{"timezone": "PST", "fiscal_year_start": "January"}');

INSERT INTO departments (company_id, name, budget, head_count, location, policies) VALUES 
    (1, 'Engineering', 5000000.00, 150, '{"building": "A", "floor": 3, "country": "USA"}', '{"remote_work": true, "flexible_hours": true}'),
    (1, 'Marketing', 2000000.00, 50, '{"building": "B", "floor": 1, "country": "USA"}', '{"remote_work": false, "flexible_hours": true}'),
    (1, 'Sales', 3000000.00, 80, '{"building": "B", "floor": 2, "country": "USA"}', '{"remote_work": true, "flexible_hours": false}'),
    (2, 'Development', 3500000.00, 120, '{"building": "Main", "floor": 2, "country": "USA"}', '{"remote_work": true, "flexible_hours": true}'),
    (2, 'QA', 1500000.00, 40, '{"building": "Main", "floor": 1, "country": "USA"}', '{"remote_work": false, "flexible_hours": true}'),
    (3, 'DevOps', 2500000.00, 60, '{"building": "Cloud", "floor": 4, "country": "USA"}', '{"remote_work": true, "flexible_hours": true}');

INSERT INTO users (company_id, department_id, email, name, role, salary, hire_date, tags, preferences, metadata) VALUES 
    (1, 1, 'john.doe@techcorp.com', 'John Doe', 'Senior Engineer', 95000.00, '2020-01-15', ARRAY['go', 'python', 'kubernetes'], '{"theme": "dark", "notifications": true}', '{"performance_rating": 4.5, "last_review": "2024-01-15"}'),
    (1, 1, 'jane.smith@techcorp.com', 'Jane Smith', 'Tech Lead', 120000.00, '2019-03-10', ARRAY['javascript', 'react', 'node'], '{"theme": "light", "notifications": false}', '{"performance_rating": 4.8, "last_review": "2024-02-01"}'),
    (1, 2, 'bob.wilson@techcorp.com', 'Bob Wilson', 'Marketing Manager', 85000.00, '2021-06-01', ARRAY['marketing', 'analytics'], '{"theme": "auto", "notifications": true}', '{"performance_rating": 4.2, "last_review": "2024-03-01"}'),
    (1, 3, 'alice.brown@techcorp.com', 'Alice Brown', 'Sales Director', 110000.00, '2020-09-15', ARRAY['sales', 'crm'], '{"theme": "dark", "notifications": true}', '{"performance_rating": 4.7, "last_review": "2024-01-20"}'),
    (2, 4, 'charlie.davis@datasoft.com', 'Charlie Davis', 'Full Stack Developer', 78000.00, '2022-01-20', ARRAY['php', 'mysql', 'vue'], '{"theme": "light", "notifications": false}', '{"performance_rating": 4.1, "last_review": "2024-02-15"}'),
    (2, 5, 'eva.garcia@datasoft.com', 'Eva Garcia', 'QA Lead', 72000.00, '2021-11-30', ARRAY['testing', 'automation'], '{"theme": "dark", "notifications": true}', '{"performance_rating": 4.4, "last_review": "2024-03-10"}'),
    (3, 6, 'frank.miller@cloudnet.com', 'Frank Miller', 'DevOps Engineer', 88000.00, '2022-05-10', ARRAY['aws', 'docker', 'terraform'], '{"theme": "auto", "notifications": false}', '{"performance_rating": 4.3, "last_review": "2024-02-28"}');

INSERT INTO user_profiles (user_id, bio, skills, experience_years, education, contacts, achievements) VALUES 
    (1, 'Experienced backend engineer specializing in microservices', ARRAY['Go', 'Python', 'Kubernetes', 'gRPC'], 8, '{"degree": "BS Computer Science", "university": "Stanford", "year": 2016}', '{"linkedin": "johndoe", "github": "jdoe"}', '[{"title": "Employee of the Month", "date": "2023-06"}, {"title": "Innovation Award", "date": "2023-12"}]'),
    (2, 'Frontend architect with expertise in modern web technologies', ARRAY['JavaScript', 'React', 'TypeScript', 'GraphQL'], 10, '{"degree": "MS Software Engineering", "university": "MIT", "year": 2014}', '{"linkedin": "janesmith", "github": "jsmith"}', '[{"title": "Tech Lead of the Year", "date": "2023-12"}, {"title": "Open Source Contributor", "date": "2023-09"}]'),
    (3, 'Marketing strategist focused on growth and analytics', ARRAY['Analytics', 'SEO', 'Content Marketing'], 6, '{"degree": "MBA Marketing", "university": "Wharton", "year": 2018}', '{"linkedin": "bobwilson", "twitter": "bwilson"}', '[{"title": "Campaign Excellence", "date": "2023-08"}]'),
    (4, 'Sales leader with proven track record in B2B', ARRAY['Sales', 'CRM', 'Negotiation'], 12, '{"degree": "BA Business", "university": "Harvard", "year": 2012}', '{"linkedin": "alicebrown", "salesforce": "abrown"}', '[{"title": "Top Sales Performer", "date": "2023-12"}, {"title": "Customer Success Award", "date": "2023-06"}]');

INSERT INTO categories (parent_id, name, description, level, properties) VALUES 
    (NULL, 'Electronics', 'Electronic devices and components', 1, '{"featured": true, "commission_rate": 0.05}'),
    (1, 'Computers', 'Desktop and laptop computers', 2, '{"warranty_months": 24, "return_policy": "30 days"}'),
    (1, 'Mobile Devices', 'Smartphones and tablets', 2, '{"warranty_months": 12, "return_policy": "14 days"}'),
    (2, 'Laptops', 'Portable computers', 3, '{"shipping_weight_limit": 5, "special_handling": true}'),
    (2, 'Desktops', 'Desktop computers', 3, '{"shipping_weight_limit": 20, "assembly_required": true}'),
    (NULL, 'Office Supplies', 'Business and office equipment', 1, '{"featured": false, "commission_rate": 0.02}'),
    (6, 'Furniture', 'Office furniture and accessories', 2, '{"assembly_required": true, "delivery_time": "5-7 days"}');

INSERT INTO products (category_id, name, description, price, cost, stock_quantity, weight, dimensions, specifications, tags) VALUES 
    (4, 'MacBook Pro 16"', 'High-performance laptop for professionals', 2499.99, 1800.00, 50, 2.140, '{"width": 35.57, "height": 24.59, "depth": 1.68}', '{"cpu": "M2 Pro", "ram": "16GB", "storage": "512GB SSD", "display": "16-inch Retina"}', ARRAY['apple', 'laptop', 'professional']),
    (4, 'Dell XPS 13', 'Ultra-portable Windows laptop', 1299.99, 900.00, 75, 1.270, '{"width": 29.6, "height": 19.9, "depth": 1.47}', '{"cpu": "Intel i7", "ram": "16GB", "storage": "256GB SSD", "display": "13.3-inch FHD"}', ARRAY['dell', 'laptop', 'ultrabook']),
    (5, 'iMac 24"', 'All-in-one desktop computer', 1499.99, 1100.00, 30, 4.460, '{"width": 54.7, "height": 46.1, "depth": 14.7}', '{"cpu": "M1", "ram": "8GB", "storage": "256GB SSD", "display": "24-inch 4.5K Retina"}', ARRAY['apple', 'desktop', 'all-in-one']),
    (3, 'iPhone 15 Pro', 'Latest generation smartphone', 999.99, 700.00, 200, 0.187, '{"width": 7.67, "height": 14.67, "depth": 0.83}', '{"cpu": "A17 Pro", "ram": "8GB", "storage": "128GB", "display": "6.1-inch Super Retina XDR"}', ARRAY['apple', 'smartphone', 'pro']),
    (3, 'Samsung Galaxy S24', 'Android flagship smartphone', 799.99, 550.00, 150, 0.168, '{"width": 7.06, "height": 14.7, "depth": 0.76}', '{"cpu": "Snapdragon 8 Gen 3", "ram": "8GB", "storage": "256GB", "display": "6.2-inch Dynamic AMOLED"}', ARRAY['samsung', 'smartphone', 'android']),
    (7, 'Executive Desk', 'Premium executive office desk', 899.99, 400.00, 20, 45.000, '{"width": 152.4, "height": 76.2, "depth": 76.2}', '{"material": "Oak wood", "drawers": 3, "finish": "Natural", "weight_capacity": "100kg"}', ARRAY['furniture', 'desk', 'executive']);

INSERT INTO orders (user_id, order_number, status, total_amount, discount_amount, tax_amount, shipping_address, billing_address, payment_info, metadata) VALUES 
    (1, 'ORD-2024-001', 'completed', 2674.99, 125.00, 200.00, '{"street": "123 Main St", "city": "San Francisco", "state": "CA", "zip": "94105", "country": "USA"}', '{"street": "123 Main St", "city": "San Francisco", "state": "CA", "zip": "94105", "country": "USA"}', '{"method": "credit_card", "last4": "1234"}', '{"gift_wrap": false, "priority_shipping": true}'),
    (2, 'ORD-2024-002', 'shipped', 1429.99, 50.00, 110.00, '{"street": "456 Oak Ave", "city": "Palo Alto", "state": "CA", "zip": "94301", "country": "USA"}', '{"street": "789 Pine St", "city": "Palo Alto", "state": "CA", "zip": "94301", "country": "USA"}', '{"method": "paypal", "email": "jane@example.com"}', '{"gift_wrap": true, "priority_shipping": false}'),
    (3, 'ORD-2024-003', 'pending', 999.99, 0.00, 80.00, '{"street": "321 Elm St", "city": "Berkeley", "state": "CA", "zip": "94720", "country": "USA"}', '{"street": "321 Elm St", "city": "Berkeley", "state": "CA", "zip": "94720", "country": "USA"}', '{"method": "credit_card", "last4": "5678"}', '{"gift_wrap": false, "priority_shipping": false}'),
    (4, 'ORD-2024-004', 'completed', 1699.98, 100.00, 130.00, '{"street": "654 Maple Dr", "city": "San Jose", "state": "CA", "zip": "95110", "country": "USA"}', '{"street": "654 Maple Dr", "city": "San Jose", "state": "CA", "zip": "95110", "country": "USA"}', '{"method": "debit_card", "last4": "9012"}', '{"gift_wrap": true, "priority_shipping": true}');

INSERT INTO order_items (order_id, product_id, quantity, unit_price, total_price, discount_percentage, item_metadata) VALUES 
    (1, 1, 1, 2499.99, 2499.99, 5.0, '{"warranty_extended": true, "insurance": false}'),
    (2, 2, 1, 1299.99, 1299.99, 3.0, '{"warranty_extended": false, "insurance": true}'),
    (2, 4, 1, 999.99, 999.99, 0.0, '{"case_included": true, "screen_protector": true}'),
    (3, 4, 1, 999.99, 999.99, 0.0, '{"case_included": false, "screen_protector": false}'),
    (4, 5, 2, 799.99, 1599.98, 6.0, '{"bulk_discount": true, "corporate_account": true}');

-- Create comprehensive indexes
CREATE INDEX idx_users_company_id ON users(company_id);
CREATE INDEX idx_users_department_id ON users(department_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_salary ON users(salary);
CREATE INDEX idx_users_hire_date ON users(hire_date);
CREATE INDEX idx_users_preferences_gin ON users USING GIN(preferences);
CREATE INDEX idx_users_metadata_gin ON users USING GIN(metadata);

CREATE INDEX idx_departments_company_id ON departments(company_id);
CREATE INDEX idx_departments_location_gin ON departments USING GIN(location);
CREATE INDEX idx_departments_policies_gin ON departments USING GIN(policies);

CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_price ON products(price);
CREATE INDEX idx_products_stock_quantity ON products(stock_quantity);
CREATE INDEX idx_products_specifications_gin ON products USING GIN(specifications);
CREATE INDEX idx_products_dimensions_gin ON products USING GIN(dimensions);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_order_date ON orders(order_date);
CREATE INDEX idx_orders_total_amount ON orders(total_amount);
CREATE INDEX idx_orders_shipping_address_gin ON orders USING GIN(shipping_address);
CREATE INDEX idx_orders_metadata_gin ON orders USING GIN(metadata);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
CREATE INDEX idx_order_items_metadata_gin ON order_items USING GIN(item_metadata);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_level ON categories(level);
CREATE INDEX idx_categories_properties_gin ON categories USING GIN(properties);

CREATE INDEX idx_audit_log_entity_type ON audit_log(entity_type);
CREATE INDEX idx_audit_log_entity_id ON audit_log(entity_id);
CREATE INDEX idx_audit_log_changed_by ON audit_log(changed_by);
CREATE INDEX idx_audit_log_old_values_gin ON audit_log USING GIN(old_values);
CREATE INDEX idx_audit_log_new_values_gin ON audit_log USING GIN(new_values);

-- Create views for testing
CREATE OR REPLACE VIEW user_department_summary AS 
SELECT 
    d.company_id,
    d.id as department_id,
    d.name as department_name,
    COUNT(u.id) as user_count,
    AVG(u.salary) as avg_salary,
    JSONB_AGG(
        JSONB_BUILD_OBJECT(
            'user_id', u.id,
            'name', u.name,
            'role', u.role,
            'preferences', u.preferences
        )
    ) as users_data
FROM departments d
LEFT JOIN users u ON d.id = u.department_id
GROUP BY d.company_id, d.id, d.name;

CREATE OR REPLACE VIEW order_analytics AS
SELECT 
    u.company_id,
    u.department_id,
    o.id as order_id,
    o.order_number,
    o.status,
    o.total_amount,
    COUNT(oi.id) as item_count,
    JSONB_AGG(
        JSONB_BUILD_OBJECT(
            'product_id', oi.product_id,
            'quantity', oi.quantity,
            'unit_price', oi.unit_price,
            'metadata', oi.item_metadata
        )
    ) as items_data
FROM orders o
JOIN users u ON o.user_id = u.id
LEFT JOIN order_items oi ON o.id = oi.order_id
GROUP BY u.company_id, u.department_id, o.id, o.order_number, o.status, o.total_amount;