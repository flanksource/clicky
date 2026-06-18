# ANSI Output Fixtures

Tests that clicky produces correct ANSI output under various terminal and color settings.

---
build: go build -o {{.executablePath}} ./cmd/clicky
cwd: ..
---

## --no-color flag

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| suppresses all ANSI | {{.executablePath}} --schema examples/order-schema.yaml --no-color --no-progress examples/example-data.json | !ansi.has_any && stdout.contains("ORD-2024-4567") |
| preserves content | {{.executablePath}} --schema examples/order-schema.yaml --no-color --no-progress examples/example-data.json | stdout.contains("Acme Corporation") && stdout.contains("processing") |

## Default output (colored)

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| has color codes | env TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | ansi.has_color |
| has content | env TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | stdout.contains("ORD-2024-4567") |

## NO_COLOR env var

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| NO_COLOR=true suppresses ANSI | env NO_COLOR=true TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | !ansi.has_any |
| NO_COLOR=1 suppresses ANSI | env NO_COLOR=1 TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | !ansi.has_any |

## COLOR env var

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| COLOR=no suppresses ANSI | env COLOR=no TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | !ansi.has_any |
| COLOR=false suppresses ANSI | env COLOR=false TERM=xterm-256color {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | !ansi.has_any |

## TERM=dumb

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| TERM=dumb suppresses ANSI | env TERM=dumb {{.executablePath}} --schema examples/order-schema.yaml --no-progress examples/example-data.json | !ansi.has_any |

## Non-pretty formats never have ANSI

| Test Name | CLI Args | CEL |
|-----------|----------|-----|
| json has no ANSI | {{.executablePath}} --schema examples/order-schema.yaml --format json --no-progress examples/example-data.json | !ansi.has_any |
| yaml has no ANSI | {{.executablePath}} --schema examples/order-schema.yaml --format yaml --no-progress examples/example-data.json | !ansi.has_any |
