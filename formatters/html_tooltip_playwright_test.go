package formatters

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/playwright-community/playwright-go"
)

func TestHTMLTooltipsInBrowser(t *testing.T) {
	// Install playwright browsers if needed
	if err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	}); err != nil {
		t.Fatalf("Failed to install playwright: %v", err)
	}

	// Start playwright
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start playwright: %v", err)
	}
	defer pw.Stop()

	// Launch browser
	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer browser.Close()

	t.Run("Table with tooltips", func(t *testing.T) {
		testTableTooltips(t, browser)
	})

	t.Run("Grid.js table with tooltips", func(t *testing.T) {
		testGridJSTableTooltips(t, browser)
	})

	t.Run("Tree with tooltips", func(t *testing.T) {
		testTreeTooltips(t, browser)
	})
}

func testTableTooltips(t *testing.T, browser playwright.Browser) {
	// Create PrettyData with tooltips
	data := &api.PrettyData{
		Schema: &api.PrettyObject{
			Fields: []api.PrettyField{
				{
					Name:   "products",
					Type:   "table",
					Format: api.FormatTable,
					TableOptions: api.PrettyTable{
						Fields: []api.PrettyField{
							{Name: "name", Type: "string", Label: "Product Name"},
							{Name: "status", Type: "string", Label: "Status"},
							{Name: "price", Type: "float", Label: "Price", Format: api.FormatCurrency},
						},
					},
				},
			},
		},
		Tables: map[string][]api.PrettyDataRow{
			"products": {
				{
					"name": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "Widget A",
							}.WithTooltip(api.Text{Content: "Premium quality widget"})
							return &t
						}(),
						Field: api.PrettyField{Name: "name"},
					},
					"status": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "Active",
								Style:   "text-green-600",
							}.WithTooltip(api.Text{Content: "Currently in production"})
							return &t
						}(),
						Field: api.PrettyField{Name: "status"},
					},
					"price": api.FieldValue{
						Value: 99.99,
						Field: api.PrettyField{Name: "price", Format: api.FormatCurrency},
					},
				},
				{
					"name": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "Widget B",
							}.WithTooltip(api.Text{Content: "Standard quality widget"})
							return &t
						}(),
						Field: api.PrettyField{Name: "name"},
					},
					"status": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "Inactive",
								Style:   "text-red-600",
							}.WithTooltip(api.Text{Content: "Discontinued product"})
							return &t
						}(),
						Field: api.PrettyField{Name: "status"},
					},
					"price": api.FieldValue{
						Value: 149.99,
						Field: api.PrettyField{Name: "price", Format: api.FormatCurrency},
					},
				},
			},
		},
	}

	// Generate HTML with static table (PDF mode = true)
	formatter := NewHTMLFormatter()
	formatter.IsPDFMode = true // Use static tables instead of Grid.js
	html, err := formatter.FormatPrettyData(data)
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Test tooltips in the browser
	testTooltipsInHTML(t, browser, html, []tooltipTest{
		{
			selector:        "text=Widget A",
			expectedTooltip: "Premium quality widget",
		},
		{
			selector:        "text=Active",
			expectedTooltip: "Currently in production",
		},
		{
			selector:        "text=Widget B",
			expectedTooltip: "Standard quality widget",
		},
		{
			selector:        "text=Inactive",
			expectedTooltip: "Discontinued product",
		},
	})
}

func testGridJSTableTooltips(t *testing.T, browser playwright.Browser) {
	// Create test data with tooltips for Grid.js table
	data := &api.PrettyData{
		Schema: &api.PrettyObject{
			Fields: []api.PrettyField{
				{
					Name:   "items",
					Type:   "table",
					Format: api.FormatTable,
					TableOptions: api.PrettyTable{
						Fields: []api.PrettyField{
							{Name: "id", Type: "string", Label: "ID"},
							{Name: "description", Type: "string", Label: "Description"},
						},
					},
				},
			},
		},
		Tables: map[string][]api.PrettyDataRow{
			"items": {
				{
					"id": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "001",
							}.WithTooltip(api.Text{Content: "First item identifier"})
							return &t
						}(),
						Field: api.PrettyField{Name: "id"},
					},
					"description": api.FieldValue{
						Text: func() *api.Text {
							t := api.Text{
								Content: "Test Item",
							}.WithTooltip(api.Text{Content: "This is a test item with special chars: \"quotes\" & <tags>"})
							return &t
						}(),
						Field: api.PrettyField{Name: "description"},
					},
				},
			},
		},
	}

	// Generate HTML with Grid.js (PDF mode = false)
	formatter := NewHTMLFormatter()
	formatter.IsPDFMode = false // Use Grid.js interactive tables
	html, err := formatter.FormatPrettyData(data)
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Test tooltips in Grid.js table
	// Need to wait for Grid.js to render before checking tooltips
	testTooltipsInHTML(t, browser, html, []tooltipTest{
		{
			selector:        "text=001",
			expectedTooltip: "First item identifier",
			waitForGridJS:   true,
		},
		{
			selector:        "text=Test Item",
			expectedTooltip: "This is a test item with special chars: \"quotes\" & <tags>",
			waitForGridJS:   true,
		},
	})
}

// TreeNodeWithTooltip is a custom tree node that supports tooltips
type TreeNodeWithTooltip struct {
	Label    string
	Icon     string
	Style    string
	Tooltip  api.Text
	Children []api.TreeNode
}

func (n *TreeNodeWithTooltip) Pretty() api.Text {
	text := api.Text{Content: n.Label}

	// Add icon if present
	if n.Icon != "" {
		text.Content = n.Icon + " " + text.Content
	}

	// Apply style if present
	if n.Style != "" {
		text.Style = n.Style
	}

	// Add tooltip
	if !n.Tooltip.IsEmpty() {
		text = text.WithTooltip(n.Tooltip)
	}

	return text
}

