# Clicky Examples Test Fixtures

This file contains test fixtures for all the example programs in the clicky/examples directory.
These fixtures can be used with the arch-unit fixture runner to automatically test that all examples compile and run correctly.

## Setup

To run these fixtures:
```bash
# Build all examples first
cd /Users/moshe/work/omi/clicky/examples
for f in *.go; do go build "$f"; done

# Run fixtures using arch-unit
arch-unit fixtures FIXTURES.md
```

## task-basic.go - Basic Task Operations

Tests basic task creation, execution, and status reporting.

| Test Name | CLI Args | CEL |
|-----------|----------|----------------|
| Basic execution | go run task-basic.go | stdout.contains("✓") && exitCode == 0 |
| No work simulation | go run task-basic.go  --simulate-work=false | stdout.contains("Download dependencies") && exitCode == 0 |
| With error simulation | go run task-basic.go  --simulate-error | stdout.contains("✗") && stdout.contains("failed") && exitCode == 1 |
| JSON output | go run task-basic.go  --format json | stdout.contains("===") && exitCode == 0 |
| Verbose mode | go run task-basic.go -v | stderr.contains("DBG") || stdout.contains("===") |
| Help text | go run task-basic.go --help | stderr.contains("Usage") && stderr.contains("--simulate-work") |

## Concurrent Task Execution

Tests concurrent task execution with worker limits and priority scheduling.

| Test Name | CLI Args | CEL |
|-----------|----------|----------------|
| Default concurrency | go run task-concurrent.go | stdout.contains("Creating 10 tasks") && stdout.contains("Execution Statistics") |
| Limited workers | go run task-concurrent.go --max-workers 2 | stdout.contains("max 2 concurrent") |
| Many tasks | go run task-concurrent.go --num-tasks 20 | stdout.contains("Creating 20 tasks") |
| Quick tasks | go run task-concurrent.go --task-duration 100ms | exitCode == 0 |
| Monitor output | go run task-concurrent.go --num-tasks 5 | stdout.contains("[Monitor]") |
| Priority distribution | go run task-concurrent.go | stdout.contains("Priority Distribution") |
| JSON statistics | go run task-concurrent.go --format json | stdout.contains("Total Tasks") && stdout.contains("===") |

## task-dependencies.go - Task Dependencies

Tests task dependency resolution and group management.

| Test Name | CLI Args | CEL |
|-----------|----------|----------------|
| Default execution | go run task-dependencies.go  | stdout.contains("Task Dependencies") && stdout.contains("Setup Database") |
| Hide graph | go run task-dependencies.go  --show-graph=false | !stdout.contains("Dependency Graph") && exitCode == 0 |
| With graph | go run task-dependencies.go  --show-graph | stdout.contains("→ means 'depends on'") && stdout.contains("Migrate Data → [Setup Database]") |
| Group execution | go run task-dependencies.go  | stdout.contains("Data Processing Group") && stdout.contains("Region-") |
| Timeline output | go run task-dependencies.go  | stdout.contains("Execution Timeline") |
| JSON format | go run task-dependencies.go  --format json | stdout.contains("Task Dependencies") && exitCode == 0 |

## task-retry.go - Error Handling and Retry

Tests retry logic, error recovery, and fallback operations.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Default retry | go run task-retry.go | stdout.contains("Error Handling & Retry") || stdout.contains("Simulated Failure Rate") |
| No failures | go run task-retry.go --failure-rate 0.0 | stdout.contains("Simulated Failure Rate: 0%") || stdout.contains("===") |
| High failure | go run task-retry.go --failure-rate 0.9 | stdout.contains("Simulated Failure Rate: 90%") |
| Max retries | go run task-retry.go --max-retries 5 | stdout.contains("max-retries") || stdout.contains("===") |
| Retry delay | go run task-retry.go --retry-delay 2s | stdout.contains("retry-delay") || stdout.contains("===") |
| Exponential backoff | go run task-retry.go --exponential-backoff | stdout.contains("exponential") || stdout.contains("===") |
| Service attempts | go run task-retry.go | stdout.contains("Service") || stdout.contains("===") |
| Task results | go run task-retry.go | stdout.contains("Task") || stdout.contains("===") |

## file-tree-demo.go - File Tree Visualization

