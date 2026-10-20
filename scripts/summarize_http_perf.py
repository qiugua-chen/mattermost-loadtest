#!/usr/bin/env python3
# =============================================================================
# summarize_http_perf.py —— nginx-perf 访问日志的 HTTP 性能汇总器
#
# 解析 nginx-perf 竖线分隔格式的 access log，输出：
#   请求总量、吞吐量(QPS)、响应码分布、request_time / upstream_time 的
#   分位数（P50/P90/P95/P99/Max），支持按 URI 路径 + HTTP 方法过滤。
# 通常由 summarize_http_perf.sh 包装调用，也可独立使用：
#   python3 summarize_http_perf.py <access.log> <窗口秒数> [URI过滤] [方法过滤]
# =============================================================================

import collections
import datetime
import math
import os
import sys


def percentile(values, rate):
    if not values:
        return 0.0

    ordered = sorted(values)
    position = math.ceil(len(ordered) * rate)
    position = max(1, min(position, len(ordered)))

    return ordered[position - 1]


def local_time(epoch):
    if epoch is None:
        return ""

    return datetime.datetime.fromtimestamp(
        epoch
    ).astimezone().isoformat(timespec="seconds")


def main():
    if len(sys.argv) < 2:
        print("ERROR：缺少Nginx访问日志参数")
        return

    access_log = sys.argv[1]

    planned_duration = 0.0

    if len(sys.argv) >= 3 and sys.argv[2]:
        try:
            planned_duration = float(sys.argv[2])
        except ValueError:
            planned_duration = 0.0

    uri_filter = sys.argv[3] if len(sys.argv) >= 4 else ""
    method_filter = sys.argv[4].upper() if len(sys.argv) >= 5 else ""

    if not os.path.isfile(access_log):
        print(f"ERROR：访问日志不存在：{access_log}")
        return

    rows = []
    status_counts = collections.Counter()
    requests_per_second = collections.Counter()

    malformed_lines = 0

    with open(
        access_log,
        "r",
        encoding="utf-8",
        errors="replace"
    ) as file:
        for raw_line in file:
            fields = raw_line.rstrip("\n").split("|")

            if len(fields) < 7:
                malformed_lines += 1
                continue

            try:
                epoch = float(fields[0])
                method = fields[1].upper()
                uri = fields[2]
                status = int(fields[3])
                request_time = float(fields[4])
                response_bytes = int(fields[6])
            except ValueError:
                malformed_lines += 1
                continue

            request_path = uri.split("?", 1)[0]

            if uri_filter and request_path != uri_filter:
                continue

            if method_filter and method != method_filter:
                continue

            rows.append(
                {
                    "epoch": epoch,
                    "method": method,
                    "uri": uri,
                    "status": status,
                    "request_time": request_time,
                    "response_bytes": response_bytes,
                }
            )

            status_counts[status] += 1
            requests_per_second[int(epoch)] += 1

    total = len(rows)

    success = sum(
        1
        for row in rows
        if 200 <= row["status"] < 400
    )

    status_101 = sum(
        1
        for row in rows
        if row["status"] == 101
    )

    errors_4xx = sum(
        1
        for row in rows
        if 400 <= row["status"] < 500
    )

    errors_5xx = sum(
        1
        for row in rows
        if 500 <= row["status"] < 600
    )

    other_status = total - success - errors_4xx - errors_5xx

    # WebSocket 101的request_time是连接持续时间，
    # 不能纳入普通HTTP接口响应时间百分位。
    latency_values = [
        row["request_time"]
        for row in rows
        if row["status"] != 101
    ]

    first_epoch = (
        min(row["epoch"] for row in rows)
        if rows
        else None
    )

    last_epoch = (
        max(row["epoch"] for row in rows)
        if rows
        else None
    )

    actual_window = (
        max(last_epoch - first_epoch, 1.0)
        if first_epoch is not None and last_epoch is not None
        else 0.0
    )

    tps_duration = (
        planned_duration
        if planned_duration > 0
        else actual_window
    )

    average_tps = (
        total / tps_duration
        if tps_duration > 0
        else 0.0
    )

    peak_tps = max(
        requests_per_second.values(),
        default=0
    )

    average_latency = (
        sum(latency_values) / len(latency_values)
        if latency_values
        else 0.0
    )

    minimum_latency = (
        min(latency_values)
        if latency_values
        else 0.0
    )

    maximum_latency = (
        max(latency_values)
        if latency_values
        else 0.0
    )

    total_bytes = sum(
        row["response_bytes"]
        for row in rows
    )

    print("========== HTTP性能汇总 ==========")
    print(f"访问日志={access_log}")
    print(f"URI过滤={uri_filter or '全部'}")
    print(f"方法过滤={method_filter or '全部'}")
    print(f"计划持续时间={planned_duration:.3f}秒")
    print(f"实际请求窗口={actual_window:.3f}秒")
    print(f"首个请求时间={local_time(first_epoch)}")
    print(f"最后请求时间={local_time(last_epoch)}")
    print(f"请求总数={total}")
    print(f"成功请求数={success}")
    print(f"WebSocket_101数量={status_101}")
    print(f"4xx数量={errors_4xx}")
    print(f"5xx数量={errors_5xx}")
    print(f"其他状态数量={other_status}")
    print(
        "成功率="
        f"{(success / total * 100) if total else 0:.4f}%"
    )
    print(
        "错误率="
        f"{((errors_4xx + errors_5xx) / total * 100) if total else 0:.4f}%"
    )
    print(f"平均TPS={average_tps:.3f}")
    print(f"峰值TPS={peak_tps}")
    print(f"响应时间样本数={len(latency_values)}")
    print(f"最小响应时间={minimum_latency:.6f}秒")
    print(f"平均响应时间={average_latency:.6f}秒")
    print(f"P90={percentile(latency_values, 0.90):.6f}秒")
    print(f"P95={percentile(latency_values, 0.95):.6f}秒")
    print(f"P99={percentile(latency_values, 0.99):.6f}秒")
    print(f"最大响应时间={maximum_latency:.6f}秒")
    print(f"响应总字节数={total_bytes}")
    print(f"异常格式日志行数={malformed_lines}")

    print("========== HTTP状态码分布 ==========")

    for status in sorted(status_counts):
        print(f"HTTP_{status}={status_counts[status]}")


if __name__ == "__main__":
    main()
