# Implementation Plan

1. Add provider-aware by-name resolution.
   - Update `component/proxydialer/byname.go` with a local resolver type and
     local-first lookup.
   - Verify with a focused unit test that local wins over global and fallback
     still works.

2. Thread resolver and hidden metadata through proxy parsing.
   - Add internal parser option for a local dialer-proxy resolver.
   - Decode `hidden` in `outbound.BasicOption`.
   - Store hidden on `adapter.Proxy` and expose it through `ProxyInfo`.
   - Ensure proxy JSON does not overwrite group-level `hidden` fields.

3. Retain provider-local dependencies while exposing only visible proxies.
   - Rework `NewProxiesParser` to parse non-excluded provider proxies into a
     local resolver map and return only non-hidden, filter-matching proxies.
   - Publish the local resolver map only after the full parse succeeds.
   - Keep provider-level `dialer-proxy` and `override` application order.

4. Hide dependency proxies from group selection.
   - Filter hidden proxies in provider visible lists and group expansion.
   - Keep hidden proxies available in the provider-local resolver.

5. Add integration-style tests.
   - Parse an inline provider with:
     - global proxy `outer` as `reject`;
     - provider-local hidden `outer` as `direct`;
     - provider `inner` socks5 proxy with `dialer-proxy: outer`;
     - group `PROXY` using the provider.
   - Run a local minimal SOCKS5 server and verify `inner.DialContext` succeeds,
     proving provider-local priority over the global same-name proxy.
   - Verify selector JSON/group choices include `inner` and omit hidden `outer`.

6. Validate.
   - `rtk go test ./component/proxydialer ./adapter ./adapter/provider ./adapter/outboundgroup ./config`
   - `rtk go test ./...` if focused tests pass within the available runtime.
   - `rtk proxy git diff --check`
   - Attempt live reproduction with the supplied provider config if practical.
