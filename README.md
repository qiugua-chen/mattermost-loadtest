# Mattermost IM 性能测试扩展工具套件（mattermost-loadtest）

> 基于官方 [mattermost-load-test-ng](https://github.com/mattermost/mattermost-load-test-ng) 的二次开发扩展版，
> 面向真实企业 IM 环境（预置账号、免注册、免加入频道）的**登录 / 单聊 / 群聊 / 频道群发**全链路压测与**端到端消息延迟度量**。
>
> **EN**: An extended fork of Mattermost's official load-test-ng, adding deterministic DM/group messaging scenarios,
> end-to-end receiver-latency measurement over WebSocket, embedded Prometheus metrics, login throttling/retry,
> and a staged execution + summarization script suite — designed for pre-provisioned enterprise IM environments.

---

## 与上游的关系

- **上游基线**：`mattermost/mattermost-load-test-ng` @ `784dbc1`（"Add Agents load-test package link (#988)"）
- **许可证**：沿用上游 [Apache-2.0](LICENSE.txt)，上游版权声明保留；本仓库所有新增/修改文件同样以 Apache-2.0 发布
- **改造清单**：逐文件说明见 [docs/EXT-CHANGES.md](docs/EXT-CHANGES.md)
- **复用指南**：不同被测系统下的迁移方法见 [docs/REUSE.md](docs/REUSE.md)

## 文档导航

**使用路径**（按阅读顺序）：

| 文档 | 内容 | 什么时候读 |
|---|---|---|
| [docs/DESIGN.md](docs/DESIGN.md) | 改造设计动机：上游工具的 6 个痛点与本扩展的解法 | 想理解"为什么有这个 fork" |
| [docs/SCENARIOS.md](docs/SCENARIOS.md) | 四类核心场景（登录/单聊/GM/频道大群）执行手册 | 正式执行压测前 |
| [docs/METRICS.md](docs/METRICS.md) | 指标口径：五层指标、主结论边界、报告输出模板 | 看结果、写报告前 |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | 问题排查：9+ 个典型故障模式、排查顺序、命令集 | 压测报错时 |
| [docs/RUNBOOK.md](docs/RUNBOOK.md) | 运行手册：结果文件字典、标准流程、交接注意事项 | 日常执行与交接 |
| [monitoring/README.md](monitoring/README.md) | 三层监控模型与 nginx 链路采集启用方法 | 需要 HTTP 全链路证据时 |

**扩展版专题**：[EXT-CHANGES.md](docs/EXT-CHANGES.md)（逐文件改造清单）、[REUSE.md](docs/REUSE.md)（跨环境/跨产品复用决策表）

**上游文档**（中文译版）：[docs/readme.md](docs/readme.md) 为索引入口；核心篇目含 [本地运行](docs/local_loadtest.md)、[Controller 体系](docs/controllers.md)、[配置说明](docs/config/config.md)、[FAQ](docs/faq.md)

## 改造亮点

| 能力 | 说明 | 关键文件 |
|---|---|---|
| **确定性单聊压测（S02）** | 按用户名数字后缀配对（1↔2、3↔4…），DM 频道建一次后缓存，稳态流量不含建频道开销 | `simplecontroller/actions.go` |
| **确定性群聊压测（S03）** | 按 `LT_GROUP_SIZE` 分组（默认 3 人群），编号连续分组，成员确定性解析 | `simplecontroller/actions.go` |
| **频道群发压测（S03B）** | 单频道 N 发 M 收，开始信号文件门控（`LT_ACTION_START_FILE`），收发两侧可分别部署 | `simplecontroller/actions.go` |
| **端到端接收延迟** | 消息体携带 `send_unix_nano`，接收侧 WebSocket `posted` 事件实时解析，毫秒级延迟落盘 | `simplecontroller/websocket.go` |
| **内嵌 Prometheus 指标** | ltagent 自带 `/metrics`（默认 `127.0.0.1:4001`）+ `/status`，新增 WS 事件计数与序列空洞指标 | `cmd/ltagent/loadtest.go`、`performance/metrics.go` |
| **登录治理** | 登录并发限流 `LT_LOGIN_CONCURRENCY`、失败重试 `LT_LOGIN_RETRIES`、跳过注册 `LT_SKIP_SIGNUP` | `loadtest/control/actions.go` |
| **预置账号适配** | 免注册、免加入团队/频道的容错降级（插件禁用、无当前团队不再判失败） | `loadtest/control/actions.go`、`noopcontroller` |
| **舞台化执行脚本** | 就绪门控（实际在线数 + WS 连接数双达标才计时）、场景化一键执行、结果自动汇总 | `loadtest-stage-functions.sh`、`scripts/` |
| **用户工程脚本** | 批量预置用户、幂等分配团队/频道成员（凭据走环境变量，不落盘） | `scripts/mattermost_*.py` |
| **可观测性子系统** | nginx 反向代理全链路 HTTP 采集（request_time/upstream_time 落盘）+ 环境定义模板 + 三层监控模型 | `monitoring/` |

## 环境变量一览

| 变量 | 默认 | 作用 |
|---|---|---|
| `LT_SKIP_SIGNUP` | - | `true` 时跳过注册（预置账号场景必设） |
| `LT_LOGIN_CONCURRENCY` | 0（不限） | 登录并发上限 |
| `LT_LOGIN_RETRIES` | 0 | 登录失败重试次数 |
| `LT_LOGIN_RETRY_SLEEP_MS` | 1000 | 重试间隔（毫秒） |
| `LT_METRICS_ADDR` | `127.0.0.1:4001` | 内嵌 metrics 服务监听地址 |
| `LT_GROUP_SIZE` | 3 | 群聊分组人数 |
| `LT_CHANNEL_GROUP_CHANNEL_ID` | - | 频道群发场景目标频道 ID |
| `LT_CHANNEL_GROUP_SENDERS` | 1 | 频道群发场景发送者数量 |
| `LT_ACTION_START_FILE` | - | 正式开始信号文件（存在即进入发送态） |
| `LT_RECEIVER_LATENCY_FILE` | - | 接收延迟落盘文件（不设置则不采集） |
| `MM_TOKEN` | - | 用户工程脚本的 admin token（仅 `scripts/mattermost_*.py` 使用） |

## 构建

要求 Go 1.24+（开发基线为 go1.26.3）：

```bash
# 核心压测 agent
go build -o bin/ltagent ./cmd/ltagent

# 全量编译检查
go build ./...
```

## 最小复现（冒烟：登录 → 单聊 → 群聊）

> 前置：一个可用的 Mattermost 实例（地址记为 `http://<MM_HOST>:8065`），一个 system_admin 账号。
> **压测前请关闭或放宽服务端内置速率限制**（系统控制台 → 环境 → 速率限制），
> 否则冒烟会被 429 打挂、数字失真（实战经验高频坑）。

**1. 预置测试用户**（注册是登录的前置条件；用户名形如 `imlt00001`，密码统一，从环境变量读取）：

```bash
export MM_TOKEN=<admin_token>
export IMLT_USER_PASSWORD='<统一密码>'
python3 scripts/mattermost_prepare_users.py \
  --server http://<MM_HOST>:8065 \
  --count 20 --prefix imlt \
  --output ./config/users/imlt-users.txt \
  --result-log ./results/prepare_users.jsonl
```

> 冒烟的登录/单聊/群聊用例只需建号即可（扩展 action 对无团队/频道已容错）；
> 若要跑**频道大群（S03B）**，还须执行入团入频道，完整链路见
> [docs/SCENARIOS.md 第 3 节 S00](docs/SCENARIOS.md#3-s00-用户注册与准备前置场景)。

**2. 配置连接**：复制 `config/config-smoke.sample.json` 为 `config/config-smoke.json`，
填入 `ServerURL` / `WebSocketURL` / `AdminEmail` / `AdminPassword`，
并把 `UsersConfiguration.UsersFilePath` 由 `""` 改为 `"./config/users/imlt-users.txt"`（指向上一步生成的账号文件）。
真实配置已被 `.gitignore` 忽略，不会误提交。

> ⚠️ 关键：`UsersFilePath` 留空时 ltagent 使用内置生成账号（用户名 `<prefix>-N`、固定密码），
> 与 `LT_SKIP_SIGNUP=true` 的预置账号模式不匹配，登录会全部失败；且消息场景（单聊/群聊）
> 依赖用户名末 5 位数字做确定性配对，必须使用预置账号文件。

**2.1 准备运行目录**（ltagent 不会自动创建日志/结果目录；`config-smoke.sample.json` 的日志路径为 `./logs/agent01/`）：

```bash
mkdir -p logs/agent01 results
```

**3. 登录冒烟**（10 用户，60 秒）：

```bash
export LT_SKIP_SIGNUP=true LT_LOGIN_CONCURRENCY=5 LT_LOGIN_RETRIES=3
./bin/ltagent loadtest -c config/config-smoke.json -n 10 -d 60 \
  --user-prefix imlt --controller-config config/simplecontroller-login.sample.json
```

**4. 单聊冒烟**（10 用户 = 5 对，120 秒，采集接收延迟）：

```bash
export LT_RECEIVER_LATENCY_FILE=./results/smoke_direct_latency.txt
./bin/ltagent loadtest -c config/config-smoke.json -n 10 -d 120 \
  --user-prefix imlt --controller-config config/formal/simplecontroller-direct.json
```

**5. 群聊冒烟**（9 用户 = 3 组三人行，120 秒）：

```bash
export LT_GROUP_SIZE=3
./bin/ltagent loadtest -c config/config-smoke.json -n 9 -d 120 \
  --user-prefix imlt --controller-config config/formal/simplecontroller-group.json
```

**6. 验证结果**：

```bash
# 延迟分布汇总
bash scripts/summarize_receiver_latency.sh ./results/smoke_direct_latency.txt
# ltagent 指标
curl -s http://127.0.0.1:4001/metrics | head
```

预期：无 `ERRO` 级别日志；单聊/群聊日志出现 `direct post created` / `group post created`；延迟文件有数据且 P95 在可解释范围内。

## 目录说明（自定义部分）

```
loadtest-stage-functions.sh        # 舞台化执行函数库（就绪门控、监控采集编排）
scripts/
  mattermost_prepare_users.py      # 批量预置测试用户
  mattermost_assign_users.py       # 幂等分配用户到团队/频道
  lt_run_message.sh                # 消息场景一键执行入口
  collect_ltagent_metrics.sh       # 采集各 agent 的 Prometheus 指标
  summarize_http_perf.sh/.py       # HTTP 性能汇总
  summarize_message_metrics.sh     # 消息指标汇总
  summarize_receiver_latency.sh    # 接收延迟分布汇总
  S02_direct_window_report.py      # 单聊时间窗口报告
config/formal/                     # 正式场景参数样例（1200 用户级）
config/*.sample.json               # 冒烟/agent 配置样例（复制后填真实值）
monitoring/                        # 可观测性子系统
  nginx-perf/nginx.conf.sample     #   nginx 反向代理全链路采集配置（perf 日志格式）
  env/*.env.sample                 #   环境定义模板（loadtest/monitoring/target/channel-group）
  README.md                        #   三层监控模型与对接说明
docs/EXT-CHANGES.md                # 逐文件改造清单
docs/REUSE.md                      # 跨环境/跨产品复用指南
```

## 可观测性（监控子系统）

本套件采用**三层监控模型**，压测数据从三个层面交叉核对，任何一层异常都能被发现：

| 层 | 数据源 | 采集方式 | 回答的问题 |
|---|---|---|---|
| **客户端层** | ltagent 内嵌 `/metrics` + `/status` | `scripts/collect_ltagent_metrics.sh` 周期抓取为时序 CSV | 压测侧发出了多少请求？WS 连接是否达标？有没有丢事件？ |
| **链路层** | nginx-perf 代理 access log | `monitoring/nginx-perf/` 反向代理，所有压测 HTTP 流经代理落盘 | 服务端实际收到多少？request_time / upstream_time 分布如何？ |
| **业务层** | 端到端接收延迟 CSV | 消息体内嵌 `send_unix_nano`，接收侧 WS 事件实时解析 | 用户视角的消息收发延迟是多少？ |

快速启用链路层采集：

```bash
cp monitoring/nginx-perf/nginx.conf.sample monitoring/nginx-perf/nginx.conf
# 编辑 nginx.conf：把 mattermost.example.com 替换为真实 Mattermost 地址
cp monitoring/env/loadtest.env.sample  runtime/env/loadtest.env   # 按注释填值
```

详细说明见 [monitoring/README.md](monitoring/README.md)。

> 服务端侧监控（Prometheus + Grafana 部署、Mattermost 主机 exporter）属基础设施范畴，
> 不包含在本仓库内；`monitoring/README.md` 给出了对接约定。

## 许可证

[Apache-2.0](LICENSE.txt)。本项目包含来自 [mattermost/mattermost-load-test-ng](https://github.com/mattermost/mattermost-load-test-ng) 的代码，
原版权归 Mattermost, Inc. 所有；新增与修改部分同样以 Apache-2.0 发布，使用时请保留上游许可与来源声明。

---

<details>
<summary>上游 README 原文</summary>

Mattermost load-test-ng provides a set of tools written in Go to help profiling Mattermost under heavy load,
simulating real-world usage of a server installation at scale. It's a complete rewrite of the previous load-test tool.
Documentation and implementation details can be found in the [docs](docs/) folder.

</details>
