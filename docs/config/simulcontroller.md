# SimulController 配置

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/config/simulcontroller.md

## MinIdleTimeMs

*int*

受控用户在两次动作之间的最短等待时间（毫秒）。

## AvgIdleTimeMs

*int*

受控用户在两次动作之间的平均等待时间（毫秒）。

## EnabledPlugins

*[]string*

Mattermost 插件 manifest ID 列表。控制器启动时会为列表中的插件注册其自带的仿真动作。已启用插件的动作会出现在日志、coordinator 指标和速率注入中，命名格式为 `<pluginId>.<ActionName>`（见下文 `mattermost-ai`）。

在这里配置插件 ID 只影响**压测仿真器运行什么动作**——Mattermost 服务端仍需实际安装并启用该插件（见 terraform / deployer 文档中的 [`MattermostPlugins`](deployer.md#mattermostplugins)）。

### `mattermost-ai`（Agents）

列表中包含 `mattermost-ai` 时，load-test-ng 会加入 Agents 插件的仿真动作：`mattermost-ai.AskAgentChannelMention` 和 `mattermost-ai.AskAgentDM`。动作名前缀由框架添加，Agents 中定义的动作 `Name` 值本身**不含**插件 ID。

**频率语义**

每个插件动作的 `frequency` 是与内置仿真动作共用一个加权选择器的**相对权重**，而不是独立的全局概率。例如核心动作 `CreatePost` 的 frequency 为 `1.0`，那么 Agents 默认的 `0.001`、`0.005`、`0.01` 大致相当于 `CreatePost` 权重的千分之一、千分之五、百分之一（未计入列表中其他动作时）。

建议收集约一周的遥测数据（Prometheus / Grafana 比率）后，参考 [coverage-frequency.md](../coverage-frequency.md) 调整频率。

**Agents 专属配置**

Agents 的压测行为由 Agents 的 `loadtest` 包配置，而非 `simulcontroller` 的 JSON：

- 当 `./config/mattermost-ai-loadtest.json` 不存在且未设置 `MM_AGENTS_LOADTEST_CONFIG` 时，使用默认值。
- 将 `MM_AGENTS_LOADTEST_CONFIG` 设为绝对路径，可覆盖 Agents 触发频率、`triggerMode`、`agentUsername` 等字段所用的 JSON 文件。
