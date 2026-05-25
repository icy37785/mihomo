# Configuration And Runtime State

The project has no relational database or migration system. Persistent and reloadable state is represented by YAML config, runtime singletons, provider/cache files, and external controller patch/update endpoints.

## Config Parse Contract

Primary flow:

```text
main.go flags/env/stdin/base64/file
  -> hub.Parse
  -> executor.Parse / ParseWithPath / ParseWithBytes
  -> config.Parse / UnmarshalRawConfig / ParseRawConfig
  -> executor.ApplyConfig
```

Reference files:
- `main.go`
- `hub/hub.go`
- `hub/executor/executor.go`
- `config/config.go`
- `config/initial.go`

## Raw vs Runtime Config

- YAML-facing structs live in `config/config.go` and use `yaml`/`json` tags.
- Runtime structs (`General`, `DNS`, `Config`, etc.) hold parsed types such as `netip.Prefix`, `dns.NameServer`, `C.Rule`, and provider interfaces.
- Defaults come from `DefaultRawConfig()`, then `yaml.Unmarshal` overlays user config in `UnmarshalRawConfig`.
- Validation happens while converting raw config to runtime config. Return contextual errors from the relevant `parse*` function.

Example pattern:

```go
dnsCfg, err := parseDNS(rawCfg, ruleProviders)
if err != nil {
	return nil, err
}
config.DNS = dnsCfg
```

## Runtime Apply Order

`executor.ApplyConfig` is the only normal place to apply a full parsed config. Preserve its ordered lifecycle:

1. set log level;
2. suspend tunnel;
3. reset certificates and Smart state;
4. update users, proxies, rules, sniffer, hosts, general, NTP, DNS, listeners, TUN, IPTables, and tunnels;
5. mark inner loading, initialize inner TCP, load providers, update profile, run GC, mark running;
6. update auto-updaters and reset resolver connections.

Do not apply a new subsystem in isolation if it depends on proxies, rules, DNS, listeners, or providers.

## Scenario: Provider-Local Dialer Proxy Dependencies

### 1. Scope / Trigger
- Trigger: changing proxy-provider parsing, proxy construction, or group
  expansion for proxies that use `dialer-proxy`.
- This scenario spans config YAML, provider runtime state, outbound dialer
  construction, group selection lists, and REST proxy output.

### 2. Signatures
- Proxy YAML fields:
  - `dialer-proxy: <proxy-name>` selects another proxy as the outbound dialer.
  - `hidden: true` marks a leaf proxy as an internal dependency.
- Dialer construction:
  - `proxydialer.NewByName(proxyName, tunnel, resolver...)` accepts an optional
    provider-local resolver.
- Parser construction:
  - `adapter.ParseProxy(..., adapter.WithDialerProxyResolver(resolver))`
    threads provider-local lookup into outbound adapters.

### 3. Contracts
- Lookup order for provider-loaded proxies is provider-local first, then global
  `tunnel.Proxies()`.
- The provider-local resolver must reference a mutable provider store so proxies
  created before the full provider map exists can still resolve dependencies at
  dial time.
- `hidden: true` only hides a proxy from provider/group user selection lists and
  API-visible provider expansion. Hidden proxies must remain available to the
  provider-local `dialer-proxy` resolver.
- Provider `exclude-filter` and `exclude-type` are hard exclusions from both
  visible output and local dependency lookup. Provider `filter` controls visible
  output; hidden proxies may still be retained for local dependency lookup.

### 4. Validation & Error Matrix
| Condition | Required behavior |
| --- | --- |
| Provider proxy references same-provider dependency | Resolve it locally before global lookup |
| Local and global proxies share a dependency name | Use the provider-local proxy |
| Local resolver misses a dependency | Fall back to existing global `tunnel.Proxies()` lookup |
| Dependency proxy has `hidden: true` | Omit it from group choices, keep it callable by `dialer-proxy` |
| Proxy is excluded by `exclude-filter` or `exclude-type` | Do not expose it and do not keep it as a local dependency |

