---
title: Errors
description: Classify failures with StatusError so each surface reports the right status and message.
---

Handlers return plain Go errors. clicky reports them on every surface. How much reaches the client depends on whether you **classified** the error.

## StatusError

```go
import "github.com/flanksource/clicky/entity"

func (s *Store) Get(ctx context.Context, id string) (Stack, error) {
	stack, err := s.db.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Stack{}, entity.NewStatusErrorf(http.StatusNotFound, "stack_not_found", "stack %q does not exist", id)
	}
	if err != nil {
		return Stack{}, fmt.Errorf("loading stack %s: %w", id, err) // unclassified -> 500
	}
	return stack, nil
}
```

`StatusError` has a stable machine-readable `Code` and a human-readable `Message`. Clients branch on `code`. The message can be reworded without breaking them.

Over HTTP:

- **Classified** errors (`*entity.StatusError` anywhere in the `errors.Unwrap` chain) are written with their own status. This holds in both the legacy and the structured error formats.
- **Unclassified** errors are a `500`.

clicky returns some classified errors itself, for example `400 invalid_sort` for a bad [sort key](/entities/sorting-and-paging/) and `400 invalid_parameters` for request parameters that cannot be decoded.

## Structured error responses

Start the server with `--structured-errors` (`ServeConfig.StructuredErrorResponses`) to get a traceable JSON envelope for every failure:

```json
{
  "code": "stack_not_found",
  "message": "stack \"stk-9\" does not exist",
  "trace": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

- The trace is also sent in the `X-Trace-ID` response header. It comes from the context (`entity.ContextWithTraceID`), the active OpenTelemetry span, or a generated ID.
- `--hide-error-details` (`ServeConfig.HideErrorDetails`), or the `HIDE_ERRORS=true` environment variable, replaces unclassified messages with `internal server error`. Classified `StatusError` messages are still shown, because you chose to expose them.
- Messages and details are sanitized and size-capped (`DefaultMaxErrorDetailBytes`, `DefaultMaxErrorResponseBytes`).
- Unclassified 5xx errors are logged on the server with their trace ID.

:::tip
Unclassified error text is whatever the failing dependency said. That routinely includes hostnames, DSNs, file paths or credentials. Production servers should run with `--hide-error-details` and classify every error a client is meant to understand.
:::

## Rich errors on the CLI

When an error in the `errors.Unwrap` chain implements a clicky render interface, such as `Pretty() api.Text`, the CLI renders it through the normal formatters and honours `--format`. The command still exits non-zero. Add `MarshalJSON` to control the JSON shape.

```go
type ValidationError struct{ Field, Problem string }

func (e ValidationError) Error() string { return e.Field + ": " + e.Problem }
func (e ValidationError) Pretty() api.Text {
	return clicky.Text("✗ ", "text-red-600").Append(e.Field, "font-bold").Append(" " + e.Problem)
}
```

## Lookup errors

A failing filter makes the whole lookup fail. This includes an error from `Lookup`, a named-filter source error, or invalid counts. With structured errors on, a classified error keeps its status (for example `503 lookup_unavailable`) and anything else is a `500`. clicky never replaces a failure with an empty option list, because an empty dropdown and a broken backend must look different.
