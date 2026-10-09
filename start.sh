#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
command -v go >/dev/null || { echo '請安裝 Go 1.21+'; exit 1; }
command -v npm >/dev/null || { echo '請安裝 Node.js 與 npm'; exit 1; }
(cd frontend && npm ci)
(cd backend && go build -o .draw-server .)
(cd backend && exec ./.draw-server) &
backend_pid=$!
cleanup() { kill "$backend_pid" "${frontend_pid:-}" 2>/dev/null || true; }
trap cleanup EXIT INT TERM
(cd frontend && exec npm run dev) &
frontend_pid=$!
echo '你畫我猜 白板：http://localhost:3346；手機請連電腦的區網 IP :3346'
wait "$backend_pid" "$frontend_pid"
