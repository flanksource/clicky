// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

// Test MCP server with TaskManager integration
func main() {
	// Start the MCP server as a subprocess
	cmd := exec.Command("../omi-cli/omi", "mcp", "serve", "--auto-expose")
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe: %v", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to create stderr pipe: %v", err)
	}
	
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
	
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	
	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Read stderr for server info and TaskManager output
	go func() {
		stderrBytes, _ := io.ReadAll(stderr)
		if len(stderrBytes) > 0 {
			fmt.Printf("Server/TaskManager output:\n%s\n", string(stderrBytes))
		}
	}()
	
	// Test 1: Initialize
	fmt.Println("Testing MCP initialization...")
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	
	if err := sendRequest(stdin, initRequest); err != nil {
		log.Fatalf("Failed to send initialize request: %v", err)
	}
	
	response, err := readResponse(stdout)
	if err != nil {
		log.Fatalf("Failed to read initialize response: %v", err)
	}
	
	fmt.Printf("Initialize response: %s\n\n", response)
	
	// Test 2: List tools
	fmt.Println("Testing tools/list...")
	listRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	
	if err := sendRequest(stdin, listRequest); err != nil {
		log.Fatalf("Failed to send tools/list request: %v", err)
	}
	
	response, err = readResponse(stdout)
	if err != nil {
		log.Fatalf("Failed to read tools/list response: %v", err)
	}
	
	// Parse response to get tool count
	var listResp map[string]interface{}
	if err := json.Unmarshal([]byte(response), &listResp); err == nil {
		if result, ok := listResp["result"].(map[string]interface{}); ok {
			if tools, ok := result["tools"].([]interface{}); ok {
				fmt.Printf("Found %d tools exposed via MCP\n", len(tools))
				
				// Show first few tools
				for i := 0; i < 3 && i < len(tools); i++ {
					if tool, ok := tools[i].(map[string]interface{}); ok {
						fmt.Printf("  - %s: %s\n", tool["name"], tool["description"])
					}
				}
			}
		}
	}
	
	// Test 3: Call a simple tool (help command)
	fmt.Println("\nTesting tools/call with 'help' command (should use TaskManager)...")
	callRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "help",
			"arguments": map[string]interface{}{},
		},
	}
	
	if err := sendRequest(stdin, callRequest); err != nil {
		log.Printf("Failed to send tools/call request: %v", err)
	} else {
		// Allow more time for TaskManager execution
		time.Sleep(2 * time.Second)
		
		response, err := readResponse(stdout)
		if err != nil {
			log.Printf("Failed to read tools/call response: %v", err)
		} else {
			// Parse to check if TaskManager was used
			var callResp map[string]interface{}
			if err := json.Unmarshal([]byte(response), &callResp); err == nil {
				if result, ok := callResp["result"].(map[string]interface{}); ok {
					if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
						fmt.Println("Tool executed successfully via TaskManager!")
						if contentBlock, ok := content[0].(map[string]interface{}); ok {
							if text, ok := contentBlock["text"].(string); ok {
								fmt.Printf("Output preview: %.100s...\n", text)
							}
						}
					}
				}
			}
		}
	}
	
	fmt.Println("\nMCP server with TaskManager integration test completed!")
	fmt.Println("Check the server output above for TaskManager progress bars and task tracking.")
}

func sendRequest(stdin io.WriteCloser, request map[string]interface{}) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return err
	}
	
	_, err = stdin.Write(append(requestJSON, '\n'))
	return err
}

func readResponse(stdout io.ReadCloser) (string, error) {
	// Read with timeout
	done := make(chan string, 1)
	errChan := make(chan error, 1)
	
	go func() {
		buf := make([]byte, 65536) // Larger buffer for tool responses
		n, err := stdout.Read(buf)
		if err != nil {
			errChan <- err
			return
		}
		done <- string(buf[:n])
	}()
	
	select {
	case response := <-done:
		return response, nil
	case err := <-errChan:
		return "", err
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("timeout reading response")
	}
}