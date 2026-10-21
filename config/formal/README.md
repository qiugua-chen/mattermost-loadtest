# 正式场景配置说明（1200 用户级）

本目录为正式压测场景的参数基线（来源于一次完整的 1200 用户 IM 性能测试实践，路径参数已改为相对路径）。每个场景由「主配置 + 控制器配置」两个文件成对组成。

> **单一事实来源**：五份 `config-*-1200.json` 主配置仅 `UserControllerConfiguration.Type` 一处不同
> （登录=`noop`，其余=`simple`），由模板 `config-1200.tpl.json` 经
> `bash scripts/render_formal_configs.sh` 渲染生成。
> 调整公共参数请改模板后重新渲染；CI 会校验渲染产物与仓库内文件一致（防漂移）。

## 场景对照表

| 场景 | 主配置 | 控制器配置 | 说明 |
|---|---|---|---|
| S01 登录/在线 | `config-login-1200.json` | `simplecontroller-login.sample.json`（上级目录，空动作） | 1200 用户登录并保持在线，验证登录吞吐与 WS 连接容量 |
| S02 单聊 | `config-direct-1200.json` | `simplecontroller-direct.json` | 按用户名数字后缀确定性两两配对发 DM，接收端经 WS 测量端到端延迟 |
| S03 群聊 | `config-group-1200.json` | `simplecontroller-group.json` | `LT_GROUP_SIZE` 确定性分组群聊（默认 4 人/组） |
| S03B 频道群发 | `config-channel-group-1200.json` | `simplecontroller-channel-group.json` | 单频道 N 发 M 收，验证广播放大；由开始信号文件门控齐发 |
| 消息通用 | `config-message-1200.json` | `simplecontroller-message.json` | 通用 CreatePost 消息压测基线 |

## 与冒烟配置的关键差异

| 维度 | 冒烟（`config-smoke.sample.json`） | 正式（本目录） |
|---|---|---|
| 用户数 | 个位数（UsersFilePath 留空或几个账号） | 1200（`UsersFilePath` 指向预生成的 `users-1200.txt`） |
| ServerURL | 直连被测服务器 | `http://127.0.0.1:18065`（经 nginx 代理采集链路数据） |
| 登录治理 | 默认 | 需配合 `LT_LOGIN_CONCURRENCY` 限流（建议 20~50） |
| 监控 | 可选 | 必须（source `monitoring/env/*.env`） |

## 使用方式

```bash
# 1. 生成 1200 个预置账号并分配到团队/频道（详见 ../README.md 第四节）
# 2. 准备环境
cp -r monitoring/env /opt/mattermost-loadtest/runtime/env   # 或你的工作布局
cd /opt/mattermost-loadtest && source runtime/env/loadtest.env
# 3. 执行（例：S02 单聊）
lt_run_direct_gated 1200 600    # 用户数 1200，持续 600 秒
```

各 env 变量的中文说明见 `monitoring/env/*.env.sample` 文件头注释。
