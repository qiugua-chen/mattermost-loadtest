# Controllers（用户控制器）


## 概述

`UserController`（用户控制器）是负责驱动/控制单个虚拟用户的组件。

它的实现定义了用户的行为模式，包括用户可能执行的任何动作，
例如登录、发消息、更新个人资料等。

## 可用的 Controller

本工具目前提供并支持以下实现：

### `SimpleController`

基础版控制器：按预定义（且可配置）的一组动作循环执行。
它的可配置性和动作粒度，使其非常适合针对单个 API 调用做性能验证。
当压测目标是确认某次代码变更是否影响了**特定动作子集**的性能时，
推荐使用该控制器。

> **扩展版说明**：本仓库的二开动作（CreateDirectPost / CreateGroupPost /
> CreateChannelGroupPost）均挂在 SimpleController 上，通过控制器配置 JSON
> 的 `actions` 列表按名引用，详见 [EXT-CHANGES.md](EXT-CHANGES.md)。

### `SimulController`

仿真度最高的控制器：尽可能模拟真实用户的行为。
当压测目标是摸清目标实例能支撑多少**并发活跃用户**时，推荐使用该控制器。
它也可以用于对后端代码做冒烟测试。

### `NoopController`

仅执行"让用户连上服务器"所需的最少动作的控制器。
用于测量 Mattermost 实例的**理想性能基线**：
让用户登录一次、建立一条 WebSocket 连接、只发一个请求。
它的运行结果是与其他控制器结果进行对比的基准线。

> **扩展版说明**：本扩展版对 NoopController 做了一处降级改造——预置账号
> 未设置当前团队时，joinChannel 由"报错"降级为"跳过"，
> 适合纯连接/在线率压测场景。

### `GenController`

用途是生成数据（团队、频道、帖子等）。
当需要更真实的初始数据环境时特别有用；
也用于 init 初始化过程中向空数据库灌注数据。

### `BrowserController`

基于 Playwright 运行浏览器仿真的专用控制器。
与其他控制器不同，它不能仅通过设置 `UserControllerConfiguration.Type` 启用，
而是需要独立的 load-test agent 部署方式并通过 API server 配置。
实现细节见 [loadtest/control/browsercontroller/controller.go](../loadtest/control/browsercontroller/controller.go)，
配置说明见 [docs/config/browsercontroller.md](config/browsercontroller.md)。
