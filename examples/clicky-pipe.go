package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/flanksource/clicky"
)

// Order represents a complete order with all details
type Order struct {
	ID                string      `json:"id" pretty:"color,blue"`
	Customer          Customer    `json:"customer"`
	Status            string      `json:"status" pretty:"color,green=completed,yellow=processing,orange=pending,red=cancelled"`
	Priority          string      `json:"priority" pretty:"color,red=high,yellow=medium,green=low"`
	TotalAmount       float64     `json:"total_amount" pretty:"currency"`
	Currency          string      `json:"currency"`
	OrderDate         string      `json:"order_date" pretty:"date,format=epoch"`
	EstimatedDelivery string      `json:"estimated_delivery" pretty:"date,format=epoch"`
	ShippingAddress   Address     `json:"shipping_address"`
	Items             []OrderItem `json:"items" pretty:"table,sort=line_total,dir=desc"`
	Payment           PaymentInfo `json:"payment"`
	Notes             string      `json:"notes"`
	InternalReference string      `json:"internal_reference"`
}

type Customer struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	AccountType string `json:"account_type" pretty:"color,gold=premium,silver=business,blue=standard"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}

type OrderItem struct {
	SKU             string  `json:"sku" pretty:"hide"`
	Name            string  `json:"name"`
	Category        string  `json:"category" pretty:"color,blue=Electronics,green=Accessories,purple=Software"`
	Quantity        int     `json:"quantity"`
	UnitPrice       float64 `json:"unit_price" pretty:"currency"`
	DiscountPercent float64 `json:"discount_percent" pretty:"float,digits=1"`
	LineTotal       float64 `json:"line_total" pretty:"currency"`
	WarrantyMonths  int     `json:"warranty_months" pretty:"color,green=>=36,yellow=>=24,red=<24"`
}

type PaymentInfo struct {
	Method            string `json:"method"`
	CardLastFour      string `json:"card_last_four"`
	AuthorizationCode string `json:"authorization_code"`
	ProcessedAt       string `json:"processed_at" pretty:"date,format=epoch"`
}

func main() {
	var data []byte
	var err error

	// Read from stdin if available, otherwise read from example file
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Error reading from stdin: %v", err)
		}
	} else {
		// Read from example file
		data, err = os.ReadFile("example-data.json")
		if err != nil {
			log.Fatalf("Error reading example-data.json: %v", err)
		}
	}

	// Parse JSON
	var order Order
	if err := json.Unmarshal(data, &order); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	// Create format manager
	formatManager := clicky.NewFormatManager()

	// Get format from command line argument or default to pretty
	format := "pretty"
	if len(os.Args) > 1 {
		format = strings.ToLower(os.Args[1])
	}

	// Format and display
	var result string
	switch format {
	case "json":
		result, err = formatManager.JSON(order)
	case "yaml":
		result, err = formatManager.YAML(order)
	case "csv":
		result, err = formatManager.CSV(order)
	case "html":
		result, err = formatManager.HTML(order)
	case "markdown":
		result, err = formatManager.Markdown(order)
	case "pretty":
		fallthrough
	default:
		result, err = formatManager.Pretty(order)
	}

	if err != nil {
		log.Fatalf("Error formatting with %s: %v", format, err)
	}

	fmt.Println(result)
}
