package clicky

import (
	"fmt"
	"time"
	"github.com/flanksource/clicky/api"
	"github.com/charmbracelet/lipgloss"
)

// Usage examples demonstrating all features of the Pretty formatter

// ExampleUsage demonstrates basic usage of the Pretty formatter
func ExampleUsage() {
	parser := NewPrettyParser()

	// Example 1: Basic struct formatting
	type Person struct {
		Name     string    `json:"name"`
		Age      int       `json:"age" pretty:"color,green=>18,red=<18"`
		Salary   float64   `json:"salary" pretty:"currency"`
		JoinDate time.Time `json:"join_date" pretty:"date"`
	}

	person := Person{
		Name:     "John Doe",
		Age:      25,
		Salary:   75000.50,
		JoinDate: time.Now(),
	}

	result, _ := parser.Parse(person)
	fmt.Println("=== PERSON ===")
	fmt.Println(result)
}

// ExampleTable demonstrates table formatting with sorting
func ExampleTable() {
	parser := NewPrettyParser()

	type Employee struct {
		ID     string  `json:"id" pretty:"hide"`
		Name   string  `json:"name"`
		Salary float64 `json:"salary" pretty:"currency"`
		Rating float64 `json:"rating" pretty:"float,digits=1"`
		Status string  `json:"status" pretty:"color,green=active,red=inactive"`
	}

	type Department struct {
		Name      string     `json:"name"`
		Employees []Employee `json:"employees" pretty:"table,sort=salary,dir=desc"`
		Budget    float64    `json:"budget" pretty:"currency"`
	}

	dept := Department{
		Name:   "Engineering",
		Budget: 500000.00,
		Employees: []Employee{
			{ID: "E001", Name: "Alice", Salary: 95000, Rating: 4.5, Status: "active"},
			{ID: "E002", Name: "Bob", Salary: 85000, Rating: 4.2, Status: "active"},
			{ID: "E003", Name: "Charlie", Salary: 105000, Rating: 4.8, Status: "inactive"},
		},
	}

	result, _ := parser.Parse(dept)
	fmt.Println("\n=== DEPARTMENT WITH TABLE ===")
	fmt.Println(result)
}

// ExampleColorConditions demonstrates advanced color conditions
func ExampleColorConditions() {
	parser := NewPrettyParser()

	type ServerMetrics struct {
		ServerName string  `json:"server_name"`
		CPUUsage   float64 `json:"cpu_usage" pretty:"color,green=<70,yellow=<90,red=>=90"`
		Memory     float64 `json:"memory" pretty:"color,green=<80,yellow=<95,red=>=95"`
		DiskSpace  float64 `json:"disk_space" pretty:"color,green=<85,yellow=<95,red=>=95"`
		Status     string  `json:"status" pretty:"color,green=healthy,yellow=warning,red=critical"`
		Uptime     int     `json:"uptime_days" pretty:"color,green=>=30,yellow=>=7,red=<7"`
	}

	servers := []ServerMetrics{
		{ServerName: "web-01", CPUUsage: 45.2, Memory: 67.8, DiskSpace: 23.1, Status: "healthy", Uptime: 45},
		{ServerName: "web-02", CPUUsage: 78.5, Memory: 89.3, DiskSpace: 92.7, Status: "warning", Uptime: 12},
		{ServerName: "db-01", CPUUsage: 92.1, Memory: 96.4, DiskSpace: 98.2, Status: "critical", Uptime: 3},
	}

	fmt.Println("\n=== SERVER METRICS ===")
	for _, server := range servers {
		result, _ := parser.Parse(server)
		fmt.Println(result)
		fmt.Println()
	}
}

