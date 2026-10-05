#!/usr/bin/env bash
#
# 开发期一键起服务：ai-service(:8000) → backend(:8080) → frontend(:5173)
#
# 为什么要这个文件
#   三条启动命令的工作目录和解释器都不同（go run / .venv 里的 python / npm），
#   最容易抄错的就是"在哪个目录跑"和"用哪个 python"。把这两样钉死在脚本里。
#
# 用法
#   bash scripts/dev.sh                 三个都起
#   bash scripts/dev.sh ai              只起 ai-service
#   bash scripts/dev.sh backend         只起 Go 后端
#   bash scripts/dev.sh frontend        只起 Vite
#   bash scripts/dev.sh ai backend      挑几个也行
#   bash scripts/dev.sh --help
#
# 停止
#   Ctrl+C 一次全停。按 Windows 进程树杀（taskkill /T）——
#   只杀父进程不够：go run 会再起一个临时编译出的二进制、npm run dev 会再起 node，
#   父进程死了子进程照样占着端口，下次启动就报"端口已被占用"。
#
# 这个脚本不做什么
#   - 不启动 PostgreSQL：原生 PG 是 Windows 服务、开机自启，这里只探端口。
#   - 不装依赖：缺 node_modules / .venv 直接报错退出。装依赖会动 lock 文件，
#     那是要单独 review 的改动，不该藏在"起服务"里顺手做掉。
#   - 不改 .env：缺了只提示你从 .env.example 复制。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 端口要和 .env 对得上：起 ai-service 的端口必须等于 backend/.env 里的 AI_SERVICE_URL，
# 起 backend 的端口必须等于 backend/.env 里的 SERVER_PORT。改了一处记得改另一处。
PORT_AI=8000
PORT_BACKEND=8080
PORT_FRONTEND=5173

