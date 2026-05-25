# Directory Structure

This repository is a Go single-module proxy application (`github.com/metacubex/mihomo`). Keep new code in the existing runtime boundary that owns the behavior; do not create generic service layers or framework-style directories.

## Runtime Boundaries

| Area | Owns | Reference files |
| --- | --- | --- |
| CLI entry and process lifecycle | flags, environment overrides, config source selection, signal handling | `main.go` |
| Config parsing | YAML/default config, raw-to-runtime conversion, validation | `config/config.go`, `config/utils.go` |
| Runtime apply/reload | applying parsed config into global runtime components | `hub/executor/executor.go`, `hub/hub.go` |
| External controller API | HTTP routes, request schemas, response errors | `hub/route/server.go`, `hub/route/configs.go`, `hub/route/errors.go` |
| Inbound listeners | local HTTP/SOCKS/TProxy/TUN/server listener creation | `listener/`, `adapter/inbound/` |
| Outbound adapters | protocol client options, dial/listen packet behavior | `adapter/outbound/`, `adapter/parser.go` |
| Proxy groups/providers | group selection, health checks, provider loading | `adapter/outboundgroup/`, `adapter/provider/` |
| Tunnel/routing runtime | TCP/UDP handling, metadata pre-handle, rules, NAT | `tunnel/` |
| DNS runtime | DNS clients, middleware, server recreation, cache behavior | `dns/`, `component/resolver/` |
| Rules | rule parser, concrete rule matchers, rule providers | `rules/parser.go`, `rules/common/`, `rules/provider/` |
| Transport protocols | protocol handshakes, framing, obfs, packet streams | `transport/` |
| Shared primitives | reusable low-level data structures and helpers | `common/` |
| Constants/contracts | interfaces, enums, metadata, paths, build tags | `constant/` |

## Placement Rules

- Add a new outbound protocol under `adapter/outbound/<protocol>.go`, wire it in `adapter/parser.go`, and put protocol framing/handshake code under `transport/<protocol>/` when it is reusable outside the adapter.
- Add a new rule by implementing the matcher in `rules/common/` or `rules/logic/`, then register the rule type in `rules/parser.go`.
- Add external controller endpoints under `hub/route/`; keep request schemas in the route file that owns the endpoint.
- Add config fields first to `RawConfig`/raw nested structs in `config/config.go`, then parse into runtime structs in the relevant `parse*` function.
- Add shared helpers only under `common/` when they are reusable across packages. One-off helpers should stay near the caller.
- Keep platform/build-tag variants next to the feature owner, as seen in `constant/features/*_stub.go`, `dns/system_*.go`, and `adapter/inbound/listen_*.go`.

## Naming Conventions

- Package names are short lowercase nouns (`config`, `tunnel`, `resolver`, `outboundgroup`).
- Protocol files use the protocol name (`snell.go`, `trojan.go`, `sudoku.go`).
- Option structs are named `<Protocol>Option` and decoded from config tags, for example `outbound.SnellOption` and `outbound.BasicOption`.
- Runtime interfaces from `constant` are usually imported as `C`; provider contracts from `constant/provider` are imported as `P`.
- Listener config is commonly imported as `LC`; rules packages often use aliases such as `R`, `RC`, `RP`, and `RW` when multiple rule packages are in one file.

## Common Mistakes

- Do not add new API routes in `main.go`; `main.go` only chooses config input, sets options, starts `hub.Parse`, and handles process signals.
- Do not bypass `executor.ApplyConfig` for reloadable runtime changes. It owns the ordered update of tunnel, proxies, rules, DNS, listeners, profile, providers, and updater state.
- Do not put protocol transport state in `common/`; keep protocol-specific handshake/framing in `transport/<protocol>/`.
- Do not add root-level generated binaries such as `mihomo`; build outputs belong under `bin/` or should stay untracked.