### 5. Good/Base/Bad Cases
- Good: an inline/http provider contains `inner` with `dialer-proxy: outer` and
  hidden `outer`; a selector using the provider lists only `inner`, and `inner`
  dials through provider-local `outer`.
- Base: a top-level proxy with `dialer-proxy` and no provider-local resolver
  continues using global `tunnel.Proxies()`.
- Bad: API or group output hides `outer` but the dial path still looks only in
  `tunnel.Proxies()`, causing `proxyName[outer] not found`.
- Bad: string-prefix checks such as generated `outer-` names define hidden or
  dependency behavior.

### 6. Tests Required
- Unit test `proxydialer.NewByName` local-over-global priority and global
  fallback.
- Config/provider integration test where a hidden provider proxy is omitted from
  selector choices but remains usable by another provider proxy's
  `dialer-proxy`.
- Existing full parser/runtime tests must still pass with `rtk go test ./...`.

### 7. Wrong vs Correct
#### Wrong
```go
proxy := tunnel.Proxies()[name]
```

#### Correct
```go
if proxy, ok := localResolver(name); ok {
	return proxy
}
proxy := tunnel.Proxies()[name]
```

## External Controller Updates

`hub/route/configs.go` has two update styles:

- `PUT /configs`: parse a full config from `payload` or a safe absolute `path`, then call `executor.ApplyConfig(cfg, force)`.
- `PATCH /configs`: patch selected runtime knobs with pointer fields so omitted JSON fields do not overwrite current values.

When adding patchable fields:

- use pointer fields in `configSchema`;
- read current runtime defaults via `executor.GetGeneral()` or listener state;
- only mutate the owned runtime component;
- respond with `render.NoContent(w, r)` on success.

## Path Safety

`constant/path.go` owns config paths and safe path checks:

- default home is `$HOME/.config/mihomo`, with `XDG_CONFIG_HOME` fallback;
- relative paths resolve under `C.Path.HomeDir()`;
- file update APIs must reject unsafe paths unless `SKIP_SAFE_PATH_CHECK`, `SAFE_PATHS`, or CMFA allows them.

Do not read or write arbitrary user-provided paths from route handlers without `C.Path.IsSafePath`.

## Environment And Flags

`main.go` maps selected environment variables to CLI flags:

| Env/flag | Purpose |
| --- | --- |
| `CLASH_HOME_DIR` / `-d` | config directory |
| `CLASH_CONFIG_FILE` / `-f` | config file path |
| `CLASH_CONFIG_STRING` / `-config` | base64-encoded config bytes |
| `CLASH_OVERRIDE_EXTERNAL_CONTROLLER` / `-ext-ctl` | override REST controller address |
| `CLASH_OVERRIDE_SECRET` / `-secret` | override REST API secret |
| `CLASH_POST_UP` / `-post-up` | shell hook after startup |
| `CLASH_POST_DOWN` / `-post-down` | shell hook on shutdown |

Keep new startup overrides in this boundary. Do not read process environment deep inside protocol or tunnel packages unless the existing component already owns that env key.

## Tests Required

- Config validation changes: add table-driven tests near `config/utils_test.go` or a focused `config/*_test.go`.
- Runtime patch/update changes: cover omitted-vs-zero behavior for pointer fields when practical.
- Path safety changes: include unsafe absolute path, safe absolute path, and default path cases.
- Full config parser changes must still pass `rtk go test ./config ./hub/executor`.

## Wrong vs Correct

Wrong:

```go
// Missing safe path check for user-provided path.
cfg, err = executor.ParseWithPath(req.Path)
```

Correct:

```go
if !filepath.IsAbs(req.Path) || !C.Path.IsSafePath(req.Path) {
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, newError(C.Path.ErrNotSafePath(req.Path).Error()))
	return
}
cfg, err = executor.ParseWithPath(req.Path)
```
