# 贡献指南（CONTRIBUTING）

感谢关注本项目！这是一个由测试实践驱动的二次开发仓库，欢迎提交 Issue 与 PR。
为了保持仓库风格一致、可复现，请遵守以下约定。

## 1. 提交 Issue

- 压测执行类问题请使用「压测问题上报」模板，务必附：场景编号、并发规模、
  被测版本、关键日志片段。
- 提交前请先检索 [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)，
  常见故障模式已有排查路径。

## 2. 代码约定

- **注释语言**：新增/修改的 Go 方法必须带中文方法级注释（说明做什么、关键参数、
  失败行为），供不走 Go 的测试同学走查。
- **环境变量**：本扩展新增的开关统一使用 `LT_` 前缀（如 `LT_SKIP_SIGNUP`、
  `LT_GROUP_SIZE`），并在 [docs/EXT-CHANGES.md](docs/EXT-CHANGES.md) 登记。
- **场景编号**：沿用 `S00 注册准备 / S01 登录 / S02 单聊 / S03A GM 群聊 /
  S03B 频道大群` 的编号体系，新场景顺延并在 [docs/SCENARIOS.md](docs/SCENARIOS.md)
  的对照表中登记。
- **改代码必重编**：任何 Go 改动后必须重新执行
  `go build -o bin/ltagent ./cmd/ltagent`，并通过 `go vet ./loadtest/... ./cmd/...`。

## 3. 配置与文档约定

- JSON 配置不支持注释，参数说明统一维护在 [config/README.md](config/README.md)。
- `config/*.json` 为本地真实配置（已被 .gitignore 忽略）；入库的只能是
  `*.sample.json` 与 `config/formal/` 下的基线配置。
- **严禁提交**真实 IP、域名、账号、密码、token；新文档请用
  `<MM_HOST>`、`example.com` 等占位符。

## 4. 提交流程

1. Fork 本仓库，自建分支开发。
2. commit message 采用 `type(scope): 中文描述`，type 取
   `feat / fix / docs / chore / perf`。
3. PR 描述中注明：涉及场景编号、验证方式（哪一轮压测/冒烟跑过）。
