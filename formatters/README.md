# Non-terminal formatting

Configure the manager before constructing its renderers when formatting HTTP or other non-terminal output:

```go
manager := formatters.NewFormatManager(formatters.ManagerOptions{NoColor: true})
output, err := manager.FormatWithOptions(formatters.FormatOptions{Format: "pretty"}, value)
```

Import `formatters` from `github.com/flanksource/clicky/formatters`. `ManagerOptions.NoColor` selects the explicit no-color theme without querying the terminal and strips ANSI returned by text values or custom formatting callbacks. It remains effective when per-call options omit `NoColor`. Create a manager per concurrent operation; managers hold mutable renderer state.

SwaggerServer always selects this mode for response formatting. Its `format_setup` Server-Timing metric covers manager construction; the existing `format` metric covers rendering. Default CLI managers continue using automatic terminal themes.

Run the regression examples with `go test ./formatters ./rpc -run 'TestFormatters|TestServerTiming' -ginkgo.focus 'Non-terminal|Swagger no-color'`.
