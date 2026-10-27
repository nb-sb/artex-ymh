#!/usr/bin/env bash
# 开发模式：后端(:8787) + 流量代理(:8788) 与 前端 next dev(:5173) 一起跑。
# 前端 /api 反代到后端；Ctrl-C 一并退出。
#
# 单二进制（前端内嵌）方式见 README「单二进制」一节，不走这个脚本。
set -euo pipefail
cd "$(dirname "$0")"

# 退出时结束本进程组内的所有子进程（后端 + 前端）。
cleanup() { kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

# 后端监听端口。页面和接口可以改到别的端口；这里只服务本地 next dev。
# 实时流在开发时直连这个端口（dev 反代会缓冲 SSE）。正式运行的单二进制不读它，
# 浏览器跟随自己打开的页面地址。
API_PORT="${ARTEX_API_PORT:-8787}"

# 后端（普通 go run，不内嵌前端）；并发 work agent 数在「系统设置」里配置。
go run ./cmd/artex -addr ":${API_PORT}" -proxy 127.0.0.1:8788 &

# 前端热更新（Next dev，/api 反代到后端）。
# NEXT_PUBLIC_SSE_BASE 只放在这个子进程里。构建内嵌前端时不要带上它，
# 否则这个地址会被写进页面，正式运行又变回固定端口。
(
  cd web &&
    AUTOPENTEST_API="http://127.0.0.1:${API_PORT}" \
    NEXT_PUBLIC_SSE_BASE="http://127.0.0.1:${API_PORT}" \
    npm run dev
) &

echo "[dev] 后端 :${API_PORT} / 代理 :8788 / 前端 http://localhost:5173  (Ctrl-C 退出)"
wait
