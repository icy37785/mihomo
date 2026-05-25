# Quality Guidelines

This repository values small, source-backed Go changes with targeted tests. Most code is performance-sensitive proxy/runtime logic, so verify both behavior and integration boundaries.

## Required Patterns

- Keep changes scoped to the owning package from `directory-structure.md`.
- Prefer existing parser/option patterns over ad hoc field extraction.
- Use table-driven tests for validation logic, as in `config/utils_test.go`.
- Use `net.Pipe`, local fake conns, timeouts, and direct byte assertions for protocol/transport tests, as in `adapter/outbound/trojan_test.go`, `transport/snell/pool_test.go`, and `transport/sudoku/handshake_test.go`.
- Use `testify/assert` or `testify/require` where the local test package already uses it; otherwise plain `testing` with `t.Fatal`/`t.Fatalf` is common.
- Run affected package tests before full repository tests after upstream merges or runtime changes.

## Forbidden Patterns

- Do not commit local build artifacts such as a root-level `mihomo` binary.
- Do not import `logrus` directly outside `log/`.
- Do not add config fields without raw struct tags, default/parse handling, and validation.
- Do not add an outbound protocol only in `adapter/outbound/`; it must also be wired through `adapter/parser.go`.
- Do not add a rule matcher without registering it in `rules/parser.go`.
- Do not push to third-party upstream forks. Third-party remotes must be fetch-only.
- Do not merge branches that have no merge base with local `Alpha`.

## Test Commands

Use the narrowest reliable test first, then broaden:

```bash
rtk go test ./config ./hub/executor
rtk go test ./adapter/outbound ./adapter/outboundgroup ./rules/...
rtk go test ./transport/snell ./transport/sudoku/... ./dns ./hub/executor
rtk go test ./...
```

CI/workflow changes require:

```bash
rtk actionlint .github/workflows/build.yml
rtk ruby -e 'require "yaml"; YAML.load_file(".github/workflows/build.yml"); puts "workflow yaml ok"'
rtk proxy git diff --check
```

If Go tests fail because the sandbox cannot write the Go build cache, rerun the same command with the approved/escalated execution path instead of changing code.

## Scenario: Multi-Upstream Fork Integration

### 1. Scope / Trigger
- Trigger: this repository is a fork that tracks more than one upstream fork.
- Primary fork remote: `upstream` -> `https://github.com/vernesong/mihomo.git`.
- Secondary fork remote: `missuo` -> `https://github.com/missuo/mihomo.git`.
- Local integration branch: `Alpha`.
- Apply this workflow before merging code from any non-`origin` fork.

### 2. Signatures
- Add a secondary fetch remote:
  ```bash
  rtk git remote add missuo https://github.com/missuo/mihomo.git
  rtk git remote set-url --push missuo DISABLED
  ```
- Refresh fork refs:
  ```bash
  rtk git fetch upstream --prune
  rtk git fetch missuo --prune
  ```
- Compare integration state:
  ```bash
  rtk git rev-list --left-right --count Alpha...missuo/Alpha
  rtk git log --oneline --no-merges Alpha..missuo/Alpha
  rtk git diff --stat Alpha...missuo/Alpha
  ```
- Conflict preflight:
  ```bash
  rtk git merge-tree --write-tree Alpha missuo/Alpha
  ```
- Merge with provenance:
  ```bash
  rtk git merge --no-ff missuo/Alpha -m "Merge remote-tracking branch 'missuo/Alpha' into Alpha"
  ```

### 3. Contracts
- `origin` is the only normal push target for this fork.
- `upstream` and `missuo` are fetch sources; do not push to them from this repository.
- Keep the secondary upstream push URL set to `DISABLED`.
- Track `missuo/Alpha` for mihomo code updates.
- Do not track `missuo/main` for this project: it has no merge base with local `Alpha` and is not the mihomo code line used here.
- Local build artifacts such as a root-level `mihomo` executable must not be committed.
- Prefer merge commits over rebasing `Alpha`, because `Alpha` is a shared/release branch and merge commits preserve which upstream fork supplied the change.

### 4. Validation & Error Matrix
| Condition | Required response |
| --- | --- |
| `rtk git status --short` is not clean | Stop and classify the changes before merging. Delete obvious local build artifacts only when explicitly approved. |
| Remote has a push URL other than `DISABLED` for a third-party fork | Set push URL to `DISABLED` before fetch/merge work continues. |
| Candidate branch has no merge base with `Alpha` | Do not merge it; inspect branch identity and choose another branch. |
| `git merge-tree --write-tree Alpha <remote>/<branch>` fails with conflicts | Create a temporary integration branch and resolve conflicts there before touching `Alpha`. |
| Merge succeeds but targeted tests fail | Keep the merge local, investigate, and do not push. |
| Full `go test ./...` fails | Treat the integration as incomplete, even if targeted tests passed. |

### 5. Good/Base/Bad Cases
- Good: `missuo/Alpha` has a merge base with `Alpha`, `merge-tree` succeeds, then `Alpha` merges `missuo/Alpha` with `--no-ff`.
- Base: `upstream/Alpha` has no new commits, so only `missuo/Alpha` is merged.
- Bad: merging `missuo/main` because it is the default branch; this branch has no merge base with local `Alpha`.
- Bad: committing a root-level `mihomo` binary produced by local builds.

### 6. Tests Required
- For Snell/Sudoku/DNS/executor integrations, run:
  ```bash
  rtk go test ./transport/snell ./transport/sudoku/... ./dns ./hub/executor
  ```
