#!/usr/bin/env bash
# render_formal_configs.sh —— 由单一模板渲染五份正式场景主配置（幂等）。
#
# 背景：config/formal/ 下五份 1200 用户级主配置仅 UserControllerConfiguration.Type
# 一处不同（登录场景=noop，其余消息场景=simple），其余字段完全一致。
# 因此收敛为 config-1200.tpl.json 单一模板 + 本脚本渲染，避免五份文件漂移。
#
# 用法：
#   bash scripts/render_formal_configs.sh        # 原地重渲染（覆盖同名文件，内容一致则无 diff）
#
# 依赖：envsubst（gettext 包，常规 Linux 发行版自带）。
set -euo pipefail

FORMAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../config/formal" && pwd)"
TPL="$FORMAL_DIR/config-1200.tpl.json"

command -v envsubst >/dev/null || { echo "缺少 envsubst（请安装 gettext）" >&2; exit 1; }
[ -f "$TPL" ] || { echo "模板不存在：$TPL" >&2; exit 1; }

render() { # $1=控制器类型 $2=输出文件名
  LT_CONTROLLER_TYPE="$1" envsubst '${LT_CONTROLLER_TYPE}' < "$TPL" > "$FORMAL_DIR/$2"
  echo "rendered $2 (Type=$1)"
}

render noop   config-login-1200.json          # S01 登录/在线
render simple config-direct-1200.json         # S02 单聊
render simple config-group-1200.json          # S03A GM 群聊
render simple config-channel-group-1200.json  # S03B 频道大群
render simple config-message-1200.json        # 消息通用基线
