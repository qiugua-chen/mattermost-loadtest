
lt_wait_action_ready() {
  local run_dir="$1"
  local users="$2"
  local timeout_seconds="${3:-180}"
  local status_file="$run_dir/ltagent-status.csv"
  local deadline=$(( $(date +%s) + timeout_seconds ))

  while [ "$(date +%s)" -lt "$deadline" ]; do
    if [ -s "$status_file" ]; then
      local line actual ws errors
      line="$(tail -1 "$status_file")"

      actual="$(echo "$line" | awk -F',' '{print $3}')"
      errors="$(echo "$line" | awk -F',' '{print $5}')"
      ws="$(echo "$line" | awk -F',' '{print $8}')"

      if [ "${actual:-0}" -ge "$users" ] && [ "${ws:-0}" -ge "$users" ]; then
        return 0
      fi
    fi

    sleep 1
  done

  return 1
}

lt_run_monitored_scenario_gated() {
  local scenario="$1"
  local users="$2"
  local duration="$3"
  local base_config="$4"
  local controller_config="$5"
  local expected_receivers="$6"

  local run_time
  run_time="$(date +%Y%m%d_%H%M%S)"

  export CURRENT_SCENARIO="$scenario"
  export CURRENT_USERS="$users"
  export CURRENT_DURATION="$duration"
  export CURRENT_RUN_ID="${scenario}_${users}U_${run_time}"
  export CURRENT_RUN_DIR="$RESULTS_ROOT/$CURRENT_RUN_ID"
  export CURRENT_CONSOLE="$CURRENT_RUN_DIR/console.log"
  export CURRENT_ACCESS_LOG="$CURRENT_RUN_DIR/http-access.log"
  export CURRENT_NGINX_ERROR="$CURRENT_RUN_DIR/nginx-error.log"
  export CURRENT_EXIT_FILE="$CURRENT_RUN_DIR/exit_code.txt"
  export CURRENT_STATUS_FILE="$CURRENT_RUN_DIR/ltagent-status.csv"
  export CURRENT_PROXY_CONFIG="$CURRENT_RUN_DIR/config-proxy.json"

  mkdir -p "$CURRENT_RUN_DIR"
  export LT_RECEIVER_LATENCY_FILE="$CURRENT_RUN_DIR/receiver-latency.csv"

  jq --arg server_url "$PROXY_SERVER_URL" \
     --arg websocket_url "$PROXY_WEBSOCKET_URL" \
     '.ConnectionConfiguration.ServerURL = $server_url
      | .ConnectionConfiguration.WebSocketURL = $websocket_url
      | del(.ServerURL)
      | del(.WebSocketURL)' \
     "$base_config" > "$CURRENT_PROXY_CONFIG" || return 1

  if ! jq empty "$CURRENT_PROXY_CONFIG" >/dev/null 2>&1; then
    echo "ERROR：代理配置生成失败"
    return 1
  fi

  export LT_LOGIN_CONCURRENCY="${LT_LOGIN_CONCURRENCY:-20}"
  export LT_LOGIN_RETRIES="${LT_LOGIN_RETRIES:-3}"
  export LT_LOGIN_RETRY_SLEEP_MS="${LT_LOGIN_RETRY_SLEEP_MS:-1000}"
  export LT_ACTION_DURATION_SECONDS="$duration"
  export LT_ACTION_START_FILE="$CURRENT_RUN_DIR/action-start.gate"

  local warmup_timeout="${LT_WARMUP_TIMEOUT_SECONDS:-180}"
  local drain_seconds="${LT_DRAIN_SECONDS:-10}"
  local total_duration=$(( duration + warmup_timeout + drain_seconds ))

  lt_proxy_start || return 1
  sleep 2

  : > "$PERF_PROXY_ROOT/logs/access.log"
  : > "$PERF_PROXY_ROOT/logs/error.log"

  cd "$LT_ROOT" || return 1

  local command=(
    ./bin/ltagent
    -c "$CURRENT_PROXY_CONFIG"
    -n "$users"
    -d "$total_duration"
    --user-prefix "$LT_USER_PREFIX"
    --user-offset="$LT_USER_OFFSET"
  )

  if [ -n "$controller_config" ]; then
    command+=(--controller-config "$controller_config")
  fi

  echo "========== 本轮场景 =========="
  echo "场景=$scenario"
  echo "用户数=$users"
  echo "正式压测窗口=${duration}s"
  echo "登录限流=$LT_LOGIN_CONCURRENCY"
  echo "登录预热等待上限=${warmup_timeout}s"
  echo "运行目录=$CURRENT_RUN_DIR"
  echo "执行命令=${command[*]}"
  echo

  "${command[@]}" > >(tee "$CURRENT_CONSOLE") 2>&1 &
  export LT_PID=$!

  "$LT_ROOT/scripts/collect_ltagent_metrics.sh" "$CURRENT_RUN_DIR" "$users" "http://127.0.0.1:4001" \
    > "$CURRENT_RUN_DIR/metrics-collector.log" 2>&1 &
  local metrics_collector_pid=$!

  pidstat -h -u -r -d -p "$LT_PID" 1 > "$CURRENT_RUN_DIR/ltagent-pidstat.log" &
  local pidstat_pid=$!

  sar -n DEV,TCP,ETCP 1 > "$CURRENT_RUN_DIR/ltagent-network.log" &
  local sar_pid=$!

  echo "========== 等待登录与WebSocket就绪 =========="
  if ! lt_wait_action_ready "$CURRENT_RUN_DIR" "$users" "$warmup_timeout"; then
    echo "ERROR：预热超时，未达到 users=$users 且 ws_connections=$users"
    kill "$LT_PID" 2>/dev/null || true
    wait "$LT_PID" 2>/dev/null || true
    echo 124 > "$CURRENT_EXIT_FILE"
    kill "$metrics_collector_pid" "$pidstat_pid" "$sar_pid" 2>/dev/null || true
    wait "$metrics_collector_pid" "$pidstat_pid" "$sar_pid" 2>/dev/null || true
    lt_proxy_stop
    cp -f "$PERF_PROXY_ROOT/logs/access.log" "$CURRENT_ACCESS_LOG"
    cp -f "$PERF_PROXY_ROOT/logs/error.log" "$CURRENT_NGINX_ERROR"
    return 124
  fi

  local action_start_epoch
  action_start_epoch="$(date +%s)"
  if [[ "$scenario" == S03_GROUP* ]]; then
    echo "========== 群聊初始化等待 =========="
    sleep "${LT_GROUP_INIT_SECONDS:-15}"
fi

touch "$LT_ACTION_START_FILE"
  echo "$action_start_epoch" > "$CURRENT_RUN_DIR/action-start-epoch.txt"

  echo "========== 正式发消息窗口开始 =========="
  echo "ACTION_START_EPOCH=$action_start_epoch"
  echo "ACTION_DURATION_SECONDS=$duration"

  sleep $(( duration + drain_seconds ))

  kill "$LT_PID" 2>/dev/null || true
  wait "$LT_PID" 2>/dev/null || true
  local run_exit=$?

  echo "$run_exit" > "$CURRENT_EXIT_FILE"

  kill "$metrics_collector_pid" "$pidstat_pid" "$sar_pid" 2>/dev/null || true
  wait "$metrics_collector_pid" "$pidstat_pid" "$sar_pid" 2>/dev/null || true

  lt_proxy_stop

  cp -f "$PERF_PROXY_ROOT/logs/access.log" "$CURRENT_ACCESS_LOG"
  cp -f "$PERF_PROXY_ROOT/logs/error.log" "$CURRENT_NGINX_ERROR"

  "$LT_ROOT/scripts/summarize_http_perf.sh" "$CURRENT_ACCESS_LOG" "$duration" \
    > "$CURRENT_RUN_DIR/http-summary.txt" 2>&1 || true

  "$LT_ROOT/scripts/summarize_http_perf.sh" "$CURRENT_ACCESS_LOG" "$duration" "/api/v4/posts" "POST" \
    > "$CURRENT_RUN_DIR/post-summary.txt" 2>&1 || true

  if [ -x "$LT_ROOT/scripts/S02_direct_window_report.py" ]; then
    python3 "$LT_ROOT/scripts/S02_direct_window_report.py" \
      "$CURRENT_RUN_DIR" "$action_start_epoch" "$duration" "$users" "$drain_seconds" \
      > "$CURRENT_RUN_DIR/window-report.txt" 2>&1 || true
  elif [ -f "$LT_ROOT/scripts/S02_direct_window_report.py" ]; then
    python3 "$LT_ROOT/scripts/S02_direct_window_report.py" \
      "$CURRENT_RUN_DIR" "$action_start_epoch" "$duration" "$users" "$drain_seconds" \
      > "$CURRENT_RUN_DIR/window-report.txt" 2>&1 || true
  fi

  echo "========== 结果文件 =========="
  echo "运行目录=$CURRENT_RUN_DIR"
  echo "正式窗口开始=$(cat "$CURRENT_RUN_DIR/action-start-epoch.txt")"
  echo "HTTP汇总=$CURRENT_RUN_DIR/http-summary.txt"
  echo "发消息汇总=$CURRENT_RUN_DIR/post-summary.txt"
  echo "窗口报告=$CURRENT_RUN_DIR/window-report.txt"
  echo "控制台日志=$CURRENT_RUN_DIR/console.log"
}