- After any upstream fork merge, run:
  ```bash
  rtk go test ./...
  ```
- Assertion points:
  - affected-package tests pass;
  - full repository tests pass;
  - `rtk git status --short` is clean after merge validation.

### 7. Wrong vs Correct
#### Wrong
```bash
rtk git merge missuo/main
rtk git push missuo Alpha
```

#### Correct
```bash
rtk git remote set-url --push missuo DISABLED
rtk git fetch missuo --prune
rtk git merge-tree --write-tree Alpha missuo/Alpha
rtk git merge --no-ff missuo/Alpha -m "Merge remote-tracking branch 'missuo/Alpha' into Alpha"
rtk go test ./...
```

## Scenario: Self-Use Release Build Matrix

### 1. Scope / Trigger
- Trigger: GitHub Actions release builds are intentionally limited for self-use.
- Active build artifacts are:
  - `mihomo-linux-amd64-v3-*-smart`
  - `mihomo-darwin-arm64-*-smart`

### 2. Signatures
- Active matrix entries live in `.github/workflows/build.yml` under `jobs.build.strategy.matrix.jobs`.
- Active standalone test entries live in `.github/workflows/test.yml` under `jobs.test.strategy.matrix.os` and `jobs.test.strategy.matrix.go-version`.
- Docker artifact selection for `linux/amd64` lives in `docker/file-name.sh`.
- The Docker `linux/amd64` selector must resolve to `amd64-v3`.

### 3. Contracts
- Keep inactive matrix entries commented, not deleted, so future user-requested targets can be restored deliberately.
- Every active matrix object must declare all optional fields referenced by workflow expressions (`goversion`, `abi`, `ndk`, `go386`, `goarm`, `gomips`, `debian`, `rpm`, `pacman`, `tarball`, `test`) even when the value is `''`.
- Docker platforms must match available Linux artifacts. With only `linux/amd64-v3` enabled, Docker `platforms` must only include `linux/amd64`.
- Standalone CI tests should mirror the active self-use targets: keep `ubuntu-latest` and `macos-latest` active, keep the current default Go version active, and leave inactive OS/Go versions commented for future restoration.
- If additional Linux architectures are re-enabled, update both the build matrix and Docker artifact selector in the same change.

### 4. Validation & Error Matrix
| Condition | Required response |
| --- | --- |
| `actionlint` reports undefined `matrix.jobs.<field>` | Add the optional field to every active matrix object. |
| Docker references a platform without a matching build artifact | Disable that Docker platform or re-enable the matching artifact. |
| `docker/file-name.sh` selects `amd64-v1` while matrix builds `amd64-v3` | Change the selector to `amd64-v3`. |
| `.github/workflows/test.yml` still tests disabled OS targets or old Go versions | Comment those matrix entries until the matching build targets are re-enabled. |
| Workflow YAML parses but `actionlint` fails | Treat the workflow as invalid until `actionlint` passes. |

### 5. Good/Base/Bad Cases
- Good: active matrix contains only Linux `amd64-v3` and Darwin `arm64`, with all optional fields present.
- Base: Docker builds only `linux/amd64` and `docker/file-name.sh` selects `amd64-v3`.
- Base: standalone tests run only `ubuntu-latest` and `macos-latest` on the active default Go version.
- Bad: commenting out matrix entries but leaving Docker platforms `linux/386`, `linux/arm64`, or `linux/arm/v7` enabled.
- Bad: limiting release builds but still running the standalone test matrix across Windows, Linux ARM, macOS Intel, and old Go versions.
- Bad: relying on missing matrix keys to evaluate as empty strings; `actionlint` rejects this.

### 6. Tests Required
- Workflow lint:
```bash
rtk actionlint .github/workflows/build.yml
rtk actionlint .github/workflows/test.yml
```
- YAML parse check:
```bash
rtk ruby -e 'require "yaml"; YAML.load_file(".github/workflows/build.yml"); puts "workflow yaml ok"'
rtk ruby -e 'require "yaml"; YAML.load_file(".github/workflows/test.yml"); puts "workflow yaml ok"'
```
- Docker helper syntax:
  ```bash
  rtk sh -n docker/file-name.sh
  ```
- Whitespace check before commit:
  ```bash
  rtk proxy git diff --check
  ```

### 7. Wrong vs Correct
#### Wrong
```yaml
- { goos: linux, goarch: amd64, goamd64: v3, output: amd64-v3 }
```

#### Correct
```yaml
- { goos: linux, goarch: amd64, goamd64: v3, output: amd64-v3, test: test, goversion: '', abi: '', ndk: '', go386: '', goarm: '', gomips: '', debian: '', rpm: '', pacman: '', tarball: '' }
```

#### Wrong
```yaml
os:
  - 'ubuntu-latest'
  - 'windows-latest'
  - 'macos-latest'
go-version:
  - '1.26'
  - '1.25'
```

#### Correct
```yaml
os:
  - 'ubuntu-latest'
  - 'macos-latest'
  # - 'windows-latest'
go-version:
  - '1.26'
  # - '1.25'
```

## Code Review Checklist

- Does the change belong to the package it modifies?
- Are new config fields represented in raw config, runtime config, parser logic, and API schema if applicable?
- Are parser errors contextual enough to locate the broken config item?
- Are external controller errors rendered with an appropriate HTTP status and JSON error body?
- Are runtime changes applied through `executor.ApplyConfig` or a route-owned patch path?
- Do affected tests cover both good and bad cases?
- Did full `rtk go test ./...` pass for upstream merges or cross-package changes?
