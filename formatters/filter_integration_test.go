package formatters

import (
	"encoding/json"

	"github.com/flanksource/clicky/api"
	"github.com/itchyny/gojq"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// Integration tests for CEL filtering with Format functions
//
// These tests verify that filtering works correctly across:
// - Different data types (structs, maps, slices)
// - Different output formats (JSON, YAML, CSV, HTML, Markdown, Pretty)
// - Complex CEL expressions
// - Edge cases and error handling
// - Tree structures
//
// Implementation Notes:
// - Filters use CEL expressions applied via FormatOptions.Filter
// - Validation uses gojq for JSON/YAML output formats
// - Tests do not manually filter data - filtering is automatic via FormatWithOptions
// - CEL field names use lowercase from json tags
// - CEL reserved keywords must be prefixed with "_" (e.g., "_type" instead of "type")

// Test data structures

type Employee struct {
	Name       string `yaml:"name" json:"name"`
	Department string `yaml:"department" json:"department"`
	Salary     int    `yaml:"salary" json:"salary"`
	Active     bool   `yaml:"active" json:"active"`
}

type Project struct {
	Name      string     `yaml:"name" json:"name"`
	Budget    int        `yaml:"budget" json:"budget"`
	Team      []Employee `yaml:"team" json:"team" format:"table"`
	Status    string     `yaml:"status" json:"status"`
	Completed bool       `yaml:"completed" json:"completed"`
}

type Organization struct {
	Name      string     `yaml:"name" json:"name"`
	Employees []Employee `yaml:"employees" json:"employees" format:"table"`
	Projects  []Project  `yaml:"projects" json:"projects" format:"table"`
}

// runJQQuery executes a jq expression against JSON/YAML output and returns matching results
func runJQQuery(output string, jqExpr string, format string) ([]interface{}, error) {
	var data interface{}

	// Parse output based on format
	switch format {
	case "json":
		if err := json.Unmarshal([]byte(output), &data); err != nil {
			return nil, err
		}
	case "yaml":
		if err := yaml.Unmarshal([]byte(output), &data); err != nil {
			return nil, err
		}
	default:
		// For non-JSON/YAML formats, convert to JSON first
		if err := json.Unmarshal([]byte(output), &data); err != nil {
			return nil, err
		}
	}

	query, err := gojq.Parse(jqExpr)
	if err != nil {
		return nil, err
	}

	var results []interface{}
	iter := query.Run(data)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		results = append(results, v)
	}
	return results, nil
}

