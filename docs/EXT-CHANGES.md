# 改造清单（相对上游基线）

**上游基线**：`mattermost/mattermost-load-test-ng` @ `784dbc1`（"Add Agents load-test package link (#988)"）

**总体规模**：9 个上游文件修改（+582 / -23 行），新增 1 个舞台化执行函数库 + 9 个配套脚本 + 场景化配置样例。

---

## 1. Go 源码修改（9 个文件）

### 1.1 `cmd/ltagent/loadtest.go`（+50）
- ltagent 进程内嵌 HTTP 服务：`/metrics`（Prometheus）+ `/status`（压测状态 JSON）。
- 监听地址由 `LT_METRICS_ADDR` 控制，默认 `127.0.0.1:4001`。
- 压测停止后增加 3 秒收尾缓冲，避免尾部指标/延迟数据丢失。
- **为什么**：上游只能通过外部 ltcoordinator 汇总指标，单机/多 agent 直跑场景下没有可观测出口。

### 1.2 `loadtest/control/actions.go`（+82）
- 登录并发限流：`acquireLoginSlot()`，由 `LT_LOGIN_CONCURRENCY` 控制（信号量实现，`sync.Once` 懒初始化）。
- 登录重试：`withLoginRetry()`，由 `LT_LOGIN_RETRIES` / `LT_LOGIN_RETRY_SLEEP_MS` 控制。
- `LT_SKIP_SIGNUP=true` 时跳过注册动作（预置账号环境）。
- `Reload` 容错：插件被禁用（`Plugins have been disabled`）不再判失败；用户无当前团队时回退到其第一个团队。
- **为什么**：大规模登录风暴会打挂被测端；企业预置账号环境没有"注册-加入团队"流程，上游的硬报错路径全部需要降级。

### 1.3 `loadtest/control/noopcontroller/actions.go`（±1）
- 预置用户无当前团队时，`joinChannel` 由报错降级为提示性跳过。
- **为什么**：配套 1.2，noop 控制器同样需要容忍预置账号的初始状态。

### 1.4 `loadtest/control/simplecontroller/actions.go`（+390，核心）
- **`createDirectPost`（S02 单聊）**：按用户名数字后缀确定性配对（奇偶配对 1↔2、3↔4）；DM 频道解析一次后缓存到 `directChannelID`，稳态消息流量不含建频道开销；消息体携带 `send_unix_nano` 时间戳与场景标签。
- **`createGroupPost`（S03 群聊）**：按 `LT_GROUP_SIZE`（默认 3）将连续编号的测试用户确定性分组；成员通过 `GetUsersByUsernames` 解析；消息体同样携带时间戳。
- **`createChannelGroupPost`（S03B 频道群发）**：面向指定频道（`LT_CHANNEL_GROUP_CHANNEL_ID`）的 N 发 M 收模型；未收到开始信号（`LT_ACTION_START_FILE` 文件不存在）时只做成员就位；`LT_CHANNEL_GROUP_SENDERS` 之外的用户纯接收。
- **`recordReceiverLatency`**：解析 WebSocket `posted` 事件中的 `send_unix_nano`，计算端到端接收延迟（毫秒），追加写入 `LT_RECEIVER_LATENCY_FILE`；对非本工具消息静默忽略。
- **为什么**：上游的 CreatePost 面向随机公共频道，无法构成"单聊/群聊"这种 IM 核心场景的压力模型，也没有端到端延迟度量手段。

### 1.5 `loadtest/control/simplecontroller/controller.go`（+29）
- 注册三个新 action：`CreateDirectPost` / `CreateGroupPost` / `CreateChannelGroupPost`，可在控制器 JSON 配置中直接引用。
- 新增 `directChannelID` 缓存字段。

### 1.6 `loadtest/control/simplecontroller/websocket.go`（+1）
- `wsEventHandler` 入口接入 `recordReceiverLatency`，每个 WS 事件先过延迟记录再分发。

### 1.7 `loadtest/user/userentity/metrics.go`（+14）& 1.8 `loadtest/user/userentity/websocket.go`（+3）
- UserEntity 侧上报 WS 事件计数与序列空洞（配合 1.9）。

