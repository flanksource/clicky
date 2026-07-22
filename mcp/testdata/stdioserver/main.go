package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	mcpServer := server.NewMCPServer("clicky-test", "1.0.0", server.WithToolCapabilities(true))
	mcpServer.AddTool(mcp.NewTool("echo",
		mcp.WithDescription("Echo a message"),
		mcp.WithString("message", mcp.Description("Message to echo"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message, err := request.RequireString("message")
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid message", err), nil
		}
		return mcp.NewToolResultText(message), nil
	})
	mcpServer.AddTool(mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a", mcp.Required()),
		mcp.WithNumber("b", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := request.RequireFloat("a")
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid a", err), nil
		}
		b, err := request.RequireFloat("b")
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid b", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%g", a+b)), nil
	})
	mcpServer.AddTool(mcp.NewTool("wait",
		mcp.WithDescription("Wait until the duration elapses or the request is cancelled"),
		mcp.WithNumber("seconds", mcp.DefaultNumber(10)),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		seconds := request.GetFloat("seconds", 10)
		select {
		case <-time.After(time.Duration(seconds * float64(time.Second))):
			return mcp.NewToolResultText("finished"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err := server.ServeStdio(mcpServer); err != nil {
		panic(err)
	}
}