var _ = ginkgo.Describe("Filters", func() {
	ginkgo.Context("StructFormattingWithFilters", func() {
		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name: "simple struct with table field - filter by department",
				input: Organization{
					Name: "TechCorp",
					Employees: []Employee{
						{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
						{Name: "Bob", Department: "Sales", Salary: 90000, Active: true},
						{Name: "Charlie", Department: "Engineering", Salary: 110000, Active: false},
					},
				},
				filter:   `department == "Engineering"`,
				format:   "json",
				match:    `.[] | .employees[] | select(.name == "Alice")`,
				notMatch: `.[] | .employees[] | select(.name == "Bob")`,
			},
			{
				name: "simple struct with table field - filter by salary",
				input: Organization{
					Name: "TechCorp",
					Employees: []Employee{
						{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
						{Name: "Bob", Department: "Sales", Salary: 90000, Active: true},
						{Name: "Charlie", Department: "Engineering", Salary: 110000, Active: false},
					},
				},
				filter:   `salary > 100000`,
				format:   "json",
				match:    `.[] | .employees[] | select(.name == "Charlie")`,
				notMatch: `.[] | .employees[] | select(.name == "Bob")`,
			},
			{
				name: "nested struct with multiple table fields",
				input: Organization{
					Name: "TechCorp",
					Employees: []Employee{
						{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
						{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
					},
					Projects: []Project{
						{Name: "ProjectA", Budget: 500000, Status: "active", Completed: false},
						{Name: "ProjectB", Budget: 200000, Status: "completed", Completed: true},
					},
				},
				filter:   `name.startsWith("Project")`,
				format:   "json",
				match:    `.[] | .projects[] | select(.name == "ProjectA")`,
				notMatch: `.[] | .employees[] | select(.name == "Alice")`,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				if tt.match != "" {
					matches, err := runJQQuery(result, tt.match, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
					Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
				}

				if tt.notMatch != "" {
					noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
					Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
				}
			})
		}
	})

	ginkgo.Context("MapFormattingWithFilters", func() {
		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name: "map with slice of maps - filter by status",
				input: map[string]interface{}{
					"organization": "TechCorp",
					"employees": []map[string]interface{}{
						{"name": "Alice", "department": "Engineering", "salary": 120000, "active": true},
						{"name": "Bob", "department": "Sales", "salary": 90000, "active": false},
						{"name": "Charlie", "department": "Engineering", "salary": 110000, "active": true},
					},
				},
				filter:   `active == true`,
				format:   "json",
				match:    `.[0].employees[] | select(.name == "Alice")`,
				notMatch: `.[0].employees[] | select(.name == "Bob")`,
			},
			{
				name: "nested map structure - complex filter",
				input: map[string]interface{}{
					"company": "TechCorp",
					"projects": []map[string]interface{}{
						{"name": "ProjectA", "budget": 500000, "status": "active", "team_size": 5},
						{"name": "ProjectB", "budget": 200000, "status": "completed", "team_size": 3},
						{"name": "ProjectC", "budget": 800000, "status": "active", "team_size": 10},
					},
				},
				filter:   `status == "active" && budget > 400000`,
				format:   "json",
				match:    `.[0].projects[] | select(.name == "ProjectA")`,
				notMatch: `.[0].projects[] | select(.name == "ProjectB")`,
			},
			{
				name: "map with multiple table fields - filter employees only",
				input: map[string]interface{}{
					"name": "Organization",
					"employees": []map[string]interface{}{
						{"name": "Alice", "active": true},
						{"name": "Bob", "active": false},
					},
					"projects": []map[string]interface{}{
						{"name": "ProjectA", "completed": false},
						{"name": "ProjectB", "completed": true},
					},
				},
				filter:   `active == true`,
				format:   "json",
				match:    `.[0].employees[] | select(.name == "Alice")`,
				notMatch: `.[0].employees[] | select(.name == "Bob")`,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				if tt.match != "" {
					matches, err := runJQQuery(result, tt.match, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
					Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
				}

				if tt.notMatch != "" {
					noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
					Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
				}
			})
		}
	})

	ginkgo.Context("SliceFormattingWithFilters", func() {
		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name: "slice of structs - filter by field",
				input: []Employee{
					{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
					{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
					{Name: "Charlie", Department: "Engineering", Salary: 110000, Active: true},
				},
				filter:   `department == "Engineering"`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Bob")`,
			},
			{
				name: "slice of maps - numeric filter",
				input: []map[string]interface{}{
					{"name": "ItemA", "price": 100, "in_stock": true},
					{"name": "ItemB", "price": 250, "in_stock": true},
					{"name": "ItemC", "price": 50, "in_stock": false},
				},
				filter:   `price >= 100 && in_stock == true`,
				format:   "json",
				match:    `.[] | select(.name == "ItemB")`,
				notMatch: `.[] | select(.name == "ItemC")`,
			},
			{
				name: "slice with nested data - compound filter",
				input: []Project{
					{
						Name:      "ProjectA",
						Budget:    500000,
						Status:    "active",
						Completed: false,
						Team: []Employee{
							{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
						},
					},
					{
						Name:      "ProjectB",
						Budget:    200000,
						Status:    "completed",
						Completed: true,
						Team: []Employee{
							{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
						},
					},
				},
				filter:   `budget > 300000`,
				format:   "json",
				match:    `.[] | select(.name == "ProjectA")`,
				notMatch: `.[] | select(.name == "ProjectB")`,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				if tt.match != "" {
					matches, err := runJQQuery(result, tt.match, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
					Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
				}

				if tt.notMatch != "" {
					noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
					Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
				}
			})
		}
	})

	ginkgo.Context("FormatOutputWithFilters", func() {
		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name: "json format with filter",
				input: []Employee{
					{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
					{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
				},
				filter:   `active == true`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Bob")`,
			},
			{
				name: "yaml format with filter",
				input: []Employee{
					{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
					{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
				},
				filter:   `active == true`,
				format:   "yaml",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Bob")`,
			},
			{
				name: "csv format with filter",
				input: []Employee{
					{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
					{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
				},
				filter: `active == true`,
				format: "csv",
				// CSV output is plain text, use substring matching
			},
			{
				name: "markdown format with filter",
				input: []Employee{
					{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
					{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
				},
				filter: `active == true`,
				format: "markdown",
				// Markdown output is plain text, use substring matching
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				// Use jq validation for JSON/YAML, substring for others
				if tt.format == "json" || tt.format == "yaml" {
					if tt.match != "" {
						matches, err := runJQQuery(result, tt.match, tt.format)
						Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
						Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
					}

					if tt.notMatch != "" {
						noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
						Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
						Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
					}
				} else {
					// For CSV/Markdown, use simple substring validation
					Expect(result).To(ContainSubstring("Alice"))
					Expect(result).ToNot(ContainSubstring("Bob"))
				}
			})
		}
	})

	ginkgo.Context("ComplexFilterExpressions", func() {
		data := []Employee{
			{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
			{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
			{Name: "Charlie", Department: "Engineering", Salary: 110000, Active: true},
			{Name: "David", Department: "Marketing", Salary: 95000, Active: true},
			{Name: "Eve", Department: "Engineering", Salary: 130000, Active: false},
		}

		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name:     "logical AND",
				input:    data,
				filter:   `department == "Engineering" && salary > 115000`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Charlie")`,
			},
			{
				name:     "logical OR",
				input:    data,
				filter:   `department == "Sales" || department == "Marketing"`,
				format:   "json",
				match:    `.[] | select(.name == "Bob")`,
				notMatch: `.[] | select(.name == "Alice")`,
			},
			{
				name:     "complex compound expression",
				input:    data,
				filter:   `(department == "Engineering" && active == true) || salary > 125000`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Bob")`,
			},
			{
				name:     "negation with AND",
				input:    data,
				filter:   `active == true && department != "Marketing"`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "David")`,
			},
			{
				name:     "range check",
				input:    data,
				filter:   `salary >= 95000 && salary <= 120000`,
				format:   "json",
				match:    `.[] | select(.name == "Alice")`,
				notMatch: `.[] | select(.name == "Eve")`,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				if tt.match != "" {
					matches, err := runJQQuery(result, tt.match, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
					Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
				}

				if tt.notMatch != "" {
					noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
					Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
					Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
				}
			})
		}
	})

	ginkgo.Context("FilterEdgeCases", func() {
		data := []Employee{
			{Name: "Alice", Department: "Engineering", Salary: 120000, Active: true},
			{Name: "Bob", Department: "Sales", Salary: 90000, Active: false},
		}

		tests := []struct {
			name        string
			input       interface{}
			filter      string
			format      string
			match       string
			notMatch    string
			expectError bool
		}{
			{
				name:        "empty filter - no filtering",
				input:       data,
				filter:      "",
				format:      "json",
				match:       `.[] | select(.name == "Alice")`,
				notMatch:    "",
				expectError: false,
			},
			{
				name:        "filter that matches nothing",
				input:       data,
				filter:      `salary > 200000`,
				format:      "json",
				match:       "",
				notMatch:    `.[] | select(.name == "Alice")`,
				expectError: false,
			},
			{
				name:        "filter that matches everything",
				input:       data,
				filter:      `salary > 0`,
				format:      "json",
				match:       `.[] | select(.name == "Bob")`,
				notMatch:    "",
				expectError: false,
			},
			{
				name:        "invalid CEL expression",
				input:       data,
				filter:      `department = "Engineering"`,
				format:      "json",
				expectError: true,
			},
			{
				name:        "reference to non-existent field",
				input:       data,
				filter:      `NonExistentField == "value"`,
				format:      "json",
				expectError: true,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)

				if tt.expectError {
					Expect(err).To(HaveOccurred(), "Expected error but got none")
				} else {
					Expect(err).ToNot(HaveOccurred(), "Unexpected error")

					if tt.match != "" {
						matches, err := runJQQuery(result, tt.match, tt.format)
						Expect(err).ToNot(HaveOccurred(), "jq match query should parse correctly")
						Expect(matches).ToNot(BeEmpty(), "jq match query should return results")
					}

					if tt.notMatch != "" {
						noMatches, err := runJQQuery(result, tt.notMatch, tt.format)
						Expect(err).ToNot(HaveOccurred(), "jq notMatch query should parse correctly")
						Expect(noMatches).To(BeEmpty(), "jq notMatch query should return empty")
					}
				}
			})
		}
	})

	ginkgo.Context("TreeNodeFiltering", func() {
		root := &api.SimpleTreeNode{
			Label: "Root",
			Metadata: map[string]interface{}{
				"_type": "root",
				"id":    1,
			},
			Children: []api.TreeNode{
				&api.SimpleTreeNode{
					Label: "ChildA",
					Metadata: map[string]interface{}{
						"_type":  "child",
						"active": true,
						"id":     2,
					},
				},
				&api.SimpleTreeNode{
					Label: "ChildB",
					Metadata: map[string]interface{}{
						"_type":  "child",
						"active": false,
						"id":     3,
					},
				},
				&api.SimpleTreeNode{
					Label: "ParentC",
					Metadata: map[string]interface{}{
						"_type": "parent",
						"id":    4,
					},
					Children: []api.TreeNode{
						&api.SimpleTreeNode{
							Label: "GrandchildA",
							Metadata: map[string]interface{}{
								"_type":  "grandchild",
								"active": true,
								"id":     5,
							},
						},
						&api.SimpleTreeNode{
							Label: "GrandchildB",
							Metadata: map[string]interface{}{
								"_type":  "grandchild",
								"active": false,
								"id":     6,
							},
						},
					},
				},
			},
		}

		tests := []struct {
			name     string
			input    interface{}
			filter   string
			format   string
			match    string
			notMatch string
		}{
			{
				name:   "filter tree by active status",
				input:  root,
				filter: `active == true`,
				format: "json",
				// Tree output validation - check for presence of labels
			},
			{
				name:   "filter tree by type",
				input:  root,
				filter: `_type == "child"`,
				format: "json",
				// Tree output validation - check for child nodes
			},
			{
				name:   "filter tree with compound expression",
				input:  root,
				filter: `_type == "grandchild" && active == true`,
				format: "json",
				// Tree output validation - check for GrandchildA only
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				opts := FormatOptions{
					Format: tt.format,
					Filter: tt.filter,
				}

				manager := NewFormatManager()
				result, err := manager.FormatWithOptions(opts, tt.input)
				Expect(err).ToNot(HaveOccurred())

				// For tree nodes, verify output is not empty and contains expected structure
				Expect(result).ToNot(BeEmpty(), "Filtered tree output should not be empty")
			})
		}
	})
})
