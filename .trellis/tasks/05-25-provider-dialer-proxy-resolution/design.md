# Design

## Current Behavior

`outbound.BasicOption.NewDialer` creates `proxydialer.NewByName` when a proxy
has `dialer-proxy`. `component/proxydialer/byname.go` resolves the target only
through `tunnel.Proxies()`. Provider-loaded proxies are kept in provider
instances and expanded later by groups using `use:`, so same-provider dependency
proxies are invisible to `NewByName`.

Provider and group visible lists currently use `P.ProxyProvider.Proxies()`.
Leaf proxies do not have a hidden marker; only proxy groups expose `hidden` in
their own JSON.

## Proposed Contracts

- Add an optional local resolver to `proxydialer.NewByName`.
  - Signature remains compatible by using a variadic resolver parameter.
  - Resolver order is provider-local first, then global `tunnel.Proxies()`.
  - Existing callers with no resolver keep current behavior.
- Thread the resolver through proxy construction:
  - `adapter.ParseProxy` accepts an internal proxy option carrying
    `func(string) (C.Proxy, bool)`.
  - `outbound.BasicOption` stores that resolver in an internal field and passes
    it to `proxydialer.NewByName`.
- Provider parser owns a provider-local proxy store:
  - `NewProxiesParser` creates a shared store per provider parser.
  - Each parsed proxy receives a resolver closure pointing at that store.
  - A parse builds a complete local map first and publishes it only after the
    parse succeeds, so failed updates do not poison existing proxies.
- Hidden proxy marker:
  - Add `hidden: true` to `outbound.BasicOption`.
  - Store hidden state in the `adapter.Proxy` wrapper so every outbound type can
    support it without changing every protocol adapter.
  - Add `Hidden bool` to `C.ProxyInfo` so shared group/provider logic can filter
    leaf proxies.

## Provider Filtering

Provider `filter` should continue to define the user-visible provider output.
Hidden/internal dependencies should be parsed into the local resolver map even
when they do not match the include filter. `exclude-filter` and `exclude-type`
remain hard exclusions from both visible output and the local dependency map.
`override` is applied before proxy construction, matching the current order.

## Selection And API Output

- Provider `Proxies()` returns visible proxies only.
- Provider health checks receive visible proxies only.
- Group expansion filters hidden proxies as a final safety net.
- Hidden provider proxies remain accessible only through the provider-local
  resolver used by `dialer-proxy`.

## Risks

- Provider parser ordering is sensitive because proxy constructors are called
  before the full provider map exists. The resolver must reference a mutable
  store, not a snapshot.
- A provider update parse failure must leave the previous resolver map intact.
- Hidden should not overwrite existing group JSON `hidden` values.

## Rollback

The change is localized to `component/proxydialer`, `adapter.ParseProxy`,
`adapter.Proxy`, provider parsing/storage, and group expansion. Reverting these
files restores the old global-only lookup and visible-only provider parsing.
