# FAQ（常见问题）

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/faq.md

### load-test agent 支持哪些登录方式？

目前仅支持 邮箱/密码 认证方式。

### 什么是 bounded（定界）负载测试？

模拟用户数固定不变的负载测试称为 bounded。
特别适用于在两个集群/两个构建版本之间做性能对比。

### 什么是 unbounded（非定界）负载测试？

模拟用户数可在预配置上限内动态变化的负载测试称为 unbounded。
用于测定系统容量，最终会输出一个估算的可支撑用户数。
经验法则：启动 unbounded 压测时，目标用户数应始终设得**高于**你预估的实例承载能力。

### 如何配置测试为 bounded 或 unbounded？

测试的性质由 [`coordinator`](coordinator.md) 如何控制[反馈回路](coordinator.md#反馈回路)决定。如果 coordinator 配置为"某些指标超阈值时减少用户"（例如服务端 P99 延迟超过 2 秒），则为 unbounded；如果 coordinator 不监控这些指标、让所有用户自由连接，则为 bounded。

具体配置见 [`coordinator.json` 中的 `MonitorConfig.Queries` 配置](config/coordinator.md#queries)：

- 查询数组为空，或全部禁用（`Alert` 设为 `false`）→ **bounded**。
- 至少有一条查询启用 → **unbounded**。

注意两种情况下，`ClusterConfig.MaxActiveUsers` 都应设为 `AgentInstanceCount * UsersConfiguration.MaxActiveUsers`。

### 单个 agent 支持多少用户？

AWS `c5.xlarge` 实例（4 vCPU、8 GB 内存）上运行的 agent，建议最大用户数为 2000。

### 可以使用已有的 Mattermost 或数据库部署吗？

可以，可使用已有的 Mattermost 部署，或仅复用其数据库部分。

注意：**不要**直接对生产环境做压测——压测 agent 会创建用户、帖子、团队和频道，并大量占用服务器资源。最佳实践是克隆一套生产环境用于压测。

#### 使用已有的 Mattermost 部署

1. 在 `config/deployer.json` 中将 `AppInstanceCount` 和 `TerraformDBSettings.InstanceCount` 设为 `0`，避免创建 Mattermost 与数据库集群。
2. 将 `config/config.json` 中的 `ConnectionConfiguration` 改为你的既有 Mattermost 部署信息。

    `ServerURL` 和 `WebSocketURL` 可在 **Mattermost** 的 config.json 的 `ServiceSettings` 段找到。若 `WebSocketURL` 为空，将 `http`/`https` 相应替换为 `ws`/`wss`。

    ```json
    "ConnectionConfiguration": {
    	"ServerURL": "http://localhost:8065",
    	"WebSocketURL": "ws://localhost:8065",
    	"AdminEmail": "sysadmin@sample.mattermost.com",
    	"AdminPassword": "Sys@dmin-sample1"
    }
    ```

#### 使用已有的数据库

使用已有数据库前，确保数据库允许来自 AWS 资源部署网段的连接。

1. 在 `config/deployer.json` 中将 `TerraformDBSettings.InstanceCount` 设为 `0`，避免创建数据库集群。
2. 创建一个 config patch 文件，填入访问数据库所需的配置。patch 文件内容参考下例（值按你的部署填写）。

    注：本例将以下文件命名为 `configPatch.json`，放在 loadtest 根目录。

    ```json
    {
        "SqlSettings": {
            "DriverName": "postgres",
            "DataSource": "postgres://mmuser:mostest@databaseURL:port/mattermost_test?sslmode=disable&connect_timeout=10&binary_parameters=yes",
            "DataSourceReplicas": [],
            "DataSourceSearchReplicas": [],
            "MaxIdleConns": 20,
            "ConnMaxLifetimeMilliseconds": 3600000,
            "ConnMaxIdleTimeMilliseconds": 300000,
            "MaxOpenConns": 300,
            "Trace": false,
            "AtRestEncryptKey": "",
            "QueryTimeout": 30,
            "DisableDatabaseSearch": false,
            "MigrationsStatementTimeoutSeconds": 100000,
            "ReplicaLagSettings": []
        }
    }
    ```

3. 将 `config/deployer.json` 中的 `MattermostConfigPatchFile` 改为该 patch 文件的绝对路径。

    示例：

    ```json
    "MattermostConfigPatchFile": "/home/ubuntu/mattermost-load-test-ng/configPatch.json",
    ```

4. 继续执行 deployment create 流程。

### 可以用同一个 AWS 账号同时跑多轮压测吗？

可以。关键要求是每个部署在 `config/deployer.json` 中的 `ClusterName` 和 `TerraformStateDir` 必须唯一。

`ClusterName` 用作部署创建的所有 AWS 资源（EC2 实例、安全组、S3 桶等）的前缀；`TerraformStateDir` 是 Terraform 在你本机存放状态文件的目录。两者都必须唯一以避免部署间冲突。

并行多部署的步骤：

1. 为每个部署复制一份 deployer 配置：
    ```sh
    cp config/deployer.json config/deployer-test1.json
    cp config/deployer.json config/deployer-test2.json
    ```
1. 编辑各配置文件，设置唯一的 `ClusterName` 和 `TerraformStateDir`：
    - `ClusterName`：如 `"loadtest1"`、`"loadtest2"` 等。
    - `TerraformStateDir`：如 `"/var/lib/mattermost-load-test-ng/loadtest1"`、`"/var/lib/mattermost-load-test-ng/loadtest2"` 等。

1. 在不同终端分别为各部署执行创建命令：
    ```sh
    go run ./cmd/ltctl deployment create --config config/deployer-test1.json
    go run ./cmd/ltctl deployment create --config config/deployer-test2.json
    ```

### 可以导出 Grafana 仪表盘留存吗？

可以，通过[发布快照到 Raintank](https://grafana.com/docs/grafana/latest/dashboards/share-dashboards-panels/#publish-a-snapshot) 功能实现。快照示例：[MySQL bounded 测试对比 `v7.9.1` vs `v7.10.0-rc2`](https://snapshots.raintank.io/dashboard/snapshot/h356ygrRZIUFWf5u5cctLjFavu97lFR2?orgId=2)。

两点注意：

- 由于[此 issue](https://github.com/grafana/grafana/issues/32585)，需要登录才能在分享对话框中看到 Snapshot 选项。虽然这些临时实例通常无需登录，但为此目的可以用 `deployment info` 输出的 Grafana URL 下方列出的 `admin` 凭据登录。
- 快照虽便于留存参考，但不是功能完整的仪表盘，无法用它查询新数据。参考上面的示例理解其工作方式。

### 可以对 ElasticSearch 任务做压力测试吗？

ElasticSearch 每天调度一次帖子聚合任务，配置方式是修改 Mattermost 服务端的 `ElasticsearchSettings.PostsAggregatorJobStartTime`（接受服务器本地时间的硬编码字符串，格式如 `"15:04"`）。
如果想在压测期间专门压这个任务，可以用 deployer 配置中的 [config patch](config/deployer.md#mattermost-config-patch-file) 把任务时间改到压测运行时段，patch 内容示例：

```json
{
    "ElasticSearchSettings": {
        "PostsAggregatorJobStartTime": "19:34"
    }
}
```

### 可以在 Mattermost 前面使用自定义负载均衡器（如 ALB/NLB）吗？

可以：禁用内置代理服务器，并手动把 `ServerURL` 指向你的反向代理。app server 需要在环境创建过程中手动注册到负载均衡器，因此理想做法是提前建好 LB/Target Group，实例就绪后逐个注册：

- `deployer.json` 配置：`ProxyInstanceCount` 设为 `0`，`ServerURL`（注意**不是** `SiteURL`）指向你的反向代理地址。
- 环境创建过程中，app server 就绪后把你的反向代理指向它们。

## 故障排查

### 提高调试级别

排查问题的第一步是同时提高 Mattermost 服务端和 agent 的调试级别：

- Mattermost 服务端：使用 [config patch](config/deployer.md#mattermost-config-patch-file)，内容可包含：

```json
{
    "LogSettings": {
        "FileLevel": "DEBUG"
    }
}
```

- agent：将 [config.json 中的 `LogSettings.FileLevel`](config/config.md#file-level) 同样设为 `"DEBUG"`。

### 用户连不上

确认 MM 系统控制台中 `Enable Account Creation` 和 `Enable Open Server` 均为 `true`。

### agent 日志出现大量 `current team should be set` 错误，用户无法加入团队和频道

可能是 app server 尚未初始化（至少需要一个开放团队）。可手动创建，或执行 `ltctl loadtest init`。
手动创建时，`Team Settings` 下的 `Allow any user with an account on this server to join this team` 应设为 `Yes`。
同时，Mattermost 系统控制台的 `Max Users Per Team` 设置应足以容纳模拟用户数。

> **扩展版说明**：本扩展版已对该错误做容错处理——预置账号无当前团队时
> Reload 自动回退到首个所属团队，noopcontroller 的 joinChannel 降级为跳过，
> 不再直接判定失败。详见 [EXT-CHANGES.md](EXT-CHANGES.md)。

### agent 报 `MaxActiveUsers is not compatible with max Rlimit value` 错误

表示最大文件描述符数低于 agent 运行所需。

执行以下命令提高上限：

```sh
ulimit -n VALUE
```

#### 说明

Terraform 部署中，该值硬编码在 loadtest api 的 `systemd` 文件里。如需修改，把 `/lib/systemd/system/ltapi.service` 中的 `LimitNOFILE` 调大：

1. ssh 登录压测 agent（可用 `go run ./cmd/ltctl ssh` 查看有哪些 agent）。
2. 修改 `/lib/systemd/system/ltapi.service` 中的值。
3. 重启相关进程：

```bash
sudo systemctl daemon-reload
sudo systemctl restart ltapi
```

每台 loadtest agent 都要执行一遍（上述 `ltctl ssh` 输出中以 `agent-` 开头的即 agent 机器）。

### `ServerURL` 和 `ServerScheme` 设置的用途？

用于需要在压测环境中覆盖 Mattermost 连接 URL 的场景——最常见的是压测部署前有自定义反向代理。

**大多数情况下不应使用**，因为 loadtest agent 会从配置和已部署服务中自动探测 server URL 与 scheme。

### 执行 deployment create 时报 `No default subnet for availability zone: 'AWS_SUBNETS_HERE'.`

通常是因为 AWS 在 Terraform 配置指定的可用区中没有可用子网。将 config/deployer.json 中的 AWSAvailabilityZone 留空即可解决。

提醒：留空后资源不一定部署在同一可用区，超大规模测试的流量相关 AWS 费用可能上升。

### 为什么对 v11.0.1 之前的 Mattermost 服务端，agent 会报 501 错误？

自 load-test-ng **v1.30.0** 起，工具使用 `GetClientConfig` API 端点，而该端点在 Mattermost **v11.0.1** 发生变更（`format` 参数被废弃）。用 v1.30.0 及以上版本压测 v11.0.1 之前的服务端会返回 501。

若你的 Mattermost 服务端低于 v11.0.1，请使用 load-test-ng **v1.29.0** 或更早版本——它使用带 `format=old` 的旧版 client config API，兼容 v11 之前的服务端。

**注意**：旧版 load-test-ng 可能缺少最新修复与功能。请查阅所选版本的 [changelog](https://github.com/mattermost/mattermost-load-test-ng/releases) 了解潜在限制。
