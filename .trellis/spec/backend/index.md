# Backend Development Guidelines

This repository is a Go single-module backend/proxy runtime. These specs document the conventions future agents should follow when changing config parsing, runtime apply/reload logic, routes, adapters, rules, transports, DNS, and CI/release integration.

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Package ownership, placement rules, naming conventions | Active |
| [Configuration And Runtime State](./config-and-runtime-state.md) | YAML config, runtime state, reload/apply contracts, safe paths | Active |
| [Error Handling](./error-handling.md) | Go error propagation, config validation, API error responses | Active |
| [Logging Guidelines](./logging-guidelines.md) | Local logging wrapper, levels, sensitive data rules | Active |
| [Quality Guidelines](./quality-guidelines.md) | Testing, review checks, multi-upstream and CI build rules | Active |

## Pre-Development Checklist

- Identify the owning package before editing. Use [Directory Structure](./directory-structure.md).
- For config, reload, path, or API update work, read [Configuration And Runtime State](./config-and-runtime-state.md).
- For parser, route, or runtime failure behavior, read [Error Handling](./error-handling.md).
- For diagnostics or operator-facing messages, read [Logging Guidelines](./logging-guidelines.md).
- For tests, upstream merges, release workflow, or CI changes, read [Quality Guidelines](./quality-guidelines.md).

## Quality Check

- Run focused package tests for the changed area.
- Run `rtk go test ./...` for cross-package changes, upstream fork merges, and runtime config changes.
- Run `rtk actionlint <changed-workflow.yml>` for GitHub Actions edits.
- Run `rtk proxy git diff --check` before committing.

## Source Examples

Representative files used to derive these specs:

- `main.go`
- `config/config.go`
- `config/utils.go`
- `hub/executor/executor.go`
- `hub/route/server.go`
- `hub/route/configs.go`
- `adapter/parser.go`
- `adapter/outbound/base.go`
- `rules/parser.go`
- `rules/common/base.go`
- `log/log.go`
- `constant/path.go`
- `transport/snell/pool_test.go`
- `transport/sudoku/handshake_test.go`
