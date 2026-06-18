---
exec: "go run cmd/clicky/main.go"
---

## Date Formatting Tests - All Formats

| Test Name | CWD | CLI Args | Expected Output | CEL Validation |
|-----------|-----|----------|-----------------|----------------|
| Pretty Format - RFC3339 Input | . | --schema example/date-test-schema.yaml example/date-test-data.json | Created At: 2024-08-01 10:30:00 | output.contains("Created At: 2024-08-01 10:30:00") |
| Pretty Format - ISO Date Input | . | --schema example/date-test-schema.yaml example/date-test-data.json | Updated At: 2024-08-01 | output.contains("Updated At: 2024-08-01") |
| Pretty Format - Custom Format String | . | --schema example/date-test-schema.yaml example/date-test-data.json | Scheduled At: 01/08/2024 | output.contains("Scheduled At: 01/08/2024") |
| Pretty Format - Custom Format String 2 | . | --schema example/date-test-schema.yaml example/date-test-data.json | Completed At: 08/01/2024 | output.contains("Completed At: 08/01/2024") |
| Pretty Format - Table Date Formatting | . | --schema example/date-test-schema.yaml example/date-test-data.json | 2025-08-01 03:00:00 | output.contains("2025-08-01 03:00:00") |
| JSON Format - Raw RFC3339 | . | --schema example/date-test-schema.yaml example/date-test-data.json --json | "created_at":"2024-08-01T10:30:00Z" | output.contains('"created_at":"2024-08-01T10:30:00Z"') |
| JSON Format - Raw ISO Date | . | --schema example/date-test-schema.yaml example/date-test-data.json --json | "updated_at":"2024-08-01" | output.contains('"updated_at":"2024-08-01"') |
| JSON Format - Items Array | . | --schema example/date-test-schema.yaml example/date-test-data.json --json | "items": | output.contains('"items":') |
| YAML Format - Raw RFC3339 | . | --schema example/date-test-schema.yaml example/date-test-data.json --yaml | created_at: 2024-08-01T10:30:00Z | output.contains("created_at: 2024-08-01T10:30:00Z") |
| YAML Format - ISO Date | . | --schema example/date-test-schema.yaml example/date-test-data.json --yaml | updated_at: 2024-08-01 | output.contains("updated_at: 2024-08-01") |
| YAML Format - Structure Present | . | --schema example/date-test-schema.yaml example/date-test-data.json --yaml | payment: | output.contains("payment:") |
| CSV Format - Headers Present | . | --schema example/date-test-schema.yaml example/date-test-data.json --csv | order_date,created_at,updated_at | output.contains("order_date,created_at,updated_at") |
| CSV Format - Epoch Formatted | . | --schema example/date-test-schema.yaml example/date-test-data.json --csv | 2024-08-01 03:00:00 | output.contains("2024-08-01 03:00:00") |
| CSV Format - RFC3339 Formatted | . | --schema example/date-test-schema.yaml example/date-test-data.json --csv | "Aug 01, 2024 10:30 AM" | output.contains('"Aug 01, 2024 10:30 AM"') |
| HTML Format - Timestamp Formatted | . | --schema example/date-test-schema.yaml example/date-test-data.json --html | 2024-08-01 03:00:00 | output.contains("2024-08-01 03:00:00") |
| HTML Format - RFC3339 Formatted | . | --schema example/date-test-schema.yaml example/date-test-data.json --html | Aug 01, 2024 10:30 AM | output.contains("Aug 01, 2024 10:30 AM") |
| HTML Format - Table Date Cell | . | --schema example/date-test-schema.yaml example/date-test-data.json --html | 2025-08-01 03:00:00 | output.contains("2025-08-01 03:00:00") |
| Markdown Format - Field Label | . | --schema example/date-test-schema.yaml example/date-test-data.json --markdown | **order_date**: | output.contains("**order_date**:") |
| Markdown Format - Date Value | . | --schema example/date-test-schema.yaml example/date-test-data.json --markdown | 2024-08-01 03:00:00 | output.contains("2024-08-01 03:00:00") |
| Markdown Format - Custom Format | . | --schema example/date-test-schema.yaml example/date-test-data.json --markdown | Aug 01, 2024 10:30 AM | output.contains("Aug 01, 2024 10:30 AM") |
| Format Flag - Pretty | . | --schema example/date-test-schema.yaml example/date-test-data.json --format pretty | Created At: 2024-08-01 10:30:00 | output.contains("Created At: 2024-08-01 10:30:00") |
| Format Flag - JSON | . | --schema example/date-test-schema.yaml example/date-test-data.json --format json | "updated_at":"2024-08-01" | output.contains('"updated_at":"2024-08-01"') |
| Format Flag - CSV | . | --schema example/date-test-schema.yaml example/date-test-data.json --format csv | 2024-08-01 03:00:00 | output.contains("2024-08-01 03:00:00") |
| PDF Format - Generate | . | --schema example/date-test-schema.yaml example/date-test-data.json --pdf date-test.pdf | | !output.contains("error") && !output.contains("Error") |

## Date Formatting Edge Cases

| Test Name | CWD | CLI Args | Expected Output | CEL Validation |
|-----------|-----|----------|-----------------|----------------|
| Zero Unix Timestamp - Pretty | . | --schema example/date-test-schema.yaml example/zero-date.json | Created At: 1970-01-01T00:00:00Z | output.contains("Created At: 1970-01-01T00:00:00Z") |
| Future Date - Pretty | . | --schema example/date-test-schema.yaml example/future-date.json | Created At: 2030-12-31T23:59:59Z | output.contains("Created At: 2030-12-31T23:59:59Z") |
| Leap Year Date - Pretty | . | --schema example/date-test-schema.yaml example/leap-year.json | Updated At: 2024-02-29 | output.contains("Updated At: 2024-02-29") |
| Invalid Date String - Pretty | . | --schema example/date-test-schema.yaml example/invalid-date.json | invalid-date-string | output.contains("invalid-date-string") |

## Using Existing Order Schema

| Test Name | CWD | CLI Args | Expected Output | CEL Validation |
|-----------|-----|----------|-----------------|----------------|
| Order Schema - Pretty Format | . | --schema example/order-schema.yaml example/example-data.json | Order Date: epoch | output.contains("Order Date: epoch") |
| Order Schema - JSON Format | . | --schema example/order-schema.yaml example/example-data.json --json | "order_date":"1722470400" | output.contains('"order_date":"1722470400"') |
| Order Schema - Basic Structure | . | --schema example/order-schema.yaml example/example-data.json | Status: processing | output.contains("Status: processing") |

