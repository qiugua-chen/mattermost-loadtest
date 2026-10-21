# 运行手册（Runbook）

> 面向日常执行的参考：结果文件字典、标准执行流程、交接注意事项。
> 场景参数与阶梯建议见 [SCENARIOS.md](SCENARIOS.md)，报错对号入座见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md)。

## 1. 结果目录文件字典

每轮压测在 `$RESULTS_ROOT` 下生成独立结果目录，命名规则：`场景名_用户数U_时间戳`，例如 `S02_DIRECT_100U_20260930_111058`。

| 文件 | 重要性 | 说明 |
| --- | --- | --- |
| `config-proxy.json` | 重要 | 本轮走 Nginx 代理的 ltagent 配置快照 |
| `console.log` | 重要 | ltagent 控制台输出和错误明细 |
| `exit_code.txt` | 重要 | ltagent 退出码，0 表示正常结束 |
| `http-access.log` | 重要 | Nginx HTTP 请求明细（状态码/接口/耗时） |
| `http-summary.txt` | 重要 | HTTP 整体汇总（辅助排障，不作主结论） |
| `post-summary.txt` | 重要 | `/api/v4/posts` 发消息接口汇总 |
| `login-summary.txt` | 登录场景重要 | `/api/v4/users/login` 登录接口汇总 |
| `receiver-latency.csv` | 消息场景重要 | 接收延迟明细 |
| `receiver-latency-summary.txt` | 消息场景重要 | 接收延迟 P50/P90/P95/P99 汇总 |
| `ltagent-status.csv` | 重要 | 用户数、错误、WebSocket 连接和事件采样 |
| `nginx-error.log` | 重要 | Nginx 错误日志 |
| `action-start-epoch.txt` | gated 场景重要 | 正式窗口开始时间（epoch） |
| `action-start.gate` | gated 场景重要 | 正式窗口门控标记 |
| `window-report.txt` | gated 场景重要 | 正式窗口统计报告 |
| `ltagent-pidstat.log` | 辅助 | 压测进程 CPU、内存、IO |
| `ltagent-network.log` | 辅助 | 网络采样 |
| `metrics-collector.log` | 辅助 | 状态采集脚本日志 |

## 2. 标准执行流程

### 2.1 初始化

```bash
cd "$LT_ROOT"
source runtime/env/loadtest.env
source runtime/env/monitoring.env
source runtime/env/target.env 2>/dev/null || true
source runtime/env/channel-group.env 2>/dev/null || true
source loadtest-stage-functions.sh
```

### 2.2 检查

```bash
test -x "$LT_ROOT/bin/ltagent" && echo "ltagent=PASS" || echo "ltagent=FAIL"
nginx -t -c "$LT_ROOT/runtime/nginx-perf/nginx.conf"
ss -lntp | grep ':18065' || echo "18065端口未监听"

declare -p FORMAL_USERS LT_USER_PREFIX LT_USER_OFFSET 2>/dev/null || true
test -n "${FORMAL_USERS:-}" && test -s "$FORMAL_USERS" &&
  echo "用户文件=PASS：$FORMAL_USERS" || echo "用户文件=FAIL"
test -s "$FORMAL_USERS" && wc -l "$FORMAL_USERS"
```

> 用户尚未注册或需要补齐团队/频道成员关系时，先按
> [SCENARIOS.md 第 3 节 S00](SCENARIOS.md#3-s00-用户注册与准备前置场景)
> 执行建号 → 入团 → 入频道，再继续后续步骤。

### 2.3 按场景执行

```bash
lt_run_login 40 300                          # S01 登录
lt_run_direct_gated 100 300                  # S02 单聊（偶数用户）
lt_run_group_gated 120 300 5                 # S03A GM
lt_run_channel_group_gated 100 300 100 5     # S03B 频道大群
```

### 2.4 每轮结束后汇总

```bash
RUN_DIR="$CURRENT_RUN_DIR"
DURATION=300

"$LT_ROOT/scripts/summarize_http_perf.sh" \
  "$RUN_DIR/http-access.log" "$DURATION" \
  "/api/v4/posts" "POST" |
tee "$RUN_DIR/post-summary.txt"

scripts/summarize_receiver_latency.sh \
  "$RUN_DIR/receiver-latency.csv" |
tee "$RUN_DIR/receiver-latency-summary.txt"

awk -F',' 'NR == 1 { print; next } { last = $0 } END { print last }' \
  "$RUN_DIR/ltagent-status.csv"
```

### 2.5 异常中断与清理

先 `Ctrl + C`；有残留进程时：

```bash
pkill -f './bin/ltagent' 2>/dev/null || true
pkill -f 'collect_ltagent_metrics.sh' 2>/dev/null || true
pkill -f 'pidstat' 2>/dev/null || true
pkill -f 'sar -n DEV,TCP,ETCP' 2>/dev/null || true
pgrep -af 'ltagent|collect_ltagent_metrics|pidstat|sar' || echo "压测相关进程已清理"
```

## 3. 交接注意事项

1. 不要只看控制台是否结束，要看 `exit_code.txt`、`post-summary.txt`、`ltagent-status.csv`。
2. 登录场景没有接收延迟文件是正常现象。
3. 消息场景必须检查 `receiver-latency.csv` 已生成。
4. 频道大群必须确认 `channel-group.env` 中的频道 ID 真实有效。
5. 单聊用户数必须为偶数。
6. HTTP 整体成功率不等于核心接口成功率。
7. 出现 499 时先定位接口，再判断原因。
8. 修改 Go 代码后必须重新编译 `bin/ltagent`。
9. 新开 Shell 必须重新 source 环境文件和函数文件。
10. 每轮结果必须保留 Run 目录，便于报告和复查。
11. 异常中断后不要立即删除结果目录，先保留 `console.log`、`http-access.log`、`ltagent-status.csv` 分析原因。

## 4. 构建命令（唯一推荐方式）

```bash
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -mod=readonly -trimpath -o "$LT_ROOT/bin/ltagent" ./cmd/ltagent
```

> 不建议执行 `make build`——可能触发 browser/npm 前端构建或多平台构建。
