package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/clicky/mcp"
	"github.com/spf13/cobra"
)

// Example of integrating MCP into your CLI application
func main() {
	// Create your root command
	rootCmd := &cobra.Command{
		Use:   "myapp",
		Short: "My application with MCP support",
		Long:  `An example CLI application that exposes commands as MCP tools.`,
	}
	
	// Add your application commands
	rootCmd.AddCommand(newGreetCommand())
	rootCmd.AddCommand(newCalculateCommand())
	rootCmd.AddCommand(newFormatCommand())
	
	// Add MCP command to expose your CLI as an MCP server
	rootCmd.AddCommand(mcp.NewCommand())
	
	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Example command 1: Greet
func newGreetCommand() *cobra.Command {
	var name string
	var formal bool
	
	cmd := &cobra.Command{
		Use:   "greet",
		Short: "Greet someone",
		Long:  `Greets a person with their name.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			greeting := "Hello"
			if formal {
				greeting = "Good day"
			}
			
			if name == "" {
				name = "World"
			}
			
			fmt.Printf("%s, %s!\n", greeting, name)
			return nil
		},
	}
	
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name to greet")
	cmd.Flags().BoolVarP(&formal, "formal", "f", false, "Use formal greeting")
	
	return cmd
}

// Example command 2: Calculate
func newCalculateCommand() *cobra.Command {
	var operation string
	var x, y float64
	
	cmd := &cobra.Command{
		Use:   "calculate",
		Short: "Perform a calculation",
		Long:  `Performs basic arithmetic operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var result float64
			
			switch operation {
			case "add":
				result = x + y
			case "subtract":
				result = x - y
			case "multiply":
				result = x * y
			case "divide":
				if y == 0 {
					return fmt.Errorf("cannot divide by zero")
				}
				result = x / y
			default:
				return fmt.Errorf("unknown operation: %s", operation)
			}
			
			fmt.Printf("Result: %f\n", result)
			return nil
		},
	}
	
	cmd.Flags().StringVarP(&operation, "operation", "o", "add", "Operation: add, subtract, multiply, divide")
	cmd.Flags().Float64VarP(&x, "x", "x", 0, "First number")
	cmd.Flags().Float64VarP(&y, "y", "y", 0, "Second number")
	cmd.MarkFlagRequired("x")
	cmd.MarkFlagRequired("y")
	
	return cmd
}

// Example command 3: Format
func newFormatCommand() *cobra.Command {
	var text string
	var style string
	
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format text",
		Long:  `Formats text in various styles.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch style {
			case "upper":
				fmt.Println(strings.ToUpper(text))
			case "lower":
				fmt.Println(strings.ToLower(text))
			case "title":
				fmt.Println(strings.Title(text))
			case "reverse":
				runes := []rune(text)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				fmt.Println(string(runes))
			default:
				fmt.Println(text)
			}
			return nil
		},
	}
	
	cmd.Flags().StringVarP(&text, "text", "t", "", "Text to format")
	cmd.Flags().StringVarP(&style, "style", "s", "plain", "Style: upper, lower, title, reverse, plain")
	cmd.MarkFlagRequired("text")
	
	return cmd
}

// Usage:
// 1. Build the application:
//    go build -o myapp mcp-integration.go
//
// 2. Run as a regular CLI:
//    ./myapp greet --name Alice
//    ./myapp calculate -x 10 -y 5 -o multiply
//    ./myapp format -t "Hello World" -s upper
//
// 3. Run as an MCP server:
//    ./myapp mcp serve
//
// 4. Configure for Claude Desktop (add to claude_desktop_config.json):
//    {
//      "mcpServers": {
//        "myapp": {
//          "command": "/path/to/myapp",
//          "args": ["mcp", "serve"]
//        }
//      }
//    }
//
// 5. View MCP configuration:
//    ./myapp mcp config