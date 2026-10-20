#!/usr/bin/env python3
"""Summarize the steady-state window of one S02 Direct Channel run.

Usage: python3 S02_direct_window_report.py RUN_DIR START_EPOCH [DURATION] [USERS] [DRAIN_SECONDS]
START_EPOCH must be recorded after all users have authenticated and before
CreateDirectPost is released. The access log format is the existing nginx-perf
pipe-delimited format (timestamp|method|uri|status|...|request_time|upstream_time|bytes).
"""

import csv
import math
import sys
from collections import Counter
from datetime import datetime
from pathlib import Path


def percentile(values, percentage):
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * percentage / 100) - 1)]


def main():
    if len(sys.argv) not in (3, 4, 5, 6):
        print(__doc__.strip(), file=sys.stderr)
        return 2
    directory = Path(sys.argv[1])
    start = int(sys.argv[2])
    duration = int(sys.argv[3]) if len(sys.argv) > 3 else 300
    users = int(sys.argv[4]) if len(sys.argv) > 4 else 100
    drain = int(sys.argv[5]) if len(sys.argv) > 5 else 0
    if duration <= 0 or users <= 0 or drain < 0:
        raise ValueError("DURATION and USERS must be positive; DRAIN_SECONDS must be nonnegative")
    end = start + duration
    status_path = directory / "ltagent-status.csv"
    access_path = directory / "http-access.log"
    if not status_path.is_file() or not access_path.is_file():
        print("INVALID: status or HTTP access log is missing", file=sys.stderr)
        return 2

    with status_path.open(newline="", encoding="utf-8") as stream:
        samples = list(csv.DictReader(stream))
    needed = {"epoch", "ws_connections", "ws_posted_events", "ws_missed_events",
              "loadtest_errors", "http_errors", "http_timeouts"}
    if not samples or not needed.issubset(samples[0]):
        print("INVALID: status CSV has missing columns or no samples", file=sys.stderr)
        return 2
    baseline = [s for s in samples if int(s["epoch"]) <= start]
    finish = [s for s in samples if int(s["epoch"]) <= end]
    delivery_finish = [s for s in samples if int(s["epoch"]) <= end + drain]
    interval = [s for s in samples if start <= int(s["epoch"]) <= end]
    reasons = []
    if not baseline or not finish or not delivery_finish or not interval:
        print("INVALID: no status samples covering the selected window", file=sys.stderr)
        return 2
    before, after = baseline[-1], finish[-1]
    delivered = delivery_finish[-1]
    if start - int(before["epoch"]) > 3 or end - int(after["epoch"]) > 3:
        reasons.append("status sampling does not cover the full measurement window")
    if end + drain - int(delivered["epoch"]) > 3:
        reasons.append("status sampling does not cover the delivery drain")
    timestamps = [int(s["epoch"]) for s in interval]
    if any(b - a > 5 for a, b in zip(timestamps, timestamps[1:])):
        reasons.append("status sampling has a gap greater than 5 seconds")
    min_ws = min(int(s["ws_connections"]) for s in interval)
    if min_ws < users:
        reasons.append(f"WebSocket connections fell below {users} (minimum {min_ws})")

    deltas = {}
    for key in needed - {"epoch", "ws_connections"}:
        deltas[key] = int(after[key]) - int(before[key])
        if deltas[key] < 0:
            reasons.append(f"counter {key} reset within the window")

    requests = 0
    posts = 0
    failures = 0
    logins = 0
    response_times = []
    seconds = Counter()
    status_codes = Counter()
    with access_path.open(encoding="utf-8", errors="replace") as stream:
        for line in stream:
            fields = line.rstrip("\n").split("|")
            if len(fields) < 7:
                continue
            try:
                moment = datetime.fromisoformat(fields[0]).timestamp()
            except ValueError:
                continue
            if not start <= moment < end:
                continue
            requests += 1
            method, uri, status = fields[1:4]
            if method == "POST" and uri == "/api/v4/users/login":
                logins += 1
            if method == "POST" and uri == "/api/v4/posts":
                status_codes[status] += 1
                if status == "201":
                    posts += 1
                    seconds[int(moment)] += 1
                    try:
                        response_times.append(float(fields[-3]))
                    except ValueError:
                        reasons.append("a successful post lacks request_time")
                else:
                    failures += 1

    if posts == 0:
        reasons.append("no successful posts in the window")
    if failures:
        reasons.append(f"{failures} post requests did not return HTTP 201")
    if logins:
        reasons.append(f"{logins} login requests occurred in the measurement window")
    for key in ("loadtest_errors", "http_errors", "http_timeouts", "ws_missed_events"):
        if deltas[key] != 0:
            reasons.append(f"{key} delta is {deltas[key]}")
    expected_events = posts * 2
    received_events = int(delivered["ws_posted_events"]) - int(before["ws_posted_events"])
    missed_events = int(delivered["ws_missed_events"]) - int(before["ws_missed_events"])
    if missed_events != 0:
        reasons.append(f"missed WebSocket events through drain are {missed_events}")
    if received_events != expected_events:
        reasons.append(f"posted events {received_events} differ from expected {expected_events}")

    print(f"窗口=[{start},{end}) 持续={duration}s 目标用户={users} 事件排空={drain}s")
    print(f"窗口总HTTP请求={requests} 登录请求={logins}")
    print(f"消息HTTP状态分布={dict(sorted(status_codes.items()))}")
    print(f"消息发送成功={posts} 失败={failures} 平均TPS={posts / duration:.3f} "
          f"峰值每秒请求={max(seconds.values(), default=0)}")
    if response_times:
        print(f"成功消息响应时间：平均={sum(response_times)/len(response_times):.3f}s "
              f"P95={percentile(response_times,95):.3f}s "
              f"P99={percentile(response_times,99):.3f}s")
    print(f"WebSocket最低连接数={min_ws} posted事件增量={received_events} "
          f"预期事件数={expected_events}")
    print("状态计数增量=" + ", ".join(f"{k}:{v}" for k, v in sorted(deltas.items())))
    if reasons:
        print("结果=INVALID")
        for reason in sorted(set(reasons)):
            print("原因=" + reason)
        return 1
    print("结果=数据校验通过（仍需核对配对与目标服务器监控）")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, KeyError) as exc:
        print(f"INVALID: {exc}", file=sys.stderr)
        raise SystemExit(2)
