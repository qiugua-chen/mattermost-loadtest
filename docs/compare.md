# 负载测试结果对比


## 前置条件

测试报告包含两类数据：
- 整个测试期间取平均的 API 与 store（存储层）耗时。
- 用户自定义查询的细粒度数据，通过 [`deployer.json` 中的 Report 配置段](config/deployer.md#report)定义。

这些数据点在**生成报告的时刻**从 Prometheus 取回，但 Prometheus 必须在**测试运行期间**就已在抓取指标。Terraform 部署会自动提供运行着 Prometheus 的 metrics 实例；本地部署则视情况而定：
- 如果你用 `mattermost` 仓库中的 docker-compose 起本地开发环境（即 `make run` 或 `make run-server`），`localhost:9090` 上应已有 Prometheus 在运行。
- 如果你用其他方式在本地运行 Mattermost，可能需要[手动安装并运行 Prometheus](https://prometheus.io/docs/introduction/first_steps/)。确保配置了抓取本地 Mattermost 指标（通常在 `localhost:8067`）和 [node exporter](https://prometheus.io/docs/guides/node-exporter/#monitoring-linux-host-metrics-with-the-node-exporter)（通常在 `localhost:9100`）。

## 生成报告

负载测试完成后，可以生成报告并存为文件，用于不同轮次之间的对比：

```sh
go run ./cmd/ltctl report generate --output=base.out --label=base "2020-06-23 07:23:35" "2020-06-23 07:33:35"
```

注意：本地部署时需要设置 `--prometheus-url` 参数指向存有指标的 Prometheus 服务（通常是 `http://localhost:9090`）。若不设置，工具会假定存在 Terraform 部署并尝试连接其 Prometheus。

时间戳**必须**使用 UTC，表示报告取数的时间范围。建议把范围框在压测用户数稳定的区间内，避开爬坡阶段或增删用户的不稳定阶段，以保证不同轮次结果的一致性。

不同压测轮次的时间范围可以不同，对比时以基准（base）报告为准：若某报告的数据点多于基准，多余部分会被忽略。

工具不会对时间范围做归一化压缩——这交给 Prometheus 查询处理；数据点只是按序绘制在图表上进行对比。

## 对比报告

对比两份负载测试报告：

```sh
go run ./cmd/ltctl report compare base.out new.out --output=results.txt --graph
```

`results.txt` 是 Markdown 格式的表格，对比 store 与 API 指标的平均值和 P99 耗时。附加的 `--graph` 参数可生成 CPU、内存等指标的对比图（需要系统已安装 `gnuplot`）。

#### 说明

Markdown 输出开头是"劣化/改善调用"的排序摘要。该清单自动剔除差值（绝对值）小于 2ms 且差异百分比（绝对值）小于 1% 的调用。

## 负载测试对比的最佳实践

- 始终使用相同的集群配置来对比不同测试。
- 用户数应取到能让平均耗时至少达到 10ms 的规模，再用该用户数做对比。低于此规模说明数据库压力不足，会给结果引入大量噪声。
- 开始新一轮测试前，建议重置数据库并重新初始化，使每轮从完全相同的状态开始：

```sh
go run ./cmd/ltctl loadtest reset
```
