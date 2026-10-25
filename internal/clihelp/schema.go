package clihelp

import _ "embed"

//go:embed schema-documentation.txt
var SchemaDocumentation string

//go:embed example-schema.yaml
var ExampleSchema string
