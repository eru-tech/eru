package main

import "time"

// Unified test data structure that matches schema files
// This is the single source of truth for all test validations

type Company struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Industry    string                 `json:"industry"`
	FoundedYear int                    `json:"founded_year"`
	Metadata    map[string]interface{} `json:"metadata"`
	Settings    map[string]interface{} `json:"settings"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Department struct {
	ID        int                    `json:"id"`
	CompanyID int                    `json:"company_id"`
	Name      string                 `json:"name"`
	Budget    float64                `json:"budget"`
	HeadCount int                    `json:"head_count"`
	Location  map[string]interface{} `json:"location"`
	Policies  map[string]interface{} `json:"policies"`
	CreatedAt time.Time              `json:"created_at"`
}

type User struct {
	ID           int                    `json:"id"`
	CompanyID    int                    `json:"company_id"`
	DepartmentID *int                   `json:"department_id"`
	Email        string                 `json:"email"`
	Name         string                 `json:"name"`
	Role         string                 `json:"role"`
	Salary       *float64               `json:"salary"`
	HireDate     *time.Time             `json:"hire_date"`
	IsActive     bool                   `json:"is_active"`
	Tags         []string               `json:"tags"`
	Preferences  map[string]interface{} `json:"preferences"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type Category struct {
	ID          int                    `json:"id"`
	ParentID    *int                   `json:"parent_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Level       int                    `json:"level"`
	Properties  map[string]interface{} `json:"properties"`
	CreatedAt   time.Time              `json:"created_at"`
}

type Product struct {
	ID             int                    `json:"id"`
	CategoryID     *int                   `json:"category_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Price          *float64               `json:"price"`
	Cost           *float64               `json:"cost"`
	StockQuantity  int                    `json:"stock_quantity"`
	Weight         *float64               `json:"weight"`
	Dimensions     map[string]interface{} `json:"dimensions"`
	Specifications map[string]interface{} `json:"specifications"`
	Tags           []string               `json:"tags"`
	IsActive       bool                   `json:"is_active"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type Order struct {
	ID              int                    `json:"id"`
	UserID          int                    `json:"user_id"`
	OrderNumber     string                 `json:"order_number"`
	Status          string                 `json:"status"`
	TotalAmount     *float64               `json:"total_amount"`
	DiscountAmount  *float64               `json:"discount_amount"`
	TaxAmount       *float64               `json:"tax_amount"`
	ShippingAddress map[string]interface{} `json:"shipping_address"`
	BillingAddress  map[string]interface{} `json:"billing_address"`
	PaymentInfo     map[string]interface{} `json:"payment_info"`
	Metadata        map[string]interface{} `json:"metadata"`
	OrderDate       time.Time              `json:"order_date"`
	ShippedDate     *time.Time             `json:"shipped_date"`
	DeliveredDate   *time.Time             `json:"delivered_date"`
}

// Expected test data - matches exactly what's inserted in schema files
var ExpectedTestData = struct {
	Companies []Company
	Users     []User
	Products  []Product
	Categories []Category
	Orders    []Order
}{
	Companies: []Company{
		{
			ID:          1,
			Name:        "TechCorp Inc",
			Industry:    "Technology",
			FoundedYear: 2010,
			Metadata:    map[string]interface{}{"headquarters": "San Francisco", "employees": float64(5000)},
			Settings:    map[string]interface{}{"timezone": "PST", "fiscal_year_start": "January"},
		},
		{
			ID:          2,
			Name:        "DataSoft LLC",
			Industry:    "Software",
			FoundedYear: 2015,
			Metadata:    map[string]interface{}{"headquarters": "Austin", "employees": float64(1200)},
			Settings:    map[string]interface{}{"timezone": "CST", "fiscal_year_start": "April"},
		},
		{
			ID:          3,
			Name:        "CloudNet Systems",
			Industry:    "Cloud Services",
			FoundedYear: 2018,
			Metadata:    map[string]interface{}{"headquarters": "Seattle", "employees": float64(800)},
			Settings:    map[string]interface{}{"timezone": "PST", "fiscal_year_start": "January"},
		},
	},
	Users: []User{
		{
			ID:        1,
			CompanyID: 1,
			DepartmentID: func() *int { i := 1; return &i }(),
			Email:     "john.doe@techcorp.com",
			Name:      "John Doe",
			Role:      "Senior Engineer",
			Salary:    func() *float64 { f := 95000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"go", "python", "kubernetes"},
			Preferences: map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.5,
				"last_review":        "2024-01-15",
			},
		},
		{
			ID:        2,
			CompanyID: 1,
			DepartmentID: func() *int { i := 1; return &i }(),
			Email:     "jane.smith@techcorp.com",
			Name:      "Jane Smith",
			Role:      "Tech Lead",
			Salary:    func() *float64 { f := 120000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"javascript", "react", "node"},
			Preferences: map[string]interface{}{
				"theme":         "light",
				"notifications": false,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.8,
				"last_review":        "2024-02-01",
			},
		},
		{
			ID:        3,
			CompanyID: 1,
			DepartmentID: func() *int { i := 2; return &i }(),
			Email:     "bob.wilson@techcorp.com",
			Name:      "Bob Wilson",
			Role:      "Marketing Manager",
			Salary:    func() *float64 { f := 85000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"marketing", "analytics"},
			Preferences: map[string]interface{}{
				"theme":         "auto",
				"notifications": true,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.2,
				"last_review":        "2024-03-01",
			},
		},
		{
			ID:        4,
			CompanyID: 1,
			DepartmentID: func() *int { i := 3; return &i }(),
			Email:     "alice.brown@techcorp.com",
			Name:      "Alice Brown",
			Role:      "Sales Director",
			Salary:    func() *float64 { f := 110000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"sales", "crm"},
			Preferences: map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.7,
				"last_review":        "2024-01-20",
			},
		},
		{
			ID:        5,
			CompanyID: 2,
			DepartmentID: func() *int { i := 4; return &i }(),
			Email:     "charlie.davis@datasoft.com",
			Name:      "Charlie Davis",
			Role:      "Full Stack Developer",
			Salary:    func() *float64 { f := 78000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"php", "mysql", "vue"},
			Preferences: map[string]interface{}{
				"theme":         "light",
				"notifications": false,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.1,
				"last_review":        "2024-02-15",
			},
		},
		{
			ID:        6,
			CompanyID: 2,
			DepartmentID: func() *int { i := 5; return &i }(),
			Email:     "eva.garcia@datasoft.com",
			Name:      "Eva Garcia",
			Role:      "QA Lead",
			Salary:    func() *float64 { f := 72000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"testing", "automation"},
			Preferences: map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.4,
				"last_review":        "2024-03-10",
			},
		},
		{
			ID:        7,
			CompanyID: 3,
			DepartmentID: func() *int { i := 6; return &i }(),
			Email:     "frank.miller@cloudnet.com",
			Name:      "Frank Miller",
			Role:      "DevOps Engineer",
			Salary:    func() *float64 { f := 88000.00; return &f }(),
			IsActive:  true,
			Tags:      []string{"aws", "docker", "terraform"},
			Preferences: map[string]interface{}{
				"theme":         "auto",
				"notifications": false,
			},
			Metadata: map[string]interface{}{
				"performance_rating": 4.3,
				"last_review":        "2024-02-28",
			},
		},
	},
	Products: []Product{
		{
			ID:            1,
			CategoryID:    func() *int { i := 4; return &i }(),
			Name:          "MacBook Pro 16\"",
			Description:   "High-performance laptop for professionals",
			Price:         func() *float64 { f := 2499.99; return &f }(),
			Cost:          func() *float64 { f := 1800.00; return &f }(),
			StockQuantity: 50,
			Weight:        func() *float64 { f := 2.140; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  35.57,
				"height": 24.59,
				"depth":  1.68,
			},
			Specifications: map[string]interface{}{
				"cpu":     "M2 Pro",
				"ram":     "16GB",
				"storage": "512GB SSD",
				"display": "16-inch Retina",
			},
			Tags:     []string{"apple", "laptop", "professional"},
			IsActive: true,
		},
		{
			ID:            2,
			CategoryID:    func() *int { i := 4; return &i }(),
			Name:          "Dell XPS 13",
			Description:   "Ultra-portable Windows laptop",
			Price:         func() *float64 { f := 1299.99; return &f }(),
			Cost:          func() *float64 { f := 900.00; return &f }(),
			StockQuantity: 75,
			Weight:        func() *float64 { f := 1.270; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  29.6,
				"height": 19.9,
				"depth":  1.47,
			},
			Specifications: map[string]interface{}{
				"cpu":     "Intel i7",
				"ram":     "16GB",
				"storage": "256GB SSD",
				"display": "13.3-inch FHD",
			},
			Tags:     []string{"dell", "laptop", "ultrabook"},
			IsActive: true,
		},
		{
			ID:            3,
			CategoryID:    func() *int { i := 5; return &i }(),
			Name:          "iMac 24\"",
			Description:   "All-in-one desktop computer",
			Price:         func() *float64 { f := 1499.99; return &f }(),
			Cost:          func() *float64 { f := 1100.00; return &f }(),
			StockQuantity: 30,
			Weight:        func() *float64 { f := 4.460; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  54.7,
				"height": 46.1,
				"depth":  14.7,
			},
			Specifications: map[string]interface{}{
				"cpu":     "M1",
				"ram":     "8GB",
				"storage": "256GB SSD",
				"display": "24-inch 4.5K Retina",
			},
			Tags:     []string{"apple", "desktop", "all-in-one"},
			IsActive: true,
		},
		{
			ID:            4,
			CategoryID:    func() *int { i := 3; return &i }(),
			Name:          "iPhone 15 Pro",
			Description:   "Latest generation smartphone",
			Price:         func() *float64 { f := 999.99; return &f }(),
			Cost:          func() *float64 { f := 700.00; return &f }(),
			StockQuantity: 200,
			Weight:        func() *float64 { f := 0.187; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  7.67,
				"height": 14.67,
				"depth":  0.83,
			},
			Specifications: map[string]interface{}{
				"cpu":     "A17 Pro",
				"ram":     "8GB",
				"storage": "128GB",
				"display": "6.1-inch Super Retina XDR",
			},
			Tags:     []string{"apple", "smartphone", "pro"},
			IsActive: true,
		},
		{
			ID:            5,
			CategoryID:    func() *int { i := 3; return &i }(),
			Name:          "Samsung Galaxy S24",
			Description:   "Android flagship smartphone",
			Price:         func() *float64 { f := 799.99; return &f }(),
			Cost:          func() *float64 { f := 550.00; return &f }(),
			StockQuantity: 150,
			Weight:        func() *float64 { f := 0.168; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  7.06,
				"height": 14.7,
				"depth":  0.76,
			},
			Specifications: map[string]interface{}{
				"cpu":     "Snapdragon 8 Gen 3",
				"ram":     "8GB",
				"storage": "256GB",
				"display": "6.2-inch Dynamic AMOLED",
			},
			Tags:     []string{"samsung", "smartphone", "android"},
			IsActive: true,
		},
		{
			ID:            6,
			CategoryID:    func() *int { i := 7; return &i }(),
			Name:          "Executive Desk",
			Description:   "Premium executive office desk",
			Price:         func() *float64 { f := 899.99; return &f }(),
			Cost:          func() *float64 { f := 400.00; return &f }(),
			StockQuantity: 20,
			Weight:        func() *float64 { f := 45.000; return &f }(),
			Dimensions: map[string]interface{}{
				"width":  152.4,
				"height": 76.2,
				"depth":  76.2,
			},
			Specifications: map[string]interface{}{
				"material":        "Oak wood",
				"drawers":         float64(3),
				"finish":          "Natural",
				"weight_capacity": "100kg",
			},
			Tags:     []string{"furniture", "desk", "executive"},
			IsActive: true,
		},
	},
}