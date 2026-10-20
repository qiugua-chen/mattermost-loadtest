# Comparison 配置（comparison.json）

> 使用流程见[自动化负载对比测试](../comparison.md)。

## BaseBuild（基准构建）

*BuildConfig*

### Label

*string*

标识该构建的标签。

### URL

*string*

构建发布包的下载地址。以 `file://` 前缀指向本地文件时，构建文件会被上传到 app server。

## NewBuild（待对比构建）

*BuildConfig*

### Label

*string*

标识该构建的标签。

### URL

*string*

构建发布包的下载地址。以 `file://` 前缀指向本地文件时，构建文件会被上传到 app server。

## LoadTests（负载测试列表）

*[]LoadTestConfig*

### Type

*LoadTestType*

要运行的负载测试类型。

可选值：
- `"bounded"`（定界：固定用户数）
- `"unbounded"`（非定界：自动探测容量上限）

### DBEngine

*DatabaseEngine*

app server 使用的数据库引擎。

可选值：
- `"mysql"`
- `"postgresql"`

### DBDumpURL

*string*

可选：运行压测前要加载的 Mattermost 数据库 dump 文件地址。
文件应为 gzip 压缩格式。以 `file://` 前缀指向本地文件时，dump 文件会被上传到 app server。

## S3BucketDumpURI

*string*

可选：S3 桶 URI（形如 `s3://bucket-name/optional-subdir`），其内容会在压测前拷贝到部署的桶中。
该值的详细说明见 [deployer 配置中的同名设置](deployer.md#s3bucketdumpuri)。

### NumUsers

*int*

运行的用户数。仅当 `Type` 为 `"bounded"` 时生效。

### Duration

*string*

负载测试时长。仅当 `Type` 为 `"bounded"` 时生效。

## Output（输出配置）

*OutputConfig*

### UploadDashboard

*bool*

是否生成并上传对比用的 Grafana 仪表盘。

### GenerateReport

*bool*

是否在对比结束后生成 Markdown 报告。

### GenerateGraphs

*bool*

是否在对比结束后生成 gnuplot 图表。

### GraphsPath

*string*

可选：图表的输出路径。
