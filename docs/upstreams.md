# 上游来源与提交审查

## 上游约定

| remote | 仓库 | 代码分支 | 推送约定 |
| --- | --- | --- | --- |
| `origin` | 当前项目的远程仓库 | `Alpha` | 本项目的正常推送目标 |
| `upstream` | `vernesong/mihomo` | `Alpha` | fetch-only，push URL 为 `DISABLED` |
| `missuo` | `missuo/mihomo` | `Alpha` | fetch-only，push URL 为 `DISABLED` |
| `qoli` | `qoli/mihomo-Alpha` | `Alpha` | fetch-only，push URL 为 `DISABLED` |

qoli 的 Git remote 保存在本地 `.git/config`，其他克隆需要首次配置：

```sh
git remote add qoli https://github.com/qoli/mihomo-Alpha.git
git remote set-url --push qoli DISABLED
git fetch qoli --prune --no-tags
```

后续更新先检查 `git log --no-merges Alpha..qoli/Alpha`、`git cherry -v Alpha qoli/Alpha`
及具体补丁。选择性移植的提交应保留作者和原始 SHA；未导入的提交仍可能出现在比较结果中，
须结合本记录判断，不能据此直接合并整个分支。

## qoli 审查：2026-10-29

- 本地基准：`24cc3b799512c7b49aee00db9b41181b01c107d4`。
- qoli 基准：`e860c272f9d5fca5efebd1a5078b94c1e8802976`，默认代码分支 `Alpha`。
- 共同祖先：`86ece76999099381aaee80cb1410e0a8ecd2eeac`。
- 比较时本地独有 182 个提交，qoli 独有 9 个非合并提交，作者均为 `Ronnie W. <llqoli@gmail.com>`。
- 集成范围：TCP 目的地址修复、发布流程中可复用的改进及对应验证；保持当前 Smart 选路策略、
  自用双平台构建矩阵和包版本来源。qoli 私有的维护规则、下载地址和部署验收材料不作为本项目配置。

