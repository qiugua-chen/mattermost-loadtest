#!/usr/bin/env bash

run_message_main() {
  local message_users="${1:-}"
  local message_duration="${2:-}"

  source "${LT_ROOT:-/opt/mattermost-load-test-ng}/runtime/env/loadtest.env"
  source "${LT_ROOT:-/opt/mattermost-load-test-ng}/runtime/env/monitoring.env"
  source "${LT_ROOT:-/opt/mattermost-load-test-ng}/runtime/env/target.env"

  cd "$LT_ROOT" || return 1

  if ! [[ "$message_users" =~ ^[0-9]+$ ]] ||
     [ "$message_users" -le 0 ]
  then
    echo "ERROR：用户数必须为正整数"
    echo "示例：lt_run_message 10 300"
    return 1
  fi

  if ! [[ "$message_duration" =~ ^[0-9]+$ ]] ||
     [ "$message_duration" -le 0 ]
  then
    echo "ERROR：持续时间必须为正整数"
    echo "示例：lt_run_message 10 300"
    return 1
  fi

  echo "========== 场景二执行前检查 =========="

  local run_ready=PASS

  if [ ! -r "$MESSAGE_CONFIG" ]
  then
    echo "消息主配置=FAIL：$MESSAGE_CONFIG"
    run_ready=FAIL
  else
    echo "消息主配置=PASS"
  fi

  if [ ! -r "$MESSAGE_CONTROLLER_CONFIG" ]
  then
    echo "消息Controller=FAIL：$MESSAGE_CONTROLLER_CONFIG"
    run_ready=FAIL
  else
    echo "消息Controller=PASS"
  fi

  if [ ! -r "$FORMAL_USERS" ]
  then
    echo "用户文件=FAIL：$FORMAL_USERS"
    run_ready=FAIL
  else
    echo "用户文件=PASS"
  fi

  if [ ! -x "$LT_ROOT/bin/ltagent" ]
  then
    echo "ltagent=FAIL"
    run_ready=FAIL
  else
    echo "ltagent=PASS"
  fi

  if [ "${LT_SKIP_SIGNUP:-}" != "true" ]
  then
    echo "跳过注册开关=FAIL"
    run_ready=FAIL
  else
    echo "跳过注册开关=PASS"
  fi

  if [ -r "$MESSAGE_CONTROLLER_CONFIG" ]
  then
    if jq -e '
      (.Actions | type) == "array"
      and
      (.Actions | length) == 1
      and
      .Actions[0].ActionId == "CreatePost"
      and
      .Actions[0].RunPeriod == 1
      and
      .Actions[0].WaitAfterMs == 1000
    ' "$MESSAGE_CONTROLLER_CONFIG" >/dev/null
    then
      echo "CreatePost配置=PASS"
    else
      echo "CreatePost配置=FAIL"
      run_ready=FAIL
    fi
  fi

  if pgrep -x ltagent >/dev/null
  then
    echo "旧ltagent进程=FAIL"
    pgrep -a -x ltagent
    run_ready=FAIL
  else
    echo "旧ltagent进程=PASS"
  fi

  local proxy_http

  proxy_http=$(
    curl -sS \
      --connect-timeout 5 \
      --max-time 10 \
      -o /tmp/message-run-proxy.json \
      -w '%{http_code}' \
      "$PROXY_SERVER_URL/api/v4/system/ping" \
      2>/tmp/message-run-proxy.err
  )

  if [ "$proxy_http" = "200" ]
  then
    echo "Nginx代理=PASS"
  else
    echo "Nginx代理=FAIL，HTTP=$proxy_http"
    cat /tmp/message-run-proxy.err 2>/dev/null
    run_ready=FAIL
  fi

  echo "场景二执行前检查=$run_ready"

  if [ "$run_ready" != "PASS" ]
  then
    echo "本次未启动压测"
    return 1
  fi

  local run_id
  local run_dir
  local run_config
  local run_controller
  local current_console
  local status_jsonl
  local metrics_snapshots
  local collector_log
  local run_log_file
  local nginx_lines_before
  local nginx_lines_after
  local nginx_new_lines
  local collector_pid
  local ltagent_rc
  local start_epoch
  local end_epoch
  local start_time
  local end_time

  run_id=$(
    date "+S02_MESSAGE_${message_users}U_%Y%m%d_%H%M%S"
  )

  run_dir="$RESULTS_ROOT/$run_id"
  run_config="$run_dir/config/config-message.json"
  run_controller="$run_dir/config/simplecontroller-message.json"
  current_console="$run_dir/logs/console.log"
  status_jsonl="$run_dir/monitor/ltagent-status.jsonl"
  metrics_snapshots="$run_dir/monitor/ltagent-metrics.prom"
  collector_log="$run_dir/logs/metrics-collector.log"
  run_log_file="$run_dir/logs/ltagent-message.jsonl"

  mkdir -p \
    "$run_dir/config" \
    "$run_dir/logs" \
    "$run_dir/monitor"

  jq \
    --arg log_file "$run_log_file" '
      .LogSettings.FileLocation = $log_file
    ' \
    "$MESSAGE_CONFIG" \
    > "$run_config"

  local config_rc=$?

  if [ "$config_rc" -ne 0 ] ||
     ! jq empty "$run_config" 2>/dev/null
  then
    echo "ERROR：本轮消息配置生成失败"
    return 1
  fi

  cp -p \
    "$MESSAGE_CONTROLLER_CONFIG" \
    "$run_controller"

  nginx_lines_before=$(
    wc -l < "$NGINX_ACCESS_LOG"
  )

  start_epoch=$(date '+%s')
  start_time=$(date '+%Y-%m-%dT%H:%M:%S%z')

  : > "$status_jsonl"
  : > "$metrics_snapshots"
  : > "$collector_log"

  echo "========== 场景二开始 =========="
  echo "结果目录=$run_dir"
  echo "用户数=$message_users"
  echo "持续时间=$message_duration"
  echo "开始时间=$start_time"

  (
    while true
    do
      local_sample_epoch=$(date '+%s')

      curl -sS \
        --connect-timeout 1 \
        --max-time 2 \
        "$LT_STATUS_URL" \
        2>/dev/null |
      jq -c \
        --argjson sample_epoch "$local_sample_epoch" '
          . + {
            sample_epoch: $sample_epoch
          }
        ' \
        >> "$status_jsonl" \
        2>>"$collector_log"

      {
        printf '# SNAPSHOT %s\n' "$local_sample_epoch"

        curl -sS \
          --connect-timeout 1 \
          --max-time 2 \
          "$LT_METRICS_URL" \
          2>/dev/null
      } >> "$metrics_snapshots"

      sleep 1
    done
  ) &

  collector_pid=$!

  stop_collector() {
    if [ -n "${collector_pid:-}" ] &&
       kill -0 "$collector_pid" 2>/dev/null
    then
      kill -TERM "$collector_pid" 2>/dev/null
      wait "$collector_pid" 2>/dev/null
    fi
  }

  trap stop_collector EXIT

  LT_SKIP_SIGNUP=true \
  "$LT_ROOT/bin/ltagent" \
    -c "$run_config" \
    --controller-config "$run_controller" \
    -n "$message_users" \
    -d "$message_duration" \
    --user-prefix "$LT_USER_PREFIX" \
    --user-offset="$LT_USER_OFFSET" \
    2>&1 |
  tee "$current_console"

  ltagent_rc=${PIPESTATUS[0]}

  stop_collector
  trap - EXIT

  end_epoch=$(date '+%s')
  end_time=$(date '+%Y-%m-%dT%H:%M:%S%z')

  printf '%s\n' "$ltagent_rc" \
    > "$run_dir/exit_code.txt"

  {
    printf 'MESSAGE_USERS=%q\n' "$message_users"
    printf 'MESSAGE_DURATION=%q\n' "$message_duration"
    printf 'RUN_START_EPOCH=%q\n' "$start_epoch"
    printf 'RUN_END_EPOCH=%q\n' "$end_epoch"
    printf 'RUN_START_TIME=%q\n' "$start_time"
    printf 'RUN_END_TIME=%q\n' "$end_time"
    printf 'LTAGENT_RC=%q\n' "$ltagent_rc"
  } > "$run_dir/run-meta.env"

  nginx_lines_after=$(
    wc -l < "$NGINX_ACCESS_LOG"
  )

  nginx_new_lines=$(
    (
      nginx_lines_after - nginx_lines_before
    )
  )

  if [ "$nginx_new_lines" -gt 0 ]
  then
    tail -n "$nginx_new_lines" \
      "$NGINX_ACCESS_LOG" \
      > "$run_dir/http-access.log"
  else
    : > "$run_dir/http-access.log"
  fi

  if [ -x "$LT_ROOT/scripts/summarize_http_perf.sh" ] &&
     [ -s "$run_dir/http-access.log" ]
  then
    "$LT_ROOT/scripts/summarize_http_perf.sh" \
      "$run_dir/http-access.log" \
      "$message_duration" \
      > "$run_dir/http-summary.txt"

    "$LT_ROOT/scripts/summarize_http_perf.sh" \
      "$run_dir/http-access.log" \
      "$message_duration" \
      "/api/v4/posts" \
      "POST" \
      > "$run_dir/message-http-summary.txt"
  fi

  {
    printf 'export CURRENT_RUN_DIR=%q\n' "$run_dir"
    printf 'export CURRENT_MESSAGE_RUN_DIR=%q\n' "$run_dir"
  } > "$LT_ROOT/runtime/env/current-message-run.env"

  echo "========== 场景二结束 =========="
  echo "结束时间=$end_time"
  echo "ltagent返回码=$ltagent_rc"
  echo "Nginx新增日志数=$nginx_new_lines"
  echo "状态采集文件=$status_jsonl"
  echo "指标采集文件=$metrics_snapshots"
  echo "结果目录=$run_dir"

  return "$ltagent_rc"
}

run_message_main "$@"
