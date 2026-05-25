# Fix provider dialer proxy resolution

## Goal

Fix proxy-provider chained proxy dialing so a provider-loaded proxy can resolve
its `dialer-proxy` dependency from the same provider before falling back to the
global tunnel proxy map. Add a proxy-level hidden marker so provider dependency
nodes can remain callable by `dialer-proxy` without appearing in normal group
selection lists.

User value:
- Leaf-style `ss -> trojan` chained nodes work when loaded through
  `proxy-providers`, matching the behavior of equivalent top-level `proxies:`
  config.
- Internal dependency nodes such as generated outer Shadowsocks proxies do not
  clutter user-facing selector options.

## Requirements

- Provider-local `dialer-proxy` resolution:
  - A proxy parsed from provider `dog` can resolve `dialer-proxy:
    外层-430699f99f` to another proxy from the same provider.
  - Provider-local resolution takes precedence over global
    `tunnel.Proxies()` when names collide.
  - If the provider-local resolver does not find the name, existing global
    `tunnel.Proxies()` behavior remains available.
- Hidden provider proxies:
  - Support a proxy field such as `hidden: true`.
  - Hidden proxies stay available inside the provider-local resolver.
  - Hidden proxies are omitted from group/user selection expansion by default.
  - Hidden handling must not rely on string prefixes such as `外层-`.
- Existing behavior preservation:
  - Do not change the public contract of global `dialer-proxy` resolution.
  - Preserve provider `filter`, `exclude-filter`, `exclude-type`, and
    `override` behavior for user-visible provider output.
  - Preserve provider health checks for user-visible proxies; hidden dependency
    proxies should not become selectable just because they are retained for
    local resolution.
- Reproduction target:
  - The supplied `dog` HTTP provider configuration should allow selecting a
    chained node such as `🇭🇰香港[杭州电信]01` and successfully fetching
    `https://www.gstatic.com/generate_204`.

## Acceptance Criteria

- [x] Provider-local `dialer-proxy` can resolve a same-provider dependency.
- [x] Provider-local names are preferred over global proxies with the same name.
- [x] Existing global `dialer-proxy` behavior remains compatible.
- [x] `hidden: true` provider proxies are not listed in selector/group choices.
- [x] Hidden provider proxies remain usable as `dialer-proxy` dependencies.
- [x] `/proxies/<chained-node>/delay?url=https://www.gstatic.com/generate_204&timeout=10000`
      returns a delay for the chained provider node in the reproduction config.
- [x] Focused tests cover provider-local resolver, local-over-global priority,
      and hidden provider proxy selection behavior.
- [x] Existing focused package tests pass.

## Notes

- No core logic may special-case generated names such as `外层-...`.
- If live reproduction cannot be completed due external network/provider
  availability, record the attempted command and rely on deterministic tests.
