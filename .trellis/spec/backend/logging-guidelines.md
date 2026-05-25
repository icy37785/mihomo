# Logging Guidelines

Use the local `github.com/metacubex/mihomo/log` package instead of importing `logrus` directly. The wrapper publishes log events to subscribers and applies the runtime log level.

## Logger Contract

Reference files:
- `log/log.go`
- `log/level.go`
- `hub/executor/executor.go`
- `hub/route/server.go`
- `tunnel/tunnel.go`
- `dns/doh.go`

Available levels:

| Function | Use for |
| --- | --- |
| `log.Debugln` | high-volume diagnostics, retries, cache hits, sniffing/rule details |
| `log.Infoln` | lifecycle events operators expect, such as config load completion and listener addresses |
| `log.Warnln` | recoverable issues, deprecated config, fallback behavior, invalid optional inputs |
| `log.Errorln` | operation failures that prevent a requested action or listener/server from starting |
| `log.Fatalln` | process startup failures only |

`executor.ApplyConfig` calls `log.SetLevel(cfg.General.LogLevel)`. Do not cache log levels in other packages.

## Message Style

- Messages use printf-style formatting.
- Include subsystem prefixes for noisy runtime flows: `[DNS]`, `[Smart]`, `[UDP]`, `[Rule]`, `[GEO]`.
- Include the resource name, proxy/group/provider name, or address when it helps operators act.
- Use `err.Error()` or `%v` consistently with existing call sites; do not wrap secrets into logs.

Examples:

```go
log.Infoln("RESTful API listening at: %s", l.Addr().String())
log.Warnln("[Smart] Failed to update node ranking: %v", err)
log.Debugln("[DNS] cache hit %s --> %s, expire at %s", domain, msgToLogString(msg), expireTime.Format("2006-01-02 15:04:05"))
```

## What Not To Log

- REST API `Secret`, proxy passwords, private keys, ECH keys, TLS private keys, and raw authorization headers.
- Full config payloads from `CLASH_CONFIG_STRING`, stdin, or API `payload`.
- Repeated per-packet data at info/warn level.

## Common Mistakes

- Do not import `github.com/sirupsen/logrus` outside the `log` package.
- Do not use `fmt.Println` for runtime diagnostics except explicit CLI output such as version or config-test result in `main.go`.
- Do not use `Errorln` for expected high-volume network failures in tunnel/DNS paths; prefer `Debugln` unless the operator should take action.
- Do not log then return a second generic error from the same boundary if the caller will also log it; add context once and let the caller decide.
