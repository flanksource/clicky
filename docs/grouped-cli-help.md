# Grouped CLI help

Use `clicky.BindAllFlagsToCommand(root, "format", "tasks")` to register shared flags and enable grouped usage throughout a Cobra command tree. A CLI that binds its flags separately can call `clicky.SetGroupedUsage(root)` instead. Existing custom help can embed `cmd.UsageString()` to include the grouped flag reference.

Add `category` tags to typed option fields. Categories travel through nested anonymous structs and work for both typed commands and entity actions:

```go
type ModelOptions struct {
    Model  string `flag:"model" category:"Model selection" help:"Model to use"`
    Access string `flag:"access" category:"Model permissions" help:"Access contract"`
}

type RenderOptions struct {
    ModelOptions
    File string `flag:"file" help:"Document to render"`
}
```

Run the complete example from the Clicky checkout:

```bash
go run ./examples/grouped-help/main.go render --help
```

## Colored help

Call `clicky.SetColoredHelp` to install grouped usage and themed `--help` for a Cobra command tree. `DefaultHelpTheme()` supplies Clicky style classes for headings, usage, commands, flags, hints, and values. Bash examples use the value style for quoted strings, numbers, and variables. Flag descriptions wrap at word boundaries with indented continuation lines.

```go
theme := clicky.DefaultHelpTheme()
theme.Heading = "text-indigo-600 font-bold"
clicky.SetColoredHelp(root, theme)
```

Help uses color on terminal output and respects `NO_COLOR` and the `--no-color` flag when registered. `cmd.UsageString()` returns plain text for embedding. For custom help, use `clicky.CommandHelp{Command: cmd, Theme: theme}.Pretty()` to compose the help with other Clicky content, or pass the struct to `clicky.Format`.

## Categories

| Section | Contents |
| --- | --- |
| Command flags | Uncategorized local flags, such as inputs, filters, and command actions |
| Model selection | Model, fallback models, execution mode, reasoning effort, temperature, and model comparisons |
| Model permissions | Access, sandbox, permission posture, permission sets, and tool policies |
| Model runtime | Endpoints, credentials, budgets, limits, timeouts, presets, sessions, and ambient context |
| Formatting | Output formats, destinations, colors, row filtering, and display structure |
| Logging | Log levels and log presentation |
| Tasks | Concurrency, retries, progress, and shutdown |
| Global flags | Uncategorized persistent and inherited flags |

Empty sections are omitted. Custom category names follow the known categories in alphabetical order, before Global flags. Local and inherited flags share a category section, with flags sorted alphabetically and local overrides taking precedence. Hidden flags stay hidden. Positional arguments remain in the command's usage signature.

The Formatting section lists output shortcuts such as `--html` and `--pdf` together immediately below the `--format` description.

## Manually bound flags

Import `github.com/flanksource/clicky/flags` and call `flags.SetCategory(cmd.Flags().Lookup("model"), flags.CategoryModelSelection)` after registering a flag. `SetCategory` requires a registered flag and a nonempty category. `flags.Category(flag)` reads its category, returning an empty string for uncategorized flags. The pflag annotation key remains `clicky_category`, also available as `flags.CategoryAnnotation` and `clicky.FlagCategoryAnnotation`.

Custom renderers can iterate `clicky.CommandFlagGroups(cmd)` to obtain the same ordered, nonempty sections as the default usage renderer. Each `FlagGroup` supplies a `Title` and a pflag `Flags` set; the returned sets retain flag values and annotations without changing their ownership on the command.

Captain's `aiflags.ModelFlags` already supplies model selection categories. Its `AIProviderOptions`, `AIRuntimeOptions`, and prompt option structs supply runtime and permission categories, so downstream commands inherit the grouping when embedding those types. Both Clicky formatting binders annotate their flags automatically.
