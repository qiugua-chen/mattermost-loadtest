# 运行自动化负载对比测试


## 简介

本文介绍如何运行自动化的负载对比测试。**该配置面向有经验的用户**，适合周期性的自动对比；如果只是手动对比一两次，跑两轮独立压测再生成对比报告即可，参见[负载测试结果对比](compare.md)。

## 前置条件

- [Terraform](https://learn.hashicorp.com/terraform/getting-started/install)：要求 1.3.3 或更高版本（v1.x 系列即可）。
- AWS 凭据，配置方式见[这里](https://www.terraform.io/docs/providers/aws/index.html#authentication)。
- 有效的 Mattermost E20 license（通过 [`coordinator`](coordinator.md) 运行压测所需）。

## 配置

### deployer.json

首先通过 `deployer.json` 配置部署，它将作为创建对比所需全部部署的模板：

```
cp config/deployer.sample.json config/deployer.json
```

该配置文件的详细文档见[这里](config/deployer.md)。

### comparison.json

接下来配置对比本身：

```
cp config/comparison.sample.json config/comparison.json
```

该配置文件的详细文档见[这里](config/comparison.md)。

### config.json 与 coordinator.json

可选：通过编辑 `config.json` 和 `coordinator.json` 进一步调整压测特性，
详见[负载测试指南](local_loadtest.md)。

#### 说明

启动对比时，所需配置文件会自动从 `config/` 目录读取；找不到则使用默认值。

## 执行对比

### 配置 ssh-agent

自动化部署要求 [ssh-agent](https://linux.die.net/man/1/ssh-agent) 已运行并加载了私钥：

```sh
eval $(ssh-agent -s)
ssh-add PATH_TO_PRIVATE_KEY
```

`PATH_TO_PRIVATE_KEY` 替换为与 `SSHPublicKey` 配对的私钥路径（SSHPublicKey 此前已在 [deployer 配置](config/deployer.md)中设置）。

### 运行对比

```
go run ./cmd/ltctl comparison run
```

该命令会按配置启动全自动的负载对比测试，完成后输出结果。

`comparison run` 支持的几个实用参数：

- `--archive`：将全部产物（报告与图表）打包为 `.zip` 文件。
- `--format`：最终输出格式，`plain`（文本）或 `json`。
- `--output-dir`：可选的输出文件目录。

#### 说明

视配置不同，对比流程可能耗时数小时。

## 销毁环境

```
go run ./cmd/ltctl comparison destroy
```

用于永久销毁对比流程创建的全部部署资源。
