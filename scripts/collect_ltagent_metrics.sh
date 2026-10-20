#!/usr/bin/env bash
# =============================================================================
# collect_ltagent_metrics.sh —— ltagent 内嵌指标服务的周期采集器
#
# 用法: collect_ltagent_metrics.sh <输出目录> <目标用户数> [指标地址=http://127.0.0.1:4001]
#
# 每秒抓取一次 ltagent 的 /status 与 /metrics（Prometheus 文本格式），
# 汇总为时序 CSV（ltagent-status.csv），列包括：
#   epoch, 目标/实际用户数, 压测错误数, HTTP 请求/错误/超时数,
#   WS 连接数, WS posted 事件数, WS 丢事件数
# 该 CSV 是就绪门控（lt_wait_action_ready）与事后核对的数据源。
# 多 agent 同机部署时，按 LT_METRICS_ADDR 端口逐实例各起一个本采集器。
# =============================================================================

set -u

OUT_DIR="${1:?缺少输出目录}"
TARGET_USERS="${2:?缺少目标用户数}"
METRICS_URL="${3:-http://127.0.0.1:4001}"

mkdir -p "$OUT_DIR"

STATUS_FILE="$OUT_DIR/ltagent-status.csv"
TMP_METRICS=$(mktemp /tmp/ltagent-metrics.XXXXXX)

trap 'rm -f "$TMP_METRICS"' EXIT

echo 'epoch,target_users,actual_users,loadtest_errors,http_requests,http_errors,http_timeouts,ws_connections,ws_posted_events,ws_missed_events' \
  > "$STATUS_FILE"

metric_sum() {
    local metric_name="$1"

    awk -v target="$metric_name" '
        $1 !~ /^#/ {
            name=$1
            sub(/\{.*/, "", name)

            if (name == target) {
                sum += $2
            }
        }

        END {
            print sum + 0
        }
    ' "$TMP_METRICS"
}

while true; do
    NOW=$(date +%s)

    STATUS_JSON=$(
      curl -fsS \
        --max-time 1 \
        "$METRICS_URL/status" 2>/dev/null
    )

    if [ -z "$STATUS_JSON" ]; then
        sleep 1
        continue
    fi

    if ! curl -fsS \
      --max-time 1 \
      "$METRICS_URL/metrics" \
      -o "$TMP_METRICS"; then
        sleep 1
        continue
    fi

    ACTUAL_USERS=$(echo "$STATUS_JSON" | jq -r '.NumUsers // 0')
    LOADTEST_ERRORS=$(echo "$STATUS_JSON" | jq -r '.NumErrors // 0')

    HTTP_REQUESTS=$(metric_sum loadtest_http_request_time_count)
    HTTP_ERRORS=$(metric_sum loadtest_http_errors_total)
    HTTP_TIMEOUTS=$(metric_sum loadtest_http_timeouts_total)
    WS_CONNECTIONS=$(metric_sum loadtest_websocket_connections_total)

    WS_POSTED_EVENTS=$(
      awk '
        /^loadtest_websocket_events_received_total\{event="posted"\}/ {
            sum += $2
        }

        END {
            print sum + 0
        }
      ' "$TMP_METRICS"
    )

    WS_MISSED_EVENTS=$(metric_sum loadtest_websocket_missed_events_total)

    echo "$NOW,$TARGET_USERS,$ACTUAL_USERS,$LOADTEST_ERRORS,$HTTP_REQUESTS,$HTTP_ERRORS,$HTTP_TIMEOUTS,$WS_CONNECTIONS,$WS_POSTED_EVENTS,$WS_MISSED_EVENTS" \
      >> "$STATUS_FILE"

    sleep 1
done
