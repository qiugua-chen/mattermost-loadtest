# 本地运行负载测试

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/local_loadtest.md

## 简介

本文介绍如何在本地（以偏手动的方式）运行一次负载测试。
这在验证负载测试工具自身的改动时特别有用，
也是在尝试更高级部署形态之前、理解整个压测流程的最佳途径。

本地运行负载测试有三种方式，按复杂度递增：

- 直接运行 `ltagent` 命令。
- 通过负载测试代理 API 服务 `ltapi` 运行。
- 通过 [`coordinator`](coordinator.md) 运行。

## 前置条件

开始新的负载测试前，需要一个新部署的、正在运行的 Mattermost 实例，
以及其系统管理员（system admin）凭据。

### 克隆仓库

```sh
git clone https://github.com/mattermost/mattermost-load-test-ng
```

### 进入源码目录

```sh
cd mattermost-load-test-ng
```

### 复制并修改所需的配置文件

建议先复制配置样例文件，再按需修改：

```sh
cp config/config.sample.json config/config.json
cp config/simplecontroller.sample.json config/simplecontroller.json
```

负载测试配置文件的说明见[这里](config/config.md)。
默认的 [`UserController`](controllers.md) 是 `SimpleController`，其配置说明见[这里](config/simplecontroller.md)。

### 执行初始化

```sh
go run ./cmd/ltagent init
```

该命令会在目标 Mattermost 实例上创建初始的团队和频道，供压测用户加入。

#### 说明

`init` 命令按 `InstanceConfiguration` 配置段生成数据，但**不会**预置全部用户——
实际上只创建 50 个用户用于造数，该取值是为了最大化整体吞吐。

## 运行基础负载测试

用以下命令启动一轮新的负载测试：

```sh
go run ./cmd/ltagent -n 10 -d 60
```

这会启动一个运行 10 个用户、持续 60 秒的负载测试。

#### 说明

命令行参数的优先级高于配置文件。

> **扩展版说明**：本扩展版中 `ltagent` 新增了内嵌指标服务，
> 默认监听 `127.0.0.1:4001`（可用环境变量 `LT_METRICS_ADDR` 修改），
> 暴露 `/metrics`（Prometheus 格式）与 `/status`（运行状态 JSON）两个端点。

## 通过负载测试代理 API 服务运行

更进阶的方式是使用工具自带的 load-test agent API server。

### 启动 API 服务

```sh
go run ./cmd/ltapi
```

服务启动后默认在 4000 端口暴露 HTTP API。
另开一个终端即可发送命令来创建和运行 load-test agent：

### 创建新的 load-test agent

通过 API 创建 agent 时，请求体结构如下：

```json
{
  "LoadTestConfig": {
    ...
  },
  "SimpleControllerConfig": {
    ...
  },
  "SimulControllerConfig": {
    ...
  }
}
```

API 会基于请求内容生成相应 controller 的配置。
若请求中未携带 controller 配置或解析出错，服务端会回退读取本地的默认 controller 配置。

```sh
curl -d "{\"LoadTestConfig\": $(cat config/config.json)}" http://localhost:4000/loadagent/create\?id\=lt0
```

### 启动 load-test agent

```sh
curl -X POST http://localhost:4000/loadagent/lt0/run
```

### 增加活跃用户

```sh
curl -X POST http://localhost:4000/loadagent/lt0/addusers?amount=10
```

### 减少活跃用户

```sh
curl -X POST http://localhost:4000/loadagent/lt0/removeusers?amount=10
```

### 停止 load-test agent

```sh
curl -X POST http://localhost:4000/loadagent/lt0/stop
```

### 销毁 load-test agent

```sh
curl -X DELETE http://localhost:4000/loadagent/lt0
```

## 通过 coordinator 运行负载测试

最高级的方式是使用 [`coordinator`](coordinator.md)。
当需要摸清目标实例能支撑的**最大用户数**时必须用它；
coordinator 也支持跨 agent 集群运行负载测试。

### 前置条件

运行 [`coordinator`](coordinator.md) 需要一个正在运行的
[Prometheus](https://prometheus.io/docs/introduction/overview/) 服务，
且已针对目标 Mattermost 实例完成[配置](https://docs.mattermost.com/deployment/metrics.html)。

### 启动 load-test agent API 服务

第一步先让 API 服务跑起来：

```sh
go run ./cmd/ltapi
```

### 启动 LTBrowser 服务（可选）

如果负载测试配置了浏览器测试，则还需要启动 LTBrowser 服务。
另开一个终端执行：

```sh
nvm use
cd browser
make start
```

### 配置 coordinator

启动 [`coordinator`](coordinator.md) 前，先复制默认配置并按需修改：

```sh
cp config/coordinator.sample.json config/coordinator.json
```

配置说明见[这里](config/coordinator.md)。

### 运行 coordinator

另开终端运行 [`coordinator`](coordinator.md)：

```sh
go run ./cmd/ltcoordinator
```

这会在配置好的 load-test agent 集群上启动负载测试。

### 通过 API 服务运行 coordinator

与 load-test agent 类似，coordinator 也可以通过 API server 创建和运行（可多个）。

### 启动 API 服务

```sh
go run ./cmd/ltapi
```

### 创建 coordinator

```sh
curl -d "{\"CoordinatorConfig\": $(cat config/coordinator.json), \"LoadTestConfig\": $(cat config/config.json)}" http://localhost:4000/coordinator/create\?id\=ltc0
```

### 运行 coordinator

```sh
curl -X POST http://localhost:4000/coordinator/ltc0/run
```

### 停止 coordinator

```sh
curl -X POST http://localhost:4000/coordinator/ltc0/stop
```

### 销毁 coordinator

```sh
curl -X DELETE http://localhost:4000/coordinator/ltc0
```

## 结果对比

要对比多轮负载测试的结果，参见[这里](compare.md)。
