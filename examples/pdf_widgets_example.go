package main

import (
	"fmt"
	"log"
	"os"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters/pdf"
)

func main() {
	// Create a new PDF builder
	builder := pdf.NewBuilder()

	// Configure document with headers and footers using api.Class styling
	builder.Header = api.Text{
		Content: "PDF Widgets Demo",
		Class: api.Class{
			Font: &api.Font{
				Size: 1.5, // 1.5rem
				Bold: true,
			},
			Foreground: &api.Color{Hex: "#2563eb"}, // Blue-600
			Padding: &api.Padding{
				Bottom: 0.5,
			},
		},
	}

	builder.Footer = api.Text{
		Content: "Generated with Clicky PDF Widgets",
		Class: api.Class{
			Font: &api.Font{
				Size: 0.8,
			},
			Foreground: &api.Color{Hex: "#6b7280"}, // Gray-500
		},
	}

	builder.PageNumbers = true

	// Add first page
	builder.AddPage()

	// Example 1: Text Widget with api.Class styling
	fmt.Println("Adding styled text widget...")
	
	textWidget := pdf.Text{
		Text: api.Text{
			Content: "Welcome to PDF Widgets with API Class Styling!",
			Class: api.Class{
				Font: &api.Font{
					Size:   1.2,
					Bold:   true,
					Italic: false,
				},
				Foreground: &api.Color{Hex: "#dc2626"}, // Red-600
				Background: &api.Color{Hex: "#fef2f2"}, // Red-50
				Padding: &api.Padding{
					Top:    0.5,
					Bottom: 0.5,
					Left:   1.0,
					Right:  1.0,
				},
			},
		},
	}

	err := builder.DrawWidget(textWidget)
	if err != nil {
		log.Fatalf("Failed to draw text widget: %v", err)
	}

	// Move down a bit
	builder.MoveBy(0, 20)

	// Example 2: Text Widget using Tailwind-style resolved classes
	fmt.Println("Adding text with Tailwind-resolved styling...")
	
	// Use our ResolveStyles function to convert Tailwind classes to api.Class
	tailwindClasses := "text-green-600 font-bold text-lg p-3 bg-green-50"
	resolvedClass := api.ResolveStyles(tailwindClasses)

	tailwindTextWidget := pdf.Text{
		Text: api.Text{
			Content: "This text was styled using Tailwind classes: " + tailwindClasses,
			Class:   resolvedClass,
		},
	}

	err = builder.DrawWidget(tailwindTextWidget)
	if err != nil {
		log.Fatalf("Failed to draw Tailwind text widget: %v", err)
	}

	// Move down a bit more
	builder.MoveBy(0, 25)

	// Example 3: Table Widget with styling
	fmt.Println("Adding styled table widget...")
	
	tableWidget := pdf.Table{
		Headers: []string{"Product", "Price", "Quantity", "Total"},
		Rows: [][]any{
			{"Widget A", "$10.99", 5, "$54.95"},
			{"Widget B", "$15.50", 3, "$46.50"},
			{"Widget C", "$8.75", 7, "$61.25"},
			{"Widget D", "$22.00", 2, "$44.00"},
		},
		HeaderStyle: api.Class{
			Font: &api.Font{
				Bold: true,
				Size: 1.0,
			},
			Background: &api.Color{Hex: "#1f2937"}, // Gray-800
			Foreground: &api.Color{Hex: "#ffffff"}, // White
		},
		RowStyle: api.Class{
			Font: &api.Font{
				Size: 0.9,
			},
			Foreground: &api.Color{Hex: "#374151"}, // Gray-700
		},
		CellPadding: api.Padding{
			Top:    0.25,
			Bottom: 0.25,
			Left:   0.5,
			Right:  0.5,
		},
	}

	err = builder.DrawWidget(tableWidget)
	if err != nil {
		log.Fatalf("Failed to draw table widget: %v", err)
	}

	// Move down
	builder.MoveBy(0, 30)

	// Example 4: Box Widget with labels
	fmt.Println("Adding box widget with labels...")
	
	boxWidget := pdf.Box{
		Rectangle: api.Rectangle{Width: 120, Height: 80},
		Labels: []pdf.Label{
			{
				Text: api.Text{
					Content: "Process Box",
					Class: api.Class{
						Font: &api.Font{
							Bold: true,
							Size: 1.1,
						},
						Foreground: &api.Color{Hex: "#1f2937"},
					},
				},
				Positionable: pdf.Positionable{
					Position: &pdf.LabelPosition{
						Vertical:   pdf.VerticalCenter,
						Horizontal: pdf.HorizontalCenter,
					},
				},
			},
			{
				Text: api.Text{
					Content: "Step 1",
					Class: api.Class{
						Font: &api.Font{Size: 0.8},
						Foreground: &api.Color{Hex: "#6b7280"},
					},
				},
				Positionable: pdf.Positionable{
					Position: &pdf.LabelPosition{
						Vertical:   pdf.VerticalTop,
						Horizontal: pdf.HorizontalLeft,
						Inside:     pdf.InsideBottom, // Outside the box
					},
				},
			},
		},
		Borders: &api.Borders{
			Top: api.Line{
				Color: api.Color{Hex: "#3b82f6"},
				Width: 2,
				Style: api.Solid,
			},
			Right: api.Line{
				Color: api.Color{Hex: "#3b82f6"},
				Width: 2,
				Style: api.Solid,
			},
			Bottom: api.Line{
				Color: api.Color{Hex: "#3b82f6"},
				Width: 2,
				Style: api.Solid,
			},
			Left: api.Line{
				Color: api.Color{Hex: "#3b82f6"},
				Width: 2,
				Style: api.Solid,
			},
		},
	}

	err = builder.DrawWidget(boxWidget)
	if err != nil {
		log.Fatalf("Failed to draw box widget: %v", err)
	}

	// Move to next position
	builder.MoveBy(0, 20)

	// Example 5: Image Widget (placeholder)
	fmt.Println("Adding image widget...")
	
	imageWidget := pdf.Image{
		Source:  "", // Empty source creates placeholder
		AltText: "Chart or Diagram",
		Width:   floatPtr(80),
		Height:  floatPtr(60),
	}

	err = builder.DrawWidget(imageWidget)
	if err != nil {
		log.Fatalf("Failed to draw image widget: %v", err)
	}

	// Example 6: Complex text with children
	builder.MoveBy(0, 20)
	
	complexTextWidget := pdf.Text{
		Text: api.Text{
			Content: "Summary Report",
			Class: api.Class{
				Font: &api.Font{
					Size: 1.3,
					Bold: true,
				},
				Foreground: &api.Color{Hex: "#1f2937"},
			},
			Children: []api.Text{
				{
					Content: "This document demonstrates the PDF widget system with full api.Class support.",
					Class: api.Class{
						Font: &api.Font{Size: 1.0},
						Foreground: &api.Color{Hex: "#4b5563"},
						Padding: &api.Padding{Top: 0.5},
					},
				},
				{
					Content: "Key Features:",
					Class: api.Class{
						Font: &api.Font{
							Size: 1.0,
							Bold: true,
						},
						Foreground: &api.Color{Hex: "#1f2937"},
						Padding: &api.Padding{Top: 0.5},
					},
				},
				{
					Content: "• Full api.Class styling support",
					Class: api.Class{
						Font: &api.Font{Size: 0.9},
						Foreground: &api.Color{Hex: "#4b5563"},
						Padding: &api.Padding{Left: 0.5, Top: 0.2},
					},
				},
				{
					Content: "• Tailwind utility class parsing",
					Class: api.Class{
						Font: &api.Font{Size: 0.9},
						Foreground: &api.Color{Hex: "#4b5563"},
						Padding: &api.Padding{Left: 0.5, Top: 0.2},
					},
				},
				{
					Content: "• Comprehensive widget system",
					Class: api.Class{
						Font: &api.Font{Size: 0.9},
						Foreground: &api.Color{Hex: "#4b5563"},
						Padding: &api.Padding{Left: 0.5, Top: 0.2},
					},
				},
			},
		},
	}

	err = builder.DrawWidget(complexTextWidget)
	if err != nil {
		log.Fatalf("Failed to draw complex text widget: %v", err)
	}

	// Generate the final PDF
	fmt.Println("Generating PDF...")
	pdfData, err := builder.Output()
	if err != nil {
		log.Fatalf("Failed to generate PDF: %v", err)
	}

	// Save to file
	filename := "pdf_widgets_demo.pdf"
	err = os.WriteFile(filename, pdfData, 0644)
	if err != nil {
		log.Fatalf("Failed to save PDF: %v", err)
	}

	fmt.Printf("PDF generated successfully: %s (%d bytes)\n", filename, len(pdfData))
	fmt.Println("The PDF demonstrates:")
	fmt.Println("- Text widgets with api.Class styling")
	fmt.Println("- Tailwind class resolution using api.ResolveStyles()")
	fmt.Println("- Table widgets with header and row styling")
	fmt.Println("- Box widgets with positioned labels and borders")
	fmt.Println("- Image widgets (placeholder)")
	fmt.Println("- Complex text with nested children")
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}