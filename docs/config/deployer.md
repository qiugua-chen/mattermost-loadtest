# Deployer 配置（deployer.json）

>
> 说明：本文件是 AWS/Terraform 自动化部署的专属配置。自建机房/内网环境
> 用不到 deployer，直接按[本地运行指南](../local_loadtest.md)部署 agent 即可。

## AWSProfile

*string*

部署使用的 AWS profile，同时用于本地执行的所有 AWS CLI 命令。详见 [AWS 文档](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html)。

## AWSRegion

*string*

部署使用的 AWS 区域。详见 [AWS 文档](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)。

## AWSAvailabilityZone

部署实例的 AWS 可用区。详见 [AWS 文档](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)。

> [!TIP]
> 将实例部署在同一可用区可显著降低流量费用，详见[数据传输定价](https://aws.amazon.com/ec2/pricing/on-demand/#Data_Transfer_within_the_same_AWS_Region)。

## AWSRoleARN

*string*

工具与 AWS 交互时代入（assume）的 AWS 角色 ARN，通过默认凭据链完成代入。由于角色链限制，该角色一小时后过期，因此配置后凭据会每 25 分钟自动刷新一次——这意味着该角色需要拥有"代入自身"的权限。

## AWSAMI

*string*

部署使用的 AWS AMI，即压测工具创建的所有 EC2 实例所用的镜像，详见 [AWS AMI 文档](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html)。建议 Ubuntu 20.04 或 22.04。注意 AMI ID 随 AWS 区域不同而不同。

## AWSAMIUser

*string*

连接 AWS AMI 时使用的用户名（默认：`ubuntu`）。

## OperatingSystemKind

*string*

部署使用的操作系统类型，即压测工具创建的所有 EC2 实例所用的操作系统，详见 [AWS AMI 文档](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html)。当前支持 `debian`（默认）和 `rhel`。

虽然两者都支持，仍推荐 `debian` 搭配 Ubuntu 22.04 LTS——使用更广泛、支持更好；其他系统用 Debian/RHEL 类型可能因目标系统差异而出问题。

> [!NOTE]
> `rhel` 类型仅在 CentOS Stream 9 上测试过。

## ClusterName

*string*

集群名称，会作为前缀加到按此配置创建的所有 AWS 资源上。

## ClusterVpcID

*string*

资源关联的 VPC ID。

**注**：该设置只影响 load-test agent 实例，面向预部署环境使用。

## ClusterSubnetID

*string*

资源关联的子网 ID。

**注**：该设置只影响 load-test agent 实例，面向预部署环境使用。

## ConnectionType

*string*

定义实例的访问方式：`public`（默认）或 `private`。

它决定 deployer 如何连接服务器，以及压测工具各 CLI 工具向用户暴露的连接 IP：
- `private`：deployer 使用内网 IP 连接服务器——**用户必须通过跳板机/VPN 与服务器处于同一网络**。
- `public`：deployer 使用公网 IP 连接——用户须能访问互联网，且**服务器所在 VPC 与子网须有互联网出口**。

## AppInstanceCount

*int*

Mattermost 应用实例数量。
设为 0 表示仅部署 load-test agent。
大于 1 时，部署中会自动创建 S3 桶并将服务端配置为以它作为文件存储。

## AppInstanceType

*string*

应用服务器的 EC2 实例类型，见[实例类型表](https://aws.amazon.com/ec2/instance-types/)。建议用 c5 系列以获得稳定的性能表现。

## AppAttachIAMProfile

*string*

附加到应用服务器的 IAM profile。需要让应用服务器访问其他既有 AWS 资源时使用。

## AgentInstanceCount

*int*

load-test agent 实例数量。第一台实例同时运行 [coordinator](coordinator.md)。

## AgentInstanceType

*string*

压测 agent 的 EC2 实例类型，见[实例类型表](https://aws.amazon.com/ec2/instance-types/)。

## BrowserAgentInstanceCount

*int*

浏览器 agent 实例数量。

## BrowserAgentInstanceType

*string*

浏览器 agent 的 EC2 实例类型，见[实例类型表](https://aws.amazon.com/ec2/instance-types/)。

## AgentAllocatePublicIPAddress

*bool*

是否为 agent 实例分配公网 IP。

## ClusterSubnetIDs

### App

*[]string*

应用服务器关联的子网 ID 列表。

### Job

*[]string*

job server 关联的子网 ID 列表。

### Proxy

*[]string*

代理服务器关联的子网 ID 列表。

### Agent

*[]string*

load-test agent 关联的子网 ID 列表。

### ElasticSearch

*[]string*

Elasticsearch 实例关联的子网 ID 列表。设置多个时需同时设置 `ElasticSearchSetting.ZoneAwarenessEnabled` 和 `ElasticSearchSetting.ZoneAwarenessAZCount`。

### Metrics

*[]string*

指标服务器关联的子网 ID。

## Keycloak

*[]string*

Keycloak 服务器关联的子网 ID 列表。

## Database

*[]string*

数据库实例关联的子网 ID 列表。

## Redis

*[]string*

Redis 实例关联的子网 ID 列表。


## ElasticSearchSettings

### InstanceCount

*int*

要创建的 ElasticSearch 实例数量。当前仅支持 `1` 或 `0`。

### InstanceType

*string*

Elasticsearch 服务的实例类型。仅允许 AWS OpenSearch 实例，且不支持无 EBS 存储卷的实例。完整类型清单见 [AWS 文档](https://docs.aws.amazon.com/opensearch-service/latest/developerguide/supported-instance-types.html)。

### Version

*string*

部署的 Elasticsearch 版本。部署仅支持与 ElasticSearch 兼容（含 v7.10.0 及以下）的 AWS OpenSearch 版本，即以 `Elasticsearch_` 为前缀的那些。版本详情见 [AWS 文档](https://aws.amazon.com/opensearch-service/faqs/)，具体版本字符串格式见 [`aws` Terraform provider 文档](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/opensearch_domain#engine_version)。

### CreateRole

*bool*

Elasticsearch 依赖 `AWSServiceRoleForAmazonElasticsearchService` 服务关联角色。该角色在账号内唯一且全员共享，已存在时重复创建会报错。

可在[这里](https://console.aws.amazon.com/iam/home#roles)检查角色是否已创建；未创建则将本项设为 true。

### SnapshotRepository

*string*

若部署包含 Elasticsearch 且数据库此前已建过索引，需要同时提供快照所在仓库名与快照名。`SnapshotRepository` 即仓库名。

按 [Elasticsearch 的定义](https://www.elastic.co/guide/en/elasticsearch/reference/current/snapshot-restore.html#snapshot-workflow)，*快照仓库*是存放 Elasticsearch 集群快照的存储位置。Elasticsearch 支持多种仓库类型，但压测工具当前仅支持 S3 桶。

### SnapshotName

*string*

部署已建索引的数据库时需要同时提供仓库名与快照名，`SnapshotName` 即快照本身的名字。

### RestoreTimeoutMinutes

*int*

等待 Elasticsearch 快照恢复完成的最长时间（分钟）。默认 45 分钟。

### ClusterTimeoutMinutes

*int*

快照恢复后，等待 Elasticsearch 集群状态转为 green 的最长时间（分钟）。默认 45 分钟。

### ZoneAwarenessEnabled

*bool*

是否为 Elasticsearch 集群启用多可用区感知。设为 `true` 时需同时设置 `ZoneAwarenessAZCount`。

详见[文档](https://aws.amazon.com/blogs/big-data/increase-availability-for-amazon-opensearch-service-by-deploying-in-three-availability-zones/)。

### ZoneAwarenessAZCount

*int*

Elasticsearch 集群使用的可用区数量。仅在 `ZoneAwarenessEnabled` 为 `true` 时生效。

详见[文档](https://aws.amazon.com/blogs/big-data/increase-availability-for-amazon-opensearch-service-by-deploying-in-three-availability-zones/)。

### EnableCloudwatchLogs

*bool*（默认：`true`）

是否为 Elasticsearch 集群启用 Cloudwatch 日志。

详见[文档](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/AnalyzingLogData.html)。

## JobServerSettings

### InstanceCount

*int*

要创建的实例数量。仅支持 `1` 或 `0`。部署 job server 后，所有周期性任务都运行在该实例上。

### InstanceType

*string*

Job Server 的 EC2 实例类型，见[实例类型表](https://aws.amazon.com/ec2/instance-types/)。

## EnableAgentFullLogs

*bool*

是否把 agent 服务的命令输出（`stdout` 与 `stderr`）记录到 home 目录。

## ProxyInstanceCount

*int*

运行的代理实例数量。当前仅允许 `0` 和 `1`。详见[这条 FAQ](../faq.md)。

## ProxyInstanceType

*string*

代理服务器的 EC2 实例类型，见[实例类型表](https://aws.amazon.com/ec2/instance-types/)。

## ProxyAllocatePublicIPAddress

*bool*

是否为代理实例分配公网 IP。

## SSHPublicKey

*string*

SSH 公钥路径，用于建立到 AWS 实例的 SSH 连接。

## TerraformDBSettings

### InstanceCount

*int*

数据库实例数量。

### InstanceType

*string*

数据库实例类型，见[类型表](https://aws.amazon.com/rds/instance-types/)。

### InstanceEngine

*string*

数据库后端类型：`aurora-mysql` 或 `aurora-postgresql`。

### UserName

*string*

连接数据库的用户名。

### Password

*string*

连接数据库的密码。

### EnablePerformanceInsights

*bool*

为 true 时为创建的数据库实例启用 Performance Insights。

### ClusterIdentifier

*string*

要挂接的既有集群名。设置后 `DBDumpURI` 不再生效。该值应为已恢复的 AWS Aurora 备份集群。

### DBName

*string*

数据库名。设计上与 `ClusterIdentifier` 搭配使用，取值即为该集群中的数据库名。

也可单独使用，硬编码新建数据库的名称。未指定 `DBName` 且为部署新建数据库时，库名默认为 `${ClusterName}db`。

### DBParameters

*[]DBParameter*

为创建的实例设置的数据库参数列表。
各值详见 [AWS 文档](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_WorkingWithParamGroups.html)。

示例：

```
    "DBParameters": [
      {
        "Name": "innodb_buffer_pool_size",
        "Value": "2147483648",
        "ApplyMethod": "pending-reboot"
      }
    ]
```

## ExternalDBSettings（外部数据库）

### DriverName

*string*

访问外部数据库使用的 Mattermost 驱动名。

### DataSource

*string*

外部数据库的 DSN。

### DataSourceReplicas

*[]string*

外部数据库只读副本的 DSN 列表。

### DataSourceSearchReplicas

*[]string*

外部数据库搜索副本的 DSN 列表。

### ClusterIdentifier

*string*

既有数据库集群的 ClusterIdentifier。

## ExternalBucketSettings（外部对象存储）

### AmazonS3AccessKeyId

*string*

外部桶的 access key id。

### AmazonS3SecretAccessKey

*string*

外部桶的 secret access key。

### AmazonS3Bucket

*string*

桶名。

### AmazonS3PathPrefix

*string*

路径前缀。

### AmazonS3Region

*string*

AWS 区域。

### AmazonS3Endpoint

*string*

S3 endpoint。

### AmazonS3SSL

*bool*

是否使用 SSL。

### AmazonS3SignV2

*bool*

签名是否使用 v2 协议。

### AmazonS3SSE

*bool*

是否使用 SSE（服务端加密）。

## MattermostDownloadURL

*string*

Mattermost 发布包的下载地址，支持三种用法：
1. URL：指向包含 Mattermost Enterprise 构建的 `*.tar.gz` 文件。获取最新发布版可用 `https://latest.mattermost.com/mattermost-enterprise-linux`。若要测试某个 PR 的改动，CI 会生成对应文件，地址形如 https://pr-builds.mattermost.com/mattermost-platform/commit/$COMMIT_SHA/mattermost-enterprise-linux-amd64.tar.gz，其中 `$COMMIT_SHA` 为待测 commit 的完整 SHA。
2. 指向本地单个二进制文件的 `file://` URI：deployer 会使用最新 Mattermost 发布版，并用该 URI 指向的二进制替换其中的服务端二进制。注意此时 webapp 来自最新发布版，可能与所用二进制不匹配。该用法主要为向后兼容保留。
3. 指向本地 `*.tar.gz` 文件的 `file://` URI：行为与 URL 相同——文件应为 Mattermost Enterprise 构建包。适合不开 PR 直接测试本地改动。

## MattermostLicenseFile

*string*

Mattermost 企业版 license 文件的路径。

## MattermostConfigPatchFile

*string*

可选：Mattermost 部分配置文件的路径，部署 app server 时作为 patch 应用。

## MattermostPlugins

*map[string]string*

插件标识到插件包（`.tar.gz`）获取方式的映射。取值可以是：
- URL：插件包由 app 节点从该 URL 下载。
- `file://` 前缀路径：插件包从你的本地文件系统上传到 app 节点。

## AdminEmail

*string*

部署过程中创建 sysadmin 用户时使用的邮箱。

## AdminUsername

*string*

部署过程中创建 sysadmin 用户时使用的用户名。

## AdminPassword

*string*

部署过程中创建 sysadmin 用户时使用的密码。

## LoadTestDownloadURL

*string*

load-test-ng 二进制的下载地址。如需用自定义版本的 load-test-ng 跑压测，也可以用 `file://` 前缀指向本地打包文件。两种方式下，包内的配置文件都会在部署过程中被更新。

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

## Report（报告）

### Label

*string*

用于过滤 Prometheus 查询的标签。

### GraphQueries

*[]GraphQuery*

GraphQuery 定义了对 Prometheus 实例执行的查询，用于为报告采集数据。

#### Name

*string*

图表的友好名称。

#### Query

*string*

要执行的 Prometheus 查询。

## TerraformStateDir

*string*

Terraform 相关文件的存放目录。目录不存在时，在第一个需要它的命令执行时自动创建，默认为 `/var/lib/mattermost-load-test-ng`。创建该目录需要 root 权限，因此你可能想把它改为 `/home/youruser/.loadtest` 之类的路径。

## S3BucketDumpURI

*string*

指向 S3 桶的 URI，形如 `s3://bucket-name/optional-subdir`。
该桶内容会通过 `aws s3 cp` 拷贝到部署中创建的桶里。该命令在本地执行，因此需要本机已安装 AWS CLI。
若部署中未创建桶（见 [`AppInstanceCount`](#appinstancecount)），该值被忽略。
若部署中创建了桶但该值为空，新桶不会预置任何数据。

## DBDumpURI

*string*

可选：运行压测前要加载的 Mattermost 数据库 dump 文件 URI。
文件应为 gzip 压缩格式。
以 `file://` 前缀指向本地文件时，dump 文件会被上传到 app server。

只有 terraform 创建的数据库才能自动加载 dump；
如果使用 [`ExternalDBSettings`](#externaldbsettings外部数据库) 接入既有数据库，需要手动加载。

## SiteURL

*string*

用于两个目的的主机名：
- 覆盖服务端的 site URL。
- 在 app 节点的 /etc/hosts 中写入一条记录，指向代理的内网 IP；无代理时指向当前 app 节点。

该配置用于需要加载含 permalink 的既有数据库 dump 的测试：permalink 指向特定主机名，不做此设置时新部署的节点不认识该主机名，permalink 无法解析。

## ServerURL

*string*

agent 客户端连接 Mattermost 服务端所用的 URL。当 Mattermost 前面有代理时，用于覆盖 agent 配置中的 server URL。

> [!WARNING]
> **仅限自定义部署**：正常情况下不需要该设置。除非你清楚自己在做什么，否则不要修改。

## ServerScheme

*string*（默认 `http`）

agent 客户端连接 Mattermost 服务端所用的 URL scheme（http/https），用于覆盖 agent 配置中的 server scheme。

> [!WARNING]
> **仅限自定义部署**：正常情况下不需要该设置。除非你清楚自己在做什么，否则不要修改。

## UsersFilePath

*string*

供 controller 使用的凭据文件路径。设置后会自动上传到各 agent，并覆盖 agent 配置中自己的 [`UsersFilePath`](config.md#usersfilepath)。

## EnableNetPeekMetrics

*bool*

为 true 时通过 [netpeek](https://github.com/streamer45/netpeek) 工具采集细粒度网络指标。

> [!WARNING]
> 这些指标在高流量部署上会带来不可忽视的计算开销。

## PyroscopeSettings（持续性能剖析）

### EnableAppProfiling

*bool*

对所有 app 实例启用持续 profiling。

### EnableAgentProfiling

*bool*

对所有 agent 实例启用持续 profiling。

### BlockProfileRate

*int*

设置 pprof block profile rate。该值同时作用于 agent 与 Mattermost 服务端进程。

## CustomTags

*map[string]string*

可选的键值对映射，用于给 AWS 中部署的全部资源打标签。标签的说明与最佳实践见 [AWS 文档](https://docs.aws.amazon.com/whitepapers/latest/tagging-best-practices/what-are-tags.html)。
