# Error Handling

This project uses plain Go `error` values, contextual wrapping, and small route-level JSON error objects. Keep errors close to the boundary that can add useful context.

## Parse And Validation Errors

Config and parser errors should include enough context to locate the failing config item:

- `adapter.ParseProxy` returns direct protocol decode/create errors and reports unsupported types as `unsupport proxy type: <type>`.
- `config.parseProxies` wraps proxy errors with index context: `proxy %d: %w`.
- `config.parseRules` includes section, index, raw line, and parser error: `%s[%d] [%s] error: %s`.
- `rules.ParseRule` validates missing payloads and unsupported rule types before returning a concrete matcher.

Reference files:
- `adapter/parser.go`
- `config/config.go`
- `config/utils.go`
- `rules/parser.go`
- `rules/common/base.go`

Use `%w` when callers may need the original error. Use `%s`/`Error()` only when preserving identity is not useful and the local message is the contract.

## Runtime Errors

- Startup failures that make the process unusable call `log.Fatalln` from `main.go`.
- Reload failures return errors to `hub.Parse` callers or API responses; do not terminate the process on SIGHUP or REST config update failure.
- Long-running listeners log errors and return from their goroutine when a listener cannot start or serve.
- Tunnel packet/connection handling should drop invalid traffic and log at `Debugln` or `Warnln` depending on operator actionability.

Examples:
- `main.go` uses fatal logging for initial config and post-up failures.
- `hub/route/server.go` logs external controller listen/serve errors.
- `tunnel/tunnel.go` drops invalid UDP packets and logs invalid metadata.

## External Controller Errors

HTTP API errors are JSON objects with a single `message` field:

```go
type HTTPError struct {
	Message string `json:"message"`
}
```

Reference file: `hub/route/errors.go`.

Route handlers should:

- call `render.Status(r, <status>)` before rendering an error when the status is not 200;
- use shared errors (`ErrBadRequest`, `ErrUnauthorized`, `ErrNotFound`) for common cases;
- use `newError(err.Error())` for operation-specific messages;
- return immediately after rendering an error.

Example pattern from config update routes:

```go
if err := render.DecodeJSON(r.Body, &req); err != nil {
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, ErrBadRequest)
	return
}
```

## Validation Matrix

| Condition | Error behavior |
| --- | --- |
| Config file missing or empty | return error from `executor.ParseWithPath` / `readConfig` |
| Invalid YAML | return from `config.UnmarshalRawConfig` |
| Proxy type missing | `adapter.ParseProxy` returns `missing type` |
| Proxy/group/provider reference missing | config parser returns contextual `not found` error |
| Circular `dialer-proxy` | `validateDialerProxies` returns circular dependency error |
| API body decode failure | `400` with `ErrBadRequest` |
| Unsafe config path through API | `400` with `ErrNotSafePath` message |
| Geo database update failure | log error and return `500` JSON error |

## Common Mistakes

- Do not log and swallow config parse errors. Return them to the caller so startup, test-config mode, or route handlers can decide how to surface them.
- Do not call `log.Fatalln` from parser, adapter, tunnel, or route helpers. Reserve fatal exits for process startup boundaries.
- Do not return plain `ErrBadRequest` when a user needs a specific actionable message such as unsafe path or parse failure.
- Do not continue after `render.JSON` error responses; route handlers should return immediately.
