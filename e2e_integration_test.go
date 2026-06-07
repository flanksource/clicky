package clicky_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ExecutionResponse represents the result of command execution (mirrored from rpc package)
type ExecutionResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	Output   string `json:"output,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

var (
	testBinaryPath string
	testBinaryDir  string
)

var _ = BeforeSuite(func() {
	var err error
	testBinaryDir, err = os.MkdirTemp("", "clicky-e2e-*")
	Expect(err).ToNot(HaveOccurred(), "Should create temp dir for test binary")

	binaryName := "clicky"
	if runtime.GOOS == "windows" {
		binaryName = "clicky.exe"
	}
	testBinaryPath = filepath.Join(testBinaryDir, binaryName)

	cmd := exec.Command("go", "build", "-o", testBinaryPath, "./cmd/clicky")
	output, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), "Failed to build clicky binary: %s", string(output))
})

var _ = AfterSuite(func() {
	if testBinaryDir != "" {
		_ = os.RemoveAll(testBinaryDir)
	}
})

var _ = Describe("E2E Clicky Command Execution", func() {
	var (
		binaryPath      string
		exampleDataPath string
		schemaPath      string
	)

	BeforeEach(func() {
		binaryPath = testBinaryPath
		exampleDataPath = "examples/example-data.json"
		schemaPath = "examples/order-schema.yaml"

		// Verify test files exist
		Expect(binaryPath).To(BeAnExistingFile(), "Clicky test binary should exist")
		Expect(exampleDataPath).To(BeAnExistingFile(), "Example data file should exist")
		Expect(schemaPath).To(BeAnExistingFile(), "Schema file should exist")
	})

	Context("when executing valid pretty command", func() {
		It("should execute successfully and produce expected output", func() {
			cmd := exec.Command(binaryPath, "pretty", "--schema", schemaPath, exampleDataPath)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "Command should execute successfully")

			output := stdout.String()
			Expect(output).ToNot(BeEmpty(), "Should have output")
			Expect(output).To(ContainSubstring("ORD-2024-4567"), "Should contain order ID")
			Expect(output).To(ContainSubstring("Acme Corporation"), "Should contain customer name")
		})
	})

	Context("when schema parameter is missing", func() {
		It("should fail with schema error", func() {
			cmd := exec.Command(binaryPath, "pretty", exampleDataPath)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).To(HaveOccurred(), "Command should fail without schema")

			errorOutput := stderr.String()
			Expect(strings.ToLower(errorOutput)).To(ContainSubstring("schema"), "Error should mention schema requirement")
		})
	})

	Context("when schema file is invalid", func() {
		It("should fail with error", func() {
			cmd := exec.Command(binaryPath, "pretty", "--schema", "/nonexistent/schema.yaml", exampleDataPath)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).To(HaveOccurred(), "Command should fail with invalid schema")
		})
	})

	Context("when boolean flags are used", func() {
		It("should work correctly", func() {
			cmd := exec.Command(binaryPath, "pretty", "--schema", schemaPath, "--no-color", exampleDataPath)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "Command should execute successfully with boolean flags")

			output := stdout.String()
			Expect(output).ToNot(BeEmpty(), "Should have output")
		})
	})

	Context("when requesting pretty help", func() {
		It("keeps pretty --help concise and points to the detailed guide", func() {
			cmd := exec.Command(binaryPath, "pretty", "--help")
			cmd.Env = clickyTestEnv(false)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "pretty --help should succeed")

			output := stdout.String()
			Expect(output).To(ContainSubstring("clicky help pretty"))
			Expect(output).ToNot(ContainSubstring("Anti-patterns"))
		})

		It("shows the colored full pretty-printing API guide", func() {
			cmd := exec.Command(binaryPath, "help", "pretty")
			cmd.Env = clickyTestEnv(false)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "help pretty should succeed")

			output := stdout.String()
			Expect(output).To(ContainSubstring("Clicky Pretty Printing"))
			Expect(output).To(ContainSubstring("Intro"))
			Expect(output).To(ContainSubstring("Flag Reference"))
			Expect(output).To(ContainSubstring("--schema string"))
			Expect(output).To(ContainSubstring("Quickstart: Add Pretty() Method"))
			Expect(output).To(ContainSubstring("Add Table Support"))
			Expect(output).To(ContainSubstring("Tree Rendering"))
			Expect(output).To(ContainSubstring("Clicky Components: Reference And Examples"))
			Expect(output).To(ContainSubstring("Anti-patterns"))
			Expect(output).To(ContainSubstring("Struct Tags"))
			Expect(output).To(ContainSubstring("clicky.Format"))
			Expect(output).To(ContainSubstring("clicky.Table"))
			Expect(output).To(ContainSubstring("clicky-json"))
			Expect(output).To(ContainSubstring(".ANSI()"))
			Expect(output).To(ContainSubstring("\x1b["))
		})

		It("honors NO_COLOR for the detailed API guide", func() {
			cmd := exec.Command(binaryPath, "help", "pretty")
			cmd.Env = clickyTestEnv(true)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "NO_COLOR help pretty should succeed")

			output := stdout.String()
			Expect(output).To(ContainSubstring("Clicky Pretty Printing"))
			Expect(output).ToNot(ContainSubstring("\x1b["))
		})
	})

	Context("when running clicky lint", func() {
		It("shows the Gavel-style lint summary for violations", func() {
			dir := writeLintFixtureModule(map[string]string{
				"bad.go": `
