# monitoring — 可观测性子系统

本目录是压测套件的「眼睛」：**客户端指标**（ltagent 内嵌 Prometheus）+ **链路指标**（nginx 代理采集 HTTP 请求）+ **环境定义**（env 模板）三部分，与场景执行脚本（`loadtest-stage-functions.sh`）配套使用。

```
monitoring/
├── nginx-perf/
│   └── nginx.conf.sample      # nginx 反向代理配置样例（HTTP 全链路采集底座）
├── env/
│   ├── loadtest.env.sample    # 主环境定义：路径/地址/代理/用户前缀 + 环境自检函数
│   ├── monitoring.env.sample  # 监控场景编排：带采集的场景执行入口
│   ├── target.env.sample      # 被测对象标识：团队/频道 ID
│   ├── channel-group.env.sample # 频道群发场景专用参数
│   └── message-run.env.sample # 消息场景函数封装
└── README.md
```

## 1. 三层监控模型

| 层 | 数据源 | 覆盖 | 实现 |
|---|---|---|---|
| **客户端层** | ltagent `/metrics`（默认 `127.0.0.1:4001`） | 在线数、WS 连接数、WS 事件计数、WS 序列空洞、HTTP 错误 | Go 内嵌，见 `cmd/ltagent/loadtest.go` |
| **链路层** | nginx access.log（自定义 `perf` 格式） | 每个 HTTP 请求的方法/URI/状态码/request_time/upstream_response_time | `nginx-perf/nginx.conf.sample` |
| **业务层** | 消息体内嵌 `send_unix_nano` + WS 接收侧解析 | 端到端消息延迟（发送→接收全链路） | `simplecontroller/websocket.go` |

**为什么要 nginx 代理这一层**：ltagent 客户端只能看到「我发了什么、多久返回」，无法区分耗时在客户端、网络还是服务端。让全部压测流量经过本机 nginx 代理（`127.0.0.1:18065` → 真实服务器），
`upstream_response_time` 即服务端纯处理耗时，`request_time - upstream_response_time` 即链路+排队开销——两层数据对照，瓶颈定位立刻收敛。

## 2. nginx 代理部署

```bash
# 以样例为底，改 upstream 与 Host 为你的 Mattermost 地址
cp nginx-perf/nginx.conf.sample nginx-perf/nginx.conf
mkdir -p nginx-perf/logs
nginx -p "$PWD/nginx-perf" -c nginx.conf        # 启动
nginx -p "$PWD/nginx-perf" -c nginx.conf -s reload  # 改配置后热加载
```

关键设计（已在样例中配好，勿随意改）：
- `log_format perf`：管道分隔的精简格式，`$msec` 毫秒时间戳，供 `scripts/summarize_http_perf.*` 直接解析；
- `access_log ... buffer=256k flush=1s`：大缓冲 + 1 秒刷盘，**采集自身不能成为压测瓶颈**；
- `proxy_buffering off` + WebSocket `Upgrade` 头透传：WS 长连接不断；
- `/nginx_status`：stub_status 暴露 nginx 自身状态（仅本机）。

## 3. env 环境模板

压测涉及的多台机器（压测机/被测端/监控端）和几十项参数，全部收敛到 env 文件管理——这就是「虚拟环境」：
换环境 = 换一套 env 文件，脚本与代码零改动。

```bash
# 使用方式：复制样例为正式文件（正式文件已被 .gitignore 忽略）
cp env/loadtest.env.sample env/loadtest.env
vim env/loadtest.env        # 填入真实地址/ID
source env/loadtest.env     # 加载到当前 shell
lt_env_check                # 内置自检：打印关键变量并校验目录存在
```

各文件职责：
- **loadtest.env**：根路径、Mattermost 地址、代理地址、结果目录、用户前缀、`LT_METRICS_ADDR`，及 `lt_env_check` 自检函数；
- **monitoring.env**：消息/群聊场景配置指针 + `lt_run_monitored_scenario` 编排函数（就绪门控 + 指标采集 + 结果落盘一体化）；
- **target.env**：被测团队/频道 ID（频道群发场景的靶标）；
- **channel-group.env**：频道群发场景专用（复用已有频道，免建频道开销）；
- **message-run.env**：`lt_run_message` 函数封装（执行后自动带出本次运行目录）。

## 4. 与外部监控栈对接

ltagent 的 `/metrics` 与 nginx 的 `stub_status` 都是标准 Prometheus 端点，可直接被任何 Prometheus 抓取：

```yaml
scrape_configs:
  - job_name: 'ltagent'
    static_configs:
      - targets: ['<压测机IP>:4001']
  - job_name: 'nginx-perf'
    metrics_path: /nginx_status
    static_configs:
      - targets: ['<压测机IP>:18065']
```

Grafana 面板按三层模型组织：客户端（WS 事件/空洞/在线数）→ 链路（HTTP 延迟分布/吞吐）→ 业务（消息端到端延迟）。
