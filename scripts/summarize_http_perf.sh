#!/usr/bin/env bash
# summarize_http_perf.sh —— summarize_http_perf.py 的便捷包装：
# 自动定位同目录下的 Python 脚本并透传全部参数。
# 用法: summarize_http_perf.sh <access.log> <窗口秒数> [URI过滤] [方法过滤]

SCRIPT_DIR=$(
  cd "$(dirname "${BASH_SOURCE[0]}")" &&
  pwd
)

python3 \
  "$SCRIPT_DIR/summarize_http_perf.py" \
  "$@"
