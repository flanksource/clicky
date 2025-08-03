package clicky

type Invoice struct {
	ID         string        `json:"id"`
	Items      []InvoiceItem `json:"items" pretty:"table,sort=amount,dir=desc"`
	Total      float64       `json:"total"`
	CreatedAt  string        `json:"created_at" pretty:"date,format=epoch"`
	CustomerID string        `json:"customer_id"`

	Status string `json:"status" pretty:"color,green=paid,red=unpaid,blue=pending"`
}

type InvoiceItem struct {
	ID          string  `json:"id" pretty:"hide"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"  pretty:"currency"`
	Quantity    float64 `json:"quantity" pretty:"float,digits:2"`
	Total       float64 `json:"total" pretty:"currency"`
}
