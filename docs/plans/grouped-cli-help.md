# Group shared CLI help by purpose

Status: implemented; Gavel TODO `8fd5c639-37c5-48fd-a636-c26093b290c8` verified with 14/14 definition-of-done checks on 2026-10-25.

## Summary

Extend Clicky's existing grouped usage renderer and enable it in Captain and Gavel. Show command-specific flags first, followed by model selection, model permissions, model runtime, formatting, logging, tasks, custom categories, and remaining global flags. Preserve positional usage, flag parsing, defaults, hidden flags, and local overrides.

## Implementation

- Add a `category` struct tag and `FieldInfo.Category`; bind it to the existing `clicky_category` annotation through shared flag helpers.
- Merge local and inherited categories, sort flags alphabetically, omit empty sections, and show every visible flag once.
- Annotate Captain's shared model and runtime definitions and manually bound AI flags in the supported command trees.
- Annotate both Clicky formatting binders. Enable grouped usage in the Clicky, Captain, and Gavel roots without replacing custom help guides.
- Document the category API with a runnable example.

## Acceptance criteria

- Command-specific options precede reusable categories; inherited uncategorized flags remain separate.
- Nested typed options and entity-generated action flags retain categories.
- Local flags override inherited flags without duplication; hidden flags and empty categories stay absent.
- Captain prompt help retains terminal colors, plain output, and agent Markdown behavior.
- Fresh Clicky, Captain, and Gavel help shows the groups; focused tests pass, and build/lint results distinguish unrelated failures.

## Verification

Use focused Ginkgo regressions and the companion `grouped-cli-help.fixture.md` for the durable Gavel definition of done. Run `make lint`, `make build`, and scoped diff checks in all three repositories. No commits or publishing are included.

## Implementation checkpoint

All three builds, focused Go vet checks, the runnable example, scoped formatting/diff checks, and the canonical TODO fixture passed. The fixture covers six focused regression commands and eight help smoke tests against the freshly built binaries. Custom Gavel guides reuse the shared groups while preserving their styling. Unrelated working-tree changes were preserved; no commits were created.

Full lint is not passing: Clicky's default cached linter was built with Go 1.26 while the checkout uses Go 1.27; using the existing compatible linter reports unrelated errcheck/staticcheck findings in `exec/process_control.go` and `task/control.go`. Captain reports duplicate main functions in `hack/`. Gavel's non-mutating golangci run reports an unused `writeJSON` helper in `testrunner/ui/traces.go`; its full `make lint` target was not run because it also performs `go mod tidy` in the dirty checkout. No lint suppression or unrelated source changes were added.
