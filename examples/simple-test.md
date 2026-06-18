---
exec: "go run cmd/clicky/main.go"
---

| Test Name | CWD | CLI Args | Expected Output | CEL Validation |
|-----------|-----|----------|-----------------|----------------|
| Simple Date Test | . | --schema example/date-test-schema.yaml example/date-test-data.json | Updated At: 2024-08-01 | output.contains("Updated At: 2024-08-01") |