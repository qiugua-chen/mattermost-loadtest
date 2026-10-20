# 文档索引

> **扩展版读者提示**：如果你使用的是本扩展仓库（mattermost-loadtest），
> 建议优先阅读根目录 [README.md](../README.md)、[EXT-CHANGES.md](EXT-CHANGES.md)
> 与 [REUSE.md](REUSE.md) 三份中文文档；本目录其余文档为上游参考资料。

## 从哪里开始？

这里的文档体系比较庞大，新读者可能不知道从哪读起才能理解工具的工作方式与架构。
最佳起点取决于你的目标：

- 如果你想**用这个工具跑测试**，建议按以下顺序阅读：
    - [使用指南](load-test-how-to-use.md)：全面概述工具的工作方式以及实用技巧，并链向多份扩展文档。**跑任何测试之前请先完整读完它！**
    - [本地运行负载测试](local_loadtest.md)：在自己的电脑上（不依赖 AWS）跑负载测试的具体指南。本地跑几次能帮你理解各组件如何协同。
    - [用 Terraform 运行负载测试](terraform_loadtest.md)：更进阶的指南，在 AWS 上部署集群以执行更大规模的压测，是生产级测试的推荐方式。
    - [生成与对比报告](compare.md)：测试执行完后，如何生成分析报告、如何与另一轮测试做 diff 对比并出图。
    - [FAQ](faq.md)：对工具有疑问时先查这里，可能别人已经问过了。
- 如果你更想**了解工具架构或参与改进**，从这里开始：
    - [架构](loadtest_system.md)：鸟瞰各组件及其连接关系。
    - [实现](implementation.md)：从高层到细节介绍各组件的实现方式。
    - [覆盖率](coverage.md)：服务端实现了新特性后，如何修改 simulcontroller 为其补充压测覆盖。
    - [开发者工作流](developing.md)：参与工具开发的速查手册。

## 参考文档

### 配置

工具几乎一切都可配置：虚拟用户行为、集群搭建方式、coordinator 衡量系统稳定性的指标与阈值、甚至 AWS 专属设置等等。

这带来了灵活性，也让配置体系相当复杂。为降低门槛，每个可调项都有详尽文档（如发现缺漏欢迎提 issue 或 PR）。以下是 `config/` 目录下各配置文件的说明文档：

- [通用配置](config/config.md)：选用哪种 controller（见下文）、controller 如何连接 Mattermost 服务器、各 controller 的用户规模与活跃度、初始数据灌注等。
- 各类 Controller 的配置：
    - [simulative controller](config/simulcontroller.md)：最贴近真实用户的控制器，覆盖高频动作，动作概率经过精细调校。
    - [generative controller](config/gencontroller.md)：需要快速生成大量数据时使用。该控制器下用户行为并不真实（真实用户不会每秒发 10+ 帖），但为造数速度做了优化。
    - [simple controller](config/simplecontroller.md)：需要按自定义的动作集与频率循环执行时使用。**本扩展版的消息场景（单聊/群聊/频道群发）均基于此控制器**。
- [Deployer 配置](config/deployer.md)：控制经 Terraform 部署到 AWS 的集群及 AWS 专属设置。
- [Coordinator 配置](config/coordinator.md)：调整 coordinator 行为，如最大模拟用户数、测试类型（bounded / unbounded 及其控制方式）。
- [Comparison 配置](config/comparison.md)：控制对比流程，详见下文「进阶工作流」。

### 组件

描述主要组件及其交互方式的专题文档：

- [Controllers](controllers.md)：什么是 controller，以及各实现的清单。
- [Coordinator](coordinator.md)：coordinator 组件的定义与工作方式。

## 进阶工作流

熟悉工具并成功跑过几轮测试后，以下进阶指南可能有用：

- [自动化负载对比测试](comparison.md)：专为"固定其他变量、对比两个 Mattermost 版本性能"设计的工作流，Mattermost Server 团队的月度版本性能对比即采用此流程。
- [数据生成](generating_data.md)：更大规模压测需要更大的数据集。本文介绍如何用 gencontroller 造出任意规模的团队、频道、帖子、表情回应等，作为后续测试的初始数据。

## 配置样例

我们深知负载测试工具的配置对新手并不友好。[`examples/config`](../examples/config) 目录中维护着一批正在使用的配置模板，可参考真实世界的配置文件学习。
