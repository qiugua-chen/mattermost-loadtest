# 核心场景执行手册

> 覆盖场景：S01 登录、S02 单聊（Direct）、S03A GM 群组消息、S03B 频道大群。
> 本文侧重"怎么跑"，源码改造细节见 [EXT-CHANGES.md](EXT-CHANGES.md)，指标解释见 [METRICS.md](METRICS.md)。

## 1. 场景模型总览

| 场景编号 | 场景名称 | 执行函数 | 核心验证目标 |
| --- | --- | --- | --- |
| S00 | 用户注册与准备 | `mattermost_prepare_users.py` / `mattermost_assign_users.py` | 建号→入团→入频道，产出可用账号文件（登录的前置条件） |
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

## 3. S00 用户注册与准备（前置场景）

注册是登录不可或缺的前置条件。本扩展版默认工作模式为"**预置账号 + `LT_SKIP_SIGNUP=true`**"：
压测时不再走注册接口，直接使用已建好的账号登录。因此正式压测前必须先完成
**建号 → 入团 → 入频道** 的用户工程闭环。

### 3.1 路径 A：批量建号（推荐）

`scripts/mattermost_prepare_users.py` 通过管理员 API 批量创建确定性账号，并直接产出
ltagent 可用的凭据文件：

```bash
export MM_TOKEN=<admin_token>                 # 管理员 token（也可改用 --token-env 指定变量名）
export IMLT_USER_PASSWORD='<统一密码>'          # 所有账号共用的密码，长度 ≥10
python3 scripts/mattermost_prepare_users.py \
  --server http://<MM_HOST>:8065 \
  --count 1200 --prefix imlt \
  --output ./config/users/imlt-users.txt \
  --result-log ./results/prepare_users.jsonl
chmod 600 ./config/users/imlt-users.txt       # 凭据文件务必收紧权限
```

常用参数：`--start-index`（起始编号）、`--email-domain`、`--workers`（建号并发，默认 5）、
`--request-delay`、`--retries`。脚本幂等可重跑，密码只存于环境变量、不写入结果日志。

> 凭据文件格式为每行 `用户名 密码`（**纯数据，不支持 `#` 注释行**）。
> 用户名形如 `imlt00001`——**末 5 位数字是单聊配对与群聊分组的依据，命名规则不可随意更改**。

### 3.2 路径 B：入团入频道（幂等）

`scripts/mattermost_assign_users.py` 将账号文件中的用户批量加入指定团队和频道
（已存在的成员关系会校验并保留，可安全重复执行）：

```bash
export MM_TOKEN=<admin_token>
python3 scripts/mattermost_assign_users.py \
  --server http://<MM_HOST>:8065 \
  --users-file ./config/users/imlt-users.txt \
  --team-id <TEAM_ID> --channel-id <CHANNEL_ID> \
  --result-log ./results/assign_users.jsonl
```

| 场景 | 是否必须执行入团入频道 |
| --- | --- |
| S01 登录 / S02 单聊 / S03A GM | 不必须（扩展 action 已对无团队/无频道做容错降级），但建议执行以保证数据完整 |
| **S03B 频道大群** | **必须**——全部压测用户必须在目标频道内，否则接收侧无推送压力 |

### 3.3 路径 C：ltagent 原生注册（可选）

不设置 `LT_SKIP_SIGNUP` 时，simplecontroller 的 `SignUp` 动作会走 `/api/v4/users` 注册接口，
可用登录配置跑一轮"注册造数"（也可借此压测注册接口本身）：

```bash
unset LT_SKIP_SIGNUP
./bin/ltagent -c config/formal/config-login-1200.json \
  -n 1200 -d 300 --user-prefix imlt --user-offset=-1 \
  --controller-config config/simplecontroller-login.sample.json
```

> 注意：原生注册使用工具内置生成的账号与固定密码，可控性弱于路径 A；
> 本扩展的确定性配对/分组依赖路径 A 产出的用户名规则，正式场景请优先走路径 A。

### 3.4 就绪验证

```bash
# 1. 账号文件行数 ≥ 计划压测用户数
wc -l ./config/users/imlt-users.txt

# 2. 抽查用户已在服务端注册（可选）
TOKEN=$(curl -sS -i -H 'Content-Type: application/json' \
  -d '{"login_id":"<管理员>","password":"<密码>"}' \
  http://<MM_HOST>:8065/api/v4/users/login | awk 'BEGIN{IGNORECASE=1}/^Token:/{print $2}' | tr -d '\r')
curl -sS -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"term":"imlt","allow_inactive":true}' \
  http://<MM_HOST>:8065/api/v4/users/search | jq 'length'
```

## 4. S01 登录

执行：

```bash
lt_run_login 40 300
```

| 参数 | 含义 |
| --- | --- |
| 第 1 个参数 | 登录并发用户数 |
| 第 2 个参数 | 持续时间（秒） |

重点观察：`/api/v4/users/login` 成功率、登录 P95/P99、401、timeout、`actual_users`。

## 5. S02 单聊

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

## 6. S03A GM 群组消息

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

## 7. S03B 频道大群

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

## 7.5 S04 混合稳定性（设计参考，本轮未实测）

来源于 POC 方案设计的备选场景：**单聊 70% + 群聊 30% 混合负载，目标档位持续 60 分钟**，每 10 分钟统计一次 TPS、P95/P99 与消息核账结果，用于观察长时间运行下的内存泄漏、连接漂移与延迟劣化。

- **执行条件**：S01/S02/S03 全部通过、且工具与监控运行稳定后再执行；
- **当前实现方式**：可用两台压测机（或同一台两个 ltagent 实例，账号段不重叠）分别跑 S02 与 S03A，按 7:3 分配发送用户，统一启动时间；
- **缺口**：单实例内的混合动作权重调度需要控制器层支持，属于未来方向（见第 11 节）。

## 8. 结果汇总（每轮结束后）

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

## 9. 异常中断与进程清理

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

## 10. 新环境接入顺序

0. **注册并准备用户**（见第 3 节 S00：建号 → 入团 → 入频道）。
1. 登录 5～10 用户冒烟。
2. 单聊 2～10 用户冒烟。
3. GM 3 人小规模冒烟。
4. 频道大群 20 用户冒烟。
5. 正式阶梯压测。

需要替换的参数：`ServerURL`/`WebSocketURL`、`FORMAL_USERS`（用户文件）、`LT_USER_PREFIX`/`LT_USER_OFFSET`、团队与频道 ID、各场景规模参数。

## 11. 未来方向（尚未覆盖的场景）

| 方向 | 业务含义 | 现状 |
|---|---|---|
| 离线消息补偿 | 用户断线期间产生的消息，重连后通过消息拉取接口补齐；验收口径为补偿率 100%、重连后 60 秒内补齐 | 方案层已定义（见 [METHODOLOGY.md §2](METHODOLOGY.md)），工具链未实现，需要控制器增加「断连-等待-重连-拉取核对」动作 |
| 单实例混合负载（S04 完全体） | 单个 ltagent 实例内按权重混合单聊/群聊动作 | 目前只能多实例拼装（见 7.5 节），需要控制器层动作权重调度 |
| 分布式协调端压测 | ltcoordinator 统一调度多负载机冲击大规模在线（4000+） | 上游 ltapi/ltagent 已具备，本扩展未实测，接入顺序见第 10 节 |
