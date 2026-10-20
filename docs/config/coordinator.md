# Coordinator 配置（coordinator.json）

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/config/coordinator.md
>
> 组件原理见 [coordinator 说明](../coordinator.md)。

## ClusterConfig（集群配置）

*cluster.LoadAgentClusterConfig*

### Agents

*[]cluster.LoadAgentConfig*

#### Id

*string*

要创建并运行的 load-agent 的唯一标识。

#### ApiURL

*string*

运行该 agent 的 load-test API server 的地址。

### MaxActiveUsers

*int*

整个 load-agent 集群运行的最大并发活跃用户数。

### BrowserAgents

*[]cluster.LoadAgentConfig*

#### Id

*string*

要创建并运行的浏览器 load-agent 的唯一标识。

#### ApiURL

*string*

运行该 agent 的 load-test API server 的地址。因为同属 agent，与上面的 API server 相同。注意不要与 LTBrowser API server 混淆——后者是由 load-test API server 控制的独立服务。

### MaxActiveBrowserUsers

*int*

整个 load-agent 集群运行的最大并发浏览器用户数。注意 `config/config.json` 中也有 `MaxActiveBrowserUsers`，那个限制的是**单个** agent 的浏览器用户数，而本设置限制的是集群内所有浏览器 agent 的**总**浏览器用户数。该值必须小于等于 `UsersConfiguration.MaxActiveBrowserUsers * 浏览器 agent 数`，与 `MaxActiveUsers` 相互独立。

## MonitorConfig（监控配置）

*performance.MonitorConfig*

### PrometheusURL

*string*

采集目标实例性能指标的 [Prometheus](https://prometheus.io/docs/introduction/overview/) API server 地址。

### UpdateIntervalMs

*int*

每次查询更新之间的间隔（毫秒）。
在未发生性能劣化（无查询超阈值）时，该值也间接控制爬坡阶段新增用户的频率。
发生性能劣化时，用户增减节奏由 `coordinator.Config.RestTimeSec` 决定。

> [!NOTE]
> 自 [MM-61922](https://mattermost.atlassian.net/browse/MM-61922) 起，该设置已完全废弃，取值固定默认为 1000（即 1 秒）。

### Queries

*[]prometheus.Query*

#### Description

*string*

查询的描述。

#### Legend

*string*

在所有启用查询自动生成的 Grafana 仪表盘中，该查询面板的图例。留空则由 Grafana 自动生成。

#### Query

*string*

要执行的 [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) 查询。

#### Threshold

*float64*

阈值：查询值超过它时，性能监控向 coordinator 的反馈回路发出告警。

#### Alert

*bool*

是否启用告警。

> **提示**：Queries 数组为空或全部 `Alert=false` 时为 bounded（定界）压测；
> 至少一条启用即为 unbounded（非定界）压测。详见 [FAQ](../faq.md)。

## NumUsersInc

*int*

反馈回路每次迭代增加的活跃用户数。
应与预期测试的最大用户规模成比例。

## NumUsersDec

*int*

反馈回路每次迭代减少的活跃用户数。
应与预期测试的最大用户规模成比例。

## RestTimeSec

*int*

发生性能劣化事件后，等待多少秒再重新开始增/减用户。

**注**：实际增/减动作前的等待时间最长可达（`RestTimeSec + MonitorConfig.UpdateIntervalMs/1000`）秒。

## LogSettings（日志配置）

### EnableConsole

*bool*

为 true 时，按 ConsoleLevel 设置的级别向控制台输出日志。

### ConsoleLevel

*string*

控制台日志的详细级别。

### ConsoleJson

*bool*

为 true 时日志以机器可读的 JSON 格式输出，否则为纯文本。

### EnableFile

*bool*

为 true 时日志写入 `FileLocation` 指定的文件。

### FileLevel

*string*

文件日志的详细级别。

### FileJson

*bool*

为 true 时文件日志为 JSON 格式，否则为纯文本。

### FileLocation

*string*

日志文件路径。

### EnableColor

*bool*

为 true 时启用彩色输出。