step() { printf '\033[36m==>\033[0m %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
warn() { printf '\033[33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------- 端口检查

# 返回占用该端口的 PID；端口空闲则返回空串。
#
# 为什么不用 /dev/tcp（bash 内建那个更干净的写法）：它连的是 127.0.0.1，
# 也就是 IPv4。而 vite 在 Windows 上默认只监听 IPv6 的 [::1] —— 实测 /dev/tcp
# 报"空闲"、vite 其实好好跑着，检查形同虚设。netstat 把 IPv4 和 IPv6 一起列。
# 表头是 GBK，在 Git Bash 里显示成乱码，但 LISTENING 是 ASCII，grep 它不受影响。
#
# 末尾的 || true：grep 没匹配到会返回 1，配上 set -o pipefail 会让整个管道失败，
# 再配上 set -e 就会把脚本直接干掉 —— 而"端口空闲"恰恰是最常见的情况。
port_pid() {
  netstat -ano 2>/dev/null \
    | grep LISTENING \
    | grep -E ":$1[[:space:]]" \
    | awk '{print $NF}' \
    | head -1 || true
}

require_port_free() {
  local port="$1" name="$2" pid
  pid="$(port_pid "$port")"
  if [ -n "$pid" ]; then
    warn "$name 要用的 $port 已被占用（PID $pid）"
    warn "  清掉它：taskkill //F //T //PID $pid"
    die "$name 起不来。先停掉上面那个进程，或者改端口。"
  fi
}

# ---------------------------------------------------------------- 进程管理

STARTED=()

# 按 Windows 进程树杀。Git Bash 用 /proc/<pid>/winpid 把 MSYS 的 PID 映射到
# Windows 的 PID（本机实测可用），taskkill //T 再连子进程一起收。
# 注意 //F //T 是双斜杠：Git Bash 会把单斜杠当路径去转换，写成 /F 会失效。
kill_tree() {
  local name="$1" pid="$2" winpid=""
  [ -n "$pid" ] || return 0
  winpid="$(cat "/proc/$pid/winpid" 2>/dev/null || true)"
  if [ -n "$winpid" ] && command -v taskkill >/dev/null 2>&1; then
    taskkill //F //T //PID "$winpid" >/dev/null 2>&1 || true   # 输出是 GBK，乱码，丢掉
  else
    kill "$pid" 2>/dev/null || true
  fi
  note "已停止 $name"
}

cleanup() {
  local code=$?
  if [ "${#STARTED[@]}" -gt 0 ]; then
    printf '\n'
    step "停止服务…"
    local entry
    for entry in "${STARTED[@]}"; do
      kill_tree "${entry%%:*}" "${entry##*:}"
    done
    STARTED=()
  fi
  return "$code"
}

# EXIT 管"正常结束 / 报错退出"这一路，INT 是 Ctrl+C。不在 INT/TERM 里直接
# 调 cleanup，而是 exit 出去让 EXIT 接 —— 两个 trap 都挂 cleanup 会跑两遍。
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 三个服务共用这一个启动器，目录和命令由调用处给：三者启动方式本来就不一样，
# 硬套同一个模板反而更难读。
launch() {
  local name="$1" dir="$2"
  shift 2
  step "启动 $name …"
  # 每行加 [name] 前缀，三个服务挤在一个终端里才分得清谁在说话。
  # sed -u 关掉自己的行缓冲，否则会把实时日志攒成块再吐出来。
  #
  # 已知的副作用：加了管道之后，node 发现 stdout 不是终端，会从行缓冲切成块缓冲，
  # 于是 vite 那几行横幅可能比后端日志晚几秒才出现。Go 和 uvicorn 都写 stderr、
  # 不受影响。看到 frontend 日志排在后面不是卡住了，是 npm 在攒。
  ( cd "$dir" && exec "$@" ) > >(sed -u "s/^/[$name] /") 2>&1 &
  STARTED+=("$name:$!")
}

# ---------------------------------------------------------------- 各服务的前置检查

PY_AI=""
pick_python() {
  if [ -x "$ROOT/ai-service/.venv/Scripts/python.exe" ]; then
    PY_AI="$ROOT/ai-service/.venv/Scripts/python.exe"    # Windows
  elif [ -x "$ROOT/ai-service/.venv/bin/python" ]; then
    PY_AI="$ROOT/ai-service/.venv/bin/python"            # Linux / macOS
  else
    die "找不到 ai-service/.venv。先在 ai-service/ 下建虚拟环境并 pip install -r requirements-dev.txt"
  fi
}

check_ai() {
  require_port_free "$PORT_AI" "ai-service"
  [ -f "$ROOT/ai-service/.env" ] \
    || die "ai-service/.env 不存在：cp ai-service/.env.example ai-service/.env，再填 DEEPSEEK_API_KEY"
}

check_backend() {
  require_port_free "$PORT_BACKEND" "backend"
  [ -f "$ROOT/backend/.env" ] \
    || die "backend/.env 不存在：cp backend/.env.example backend/.env"
}

check_frontend() {
  require_port_free "$PORT_FRONTEND" "frontend"
  [ -f "$ROOT/frontend/.env" ] \
    || die "frontend/.env 不存在：cp frontend/.env.example frontend/.env"
  [ -d "$ROOT/frontend/node_modules" ] \
    || die "frontend/node_modules 不存在：先在 frontend/ 下 npm install"
}

# ---------------------------------------------------------------- 入口

usage() {
  cat <<'EOF'
用法：bash scripts/dev.sh [服务名 ...]

  服务名：ai | backend | frontend | all
  不带参数 = all（三个都起）

  bash scripts/dev.sh             三个都起
  bash scripts/dev.sh ai          只起 ai-service
  bash scripts/dev.sh ai backend  挑几个

Ctrl+C 一次停掉全部（含子进程）。
EOF
}

targets=()
if [ "$#" -eq 0 ]; then
  targets=(ai backend frontend)
else
  for arg in "$@"; do
    case "$arg" in
      all)       targets=(ai backend frontend) ;;
      ai)        targets+=(ai) ;;
      backend)   targets+=(backend) ;;
      frontend)  targets+=(frontend) ;;
      -h|--help) usage; exit 0 ;;
      *)         die "不认识的服务名：$arg（可选 ai / backend / frontend / all）" ;;
    esac
  done
fi

step "仓库根目录 $ROOT"

for t in "${targets[@]}"; do
  case "$t" in
    ai)
      check_ai
      pick_python
      launch ai "$ROOT/ai-service" "$PY_AI" -m uvicorn app.main:app --reload --port "$PORT_AI"
      note "→ http://localhost:$PORT_AI/health"
      ;;
    backend)
      check_backend
      launch backend "$ROOT/backend" go run ./cmd/server
      note "→ http://localhost:$PORT_BACKEND/health"
      ;;
    frontend)
      check_frontend
      launch frontend "$ROOT/frontend" npm run dev
      note "→ http://localhost:$PORT_FRONTEND"
      ;;
  esac
done

printf '\n'
step "已发起 ${#STARTED[@]} 个服务，日志按 [名称] 前缀混在上面"
warn "等各自的启动完成提示出现再打开浏览器。Ctrl+C 一次全停。"

# 等所有子进程，不用 wait -n：某个服务崩了让它单独死掉更好排查，
# 其余几个继续跑，日志里能看到是谁先没的。
wait || true