// ExampleComplexFormatting demonstrates complex nested structures
func ExampleComplexFormatting() {
	parser := NewPrettyParser()

	type Transaction struct {
		ID        string  `json:"id"`
		Amount    float64 `json:"amount" pretty:"currency"`
		Type      string  `json:"type" pretty:"color,green=credit,red=debit"`
		Timestamp string  `json:"timestamp" pretty:"date,format=epoch"`
	}

	type Account struct {
		AccountNumber string        `json:"account_number"`
		AccountName   string        `json:"account_name"`
		Balance       float64       `json:"balance" pretty:"currency"`
		Status        string        `json:"status" pretty:"color,green=active,red=closed,yellow=frozen"`
		Transactions  []Transaction `json:"transactions" pretty:"table,sort=amount,dir=desc"`
		LastUpdate    string        `json:"last_update" pretty:"date,format=epoch"`
	}

	account := Account{
		AccountNumber: "ACC-001-2024",
		AccountName:   "John Doe Savings",
		Balance:       12500.75,
		Status:        "active",
		LastUpdate:    fmt.Sprintf("%d", time.Now().Unix()),
		Transactions: []Transaction{
			{ID: "TXN-001", Amount: 2500.00, Type: "credit", Timestamp: "1704067200"},
			{ID: "TXN-002", Amount: -150.00, Type: "debit", Timestamp: "1704070800"},
			{ID: "TXN-003", Amount: 1000.00, Type: "credit", Timestamp: "1704074400"},
			{ID: "TXN-004", Amount: -75.25, Type: "debit", Timestamp: "1704078000"},
		},
	}

	result, _ := parser.Parse(account)
	fmt.Println("\n=== BANK ACCOUNT ===")
	fmt.Println(result)
}

// ExampleThemes demonstrates custom themes
func ExampleThemes() {
	parser := NewPrettyParser()

	// Custom dark theme
	darkTheme := api.Theme{
		Primary:   lipgloss.Color("#BB86FC"), // Purple
		Secondary: lipgloss.Color("#03DAC6"), // Teal
		Success:   lipgloss.Color("#4CAF50"), // Green
		Warning:   lipgloss.Color("#FF9800"), // Orange
		Error:     lipgloss.Color("#F44336"), // Red
		Info:      lipgloss.Color("#2196F3"), // Blue
		Muted:     lipgloss.Color("#9E9E9E"), // Gray
	}

	parser.Theme = darkTheme

	type Product struct {
		Name      string  `json:"name"`
		Price     float64 `json:"price" pretty:"currency"`
		Stock     int     `json:"stock" pretty:"color,green=>10,yellow=>0,red=0"`
		Category  string  `json:"category"`
		Rating    float64 `json:"rating" pretty:"float,digits=1"`
		Available bool    `json:"available" pretty:"color,green=true,red=false"`
	}

	product := Product{
		Name:      "Wireless Headphones",
		Price:     199.99,
		Stock:     5,
		Category:  "Electronics",
		Rating:    4.5,
		Available: true,
	}

	result, _ := parser.Parse(product)
	fmt.Println("\n=== PRODUCT (DARK THEME) ===")
	fmt.Println(result)
}

// ExampleJSONParsing demonstrates lenient JSON parsing
func ExampleJSONParsing() {
	fmt.Println("\n=== LENIENT JSON PARSING ===")

	samples := []string{
		`{"name": "Standard JSON", "valid": true}`,
		`{"name": "With Comment", /* this is ok */ "valid": true}`,
		`{"name": "Trailing Comma", "valid": true,}`,
		`"{\"name\": \"Quoted JSON\", \"valid\": true}"`,
		`{name: "Unquoted Keys", valid: false}`,
	}

	for i, sample := range samples {
		fmt.Printf("\nSample %d: %s\n", i+1, sample)
		result, err := ParseJSON([]byte(sample))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Parsed: %+v (Type: %T)\n", result, result)
		}
	}
}

// ExampleNoColor demonstrates output without colors (for CI/logging)
func ExampleNoColor() {
	parser := NewPrettyParser()
	parser.NoColor = true

	type LogEntry struct {
		Level     string `json:"level" pretty:"color,green=INFO,yellow=WARN,red=ERROR"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp" pretty:"date,format=epoch"`
		Source    string `json:"source"`
	}

	logEntry := LogEntry{
		Level:     "ERROR",
		Message:   "Database connection failed",
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Source:    "database.go:42",
	}

	result, _ := parser.Parse(logEntry)
	fmt.Println("\n=== LOG ENTRY (NO COLOR) ===")
	fmt.Println(result)
}

// RunAllExamples runs all examples
func RunAllExamples() {
	fmt.Println("🎨 PRETTY FORMATTER EXAMPLES 🎨")
	fmt.Println("================================")

	ExampleUsage()
	ExampleTable()
	ExampleColorConditions()
	ExampleComplexFormatting()
	ExampleThemes()
	ExampleJSONParsing()
	ExampleNoColor()

	fmt.Println("\n✅ All examples completed!")
}