lt_run_direct_gated() {
  local users="$1"
  local duration="$2"

  if [ -z "$users" ] || [ -z "$duration" ]; then
    echo "用法：lt_run_direct_gated <偶数用户数> <正式压测秒数>"
    return 2
  fi

  if [ $(( users % 2 )) -ne 0 ]; then
    echo "ERROR：Direct单聊用户数必须为偶数"
    return 2
  fi

  local receivers="${DIRECT_EXPECTED_RECEIVERS:-2}"

  lt_run_monitored_scenario_gated \
    "S02_DIRECT" \
    "$users" \
    "$duration" \
    "$DIRECT_CONFIG" \
    "$DIRECT_CONTROLLER_CONFIG" \
    "$receivers"
}

lt_run_group_gated() {
  local users="$1"
  local duration="$2"
  local receivers="${3:-$GROUP_EXPECTED_RECEIVERS}"

  if [ -z "$users" ] || [ -z "$duration" ]; then
    echo "用法：lt_run_group_gated <用户数> <正式压测秒数> [单消息预期接收事件数]"
    return 2
  fi

  if declare -f lt_run_group >/dev/null 2>&1 && [ -n "$GROUP_CONFIG" ] && [ -n "$GROUP_CONTROLLER_CONFIG" ]; then
    lt_run_monitored_scenario_gated \
      "S03_GROUP" \
      "$users" \
      "$duration" \
      "$GROUP_CONFIG" \
      "$GROUP_CONTROLLER_CONFIG" \
      "$receivers"
    return $?
  fi

  if [ -n "$MESSAGE_CONFIG" ] && [ -n "$MESSAGE_CONTROLLER_CONFIG" ]; then
    lt_run_monitored_scenario_gated \
      "S03_GROUP" \
      "$users" \
      "$duration" \
      "$MESSAGE_CONFIG" \
      "$MESSAGE_CONTROLLER_CONFIG" \
      "$receivers"
    return $?
  fi

  echo "ERROR：未找到S03群聊配置变量。请确认 GROUP_CONFIG/GROUP_CONTROLLER_CONFIG 或 MESSAGE_CONFIG/MESSAGE_CONTROLLER_CONFIG"
  return 2
}


