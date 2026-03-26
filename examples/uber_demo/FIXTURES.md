---
build: go build -o ./uber-demo-test . && chmod +x ./uber-demo-test
cwd: ../..
---
# Uber Demo Task ANSI Fixtures

Tests that `--no-progress` suppresses ANSI layout commands (cursor movement, screen clearing) in piped/non-interactive mode.


## --no-progress with piped output (non-interactive)

| Test Name | CLI | CEL |
|-----------|-----|-----|
| no-progress piped | ./uber-demo-test tasks --no-progress --no-sleep --no-color --scenario=dependencies | !ansi.has_updates(stderr) && exitCode == 0 && stderr.contains("Setup environment") |
| no-progress TERM=xterm piped | env TERM=xterm-256color ./uber-demo-test tasks --no-progress --no-sleep --scenario=dependencies | !ansi.has_updates(stderr) && exitCode == 0 && stderr.contains("Setup environment") |
| no-progress NO_COLOR piped | env NO_COLOR=true TERM=xterm-256color ./uber-demo-test tasks --no-progress --no-sleep --scenario=dependencies | !ansi.has_updates(stderr) && exitCode == 0 |

## Default (no --no-progress flag, piped)

| Test Name | CLI | CEL |
|-----------|-----|-----|
| piped auto-disables progress | ./uber-demo-test tasks --no-sleep --no-color --scenario=dependencies | !ansi.has_updates(stderr) && exitCode == 0 && stderr.contains("Setup environment") |
| piped TERM=xterm auto-disables | env TERM=xterm-256color ./uber-demo-test tasks --no-sleep --scenario=dependencies | !ansi.has_updates(stderr) && exitCode == 0 && stderr.contains("Setup environment") |

## Content verification

| Test Name | CLI | CEL |
|-----------|-----|-----|
| task output present | ./uber-demo-test tasks --no-progress --no-sleep --no-color --scenario=dependencies | stderr.contains("Build application") && stderr.contains("Deploy application") && exitCode == 0 |
| stdout markers present | ./uber-demo-test tasks --no-progress --no-sleep --no-color --scenario=dependencies | stdout.contains("[stdout] starting") && stdout.contains("[stdout] stopping") |
