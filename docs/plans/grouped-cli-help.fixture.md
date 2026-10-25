---
cwd: $GIT_ROOT_DIR
codeBlocks: [bash]
---

# Grouped CLI help verification

## Focused regressions

| Name | CWD | Command | Exit Code |
| --- | --- | --- | --- |
| Nested flag categories | $GIT_ROOT_DIR | go test ./flags -run '^TestFlags$' -ginkgo.focus='flag categories' | 0 |
| Shared help grouping | $GIT_ROOT_DIR | go test . -run '^TestClicky$' -ginkgo.focus='grouped CLI help' | 0 |
| Existing grouped help | $GIT_ROOT_DIR | go test . -run '^TestBindAllFlagsToCommand_GroupsHelpOutput$' | 0 |
| Captain prompt help | $GIT_ROOT_DIR/../captain | go test ./pkg/cli -run '^TestSessionGetMulti$' -ginkgo.focus='prompt help' | 0 |
| Captain runtime flags | $GIT_ROOT_DIR/../captain | go test ./pkg/cli -run '^TestSessionGetMulti$' -ginkgo.focus='grouped runtime flags' | 0 |
| Gavel command help | $GIT_ROOT_DIR/../gavel | go test ./cmd/gavel -run '^TestGavelCLI$' -ginkgo.focus='grouped command help' | 0 |

## Fresh binary smoke tests

Build each repository with `make build` before running these checks.

| Name | CWD | Command | CEL Validation |
| --- | --- | --- | --- |
| Clicky root | $GIT_ROOT_DIR | ./clicky --help | stdout.contains('Command flags:') && stdout.contains('Formatting:') |
| Clicky child | $GIT_ROOT_DIR | ./clicky pretty --help | stdout.contains('Command flags:') && stdout.contains('Formatting:') |
| Captain root | $GIT_ROOT_DIR/../captain | ./.bin/captain --help --no-color | stdout.contains('Command flags:') && stdout.contains('Formatting:') && stdout.contains('Logging:') && stdout.contains('Global flags:') |
| Captain entity action | $GIT_ROOT_DIR/../captain | ./.bin/captain prompt render --help --no-color | stdout.contains('Command flags:') && stdout.contains('Model selection:') && stdout.contains('Model permissions:') && stdout.contains('Model runtime:') && stdout.contains('Formatting:') |
| Gavel root | $GIT_ROOT_DIR/../gavel | ./.bin/gavel --help | stdout.contains('Formatting:') && stdout.contains('Logging:') && stdout.contains('Tasks:') && stdout.contains('Global flags:') |
| Gavel commit | $GIT_ROOT_DIR/../gavel | ./.bin/gavel commit --help | stdout.contains('Command flags:') && stdout.contains('Model selection:') && stdout.contains('Formatting:') |
| Gavel lint | $GIT_ROOT_DIR/../gavel | ./.bin/gavel lint --help | stdout.contains('Command flags:') && stdout.contains('Model selection:') && stdout.contains('Model permissions:') |
| Gavel TODO run | $GIT_ROOT_DIR/../gavel | ./.bin/gavel todos run --help | stdout.contains('Command flags:') && stdout.contains('Model selection:') && stdout.contains('Model runtime:') |
