#!/usr/bin/env bash
set -euo pipefail

file="${1:?用法：summarize_receiver_latency.sh receiver-latency.csv}"

if [ ! -s "$file" ]; then
  echo "receiver latency file is empty: $file"
  exit 1
fi

awk -F',' '
NR > 1 && $5 != "" {
  n++
  a[n]=$5
  sum+=$5
  if (min=="" || $5 < min) min=$5
  if ($5 > max) max=$5
}
END {
  if (n == 0) {
    print "无接收延迟样本"
    exit
  }

  asort(a)

  p50_idx=int(n*0.50); if (p50_idx < 1) p50_idx=1
  p90_idx=int(n*0.90); if (p90_idx < 1) p90_idx=1
  p95_idx=int(n*0.95); if (p95_idx < 1) p95_idx=1
  p99_idx=int(n*0.99); if (p99_idx < 1) p99_idx=1

  print "========== 频道群接收延迟汇总 =========="
  print "文件=" FILENAME
  print "样本数=" n
  printf "平均接收延迟=%.3f ms\n", sum/n
  printf "最小接收延迟=%.3f ms\n", min
  printf "P50=%.3f ms\n", a[p50_idx]
  printf "P90=%.3f ms\n", a[p90_idx]
  printf "P95=%.3f ms\n", a[p95_idx]
  printf "P99=%.3f ms\n", a[p99_idx]
  printf "最大接收延迟=%.3f ms\n", max
}
' "$file"