package fixture

import "github.com/flanksource/clicky/api"

type Server struct{ Name string }

var direct = api.Text{Content: "bad"}

func (s Server) Pretty() api.Text {
	text := api.Text{}.Append(s.Name)
	return api.Text{}.Append(text.ANSI())
}
`,
				"other.go": `
package fixture

import "github.com/flanksource/clicky/api"

var other = api.Text{Content: "other"}
`,
			})
			cmd := exec.Command(binaryPath, "lint", "--no-color", "--summary-limit", "1", ".")
			cmd.Dir = dir
			cmd.Env = append(clickyTestEnv(true), "GOWORK=off", "GOFLAGS=-mod=mod")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).To(HaveOccurred(), "lint should fail when violations are found")

			output := stdout.String()
			Expect(output).To(ContainSubstring("Lint summary:"))
			Expect(output).To(ContainSubstring("clickylint"))
			Expect(output).To(ContainSubstring("avoid direct api.Text struct literal"))
			Expect(output).To(ContainSubstring("avoid .ANSI() inside clicky render builders"))
			Expect(output).To(ContainSubstring("bad.go"))
			Expect(output).To(ContainSubstring("... 1 more"))
			Expect(stderr.String()).To(ContainSubstring("clickylint found"))
		})

		It("exits successfully for clean packages", func() {
			dir := writeLintFixtureModule(map[string]string{
				"good.go": `
package fixture

import "github.com/flanksource/clicky/api"

type Server struct{ Name string }

func (s Server) Pretty() api.Text {
	return api.Text{}.Append(s.Name)
}
`,
			})
			cmd := exec.Command(binaryPath, "lint", "--no-color", ".")
			cmd.Dir = dir
			cmd.Env = append(clickyTestEnv(true), "GOWORK=off", "GOFLAGS=-mod=mod")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "lint should pass for clean packages: %s", stderr.String())
			Expect(stdout.String()).To(ContainSubstring("Lint summary: 0 violations"))
			Expect(stdout.String()).To(ContainSubstring("clickylint"))
		})

		It("keeps raw analyzer JSON passthrough for old flags", func() {
			dir := writeLintFixtureModule(map[string]string{
				"bad.go": `
package fixture

import "github.com/flanksource/clicky/api"

var direct = api.Text{Content: "bad"}
`,
			})
			cmd := exec.Command(binaryPath, "lint", "-json", ".")
			cmd.Dir = dir
			cmd.Env = append(clickyTestEnv(true), "GOWORK=off", "GOFLAGS=-mod=mod")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "singlechecker -json exits 0 for diagnostics: %s", stderr.String())
			Expect(stdout.String()).To(ContainSubstring(`"clickylint"`))
			Expect(stdout.String()).To(ContainSubstring("avoid direct api.Text struct literal"))
		})

		It("prints structured JSON from the summary runner", func() {
			dir := writeLintFixtureModule(map[string]string{
				"bad.go": `
package fixture

import "github.com/flanksource/clicky/api"

