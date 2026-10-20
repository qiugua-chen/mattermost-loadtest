# 核心场景执行手册

> 覆盖场景：S01 登录、S02 单聊（Direct）、S03A GM 群组消息、S03B 频道大群。
> 本文侧重"怎么跑"，源码改造细节见 [EXT-CHANGES.md](EXT-CHANGES.md)，指标解释见 [METRICS.md](METRICS.md)。

## 1. 场景模型总览

| 场景编号 | 场景名称 | 执行函数 | 核心验证目标 |
| --- | --- | --- | --- |
| S01 | 登录 | `lt_run_login` | 登录接口成功率、响应时间、登录错误 |
| S02 | Direct 单聊 | `lt_run_direct_gated` | 一对一消息发送与接收延迟 |
| S03A | GM 群组消息 | `lt_run_group_gated` | 3/5/8 人群组消息发送与接收 |
| S03B | 频道大群 | `lt_run_channel_group_gated` | 大频道在线成员接收推送能力 |

关键认知：

- 登录场景不发消息，不生成 `receiver-latency.csv` 属正常。
- 单聊采用 `1↔2`、`3↔4` 固定配对，**用户数必须为偶数**，每条消息理论 posted 事件数通常为 2。
- GM 群组消息和频道大群**不是同一个场景**，不能混用结论。
- 频道大群通常不是全员发消息，而是部分用户发送、全部在线成员接收。

## 2. 执行前初始化

每次新开 Shell 先执行（`$LT_ROOT` 为仓库部署根目录）：

```bash
cd "$LT_ROOT"

source runtime/env/loadtest.env
source runtime/env/monitoring.env
source runtime/env/target.env 2>/dev/null || true
source runtime/env/channel-group.env 2>/dev/null || true
source loadtest-stage-functions.sh
```

> 说明：`runtime/env/` 为部署时从 `monitoring/env/*.env.sample` 复制生成的真实环境文件（已被 `.gitignore` 忽略）。

环境与函数自检：

```bash
test -x "$LT_ROOT/bin/ltagent" && echo "ltagent=PASS" || echo "ltagent=FAIL"
declare -f lt_run_direct_gated >/dev/null && echo "lt_run_direct_gated=PASS" || echo "lt_run_direct_gated=FAIL"
declare -f lt_run_group_gated >/dev/null && echo "lt_run_group_gated=PASS" || echo "lt_run_group_gated=FAIL"
declare -f lt_run_channel_group_gated >/dev/null && echo "lt_run_channel_group_gated=PASS" || echo "lt_run_channel_group_gated=FAIL"
```

用户文件检查：

```bash
declare -p FORMAL_USERS LT_USER_PREFIX LT_USER_OFFSET 2>/dev/null || true

test -n "${FORMAL_USERS:-}" && test -s "$FORMAL_USERS" &&
echo "用户文件=PASS：$FORMAL_USERS" ||
echo "用户文件=FAIL：请检查 FORMAL_USERS"

test -s "$FORMAL_USERS" && wc -l "$FORMAL_USERS"
```

## 3. S01 登录

执行：

```bash
lt_run_login 40 300
```

| 参数 | 含义 |
| --- | --- |
| 第 1 个参数 | 登录并发用户数 |
| 第 2 个参数 | 持续时间（秒） |

重点观察：`/api/v4/users/login` 成功率、登录 P95/P99、401、timeout、`actual_users`。

## 4. S02 单聊

执行：

```bash
lt_run_direct_gated <在线用户数> <正式压测秒数>
```

示例与推荐阶梯（先低并发确认正常，再逐步提高）：

```bash
lt_run_direct_gated 50 300
lt_run_direct_gated 100 300
lt_run_direct_gated 150 300
lt_run_direct_gated 200 300
```

> 150/200 及以上如出现 499 或 timeout，需单独分析，不要直接认定脚本异常。

## 5. S03A GM 群组消息

执行：

```bash
lt_run_group_gated <在线用户数> <正式压测秒数> <GM群组人数>
```

示例：

```bash
lt_run_group_gated 30 300 3
lt_run_group_gated 120 300 5
lt_run_group_gated 150 300 8
```

> 注意口径：`lt_run_group_gated 30 300 3` 表示"30 个用户在线，按 3 人 GM 群组模型组织消息压测"，
> 而不是"30 人在一个群里、只有 3 人在线"。

## 6. S03B 频道大群

执行：

```bash
lt_run_channel_group_gated <在线用户数> <正式压测秒数> <频道成员数> <发消息用户数>
```

执行前必须确认频道配置（来自 `channel-group.env`）：

```bash
echo "$LT_CHANNEL_GROUP_CHANNEL_ID"
echo "$LT_CHANNEL_GROUP_CHANNEL_NAME"
```

示例与场景设计建议：

| 目标 | 命令示例 |
| --- | --- |
| 冒烟 | `lt_run_channel_group_gated 20 120 20 3` |
| 小规模 | `lt_run_channel_group_gated 100 300 100 5` |
| 中规模 | `lt_run_channel_group_gated 300 300 300 10` |
| 大群专项 | `lt_run_channel_group_gated 500 300 500 40` |

> 更真实的模型是少量或一定比例用户发消息、大量在线成员接收，而非全员发送。

## 7. 结果汇总（每轮结束后）

定位最新结果目录：

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

发消息接口汇总：

```bash
"$LT_ROOT/scripts/summarize_http_perf.sh" \
  "$RUN_DIR/http-access.log" 300 \
  "/api/v4/posts" "POST" |
tee "$RUN_DIR/post-summary.txt"
```

接收延迟汇总（登录场景跳过）：

```bash
scripts/summarize_receiver_latency.sh \
  "$RUN_DIR/receiver-latency.csv" |
tee "$RUN_DIR/receiver-latency-summary.txt"
```

错误 Top：

```bash
grep -Ei 'timeout|UserRequired|memstore|permission=create_post|could not find action|failed' "$RUN_DIR/console.log" |
sort | uniq -c | sort -nr | head -30
```

499 接口分布：

```bash
awk -F'|' '
  $4 == "499" { count[$2 " " $3]++ }
  END { for (k in count) print count[k], k }
' "$RUN_DIR/http-access.log" | sort -nr
```

## 8. 异常中断与进程清理

压测卡住或目标环境异常时，先在执行窗口按 `Ctrl + C`；仍有残留则新开 SSH 窗口执行：

```bash
pkill -f './bin/ltagent' 2>/dev/null || true
pkill -f 'collect_ltagent_metrics.sh' 2>/dev/null || true
pkill -f 'pidstat' 2>/dev/null || true
pkill -f 'sar -n DEV,TCP,ETCP' 2>/dev/null || true

pgrep -af 'ltagent|collect_ltagent_metrics|pidstat|sar -n DEV,TCP,ETCP' || echo "压测相关进程已清理"
```

> ⚠️ 不要直接 `pkill nginx`——机器上可能还有其他 Nginx 服务。
> 异常中断后不要立即删除结果目录，先保留 `console.log`、`http-access.log`、`ltagent-status.csv` 分析原因。

## 9. 新环境接入顺序

1. 登录 5～10 用户冒烟。
2. 单聊 2～10 用户冒烟。
3. GM 3 人小规模冒烟。
4. 频道大群 20 用户冒烟。
5. 正式阶梯压测。

需要替换的参数：`ServerURL`/`WebSocketURL`、`FORMAL_USERS`（用户文件）、`LT_USER_PREFIX`/`LT_USER_OFFSET`、团队与频道 ID、各场景规模参数。
