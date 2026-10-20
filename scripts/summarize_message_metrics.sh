#!/usr/bin/env bash

set -euo pipefail

ACCESS_LOG="${1:?缺少Nginx访问日志}"
STATUS_FILE="${2:?缺少ltagent-status.csv}"
TEST_DURATION="${3:?缺少测试持续时间}"
EXPECTED_RECEIVERS="${4:?缺少每条消息预期接收人数}"

if [ ! -s "$ACCESS_LOG" ]; then
    echo "ERROR：访问日志不存在或为空"
    exit 1
fi

if [ ! -s "$STATUS_FILE" ]; then
    echo "ERROR：ltagent指标文件不存在或为空"
    exit 1
fi

POST_ATTEMPTS=$(
  awk -F'|' '
    $2 == "POST" && $3 == "/api/v4/posts" {
        count++
    }

    END {
        print count + 0
    }
  ' "$ACCESS_LOG"
)

POST_SUCCESS=$(
  awk -F'|' '
    $2 == "POST" &&
    $3 == "/api/v4/posts" &&
    $4 >= 200 &&
    $4 < 400 {
        count++
    }

    END {
        print count + 0
    }
  ' "$ACCESS_LOG"
)

POST_FAILED=$((POST_ATTEMPTS - POST_SUCCESS))

MAX_WS=$(
  awk -F',' '
    NR > 1 && $8 > max {
        max=$8
    }

    END {
        print max + 0
    }
  ' "$STATUS_FILE"
)

ACTUAL_RECEIVED=$(
  awk -F',' '
    NR > 1 && $9 > max {
        max=$9
    }

    END {
        print max + 0
    }
  ' "$STATUS_FILE"
)

MISSED_EVENTS=$(
  awk -F',' '
    NR > 1 && $10 > max {
        max=$10
    }

    END {
        print max + 0
    }
  ' "$STATUS_FILE"
)

DELIVERY_PEAK_TPS=$(
  awk -F',' '
    NR == 2 {
        previous_time=$1
        previous_count=$9
        next
    }

    NR > 2 {
        elapsed=$1-previous_time
        increment=$9-previous_count

        if (elapsed > 0 && increment >= 0) {
            rate=increment/elapsed

            if (rate > max_rate) {
                max_rate=rate
            }
        }

        previous_time=$1
        previous_count=$9
    }

    END {
        printf "%.2f", max_rate + 0
    }
  ' "$STATUS_FILE"
)

EXPECTED_RECEIVED=$((POST_SUCCESS * EXPECTED_RECEIVERS))

awk \
  -v attempts="$POST_ATTEMPTS" \
  -v success="$POST_SUCCESS" \
  -v failed="$POST_FAILED" \
  -v duration="$TEST_DURATION" \
  -v receivers="$EXPECTED_RECEIVERS" \
  -v expected="$EXPECTED_RECEIVED" \
  -v actual="$ACTUAL_RECEIVED" \
  -v max_ws="$MAX_WS" \
  -v missed="$MISSED_EVENTS" \
  -v peak_delivery="$DELIVERY_PEAK_TPS" '
  BEGIN {
      print "========== 消息发送与投递 =========="
      print "发送尝试数=" attempts
      print "发送成功数=" success
      print "发送失败数=" failed

      if (attempts > 0) {
          printf "发送成功率=%.4f%%\n", success * 100 / attempts
      }

      printf "发送尝试平均TPS=%.4f\n", attempts / duration
      printf "发送成功平均TPS=%.4f\n", success / duration
      print "最大WebSocket连接数=" max_ws
      print "每条消息预期接收人数=" receivers
      print "预期接收事件数=" expected
      print "实际接收posted事件数=" actual
      printf "消息投递事件平均TPS=%.4f\n", actual / duration
      printf "消息投递事件峰值TPS=%.2f\n", peak_delivery
      print "WebSocket漏序事件数=" missed

      if (expected > 0) {
          rate=actual * 100 / expected
          printf "消息到达率=%.4f%%\n", rate

          if (rate > 100) {
              print "核账状态=INVALID"
              print "原因=存在非压测消息、重复事件或预期接收人数设置错误"
          } else {
              print "核账状态=VALID"
          }
      } else {
          print "消息到达率=无法计算"
          print "核账状态=INVALID"
      }
  }
'
