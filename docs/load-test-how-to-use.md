# 负载测试使用指南

> 本文为上流文档的中文译版，英文原文见上游仓库：
> https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/load-test-how-to-use.md
>
> 说明：本文是 Mattermost 官方团队为"给服务端新特性做性能验证"所写的内部工作流，
> 其中 terraform/AWS 部署与 PR 提交流程为官方团队语境，供参考；
> 本扩展版的实际使用流程以根目录 [README.md](../README.md) 为准。

## 对新特性做负载测试的前置条件

 - 在某个 `mattermost` 分支上完成该特性的构建与测试。
 - 通读 [mattermost-load-test-ng](https://github.com/mattermost/mattermost-load-test-ng/tree/master/docs) 下的 markdown 文档。确保理解 coordinator、agent、controller 分别是什么，bounded 与 unbounded 负载测试的区别，以及为什么这套体系需要指标采集（一套 Prometheus 部署）。

## 对特性做负载测试的步骤

### 任务概览

 - 在 mattermost-load-test-ng 中编写新的压测动作（action）。
 - 在本地验证改动。
 - 在 terraform 环境中验证改动：目的是在尽可能贴近生产的环境中测试。
 - 分析压测结果。
 - 将改动合入负载测试仓库。

### 任务详述

#### 编写新的压测代码

 - 通读 [coverage.md](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/coverage.md#implementation-overview)，列出为新特性做压测所需的改动清单。
 - 可选：观看 @streamer45 的[视频讲解](https://drive.google.com/file/d/1l462zMdANwCRXUtj7nnHv2CX_6BiINHl/view)。
 - 在 `loadtest/` 目录中完成相应改动。

#### 本地验证改动

 - 通读 [local_loadtest.md](local_loadtest.md)（本仓库已提供中文版）。
 - 上述文档的补充说明：
    - 关于 `config.json` 的修改
        - 务必按本地 `mattermost` 部署情况修改 `config.json` 中的 `ConnectionConfiguration` 段。
        - `InstanceConfiguration` 的说明见[此文档](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/config/config.md#instanceconfiguration)。该配置由 `init` 命令使用，用于向 Mattermost 数据库灌注初始数据。
    - 调高新动作的执行频率，便于本地运行时调试。[示例](https://github.com/mattermost/mattermost-load-test-ng/blob/8faa4dfb485dace3bd65908c0d3d98979b7dfd17/loadtest/control/simulcontroller/controller.go#L227)
    - 遇到错误可参考[故障排查指南](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/faq.md#troubleshooting)。
    - 错误细节可查看仓库目录下的 `ltagent.log` 以及 Mattermost 服务端日志。
    - 强烈建议熟悉文档的其余章节，包括[通过 load-test agent API server 运行](local_loadtest.md#通过负载测试代理-api-服务运行)和[通过 coordinator 运行](local_loadtest.md#通过-coordinator-运行负载测试)——terraform 部署采用的正是后一种方式，理解这些底层概念会让调试部署问题容易得多。

#### 在 terraform 环境验证改动

在本地验证完新动作后，于生产级环境中压测新特性的步骤如下：

 - 阅读 [terraform_loadtest.md](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/terraform_loadtest.md)。
 - 上述文档的补充说明：
    - 执行[这一步](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/terraform_loadtest.md#copy-and-modify-the-required-configuration)时，将 `MattermostLicenseFile` 改为 License 文件所在路径。
        - `MattermostDownloadURL` 与 `LoadTestDownloadURL` 默认指向最新的 `mattermost` 与 load-test 发布包。
        - 当 `mattermost` 或 `mattermost-load-test-ng` 存在未合并的改动时，需要更新这两个值：
            - 在 `mattermost/server` 目录执行 `make build-linux`，将 `MattermostDownloadURL` 指向产出的 Mattermost 可执行文件路径，例如 `file:///somepath/mattermost/server/bin/linux_amd64/mattermost`。
            - 在 `mattermost-load-test-ng` 目录执行 `make package`，将 `LoadTestDownloadURL` 指向产出的 load-test 压缩包路径，例如 `file:///somepath/mattermost-load-test-ng/dist/v1.5.0-8-gd4f18cf/mattermost-load-test-ng-v1.5.0-8-gd4f18cf-linux-amd64.tar.gz`。
    - 配置好 ssh 后，修改 `deployer.json` 中的 `SSHPublicKey`。
    - 执行 `go run ./cmd/ltctl deployment create`
        - 若部署卡住，[检查 AWS 凭据是否过期并更新](https://community.mattermost.com/core/pl/weau31yyp38btddryjuxbsnh1r)。
        - [Terraform 操作是幂等的](https://community.mattermost.com/core/pl/jtebkneah3futd1y7pj8y9nrqy)，创建资源出错时很少需要 [destroy](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/terraform_loadtest.md#destroy-the-current-deployment) 整个部署。
    - 部署成功后，标准输出会给出 app server、agent、coordinator 和 Grafana 的地址信息。
        - 在浏览器打开 Mattermost URL 确认应用可用。
        - 此时可 [ssh 登录](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/terraform_loadtest.md#ssh-access-to-the-terraformed-hosts) app 实例查看服务端日志：`/opt/mattermost/logs/mattermost.log`。
    - 准备启动负载测试：
        - 在同一个 `config.json` 中配置 `InstanceConfiguration`（如前所述，用于向数据库灌注压测所需数据）。注意：`NumPosts` 配置较大的话灌注会非常耗时。可参考下方[备注](#备注)中"从备份手动恢复数据库"的方式绕过造数过程。
    - 用 `go run ./cmd/ltctl loadtest start` 启动压测。
        - 压测运行中可用 `go run ./ltctl loadtest status` 查看状态。
        - `ssh` 登录某台 agent 机器，`cat ~/mattermost-load-test-ng/logs/ltagent.log` 确认压测无报错。
        - 用 `go run ./cmd/ltctl deployment info` 输出的地址打开 Grafana。匿名访问为 `Viewer` 角色；需要编辑仪表盘时使用该命令显示的 `admin` 凭据登录。
        - 部署需要一段时间才能稳定。在压测工具将 `MaxActiveUsers` 数量的用户全部连接到 app 之前，可能会出现大量 HTTP 4xx 错误，属正常现象。

            错误数随时间变化的示例图：![error-vs-time](https://i.imgur.com/RSH1Szl.png)

    - "压测跑起来了，接下来呢？"
        - 若是 [bounded](https://github.com/mattermost/mattermost-load-test-ng/blob/497554e376ef23d548947bf331c8bdce6ce453d6/docs/faq.md#what-is-a-bounded-load-test)（定界）压测，需在[用户数连接稳定一小时后](https://community.mattermost.com/core/pl/45woi49ru7yrj8r8upzaqhog3a)手动执行 `go run ./cmd/ltctl loadtest stop` 停止。
        - 若是 [unbounded](https://github.com/mattermost/mattermost-load-test-ng/blob/497554e376ef23d548947bf331c8bdce6ce453d6/docs/faq.md#what-is-an-unbounded-load-test)（非定界）压测，测试会自动结束并输出该部署可支撑的最大并发用户数；状态查询命令会显示 `Done`。

#### 分析压测结果

"压测跑完了，结果怎么看？"
 - 对 unbounded 压测：结束时 `go run ./cmd/ltctl loadtest status` 会给出最大并发用户数，这是对比该版本 `mattermost` 性能的核心指标。
 - 对两轮 `MaxConcurrentUsers` 相同的 bounded 压测：可[生成对比报告](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/compare.md)，比较各项服务端指标。
 - 无论 bounded 还是 unbounded，都可以建 Grafana 仪表盘，把 API 指标过滤到与新特性 handler 相关的部分来分析。示例：
   ![sample-dashboard-creation](https://i.imgur.com/zzRfh8b.png)

#### 不用 terraform 在云端验证改动

即便不使用该框架，云端压测的通用流程也大致相同：

 - 创建数据库（供 Mattermost 服务使用）。
 - 部署 Mattermost 服务——方便起见称为 app server。
 - 创建 agent 机器，通过 [controllers](https://github.com/mattermost/mattermost-load-test-ng/tree/61c44f35224b76d3098199b0cd2b67db2222b549/loadtest/control) 对 app server 施压。
 - 灌注数据库，然后启动 app server 与 agent（即开始压测）。
 - 从 app server 与 agent 两侧采集指标——用于驱动 unbounded 压测的自动调节，或压测结束后分析 API 性能。

#### 提交 PR

完成全部代码改动后：
 - 按需[补充测试](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/coverage.md#testing)。
 - 执行 `make check-style`。
 - 执行 `go get -u github.com/mattermost/mattermost/server/v8@<master的commit-hash>` 和 `go mod tidy`。
 - 创建 PR。

#### 备注

 - **手动灌注数据库**：
    - 将 [InstanceConfiguration](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/config/config.md#instanceconfiguration) 段尽量精简，以缩短 `db init` 时间。
    - 在 [Developers:Performance](https://community.mattermost.com/core/channels/developers-performance) 频道发帖申请迁移数据文件。
    - 如果使用 [comparison](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/comparison.md) 流程，可以[直接从备份加载数据库](https://github.com/mattermost/mattermost-load-test-ng/blob/master/docs/config/comparison.md#dbdumpurl)；否则：
        - `ssh` 登录 app 机器，用 `psql` 连上数据库（连接串可通过 `ltctl deployment info` 获取）。
        - 删除并重建目标数据库，用 `zcat <backupfile> | psql <dsn>` 恢复备份数据。
        - 重启 app 服务让服务端完成必要的数据迁移：
          `sudo systemctl restart mattermost && until $(curl -sSf http://localhost:8065 --output /dev/null); do sleep 1; done;`
 - **如果特性在 feature flag 之后**：

    在 [deployment/terraform/strings.go](https://github.com/mattermost/mattermost-load-test-ng/blob/bd72575bd115112274e84823a646d8dda313c451/deployment/terraform/strings.go#L20) 的 `mattermostServiceFile` 字符串中加入 `Environment=MM_FEATUREFLAGS_<feature_flag_name>=true`。