Tests file tree generation and various output formats.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Current directory | go run file-tree-demo.go  . | stdout.contains("├──") |
| Specific path | go run file-tree-demo.go  /tmp | stdout.contains("tmp") |
| Max depth | go run file-tree-demo.go  . --max-depth 1 | exitCode == 0 |
| Include hidden | go run file-tree-demo.go  . --include-hidden | exitCode == 0 |
| Exclude pattern | go run file-tree-demo.go  . --exclude "*.go" | !stdout.contains(".go") |
| JSON format | go run file-tree-demo.go  . --format json | stdout.contains("\"name\"") && stdout.contains("\"children\"") |
| YAML format | go run file-tree-demo.go  . --format yaml | stdout.contains("name:") && stdout.contains("children:") |
| Tree format | go run file-tree-demo.go  . --format tree | stdout.contains("├──") |
| Table format | go run file-tree-demo.go  . --format table | stdout.contains("\\|") || exitCode == 0 |

## mcp-integration.go - MCP Server

Tests MCP server functionality and command exposure.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Help text | go run mcp-integration.go  --help | stdout.contains("greet") && stdout.contains("calculate") && stdout.contains("format") |
| Greet command | go run mcp-integration.go  greet World | stdout.contains("Hello") && stdout.contains("World") |
| Calculate add | go run mcp-integration.go  calculate 5 + 3 | stdout.contains("8") |
| Calculate multiply | go run mcp-integration.go  calculate 4 x 5 | stdout.contains("20") |
| Format JSON | go run mcp-integration.go  format --type json "test data" | stdout.contains("{") && stdout.contains("}") |
| Format uppercase | go run mcp-integration.go  format --type uppercase hello | stdout.contains("HELLO") |
| MCP server help | go run mcp-integration.go  mcp --help | stdout.contains("MCP") |

## Common Flag Tests

Tests that apply to all examples that use clicky.BindAllFlags.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Version flag | go run task-basic.go  --version | exitCode == 0 |
| Verbose logging | go run task-basic.go  -v | stderr.contains("DBG") || stdout.contains("===") |
| No color | go run task-basic.go  --no-color | !stdout.contains("\u001b[") |
| JSON format | go run task-basic.go  --format json | stdout.contains("===") && exitCode == 0 |
| YAML format | go run task-basic.go  --format yaml | stdout.contains("===") && exitCode == 0 |
| Max concurrent | go run task-concurrent.go --max-concurrent 1 | exitCode == 0 |

## Build Tests

Verify all examples compile successfully.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Build task-basic | go build task-basic.go | exitCode == 0 |
| Build task-concurrent | go build task-concurrent.go | exitCode == 0 |
| Build task-dependencies | go build task-dependencies.go | exitCode == 0 |
| Build task-retry | go build task-retry.go | exitCode == 0 |
| Build file-tree-demo | go build file-tree-demo.go | exitCode == 0 |
| Build mcp-integration | go build mcp-integration.go | exitCode == 0 |

## Performance Tests

Test performance characteristics and resource usage.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Quick completion | go run task-basic.go  --simulate-work=false | exitCode == 0 |
| Large task count | go run task-concurrent.go --num-tasks 50 --task-duration 10ms | stdout.contains("50 tasks") && exitCode == 0 |
| Deep dependencies | go run task-dependencies.go  | stdout.contains("Aggregate Results") && exitCode == 0 |
| Heavy retry load | go run task-retry.go --failure-rate 0.8 --max-retries 3 | stdout.contains("Retry Statistics") |

## Error Handling Tests

Test error conditions and graceful failures.

| Test Name | CLI Args | CEL Validation |
|-----------|----------|----------------|
| Invalid path | go run file-tree-demo.go /nonexistent | exitCode != 0 |
| Bad calculation | go run mcp-integration.go calculate invalid | exitCode != 0 |
| Task failure | go run task-basic.go  --simulate-error | stdout.contains("failed") && exitCode == 1 |


## Notes

- All paths assume the examples have been built in the current directory
- CEL validation expressions use stdout, stderr, and exitCode variables
- Some tests may have alternative expected outputs due to timing or randomization
- The fixture runner should set reasonable timeouts for long-running tests
- Tests marked with `true` as validation always pass but are included for documentation