lt_channel_group_prepare() {
  local channel_members="$1"

  : "${LT_MM_SERVER_URL:?需要设置 LT_MM_SERVER_URL}"
  : "${LT_CHANNEL_GROUP_TEAM_NAME:?需要设置 LT_CHANNEL_GROUP_TEAM_NAME}"
  : "${LT_MM_ADMIN_USERNAME:?需要设置 LT_MM_ADMIN_USERNAME}"
  : "${LT_MM_ADMIN_PASSWORD:?需要设置 LT_MM_ADMIN_PASSWORD}"

  local login_headers token team_json team_id channel_name channel_json channel_id

  login_headers=$(mktemp)
  curl -sS -D "$login_headers" -o /tmp/lt_channel_group_login.json \
    -H 'Content-Type: application/json' \
    -d "{\"login_id\":\"${LT_MM_ADMIN_USERNAME}\",\"password\":\"${LT_MM_ADMIN_PASSWORD}\"}" \
    "${LT_MM_SERVER_URL}/api/v4/users/login" >/dev/null

  token=$(awk 'BEGIN{IGNORECASE=1} /^Token:/ {print $2}' "$login_headers" | tr -d '\r')
  rm -f "$login_headers"

  if [ -z "$token" ]; then
    echo "ERROR：管理员登录失败，无法创建频道群测试频道"
    cat /tmp/lt_channel_group_login.json 2>/dev/null || true
    return 1
  fi

  team_json=$(curl -sS \
    -H "Authorization: Bearer $token" \
    "${LT_MM_SERVER_URL}/api/v4/teams/name/${LT_CHANNEL_GROUP_TEAM_NAME}")

  team_id=$(echo "$team_json" | jq -r '.id // empty')
  if [ -z "$team_id" ]; then
    echo "ERROR：获取 team_id 失败，请检查 LT_CHANNEL_GROUP_TEAM_NAME=${LT_CHANNEL_GROUP_TEAM_NAME}"
    echo "$team_json"
    return 1
  fi

  channel_name="perf-channel-group-${channel_members}-$(date +%Y%m%d%H%M%S)"

  channel_json=$(curl -sS \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -d "{\"team_id\":\"${team_id}\",\"name\":\"${channel_name}\",\"display_name\":\"${channel_name}\",\"type\":\"O\"}" \
    "${LT_MM_SERVER_URL}/api/v4/channels")

  channel_id=$(echo "$channel_json" | jq -r '.id // empty')
  if [ -z "$channel_id" ]; then
    echo "ERROR：创建频道失败"
    echo "$channel_json"
    return 1
  fi

  export LT_CHANNEL_GROUP_CHANNEL_ID="$channel_id"
  export LT_CHANNEL_GROUP_CHANNEL_NAME="$channel_name"

  echo "频道群初始化完成：channel_name=${channel_name} channel_id=${channel_id}"
}

