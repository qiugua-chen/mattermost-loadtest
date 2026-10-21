# 问题排查手册

> 适用范围：登录、单聊、GM 群组消息、频道大群四类场景执行中遇到的问题定位。
> 使用方式：先按第 1 节判断问题发生在哪个阶段，再按第 4 节对号入座。

## 1. 先判断问题发生的阶段

IM 压测问题通常不是单一接口报错，而是登录、session、WebSocket、频道权限、工具 action、服务端性能、数据库性能等多因素叠加。

| 阶段 | 常见问题 |
| --- | --- |
| 登录阶段 | 401、timeout、Invalid session |
| 初始化阶段 | 用户不存在、频道不存在、权限不足 |
| 发送阶段 | 499、timeout、`create post failed` |
| 接收阶段 | WebSocket 连接不足、接收延迟文件缺失、missed events |
| 汇总阶段 | 结果文件缺失、统计口径错误 |

## 2. 排查证据清单

每轮压测结果目录（`$RUN_DIR`）中可用的证据文件：

| 文件 | 作用 |
| --- | --- |
| `console.log` | 工具运行错误明细 |
| `http-access.log` | HTTP 状态码、接口、耗时（Nginx 代理日志） |
| `nginx-error.log` | Nginx 错误 |
| `ltagent-status.csv` | 用户数、错误数、WebSocket 连接和事件 |
| `receiver-latency.csv` | 接收延迟明细 |
| `post-summary.txt` | 发消息接口汇总 |
| `login-summary.txt` | 登录接口汇总 |
| `exit_code.txt` | 进程退出码 |

## 3. 排查前置：定位结果目录

```bash
RUN_DIR="$CURRENT_RUN_DIR"
echo "RUN_DIR=$RUN_DIR"
```

如果为空：

```bash
RUN_DIR=$(
  find "$RESULTS_ROOT" -maxdepth 1 -type d \
    \( -name 'S01_LOGIN_*' -o -name 'S02_DIRECT_*' -o -name 'S03_GROUP_*' \) \
    -printf '%T@|%p\n' 2>/dev/null |
  sort -t'|' -k1,1nr |
  head -1 |
  cut -d'|' -f2-
)

echo "RUN_DIR=$RUN_DIR"
```

查看结果文件：

```bash
find "$RUN_DIR" -maxdepth 1 -type f -printf '%f %s bytes\n' | sort
```

查看最后状态：

```bash
awk -F',' '
NR == 1 { print; next }
{ last = $0 }
END { print last }
' "$RUN_DIR/ltagent-status.csv"
```

## 4. 典型问题定位

### 4.1 登录慢

现象：

```text
error while logging in
context deadline exceeded
Client.Timeout exceeded
```

定位：

```bash
"$LT_ROOT/scripts/summarize_http_perf.sh" \
  "$RUN_DIR/http-access.log" 300 \
  "/api/v4/users/login" "POST"
```

处理建议：

- 降低登录并发（`LT_LOGIN_CONCURRENCY`）。
- 延长登录预热。
- 查看服务端 CPU、数据库连接、慢 SQL。

### 4.2 401

现象：`invalid_credentials`、`HTTP_401`。

定位：

```bash
awk -F'|' '
  $3 == "/api/v4/users/login" { count["HTTP_" $4]++ }
  END { for (k in count) print k, count[k] }
' "$RUN_DIR/http-access.log" | sort
```

处理建议：

- 检查用户文件内容是否正确。
- 检查密码策略。
- 检查用户是否已注册。
- 检查是否使用了错误的用户前缀或偏移。

### 4.3 499

含义：客户端在服务端响应前关闭连接。

定位接口分布：

```bash
awk -F'|' '
  $4 == "499" { count[$2 " " $3]++ }
  END { for (k in count) print count[k], k }
' "$RUN_DIR/http-access.log" | sort -nr
```

处理建议：

- 如果集中在登录接口：降低登录并发。
- 如果集中在 `/api/v4/posts`：查看消息接口 P95/P99 和服务端资源。
- 如果同时伴随 timeout：优先查服务端响应慢或客户端超时。

> 实战参考（单节点 8C16G 环境）：单聊在 150/200 并发、频道大群在 1200 在线时首次出现 499——499 的并发拐点就是该环境该链路的容量边界，排查时优先检查 Nginx worker 连接数、Keep-Alive、proxy 超时、Mattermost 连接配置与数据库连接池（详见 [BENCHMARKS.md §6](BENCHMARKS.md)）。

