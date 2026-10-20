# 数据生成（Generating data）

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/generating_data.md

运行负载测试时，影响性能的变量很多，其中之一就是系统承载数据的形态与规模。你的分析可能要求所有测试共享同一份数据集；也可能要固定其他变量、只改变数据库初始状态。两种情况都需要一套可复现的数据生成流程。

本文介绍这套流程，它由工具内置的一种控制器实现：[`gencontroller`](controllers.md#gencontroller)。

## 前置条件

开始前，请确保已读懂[本地运行负载测试](local_loadtest.md)。

## 流程

整个流程分为四个阶段：

1. 准备目标系统。
2. 配置负载测试工具。
3. 运行测试生成数据。
4. 导出并归档数据。

### 目标系统准备

负载测试会发送生成数据所需的请求，但首先需要一个能接收这些请求的系统。

这一步怎么做取决于你想在哪里生成数据：

- 本地环境：全部在本地运行时，按服务端文档在你的电脑上搭一个 Mattermost 服务（用常规开发者工作流或运行发布版本均可）。
- 已有服务器：直接复用即可 :) 注意本流程会在该系统中创建大量数据，**绝不要**对生产系统执行。
- 临时 AWS 环境：可以用 load-test deployer 在 AWS 搭建新集群，方法见 [Terraform 指南的 Deployment 一节](terraform_loadtest.md#deployment)。这一步也可以顺带把 agent 部署到独立实例上，稍后再从该实例启动测试。

无论哪种方式，最终你都会得到一个运行中的 Mattermost 服务，且必须满足两个条件：
1. 有一个 sysadmin 账号，凭据可以提供给 load-test agent。
2. 计划运行 agent 的机器能访问到它。

第一条好办：记下任一 sysadmin 的用户名密码，没有就新建一个。第二条取决于部署方式：本地环境且同机运行 agent 则天然满足；AWS 临时环境 deployer 已处理好；已有部署则确认 agent 机器与服务器网络互通即可。

### 配置

目标 Mattermost 就绪后，开始配置 agent。登录 agent 所在机器，执行：

1. 首先确保压测指向目标系统：修改 [`config/config.json` 中的 `ConnectionConfiguration`](config/config.md#connectionconfiguration连接配置)。（用 deployer 搭的 AWS 集群此步已自动完成。）
2. 确认 [`config/config.json` 中的 `UserController`](config/config.md#usercontrollerconfiguration用户控制器配置) 设为 `generative`；同时把同一文件中 [`InstanceConfiguration`](config/config.md#instanceconfiguration实例数据配置) 下的所有绝对数量设为 0——有些命令会执行 `init` 按这些配置灌库，而我们要自己造数，不需要它代劳。
3. 创建或修改 `config/gencontroller.json`，可从样例文件开始：
```sh
cp config/gencontroller.sample.json config/gencontroller.json
```
4. 按需调整控制器配置。全部设置项都有[文档](config/gencontroller.md)，这里给几条建议：
  - 私信（DM）数量必须与运行用户数匹配（ simulated users 详见后文）。跑 10 个用户时，不同 DM 组合最多 45 个，不要设得比这更大。DM 上限公式为 n(n-1)/2，n 为用户数。
  - 真实服务器中各项数据都有特定分布，决策前最好在你想模拟的服务器上取数估算。例如正常服务器中绝大多数频道是 DM 和 GM（群聊），分别约占 75% 和 15%，私有频道约 7%，公开频道仅约 3%。

使用 gencontroller 时其他配置文件不会生效，无需修改。

### 数据生成

一切就绪，进入主步骤：生成数据。

还需要确定两个变量：
- 执行造数动作的模拟用户数。
- 每个模拟用户的速率（rate），即执行动作的速度。

这两个值没有黄金法则，最好的方式是先跑几轮试探并观察系统表现：
- 用户越多，造数越快。
- rate 越大，造数越慢（没错，这个 rate 实际语义更接近"周期"而非"速率"，官方计划日后改名）。

你多半希望造数越快越好，但务必谨慎——用户数太多或循环太快时，这个控制器会迅速变成 DDoS 工具。如果你部署的是 AWS 集群，Grafana 和 Prometheus 已自动配好，用它们监控系统能否承受你制造的负载。

无论试探还是正式造数，都在 agent 机器上执行：

```sh
go run ./cmd/ltagent -n 10 -r 0.5
```

要调的就是这两个参数：
- `-n`：模拟用户数。
- `-r`：用户执行动作的速率（0.5 表示动作速度减半，2 表示翻倍）。

全部配置数据创建完成后命令自动退出，也可用 <kbd>Ctrl-C</kbd> 提前终止。

#### 分批造数

用户规模很大时，一次性为所有用户造数可能不可行。这时可以用几个参数把过程分批：

```sh
go run ./cmd/ltagent -n 100 -r 1.0 --user-offset 100 --user-prefix ltuser
```

- `-n 100`：运行 100 个用户。
- `--user-prefix ltuser`：所有用户名以 `ltuser-` 开头。
- `--user-offset 100`：第一个模拟用户是 `ltuser-100`。

即本命令运行用户 `ltuser-100`、`ltuser-101`、……、`ltuser-199`。

例如可以用下面的循环每次 100 个、共跑 1000 个用户：

```sh
for i in `seq 0 100 1000`; do
    go run ./cmd/ltagent -n 100 -r 1.0 --user-offset %d --user-prefix ltuser
done
```

### 数据归档

数据生成完毕后需要导出归档。具体方式自定，但有几点建议：

- 可以用 Mattermost 自带的导出工具，但它缺少完整备份所需的部分功能。
- 最好的方式是直接 dump 数据库。视服务端使用 MySQL 还是 PostgreSQL 而定，大致如下：
```sh
# MySQL:
mysqldump --set-gtid-purged --column-statistics=0 -h <host> -u <user> <dbname> -p<password> | gzip > mysqldump.sql.gz

# PostgreSQL:
PG_PASSWORD=<password> pg_dump -h <host> -p <port> <db_name> | gzip > postgresdump.sql.gz
```
- gencontroller 还会上传一些示例文件，文件后端也要一并备份。如果是用本工具部署的 AWS 集群且使用 S3 桶，销毁前记得拷贝一份。

完成后把数据上传到安全的位置，作为后续测试的初始状态。使用方法见对比配置中的：
- [`DBDumpURL`](config/comparison.md#dbdumpurl)
- [`S3BucketDumpURI`](config/comparison.md#s3bucketdumpuri)