var direct = api.Text{Content: "bad"}
`,
			})
			cmd := exec.Command(binaryPath, "lint", "--format", "json", ".")
			cmd.Dir = dir
			cmd.Env = append(clickyTestEnv(true), "GOWORK=off", "GOFLAGS=-mod=mod")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).To(HaveOccurred(), "structured lint JSON should still exit nonzero for violations")

			var result map[string]interface{}
			Expect(json.Unmarshal(stdout.Bytes(), &result)).To(Succeed(), "stdout should be JSON: %s", stdout.String())
			Expect(result).To(HaveKeyWithValue("linter", "clickylint"))
			Expect(result).To(HaveKey("violations"))
			Expect(stderr.String()).To(ContainSubstring("clickylint found"))
		})

		It("documents the lint display flags in command help", func() {
			cmd := exec.Command(binaryPath, "lint", "--help")
			cmd.Env = clickyTestEnv(true)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "lint --help should succeed: %s", stderr.String())
			Expect(stdout.String()).To(ContainSubstring("--summary-limit"))
			Expect(stdout.String()).To(ContainSubstring("--format"))
			Expect(stdout.String()).To(ContainSubstring("--raw"))
			Expect(stdout.String()).To(ContainSubstring("tree summary"))
		})
	})

	Context("when generating OpenAPI spec", func() {
		It("should produce valid OpenAPI JSON with required components", func() {
			cmd := exec.Command(binaryPath, "openapi", "generate")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred(), "OpenAPI generation should succeed")

			output := stdout.String()
			Expect(output).ToNot(BeEmpty(), "Should have output")

			var spec map[string]interface{}
			err = json.Unmarshal([]byte(output), &spec)
			Expect(err).ToNot(HaveOccurred(), "Output should be valid JSON")

			By("verifying required top-level components")
			Expect(spec).To(HaveKey("openapi"), "Should have openapi version")
			Expect(spec).To(HaveKey("paths"), "Should have paths")
			Expect(spec).To(HaveKey("info"), "Should have info")

			By("verifying pretty endpoint exists")
			paths, ok := spec["paths"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Paths should be a map")
			Expect(paths).To(HaveKey("/api/v1/pretty"), "Should have pretty endpoint")

			By("verifying schema parameter is required")
			prettyPath, ok := paths["/api/v1/pretty"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Pretty path should be a map")

			postMethod, ok := prettyPath["post"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "POST method should exist")

			requestBody, ok := postMethod["requestBody"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Request body should exist")

			content, ok := requestBody["content"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Content should exist")

			jsonContent, ok := content["application/json"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "JSON content should exist")

			schema, ok := jsonContent["schema"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Schema should exist")

			required, ok := schema["required"].([]interface{})
			Expect(ok).To(BeTrue(), "Required fields should exist")

			foundSchema := false
			for _, field := range required {
				if field == "schema" {
					foundSchema = true
					break
				}
			}
			Expect(foundSchema).To(BeTrue(), "Schema should be in required fields")

			By("verifying boolean flags are optional")
			properties, ok := schema["properties"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Properties should exist")

			booleanFlags := []string{"verbose", "no-color", "json", "yaml", "html"}
			for _, flagName := range booleanFlags {
				if prop, exists := properties[flagName]; exists {
					propMap, ok := prop.(map[string]interface{})
					Expect(ok).To(BeTrue(), "Property should be a map")
					Expect(propMap["type"]).To(Equal("boolean"), "Flag %s should be boolean type", flagName)

					foundInRequired := false
					for _, field := range required {
						if field == flagName {
							foundInRequired = true
							break
						}
					}
					Expect(foundInRequired).To(BeFalse(), "Boolean flag %s should not be required", flagName)
				}
			}
		})
	})
})

var _ = Describe("E2E HTTP API With Mock Server", func() {
	var (
		server  *httptest.Server
		baseURL string
	)

	BeforeEach(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/pretty" && r.Method == "POST" {
				handleMockPrettyRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}))
		baseURL = server.URL
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when making valid request with schema", func() {
		It("should succeed and return expected output", func() {
			requestBody := map[string]interface{}{
				"args":   []string{"examples/example-data.json"},
				"schema": "examples/order-schema.yaml",
				"format": "pretty",
			}

			response := makeHTTPRequest(baseURL+"/api/v1/pretty", "POST", requestBody)

			Expect(response.Success).To(BeTrue(), "Request should succeed")
			Expect(response.ExitCode).To(Equal(0), "Exit code should be 0")
			Expect(response.Stdout).ToNot(BeEmpty(), "Should have stdout output")
			Expect(response.Stdout).To(ContainSubstring("ORD-2024-4567"), "Should contain order ID")
		})
	})

	Context("when schema parameter is missing", func() {
		It("should fail with schema error", func() {
			requestBody := map[string]interface{}{
				"args":   []string{"examples/example-data.json"},
				"format": "pretty",
			}

			response := makeHTTPRequestExpectError(baseURL+"/api/v1/pretty", "POST", requestBody)

			Expect(response.Success).To(BeFalse(), "Request should fail")
			Expect(response.ExitCode).ToNot(Equal(0), "Exit code should not be 0")
			Expect(strings.ToLower(response.Error)).To(ContainSubstring("schema"), "Error should mention schema")
		})
	})

	Context("when boolean flags are provided", func() {
		It("should accept them as optional parameters", func() {
			requestBody := map[string]interface{}{
				"args":     []string{"examples/example-data.json"},
				"schema":   "examples/order-schema.yaml",
				"verbose":  true,
				"no-color": false,
			}

			response := makeHTTPRequest(baseURL+"/api/v1/pretty", "POST", requestBody)

			Expect(response.Success).To(BeTrue(), "Request with boolean flags should succeed")
			Expect(response.ExitCode).To(Equal(0), "Exit code should be 0")
		})
	})
})

// Helper functions

func makeHTTPRequest(url, method string, body interface{}) *ExecutionResponse {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		Expect(err).ToNot(HaveOccurred(), "Should marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	Expect(err).ToNot(HaveOccurred(), "Should create HTTP request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	Expect(err).ToNot(HaveOccurred(), "Should execute HTTP request")
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred(), "Should read response body")

	Expect(resp.StatusCode).To(Equal(http.StatusOK), "Should get 200 OK, got %d: %s", resp.StatusCode, string(responseBody))

	var execResponse ExecutionResponse
	err = json.Unmarshal(responseBody, &execResponse)
	Expect(err).ToNot(HaveOccurred(), "Should unmarshal execution response")

	return &execResponse
}

func makeHTTPRequestExpectError(url, method string, body interface{}) *ExecutionResponse {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		Expect(err).ToNot(HaveOccurred(), "Should marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	Expect(err).ToNot(HaveOccurred(), "Should create HTTP request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	Expect(err).ToNot(HaveOccurred(), "Should execute HTTP request")
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred(), "Should read response body")

	Expect(resp.StatusCode).To(BeNumerically(">=", 200), "Should get valid HTTP status")
	Expect(resp.StatusCode).To(BeNumerically("<", 600), "Should get valid HTTP status")

	var execResponse ExecutionResponse
	err = json.Unmarshal(responseBody, &execResponse)
	Expect(err).ToNot(HaveOccurred(), "Should unmarshal execution response")

	return &execResponse
}

func handleMockPrettyRequest(w http.ResponseWriter, r *http.Request) {
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		response := &ExecutionResponse{
			Success:  false,
			Error:    fmt.Sprintf("Failed to parse request: %v", err),
			ExitCode: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	schema, hasSchema := reqBody["schema"].(string)
	if !hasSchema || schema == "" {
		response := &ExecutionResponse{
			Success:  false,
			Error:    "--schema flag is required",
			ExitCode: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	mockOutput := `📋 Order Details
ID: ORD-2024-4567
Customer: Acme Corporation
Status: PROCESSING
Total: $15,750.00 USD`

	response := &ExecutionResponse{
		Success:  true,
		Message:  "Command executed successfully",
		Stdout:   mockOutput,
		Stderr:   "",
		ExitCode: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func clickyTestEnv(noColor bool) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "NO_COLOR=") ||
			strings.HasPrefix(item, "COLOR=") ||
			strings.HasPrefix(item, "TERM=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "COLOR=", "TERM=xterm-256color")
	if noColor {
		env = append(env, "NO_COLOR=1")
	} else {
		env = append(env, "NO_COLOR=")
	}
	return env
}

func writeLintFixtureModule(files map[string]string) string {
	dir := GinkgoT().TempDir()
	repoRoot, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred(), "Should resolve repository root for lint fixture")

	mod := fmt.Sprintf(`module example.com/clickylintfixture

go 1.26.1

require github.com/flanksource/clicky v0.0.0

replace github.com/flanksource/clicky => %s
`, filepath.ToSlash(repoRoot))
	Expect(os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644)).To(Succeed())

	for name, content := range files {
		path := filepath.Join(dir, name)
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)).To(Succeed())
	}
	return dir
}