| qoli 提交 | 内容 | 处理 | 依据 |
| --- | --- | --- | --- |
| [`efe02b91`](https://github.com/qoli/mihomo-Alpha/commit/efe02b912675b426d31e80967f8d03be09acc03c) | 建连成功后不关闭其他节点的旧连接 | 不移植 | 当前实现明确维护同一 SmartTarget 的单出口；删除关闭逻辑会改变这一约定。 |
| [`5f29934a`](https://github.com/qoli/mihomo-Alpha/commit/5f29934a07ff4589fc9d821dd6699d137e30bd6e) | 并发选点协调与探测分类 | 不移植 | 取消后的统计保护本地已有；旧协调器的 key 不区分 TCP/UDP，且将 403/405 统一视为成功，不符合当前隔离和探测分类。 |
| [`48e6b0c3`](https://github.com/qoli/mihomo-Alpha/commit/48e6b0c316f4ca32f94cd68fdc2235fd0238db12) | 活跃连接劣化监测、冷却与时序日志 | 不移植 | 低速判定无法区分传输完成后的空闲与真实故障；重复证据可触发关闭及五分钟节点冷却。 |
| [`02c0eaf8`](https://github.com/qoli/mihomo-Alpha/commit/02c0eaf88988d524cc2b03bb212250db09866cf6) | Alpha 分发与发布流程 | 适配移植 workflow | 采纳最小权限、安全版本输入、tag 校验、从源 commit 发布、手动 rolling 发布及校验和；保留本地矩阵、Docker 平台和包版本来源，省略 qoli 下载链接。 |
| [`651ca468`](https://github.com/qoli/mihomo-Alpha/commit/651ca46803a450f9d4ea99b21b815e07fef9fc5e) | fork 意图文档及 AGENTS.md | 不移植 | 描述 qoli 私有的行为和维护流程，本项目已有自己的规则。 |
| [`92844c60`](https://github.com/qoli/mihomo-Alpha/commit/92844c60b304a9c350ad192d1e88de1722063077) | UDP 首响应后再确定 winner | 不移植 | 已被 qoli 的 `dc902107` 回滚。 |
| [`dc902107`](https://github.com/qoli/mihomo-Alpha/commit/dc902107302505db81477f624eb864a36f14098a) | 回滚 UDP winner 改动 | 无需移植 | 与 `92844c60` 在 `adapter/outboundgroup` 的生产代码净差异为零；其余为 qoli 的回滚说明。 |
| [`ab654584`](https://github.com/qoli/mihomo-Alpha/commit/ab65458476de55da11da87e86586de9fee579cc8) | TCP 保留 DNS 映射的真实 IP | 移植代码与回归测试 | 通过已有 `Metadata.Pure()` 分离拨号与规则 metadata；原始 IP 供出站连接使用，域名继续用于规则和连接记录。 |
| [`e860c272`](https://github.com/qoli/mihomo-Alpha/commit/e860c272f9d5fca5efebd1a5078b94c1e8802976) | qoli DNS 修复的现场发布验收 | 不移植 | 证明的是 qoli 自身的二进制、路由器和浏览器环境，不能替代本项目验证。 |

### Smart 风险的代码依据

- 本地 `adapter/outboundgroup/smart.go` 的 `adoptUnwrapWinner`、`closeSameConnection`
  及 `smartDialBatchBounds` 有意在节点切换时维持同一目标的单出口，不能仅删除关闭逻辑。
- qoli 的 `smartSelectionFlightKey` 只有 config/group/target；本地 winner 和候选计算已按
  TCP/UDP 分开。其 `smartStatusProbeAccepted` 无条件接受 403/405，而本地
  `component/smart/response.go::ClassifyResponse` 会区分挑战、限流、区域限制和方法不支持。
- qoli 的 `smartMonitoredConn.sample` 使用累计下载量除以观察时长判定低速。
  例如 128 字节请求在一秒内已取得完整的 16 KiB 响应，之后保持连接空闲，到第三秒仍满足
  `sustained-low-throughput` 条件。此前同节点和域名的连接关闭后，第二条同样的正常连接
  可满足其确认条件，继而被关闭并触发冷却。该提交的 helper 测试没有覆盖“响应已完成”的区别。

以上未移植的 Smart 改动采用静态审查；风险来自实际代码路径推导，不作为运行验证结论。

## 验证范围

TCP 回归通过完整 tunnel、域名规则和本地 SOCKS5 握手检查线上目的地址。
原始测试在未修复代码上复现 mapped IPv4/IPv6 仍发出域名；移植后增加 DNSHosts IPv4/IPv6
及无可用映射 IP 的情况，共覆盖七种目的地址场景，并确认规则 metadata 的域名保留。

发布脚本通过 `actionlint`、YAML 解析及提取后的 shell 语法检查。使用隔离的 git/curl 模拟
执行九种版本/tag 场景，覆盖 rolling、新 tag、已有轻量/附注 tag、tag push、错误 tag 指向
及非法版本；另比较修改前后的完整 build matrix 和 Docker platforms，确认保持一致。
这些检查不等于 GitHub Actions 实际发布验收。

本次 Go 验证额外使用 `-modfile` 指向依赖文件的临时副本，保护工作区原有的 `go.sum` 改动。
完整普通/`with_gvisor` 套件采用 `.github/workflows/build.yml` 已有的
`CGO_ENABLED=0 SKIP_CONCURRENT_TEST=1 SKIP_INTEROP_TEST=1` 设置；race 检查单独运行 TCP 回归。

### 本次结果

环境：macOS ARM64、Go 1.27.3。临时 modfile 与工作区 `go.mod` 内容一致；原有 `go.sum`
已逐字节核对，未改变，也不纳入本次集成提交。

| 检查 | 结果 |
| --- | --- |
| 未修复代码运行原始 TCP 回归 | mapped IPv4/IPv6 失败，复现向 SOCKS 上游发送域名的问题 |
| 修复后的 TCP 目的地址回归 | 七种场景通过 |
| tunnel、outboundgroup、smart、DNS、executor 相关包 | 34 项通过，7 个包 |
| 普通完整套件，采用现有 release CI 环境 | 5039 项通过，189 个包 |
| `with_gvisor` 完整套件，采用相同 CI 环境 | 5039 项通过，189 个包 |
| TCP 回归 `-race -count=5` | 40 项通过，包含父测试计数 |
| `go vet ./...` | 通过 |
| workflow lint、YAML/shell 检查与九种版本/tag 场景 | 通过；构建矩阵和 Docker 平台列表与修改前相同 |
| `with_gvisor`、CGO 关闭的 Darwin ARM64 / Linux amd64-v3 构建 | 均通过；Linux 为交叉编译，未在 Linux 上运行 |
| Darwin 二进制 `-v` 与隔离的最小配置 `-t` | 通过 |
| `git diff --check` | 通过 |

首次未跳过并发压测的完整运行在 `TestInboundVless_XHTTP_H1_Encryption/stream-up`
出现失败。随后用集成前基准 `24cc3b79` 的 `tunnel/tunnel.go` 构建 overlay 对照，
未修改版本也在该组的 `nosplit/reuse/Concurrent` 出现连接重置及超时；当前版本单独复跑该组 48 项通过。
该组使用自有 `TestTunnel`，不经过本次修改的 TCP tunnel 处理函数。保留这一既有并发压测
不稳定记录，不把采用 CI 跳过设置的通过结果解释为所有压力场景均已通过。

本次仅做本地集成；未推送代码，也未触发 GitHub Actions 发布。
