---
title: List options
description: How ListOpts struct tags become CLI flags, query parameters and UI controls.
---

`ListOpts` is a plain struct. Each field with a `flag:` tag becomes:

- a cobra flag on `app <entity>` and `app <entity> list`
- an OpenAPI query parameter on `GET /api/v1/<entity>`
- a control in the clicky-ui filter bar (see [Control types](/filters/control-types/))

```go
type StackListOpts struct {
	Team            string          `flag:"team,t"   help:"Owning team"`
	Status          string          `flag:"status"   help:"Health status" enum:"healthy,degraded,paused"`
	Region          string          `flag:"region"   help:"Cloud region"`
	Tags            []string        `flag:"tags"     help:"Require all tags"`
	Owners          clicky.MultiFilter `flag:"owner" help:"Owners; prefix with ! to exclude"`
	MinReplicas     int             `flag:"min-replicas"`
	IncludeArchived bool            `flag:"include-archived"`
	From            time.Time       `flag:"from"     help:"Deployed after"  default:"now-30d"`
	To              time.Time       `flag:"to"       help:"Deployed before"`
	MaxAge          duration.Duration `flag:"max-age" default:"30d"`
}
```

## Supported field types

| Go type | CLI form | Notes |
| --- | --- | --- |
| `string` | `--team platform` | |
| `int` | `--min-replicas 3` | |
| `bool` | `--include-archived` | |
| `[]string` | `--tags a,b` or repeated | comma-separated values |
| `[]int` | `--ids 1,2` | |
| `clicky.MultiFilter` | `--owner alice,!bob` | a `[]string` whose values follow include/exclude semantics: plain values include, `!value` excludes |
| `time.Time` | `--from now-7d` | accepts RFC 3339 and date-math: `now`, `now-7d`, `now/d`, `now-1M/M` |
| `duration.Duration` | `--max-age 2w` | from `github.com/flanksource/commons/duration`; supports `d`, `w` and similar units |

Embedded (anonymous) structs are walked recursively, so shared option groups can be embedded, for example a `Paging` struct with `limit`/`offset`. A *named* struct field is not walked, and its tagged fields do not bind.

## Tags

| Tag | Effect |
| --- | --- |
| `flag:"name"` / `flag:"name,n"` | long flag name, optional one-character shorthand |
| `short:"n"` | shorthand; takes precedence over the comma form |
| `help:"…"` | flag help text and OpenAPI description |
| `default:"…"` | default value (date-math is allowed for `time.Time`) |
| `required:"true"` | mark the flag required |
| `hidden:"true"` | hide from help |
| `enum:"a,b,c"` | closed set of allowed values |
| `args:"true"` | bind positional args |
| `stdin:"true"` | the field that receives stdin input (one per struct) |
| `clicky:"cli-file-read"` | allow `@file` / `@https://…` expansion on the CLI |
| `clicky:"rpc-file-read"` | also allow it for values arriving over HTTP (this reads the **server's** disk) |

`@file` expansion is opt-in per field because it reads whatever the value names on the machine doing the expansion. For slice fields, `@file.csv:Column` and `@file.xlsx:Column` read one named column.

## Decoding

Every surface builds a `map[string]string` and decodes it with the same tags. You can use the decoder yourself, for example in an action that received a raw flag map:

```go
opts, err := clicky.BuildOpts[StackListOpts](flags)
```

## Reserved names

- `sort` and `order` are reserved when the entity uses [`Sort`](/entities/sorting-and-paging/). Declaring them in `ListOpts` panics at registration.
- A non-empty `filter` value switches a [bulk action](/entities/bulk-actions/) into filter mode. The global `--filter` CEL flag from `clicky.BindAllFlags` supplies it on the CLI.
- Keys prefixed with `__lookup` are reserved for the [lookup endpoint](/filters/lookups/).

## Date ranges

Two `time.Time` fields named `from`/`to`, or ending in `-from`/`-to` (for example `deployed-from`/`deployed-to`), are paired by the UI into a single range picker. See [Control types](/filters/control-types/#date-ranges).
