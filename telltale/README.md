# clicky/telltale

Makes a clicky process observable by [telltale](https://github.com/flanksource/telltale) and gops from one call. It embeds the telltale agent with probes for clicky exec, entity operations and tasks, commons logs and HTTP traffic, plus a gops agent so `gops <pid>` and `telltale gops <pid>` keep working.

```bash
go get github.com/flanksource/clicky/telltale
```

## Start

Call `Start` first thing in `main`, so commons HTTP clients configured afterwards pick up the instrumented `http.DefaultTransport`:

```go
agent, err := telltale.Start(telltale.Options{Service: "my-cli", Version: version})
if err != nil {
	return err
}
defer agent.Close() // also registered as a clicky shutdown hook
```

Or start it from the environment with a blank import:

```go
import _ "github.com/flanksource/clicky/telltale/auto"
```

| Variable | Effect |
|---|---|
| `TELLTALE_AGENT=1` | Start the telltale agent on a local socket, advertised in `<UserConfigDir>/telltale/agents/<pid>.json`, plus gops |
| `TELLTALE_URL`, `TELLTALE_TOKEN` | Also dial a telltale hub (`telltale agents token create <name>`) |
| `GOPS=1` | Start only the gops agent |
| `GOPS=0` | Leave gops off when telltale is started |
| `TELLTALE_ALLOW_MUTATION=1` | Allow mutating probes and ops (`goprof` mutex/block, `gc`, `setgc`) |

## Probes

Telltale arms a probe per trace session, and nothing is captured while no session is armed.

| Probe | Params | Record |
|---|---|---|
| `exec` | `name` glob on the binary, `tailBytes` (0 to 65536, default 0), `phases` (`start`, `exit`; default both) | `ExecRecord`: masked args, env keys, `pid`, `ppid`, `pgid`, `processKey`, `createdAt`, and on exit the exit code, duration, error and output tails |
| `entity` | `entity` and `verb` globs, `errorsOnly` | `EntityRecord`: entity, verb, surface (cli/http/mcp), target id, masked parameters and args, duration, error. Results are never captured |
| `task` | `name` and `group` globs, `progressIntervalMs` (default 1000) | `TaskRecord`: a `baseline` record per existing task on arm, then one per status transition plus throttled `progress`. Logs and output are dropped |
| `http` | telltale's `http` params | commons `har.Entry` for every request through `http.DefaultTransport`, including commons clients configured with TLS or proxy options |
| `logs` | `minLevel`, `match` | commons logger lines (`telltale/sdk/commonslog`) |

The SDK's own `goprof` and `stacks` probes are also available.

`processKey` is `<pid>@<create unix second>`, the key telltale's process sampler uses. An `exec` start record can therefore be joined to the child's `process_tree`/`process_samples` rows. If the child exited before its create time could be read, the key is the bare pid.

## Ops

These are gops diagnostics as telltale ops, for agents only reachable through a hub: `memstats`, `version` (build info), `stack` (all goroutines), `gc` and `setgc {"percent": N}`. The last two mutate the process.

## Try it

To arm all of them at once, run telltale's `clicky` preset. It is the built-in trace definition that fills the `collect.clicky` group (`exec`, `entity`, `task`, `http`, `logs`, plus a `pprof` heap profile read through the agent every 30s, or every `--every`). In telltale's web viewer, a run with a `task` capture gets a Tasks tab that renders the recorded tasks in clicky-ui's task viewer:

```bash
telltale clicky <pid> --duration 1m
```

In your own trace definition, the same group is `spec.collect.clicky`.

```bash
TELLTALE_AGENT=1 go run ./examples/demo          # prints its pid
telltale clicky <pid> --duration 10s             # records every clicky probe
telltale agents get <pid>                        # lists the probes and ops
go run ./hack/smoke <pid> 7                      # arms every clicky probe, prints a sample of each
gops stack <pid>
```
