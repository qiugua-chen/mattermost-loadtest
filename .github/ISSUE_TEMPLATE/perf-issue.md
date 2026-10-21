---
name: 压测问题上报
about: 执行 S00~S03B 场景压测时遇到的失败或数据异常
title: "[压测] "
labels: perf-issue
---

## 场景与规模

- 场景编号（S00 注册 / S01 登录 / S02 单聊 / S03A GM / S03B 频道大群）：
- 并发用户数 / 持续时间：
- ltagent 与被测 Mattermost 版本：

## 现象

<!-- 看到了什么？错误日志、指标异常、未生成预期文件等 -->

## 已自查项（请在 [x] 内打勾）

- [ ] 已对照 docs/TROUBLESHOOTING.md 排查
- [ ] 被测端已关闭/放宽内置速率限制
- [ ] `bin/ltagent` 为最新代码重新编译
- [ ] 用户文件行数 ≥ 本轮并发数，且 S03B 已确认全员入频道

## 附件

<!-- ltagent.log 收敛后片段、curl 127.0.0.1:4001/metrics 输出、
     summarize_* 汇总结果等，注意脱敏（隐去真实 IP/域名/账号） -->