### 4.4 `UserRequired`

现象：`Invalid or expired session`、`UserRequired`。

常见原因：登录失败、session 失效、登录阶段超时后继续执行消息 action。

处理建议：先解决登录成功率；确认登录预热完成；检查用户文件和用户状态。

### 4.5 `memstore: team store is empty`

现象：`control.CreatePost memstore: team store is empty`。

原因：使用了通用 `CreatePost`，而当前预置用户没有初始化 team/channel store。

处理建议：

- 单聊使用 `CreateDirectPost`。
- GM 使用 `CreateGroupPost`。
- 频道大群使用 `CreateChannelGroupPost`。
- 不建议在扩展场景中使用通用 `CreatePost`。

### 4.6 `receiver-latency.csv` 未生成

先判断场景：

| 场景 | 是否应生成 |
| --- | --- |
| 登录 | 不应生成（正常） |
| 单聊 / GM / 频道大群 | 应生成 |

排查：

```bash
echo "$LT_RECEIVER_LATENCY_FILE"
ls -lh "$RUN_DIR/receiver-latency.csv" 2>/dev/null || echo "receiver-latency.csv 未生成"
strings bin/ltagent | grep -E 'send_unix_nano|scenario=S02_DIRECT|scenario=S03_GM|scenario=S03_CHANNEL_GROUP'
```

### 4.7 WebSocket 连接数不足

定位：

```bash
awk -F',' '
NR > 1 && $8 ~ /^[0-9]+$/ {
  if (min == "" || $8 < min) min = $8
  if (max == "" || $8 > max) max = $8
}
END {
  print "ws_connections_min=" min
  print "ws_connections_max=" max
}
' "$RUN_DIR/ltagent-status.csv"
```

处理建议：检查登录成功率；检查 `WebSocketURL`；检查代理是否支持 WebSocket（`Upgrade`/`Connection` 头转发）；检查目标服务 WebSocket 连接限制。

### 4.8 `could not find action`

现象：`could not validate configuration: could not find action "CreateGroupPost"`。

原因：Controller 配置引用了未注册 action，或源码已改但未重新编译。

处理：

```bash
grep -Rni 'CreateGroupPost\|CreateDirectPost\|CreateChannelGroupPost' \
  loadtest/control/simplecontroller/actions.go \
  loadtest/control/simplecontroller/controller.go

go build -o bin/ltagent ./cmd/ltagent
```

### 4.9 启动阶段大量 4xx

压测启动初期用户还在陆续登录/建连，短暂 4xx 属正常现象，待 `actual_users` 达到预期后会收敛。若持续居高不下，优先检查服务端速率限制是否已放开（系统控制台 → 环境 → 速率限制）。

### 4.10 `actual_users: integer expression expected`

原因：Shell 脚本读取 `ltagent-status.csv` 时读到了表头或空值。若最终 `actual_users` 与 `ws_connections` 正常，可按脚本提示类问题处理（整数比较前过滤表头和非数字行即可规避）。

## 5. 推荐排查顺序

1. 先看 `exit_code.txt`。
2. 再看 `console.log` 错误。
3. 再看 `http-access.log` 状态码分布。
4. 再按 URI 过滤登录或发消息接口。
5. 再看 `ltagent-status.csv` 中用户数和 WebSocket。
6. 最后结合服务端日志、数据库和系统资源。

## 6. 常用命令集合

错误 Top：

```bash
grep -Ei 'timeout|UserRequired|memstore|permission=create_post|could not find action|failed' "$RUN_DIR/console.log" |
sort | uniq -c | sort -nr | head -30
```

状态码分布：

```bash
awk -F'|' '{ count[$2 " " $3 " HTTP_" $4]++ } END { for (k in count) print count[k], k }' "$RUN_DIR/http-access.log" | sort -nr | head -50
```

发消息接口状态：

```bash
awk -F'|' '$2 == "POST" && $3 == "/api/v4/posts" { count["HTTP_" $4]++ } END { for (k in count) print k, count[k] }' "$RUN_DIR/http-access.log" | sort
```

## 7. 排查结论模板

```text
问题现象：
影响场景：
影响指标：
定位证据：
初步原因：
处理动作：
复验结果：
遗留风险：
```