func (n *TreeNodeWithTooltip) GetChildren() []api.TreeNode {
	return n.Children
}

func testTreeTooltips(t *testing.T, browser playwright.Browser) {
	// Create a tree with tooltips
	tree := &TreeNodeWithTooltip{
		Label:   "Project",
		Icon:    "📁",
		Style:   "text-blue-600 font-bold",
		Tooltip: api.Text{Content: "Root project directory"},
		Children: []api.TreeNode{
			&TreeNodeWithTooltip{
				Label:   "src",
				Icon:    "📁",
				Style:   "text-blue-500",
				Tooltip: api.Text{Content: "Source code directory"},
				Children: []api.TreeNode{
					&TreeNodeWithTooltip{
						Label:   "main.go",
						Icon:    "🐹",
						Style:   "text-green-500",
						Tooltip: api.Text{Content: "Main application file"},
					},
				},
			},
			&TreeNodeWithTooltip{
				Label:   "README.md",
				Icon:    "📝",
				Style:   "text-gray-500",
				Tooltip: api.Text{Content: "Project documentation with \"quotes\" & special <chars>"},
			},
		},
	}

	// Generate HTML
	formatter := NewHTMLFormatter()
	html, err := formatter.Format(tree)
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Test tooltips in tree
	testTooltipsInHTML(t, browser, html, []tooltipTest{
		{
			selector:        "text=Project",
			expectedTooltip: "Root project directory",
		},
		{
			selector:        "text=src",
			expectedTooltip: "Source code directory",
		},
		{
			selector:        "text=main.go",
			expectedTooltip: "Main application file",
		},
		{
			selector:        "text=README.md",
			expectedTooltip: "Project documentation with \"quotes\" & special <chars>",
		},
	})
}

type tooltipTest struct {
	selector        string
	expectedTooltip string
	waitForGridJS   bool
}

func testTooltipsInHTML(t *testing.T, browser playwright.Browser, html string, tests []tooltipTest) {
	// Save HTML to temp file
	tmpFile, err := os.CreateTemp("", "tooltip-test-*.html")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(html); err != nil {
		t.Fatalf("Failed to write HTML: %v", err)
	}
	tmpFile.Close()

	// Create a new page
	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Navigate to the HTML file
	fileURL := "file://" + filepath.ToSlash(tmpFile.Name())
	if _, err := page.Goto(fileURL); err != nil {
		t.Fatalf("Failed to navigate to HTML: %v", err)
	}

	// Wait for DOMContentLoaded
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateDomcontentloaded,
	}); err != nil {
		t.Fatalf("Failed to wait for load state: %v", err)
	}

	// Run each tooltip test
	for _, test := range tests {
		t.Run(fmt.Sprintf("tooltip_%s", test.selector), func(t *testing.T) {
			// If this is a Grid.js table, wait for it to render
			if test.waitForGridJS {
				// Wait for Grid.js to initialize
				if _, err := page.WaitForSelector(".gridjs-wrapper", playwright.PageWaitForSelectorOptions{
					State:   playwright.WaitForSelectorStateVisible,
					Timeout: playwright.Float(5000),
				}); err != nil {
					t.Fatalf("Grid.js table did not render: %v", err)
				}
				// Give Grid.js time to complete rendering and tooltip initialization
				time.Sleep(500 * time.Millisecond)
			}

			// Find the element
			locator := page.Locator(test.selector)
			count, err := locator.Count()
			if err != nil {
				t.Fatalf("Failed to count elements: %v", err)
			}
			if count == 0 {
				t.Fatalf("Element not found: %s", test.selector)
			}

			// Hover over the element to trigger tooltip
			if err := locator.First().Hover(); err != nil {
				t.Fatalf("Failed to hover over element: %v", err)
			}

			// Wait for tooltip to appear
			time.Sleep(1000 * time.Millisecond)

			// Debug: Check if tooltip-target class was added
			hasTooltipTarget, _ := page.Evaluate(`() => {
				const targets = document.querySelectorAll('.tooltip-target');
				return targets.length;
			}`)
			if count, ok := hasTooltipTarget.(float64); ok {
				t.Logf("Found %d elements with tooltip-target class", int(count))
			}

			// Check if tooltip exists with expected content
			// Tippy.js creates tooltips with data-tippy-root attribute
			tooltipVisible, err := page.Evaluate(`() => {
				const tooltips = document.querySelectorAll('[data-tippy-root]');
				for (let tooltip of tooltips) {
					if (tooltip.style.visibility !== 'hidden' &&
						tooltip.style.display !== 'none' &&
						tooltip.offsetParent !== null) {
						return tooltip.textContent;
					}
				}
				return null;
			}`)
			if err != nil {
				t.Fatalf("Failed to check tooltip: %v", err)
			}

			if tooltipVisible == nil {
				// Try taking a screenshot for debugging
				screenshotPath := filepath.Join(".playwright-mcp", fmt.Sprintf("tooltip-fail-%d.png", time.Now().Unix()))
				os.MkdirAll(".playwright-mcp", 0755)
				page.Screenshot(playwright.PageScreenshotOptions{
					Path: &screenshotPath,
				})
				t.Fatalf("Tooltip did not appear for %s (screenshot: %s)", test.selector, screenshotPath)
			}

			tooltipText, ok := tooltipVisible.(string)
			if !ok {
				t.Fatalf("Tooltip content is not a string")
			}

			if tooltipText != test.expectedTooltip {
				t.Errorf("Expected tooltip %q, got %q", test.expectedTooltip, tooltipText)
			}
		})
	}
}
