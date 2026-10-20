# LoadTest 配置（config.json）


## ConnectionConfiguration（连接配置）

### ServerURL

*string*

压测流量的目标 URL，应为目标 Mattermost 实例对外可访问的地址。

> **扩展版说明**：启用链路层采集时，此处填 nginx-perf 代理地址
> （如 `http://127.0.0.1:18065`），由代理转发到真实实例并落盘全量访问日志，
> 详见 [monitoring/README.md](../../monitoring/README.md)。

### WebsocketURL

*string*

用户建立 WebSocket 连接的端点地址。
大多数情况下与 `ServerURL` 相同，仅把 `http` 换成 `ws`、`https` 换成 `wss`。

### AdminEmail

*string*

目标 Mattermost 实例系统管理员的邮箱。

### AdminPassword

*string*

目标 Mattermost 实例系统管理员的密码。

## UserControllerConfiguration（用户控制器配置）

### Type

*string*

用于驱动用户的 [`UserController`](../controllers.md) 类型。

可选值：
- `simple` —— 使用 [`SimpleController`](../controllers.md#simplecontroller)（**本扩展版消息场景使用此类型**）
- `simulative` —— 使用 [`SimulController`](../controllers.md#simulcontroller)
- `noop` —— 使用 [`NoopController`](../controllers.md#noopcontroller)
- `generative` —— 使用 [`GenController`](../controllers.md#gencontroller)

### RatesDistribution

*[]struct{
  Rate float64
  Percentage float64
}*

各 controller 的动作速率分布。

Rate 是乘数，影响 `UserController` 执行用户动作的速度：

- rate < 1.0：动作节奏加快。
- rate == 1.0：默认节奏。
- rate > 1.0：动作节奏放慢。

Percentage 是以该速率运行的 controller 所占比例。

### ServerVersion

*string*

可选：运行动作时使用的 MM 服务端版本号（如 `5.30.0`）。
设置后覆盖实际服务端版本；留空则使用服务端返回的版本。

## InstanceConfiguration（实例数据配置）

以下各项均由 `init` 初始化过程创建。

### NumTeams

*int*

目标 Mattermost 实例应拥有的团队数量。

### NumChannels

*int*

目标 Mattermost 实例应拥有的频道数量。

### NumPosts

*int*

目标 Mattermost 实例应拥有的帖子数量。

### NumReactions

*int*

目标 Mattermost 实例应拥有的表情回应（reaction）数量。

### NumAdmins

*int*

目标 Mattermost 实例应拥有的管理员数量。

## UsersConfiguration（用户配置）

### InitialActiveUsers

*int*

负载测试启动时的活跃用户数量。

### UsersFilePath

*string*

用户凭据文件路径。设置后工具使用该文件中的账号而非自动注册。
每行一个用户，格式为"邮箱（或用户名）+ 空格 + 密码"；
文件行数应不少于 MaxActiveUsers。

> **扩展版说明**：本扩展版的 `scripts/mattermost_prepare_users.py`
> 即用于批量生成符合该格式的凭据文件。

### MaxActiveUsers

*int*

load-test agent 运行的最大并发活跃用户数。

### MaxActiveBrowserUsers

*int*

每个实例上 load-test agent 运行的最大并发浏览器用户数。

### PercentOfUsersAreAdmin

*float*

生成的用户中系统管理员所占百分比。

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

文件日志的详细级别（排查问题时设为 `"DEBUG"`）。

### FileJson

*bool*

为 true 时文件日志为 JSON 格式，否则为纯文本。

### FileLocation

*string*

日志文件路径。

### EnableColor

*bool*

为 true 时启用彩色输出。
