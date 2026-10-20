# 使用 Terraform 部署运行负载测试


本文介绍如何用 [Terraform](https://www.terraform.io/intro/index.html) 部署来搭建并运行负载测试。
按此流程可以基本自动化地创建一套完整的[负载测试系统](loadtest_system.md)，
这是对生产形态 Mattermost 实例做压测的推荐方式。

## 前置条件

- [Terraform](https://learn.hashicorp.com/terraform/getting-started/install)：要求 1.3.3 或更高版本（v1.x 系列即可）。
- AWS 凭据，配置方式见[这里](https://www.terraform.io/docs/providers/aws/index.html#authentication-and-configuration)。Mattermost 内部员工请使用 [AWS Single Sign-On](https://aws.amazon.com/blogs/security/aws-single-sign-on-now-enables-command-line-interface-access-for-aws-accounts-using-corporate-credentials/) 为 [AWS credentials 文件](https://www.terraform.io/docs/providers/aws/index.html#shared-credentials-file)生成 API 凭据（凭据 12 小时后过期需重新生成；若保存为 profile，名称须用 `mm-loadtest`）。
- 有效的 Mattermost Enterprise license（通过 [`coordinator`](coordinator.md) 运行压测所需）。Mattermost 内部员工可在 [~Team: Self-Serve](https://community.mattermost.com/private-core/channels/team-self-serve) 频道获取测试 license。

### 克隆仓库

```sh
git clone https://github.com/mattermost/mattermost-load-test-ng
```

### 进入源码目录

```sh
cd mattermost-load-test-ng
```

### 复制并修改所需配置

开始部署前需要先配置 deployer：

```sh
cp config/deployer.sample.json config/deployer.json
```

deployer 配置的详细文档见[这里](config/deployer.md)。至少要设置：`SSHPublicKey` 为你的公钥路径、`MattermostLicenseFile` 为企业 license 路径、`ClusterName` 为你的 AWS 账号内的唯一值。

## 部署（Deployment）

### 配置 ssh-agent

deployer 工作需要 [ssh-agent](https://linux.die.net/man/1/ssh-agent) 已运行并加载私钥：

```sh
eval $(ssh-agent -s)
ssh-add PATH_TO_PRIVATE_KEY
```

`PATH_TO_PRIVATE_KEY` 替换为与 `SSHPublicKey` 配对的私钥路径（见 [deployer 配置](config/deployer.md)）。

### 创建新部署

```sh
go run ./cmd/ltctl deployment create
```

创建[完整](loadtest_system.md)部署可能需要几分钟。默认情况下控制台会持续输出 info 日志；如果没有输出，说明出问题了。

完成后会输出整个集群的信息，此后即可开始新的负载测试。

如果 `deployment create` 报 "resource already exists" 类错误，多半是 `ClusterName` 在你的 AWS 账号内不唯一。执行 `go run ./cmd/ltctl deployment destroy` 清理建了一半的部署，把 `ClusterName` 改得更唯一后重试。

注：非 Mattermost 员工一般需要设置 `--service_environment` 参数为 `production`，以便选用正确的 license 类型（官方内部使用测试 license）。

### 查看当前部署信息

```sh
go run ./cmd/ltctl deployment info
```

### 同步当前部署

```sh
go run ./cmd/ltctl deployment sync
```

该命令会同步对 AWS 集群所做的手动变更。

### 可选：配置 coordinator 与 load-test

用 `ltctl` 启动负载测试时，所需配置文件会自动上传到 [coordinator](coordinator.md) 所在实例；
找不到文件则使用默认值。

#### 复制默认配置

配置 [coordinator](coordinator.md)：创建/编辑 `config/coordinator.json`。

```sh
cp config/coordinator.sample.json config/coordinator.json
```

配置文档见[这里](config/coordinator.md)。

#### 复制默认配置

配置 load-test：创建/编辑 `config/config.json`。

```sh
cp config/config.sample.json config/config.json
```

配置文档见[这里](config/config.md)。

## 运行测试

### 启动负载测试

```sh
go run ./cmd/ltctl loadtest start
```

在整个 load-test agent 集群上开始压测。

注：非 Mattermost 员工一般需要设置 `--service_environment` 参数为 `production`。

### 查看负载测试状态

```sh
go run ./cmd/ltctl loadtest status
```

### 停止运行中的负载测试

```sh
go run ./cmd/ltctl loadtest stop
```

### 重新初始化负载测试

```sh
go run ./cmd/ltctl loadtest reset
```

该命令会清空目标实例数据库的全部数据，并重新执行 init 初始化过程。

## 销毁当前部署

部署用完后建议执行：

```sh
go run ./cmd/ltctl deployment destroy
```

永久销毁当前部署的全部资源。

## 结果对比

对比多轮负载测试的结果，参见[这里](compare.md)。

## 调试

### SSH 访问 terraform 创建的主机

* 列出全部主机：
  ```sh
  go run ./cmd/ltctl ssh  
  ```
* SSH 到指定主机：
  ```sh
  go run ./cmd/ltctl ssh <instance-name>
  ```

* 常用目标：
  * `coordinator` —— 第一台 agent，同时充当压测 coordinator
  * `proxy` —— 运行 Nginx 的实例
  * `metrics`、`prometheus`、`grafana` —— 运行全部指标相关服务的实例
  * `browser-agent` —— 运行浏览器 agent 的实例

### 数据库访问

* 列出全部数据库实例及其角色：
  ```sh
  go run ./cmd/ltctl db
  ```
  输出每个实例的角色（`writer` 或 `reader-N`）、endpoint 和标识符。

* 通过交互式 `psql` 会话连接数据库：
  ```sh
  go run ./cmd/ltctl db connect
  ```
  该命令通过跳板机（app server 或 metrics server）建立 SSH 隧道并打开 `psql`。默认连接第一个 reader 实例；只有一个实例时回退到 writer。

* 连接指定实例：
  ```sh
  go run ./cmd/ltctl db connect writer
  go run ./cmd/ltctl db connect reader-0
  ```

  **注**：仅支持 `aurora-postgresql`，且本机需已安装 `psql`。
