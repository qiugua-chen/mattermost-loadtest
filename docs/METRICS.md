# 指标口径说明

> 适用范围：登录、单聊（Direct）、GM 群组消息、频道大群四类场景
> 阅读建议：执行压测前先读本文，明确"看哪些指标、口径是什么、主结论怎么下"。

## 1. 为什么 IM 压测需要专门的指标口径

IM 性能测试与普通 HTTP 接口压测不同。普通接口压测只看请求成功率和响应时间通常已经足够，但 IM 消息场景还必须验证：

- 消息是否成功创建。
- 接收端是否收到 WebSocket 事件。
- 接收延迟是否满足体验要求。
- 是否存在漏接、慢接、WebSocket 连接不足。

因此，本工具的指标口径分为五层：

```text
登录指标 + 发送接口指标 + WebSocket 事件指标 + 接收延迟指标 + 工具错误指标
```

## 2. 指标采集来源（改造与指标的对应关系）

| 改造 | 指标收益 |
| --- | --- |
| Nginx 代理采集（`monitoring/nginx-perf/`） | 获取 HTTP 请求数、状态码、响应时间、TPS |
| `/api/v4/posts` 单独汇总 | 提取消息发送核心接口指标 |
| 接收延迟埋点（`recordReceiverLatency`） | 获取 WebSocket 消息到达延迟 |
| `ltagent-status.csv` | 获取用户数、连接数、错误数、事件数 |
| 错误 Top 统计 | 快速定位常见异常 |

## 3. 各场景主指标

### 3.1 S01 登录

| 指标 | 说明 |
| --- | --- |
| 登录请求数 | `/api/v4/users/login` 请求总数 |
| 登录成功率 | HTTP 200 占比 |
| 登录 P95/P99 | 登录接口响应时间分位值 |
| 401 数量 | 账号密码或用户状态异常 |
| 499 数量 | 客户端提前关闭或超时 |
| `actual_users` | 实际登录用户数 |
| `loadtest_errors` | 登录动作错误 |

> 登录场景**不统计接收延迟**——不发消息，不生成 `receiver-latency.csv` 属正常现象。

### 3.2 S02/S03 消息场景

| 指标 | 说明 |
| --- | --- |
| `/api/v4/posts` 请求数 | 发消息请求数量 |
| `/api/v4/posts` 成功率 | HTTP 201 成功占比 |
| TPS | 发消息吞吐 |
| HTTP P95/P99 | 发消息接口响应时间 |
| 接收延迟 P95/P99 | WebSocket posted 事件到达延迟 |
| `ws_connections` | WebSocket 在线连接数 |
| `ws_posted_events` | 收到的 posted 事件数 |
| `ws_missed_events` | 工具统计的 missed events |
| `loadtest_errors` | 压测动作错误 |
| `http_timeouts` | HTTP 请求超时 |

## 4. 指标口径定义

### 4.1 登录成功率

统计对象：`POST /api/v4/users/login`

```text
登录成功率 = 登录成功数 / 登录请求总数
```

### 4.2 消息接口成功率

统计对象：`POST /api/v4/posts`，成功状态为 `HTTP 201`。

> ⚠️ **HTTP 201 只表示服务端创建消息成功，不表示接收端已经收到。**
> 接收侧证据必须看接收延迟与 posted 事件数。

### 4.3 HTTP P95/P99

- P95：95% 请求响应时间小于等于该值。
- P99：99% 请求响应时间小于等于该值。

登录场景看登录接口 P95/P99；消息场景看 `/api/v4/posts` P95/P99。

### 4.4 WebSocket 到达率

用于判断消息事件是否按预期到达，需结合以下数据综合判断：

- 成功发送消息数。
- 理论接收人数（单聊每条消息理论 posted 事件数通常为 2）。
- `ws_posted_events`。
- `ws_missed_events`。
- `receiver-latency.csv` 样本数。

### 4.5 接收延迟 P95/P99

```text
接收延迟 = WebSocket 收到 posted 事件时间 - 消息发送时间
```

产物文件：

```text
receiver-latency.csv
receiver-latency-summary.txt
```

汇总命令：

```bash
scripts/summarize_receiver_latency.sh "$RUN_DIR/receiver-latency.csv"
```

### 4.6 499 / timeout / loadtest_errors

| 指标 | 含义 |
| --- | --- |
| HTTP 499 | 客户端在服务端响应前关闭连接 |
| timeout | 客户端等待服务端响应超时 |
| `loadtest_errors` | 工具执行 action 时发生错误 |

这些指标不应孤立解释，需要结合 `console.log` 和接口分布分析（见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md)）。

## 5. 口径使用的两个常见误区

### 5.1 整体 HTTP 成功率 ≠ 业务接口成功率

整体 HTTP 汇总包含登录、初始化、WebSocket 101、频道检查等全部请求；业务接口汇总只看 `/api/v4/posts`。两者不一致是预期现象。

```text
消息场景主结论应以 /api/v4/posts 和接收延迟为主。
```

### 5.2 接收延迟文件为空的判断

- 登录场景为空：**正常**。
- 消息场景为空：**异常**，排查：

```bash
echo "$LT_RECEIVER_LATENCY_FILE"
ls -lh "$RUN_DIR/receiver-latency.csv"
strings bin/ltagent | grep 'send_unix_nano'
```

### 5.3 posted events 与理论值不一致

需检查：是否包含发送者回显、是否包含窗口外消息、是否有排空阶段、是否有解析失败、是否有 WebSocket 断连。

## 6. 报告输出模板

复用本口径时，建议报告固定输出以下表格：

| 场景 | 请求数 | 成功率 | TPS | HTTP P95 | HTTP P99 | 接收 P95 | 接收 P99 | 错误 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 登录 |  |  |  |  |  | 不适用 | 不适用 |  |
| 单聊 |  |  |  |  |  |  |  |  |
| GM |  |  |  |  |  |  |  |  |
| 频道大群 |  |  |  |  |  |  |  |  |

报告中应明确：

- 哪些指标是主结论。
- 哪些指标只是辅助排障。
- 哪些场景未执行或证据不足。
- **不将局部专项通过扩大为整体通过。**