lt_run_channel_group_gated() {
  local users="$1"
  local duration="$2"
  local channel_members="$3"
  local senders="$4"

  if [ -z "$users" ] || [ -z "$duration" ] || [ -z "$channel_members" ] || [ -z "$senders" ]; then
    echo "用法：lt_run_channel_group_gated <在线用户数> <正式压测秒数> <频道人数> <发消息用户数>"
    echo "示例：lt_run_channel_group_gated 100 300 100 5"
    return 1
  fi

  if [ "$users" -lt "$channel_members" ]; then
    echo "ERROR：在线用户数不能小于频道人数。大群接收压测需要所有频道成员在线。"
    return 1
  fi

  if [ "$senders" -gt "$channel_members" ]; then
    echo "ERROR：发消息用户数不能大于频道人数。"
    return 1
  fi

  source $LT_ROOT/runtime/env/channel-group.env

  export GROUP_CONFIG="$CHANNEL_GROUP_CONFIG"
  export GROUP_CONTROLLER_CONFIG="$CHANNEL_GROUP_CONTROLLER_CONFIG"
  export GROUP_EXPECTED_RECEIVERS="$channel_members"
  export LT_CHANNEL_GROUP_MEMBERS="$channel_members"
  export LT_CHANNEL_GROUP_SENDERS="$senders"

  if [ "${LT_CHANNEL_GROUP_USE_EXISTING_CHANNEL:-0}" = "1" ]; then
    if [ -z "${LT_CHANNEL_GROUP_CHANNEL_ID:-}" ]; then
      echo "ERROR：已启用已有频道模式，但 LT_CHANNEL_GROUP_CHANNEL_ID 为空"
      return 1
    fi
    export LT_CHANNEL_GROUP_CHANNEL_NAME="${LT_CHANNEL_GROUP_CHANNEL_NAME:-existing-channel}"
    echo "使用已有频道：channel_id=${LT_CHANNEL_GROUP_CHANNEL_ID}"
  else
    lt_channel_group_prepare "$channel_members" || return 1
  fi

  echo "========== S03B 频道群场景 =========="
  echo "在线用户数=${users}"
  echo "频道人数=${channel_members}"
  echo "发消息用户数=${senders}"
  echo "正式压测窗口=${duration}s"
  echo "频道=${LT_CHANNEL_GROUP_CHANNEL_NAME}"
  echo "频道ID=${LT_CHANNEL_GROUP_CHANNEL_ID}"

  lt_run_group_gated "$users" "$duration" "$channel_members"
}