### 1.9 `performance/metrics.go`（+31）
- 新增 Prometheus 指标：
  - `loadtest_websocket_events_received_total{event=...}`：按事件类型计数；
  - `loadtest_websocket_missed_events_total`：WS 序列号空洞计数（丢事件探测）。
- **为什么**：消息压测必须回答"接收侧有没有丢事件"，上游只有连接数没有事件级指标。

---

## 2. 新增脚本

| 文件 | 作用 |
|---|---|
| `loadtest-stage-functions.sh` | 舞台化执行函数库：`lt_wait_action_ready`（实际在线数 + WS 连接数双达标门控）、`lt_run_monitored_scenario_gated`（带监控采集的场景执行）等 |
| `scripts/mattermost_prepare_users.py` | 批量预置确定性测试用户（前缀 + 5 位序号）；admin token 从 `MM_TOKEN` 读取；输出 ltagent 凭据文件（建议 chmod 600） |
| `scripts/mattermost_assign_users.py` | 幂等分配用户到团队/频道；重复执行不产生重复成员 |
| `scripts/lt_run_message.sh` | 消息场景一键执行入口（环境文件 + 参数校验 + 编排） |
| `scripts/collect_ltagent_metrics.sh` | 采集各 agent `/metrics` 到结果目录 |
| `scripts/summarize_http_perf.sh` / `.py` | HTTP 性能（吞吐/延迟/错误）汇总 |
| `scripts/summarize_message_metrics.sh` | 消息收发指标汇总 |
| `scripts/summarize_receiver_latency.sh` | 接收延迟分布（P50/P95/P99/Max）汇总 |
| `scripts/S02_direct_window_report.py` | 单聊场景按时间窗口的收发对齐报告 |
| `scripts/render_formal_configs.sh` | 由 `config/formal/config-1200.tpl.json` 单一模板渲染五份正式场景主配置（防漂移，CI 校验） |

### 2.1 新增纯函数与单测

| 文件 | 作用 |
|---|---|
| `loadtest/control/simplecontroller/ext_helpers.go` | 从场景动作中提取的纯函数：`groupMemberNumbers`（确定性分组）、`directPartnerUsername`（确定性配对）、`parseSendUnixNano`（延迟标记解析） |
| `loadtest/control/simplecontroller/ext_helpers_test.go` | 上述纯函数的 table-driven 单测（无服务端依赖，CI 直接运行） |

### 2.2 工程化（CI/CD）

| 文件 | 作用 |
|---|---|
| `.github/workflows/ci.yml` | 轻量门禁：build → vet → gofmt → `go test ./loadtest/...` → JSON 配置校验 → formal 配置漂移检查 |
| `.github/workflows/release.yml` | 打 `v*` 标签时交叉编译 linux/amd64+arm64 的 ltagent/ltapi 并打包发 Release（新设备免 Go 环境） |

### 2.3 实战沉淀文档

| 文件 | 作用 |
|---|---|
| `docs/BENCHMARKS.md` | 单节点 8C16G 参考环境下 S01~S03B 四场景分档实测数据、边界信号（HTTP 499 拐点）与瓶颈分析（PG COMMIT 慢/机械硬盘） |
| `docs/METHODOLOGY.md` | IM 压测方法论：验收标准矩阵模板、压力模型推导、消息核账口径（协议重复 vs 业务重复）、结果无效判定、执行前清单 |

## 3. 新增配置样例

- `config/config-smoke.sample.json`、`config/config-agent01/02.sample.json`、`config/config-login.sample.json`：连接与压测参数样例。
- `config/simplecontroller-{login,receiver,sender,message-smoke}.sample.json`：控制器动作序列样例。
- `config/formal/`：正式场景（1200 虚拟用户级）参数样例——登录 / 单聊 / 群聊 / 频道群发 / 消息混合，含对应控制器配置。
- `config/users/`：ltagent 凭据文件占位。

> 所有样例中的地址、邮箱、密码均已替换为占位符（`mattermost.example.com` / `sysadmin@example.com` / `CHANGEME`）。

## 4. 未改动但重要的上游资产

- `deployment/terraform/`：AWS 一键部署压测集群（本扩展未使用，保留原样）。
- `coordinator/`、`api/`：多 agent 协调框架（本扩展的单机多 agent 模式不依赖，保留原样）。
- `docs/`：上游文档（本文件与 `REUSE.md` 为新增）。
