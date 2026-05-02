#!/usr/bin/env bash
# setup-mcp.sh — QQ Bot MCP Channel 一键安装脚本
# 用法: ./plugin/scripts/setup-mcp.sh [选项]
#
# 选项:
#   --build              本地编译而非下载预编译二进制
#   --version VERSION    指定下载版本 (默认 latest)
#   --non-interactive    非交互模式，从环境变量读取配置
#   --skip-openid        跳过 openid 探测
#   --dir PATH           指定目标项目目录 (配置 .mcp.json 的位置，默认当前目录)
#   --ide TYPE           指定 IDE: codebuddy|codex|both
set -euo pipefail

# ============================================================
# 常量
# ============================================================
INSTALL_DIR="${HOME}/.local/bin"
QQBOT_CONFIG_DIR="${HOME}/.qqbot"
REPO="holy-tiger/qqbot-go"
OPENID_TIMEOUT=120

# ============================================================
# 参数解析
# ============================================================
OPT_BUILD=false
OPT_VERSION="latest"
OPT_NON_INTERACTIVE=false
OPT_SKIP_OPENID=false
OPT_IDE=""
OPT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)            OPT_BUILD=true; shift ;;
    --version)          OPT_VERSION="$2"; shift 2 ;;
    --non-interactive)  OPT_NON_INTERACTIVE=true; shift ;;
    --skip-openid)      OPT_SKIP_OPENID=true; shift ;;
    --dir)              OPT_DIR="$2"; shift 2 ;;
    --ide)
      OPT_IDE="$2"
      case "$OPT_IDE" in codebuddy|codex|both) ;; *)
        error "无效的 --ide 值: $OPT_IDE (可选: codebuddy|codex|both)"; exit 1 ;; esac
      shift 2 ;;
    -h|--help)
      echo "用法: $0 [选项]"
      echo ""
      echo "选项:"
      echo "  --build              本地编译而非下载预编译二进制"
      echo "  --version VERSION    指定下载版本 (默认 latest)"
      echo "  --non-interactive    非交互模式，从环境变量读取配置"
      echo "  --skip-openid        跳过 openid 探测"
      echo "  --dir PATH           指定目标项目目录 (默认当前目录)"
      echo "  --ide TYPE           指定 IDE: codebuddy|codex|both"
      exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 1 ;;
  esac
done

# ============================================================
# 解析目标项目目录与项目名
# ============================================================
TARGET_DIR="$(cd "${OPT_DIR:-.}" && pwd)"
PROJECT_NAME="$(basename "$TARGET_DIR")"
CONFIG_DIR="${QQBOT_CONFIG_DIR}/configs"
CONFIG_FILE="${CONFIG_DIR}/${PROJECT_NAME}.yaml"
DATA_DIR="${QQBOT_CONFIG_DIR}/data/${PROJECT_NAME}"
DB_FILE="${DATA_DIR}/qqbot.db"
HEALTH_PORT=18800

# ============================================================
# 工具函数
# ============================================================
BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
RESET='\033[0m'

info()  { echo -e "${GREEN}[INFO]${RESET} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${RESET} $*"; }
error() { echo -e "${RED}[ERROR]${RESET} $*" >&2; }

prompt_str() {
  local varname="$1" prompt="$2" default="${3:-}" required="${4:-false}"
  if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
    eval "local val=\${$varname:-\$default}"
    if [[ "$required" == true && -z "$val" ]]; then
      error "非交互模式下 ${varname} 不能为空，请设置环境变量 ${varname}"
      exit 1
    fi
    eval "${varname}=\"\${val}\""
    return
  fi
  local display_default=""
  [[ -n "$default" ]] && display_default=" [${default}]"
  read -r -p "${prompt}${display_default}: " input
  input="${input:-$default}"
  if [[ "$required" == true && -z "$input" ]]; then
    error "此项为必填"
    exit 1
  fi
  eval "${varname}=\"\${input}\""
}

prompt_secret() {
  local varname="$1" prompt="$2" required="${3:-false}"
  if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
    eval "local val=\${$varname:-}"
    if [[ "$required" == true && -z "$val" ]]; then
      error "非交互模式下 ${varname} 不能为空，请设置环境变量 ${varname}"
      exit 1
    fi
    eval "${varname}=\"\${val}\""
    return
  fi
  read -r -s -p "${prompt}: " input
  echo ""
  if [[ "$required" == true && -z "$input" ]]; then
    error "此项为必填"
    exit 1
  fi
  eval "${varname}=\"\${input}\""
}

prompt_yesno() {
  local prompt="$1" default="${2:-y}"
  if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
    return 0
  fi
  local yn
  read -r -p "${prompt} [${default}/N]: " yn
  yn="${yn:-$default}"
  [[ "$yn" =~ ^[Yy] ]]
}

prompt_choice() {
  local varname="$1" prompt="$2"; shift 2
  local options=("$@")
  if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
    eval "${varname}=\"${options[0]}\""
    return
  fi
  echo -e "${BOLD}${prompt}${RESET}"
  local i=1
  for opt in "${options[@]}"; do
    echo "  ${i}) ${opt}"
    i=$((i + 1))
  done
  local choice
  read -r -p "请选择 [1]: " choice
  choice="${choice:-1}"
  if [[ "$choice" -ge 1 && "$choice" -le "${#options[@]}" ]] 2>/dev/null; then
    eval "${varname}=\"${options[$((choice-1))]}\""
  else
    eval "${varname}=\"${options[0]}\""
  fi
}

QQBOT_PID=""
cleanup_bg_process() {
  if [[ -n "${QQBOT_PID:-}" ]] && kill -0 "$QQBOT_PID" 2>/dev/null; then
    kill "$QQBOT_PID" 2>/dev/null || true
    wait "$QQBOT_PID" 2>/dev/null || true
  fi
}

# ============================================================
# 阶段 1: 安装二进制
# ============================================================
phase_install() {
  echo ""
  echo -e "${BOLD}=== 阶段 1/5: 安装二进制 ===${RESET}"

  # 检查是否已安装
  if [[ -x "${INSTALL_DIR}/qqbot" ]]; then
    local existing_ver
    existing_ver=$("${INSTALL_DIR}/qqbot" --version 2>/dev/null || echo "unknown")
    if ! prompt_yesno "qqbot 已安装 (${existing_ver})，是否更新/重新安装?"; then
      info "跳过安装"
      return
    fi
  fi

  local install_method
  if [[ "$OPT_BUILD" == true ]]; then
    install_method="build"
  else
    if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
      install_method="download"
    else
      prompt_choice install_method "选择安装方式:" "下载预编译二进制" "本地编译 (需要 qqbot-go 源码和 Go 环境)"
      case "$install_method" in
        下载*) install_method="download" ;;
        本地*) install_method="build" ;;
      esac
    fi
  fi

  mkdir -p "$INSTALL_DIR"

  case "$install_method" in
    download)
      local version="$OPT_VERSION"
      if [[ "$version" == "latest" ]]; then
        info "查询最新版本..."
        version=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" \
          | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || true)
        if [[ -z "$version" ]]; then
          error "无法获取最新版本，请使用 --version 指定或使用 --build 本地编译"
          exit 1
        fi
      fi
      info "安装 qqbot ${version}..."

      local os arch_suffix
      os=$(uname -s | tr '[:upper:]' '[:lower:]')
      case "$(uname -m)" in
        x86_64|amd64)  arch_suffix="x86_64" ;;
        aarch64|arm64) arch_suffix="aarch64" ;;
        *) error "不支持的架构: $(uname -m)"; exit 1 ;;
      esac

      local archive="qqbot_${os}_${arch_suffix}"
      local url="https://github.com/${REPO}/releases/download/${version}/${archive}.tar.gz"
      local tmpdir
      tmpdir=$(mktemp -d)

      info "下载 ${url}..."
      if ! curl -fSL -o "${tmpdir}/${archive}.tar.gz" "$url"; then
        error "下载失败，请检查版本号或使用 --build 本地编译"
        rm -rf "$tmpdir"
        exit 1
      fi
      tar -xzf "${tmpdir}/${archive}.tar.gz" -C "${tmpdir}"
      cp "${tmpdir}/qqbot" "${INSTALL_DIR}/qqbot"
      cp "${tmpdir}/qqbot-channel" "${INSTALL_DIR}/qqbot-channel"
      chmod +x "${INSTALL_DIR}/qqbot" "${INSTALL_DIR}/qqbot-channel"
      rm -rf "$tmpdir"
      info "已安装到 ${INSTALL_DIR}/qqbot"
      ;;
    build)
      if ! command -v go &>/dev/null; then
        error "未找到 go 命令，请安装 Go 1.24+ 后重试"
        exit 1
      fi
      # 查找 qqbot-go 源码目录
      local src_dir
      src_dir="$(pwd)"
      if [[ ! -f "${src_dir}/go.mod" ]] || ! grep -qF "github.com/openclaw/qqbot" "${src_dir}/go.mod" 2>/dev/null; then
        error "本地编译需要在 qqbot-go 源码目录运行，或使用 --build 从下载模式安装"
        exit 1
      fi
      info "本地编译 qqbot..."
      (cd "$src_dir" && go build -o "${INSTALL_DIR}/qqbot" ./cmd/qqbot)
      (cd "$src_dir" && go build -o "${INSTALL_DIR}/qqbot-channel" ./cmd/qqbot-channel)
      info "编译完成: ${INSTALL_DIR}/qqbot"
      ;;
  esac

  # PATH 检测
  if ! echo ":$PATH:" | grep -q ":${INSTALL_DIR}:"; then
    warn "${INSTALL_DIR} 不在 PATH 中"
    local shell_rc="${HOME}/.bashrc"
    [[ -f "${HOME}/.zshrc" ]] && shell_rc="${HOME}/.zshrc"
    if prompt_yesno "是否将 ${INSTALL_DIR} 添加到 PATH (写入 ${shell_rc})?"; then
      echo "" >> "$shell_rc"
      echo "# Added by qqbot setup-mcp.sh" >> "$shell_rc"
      echo "export PATH=\"\${PATH}:${INSTALL_DIR}\"" >> "$shell_rc"
      export PATH="${PATH}:${INSTALL_DIR}"
      info "已添加到 ${shell_rc}，请重新打开终端或执行 source ${shell_rc}"
    fi
  fi
}

# ============================================================
# 阶段 2: 交互式配置
# ============================================================
phase_config() {
  echo ""
  echo -e "${BOLD}=== 阶段 2/5: 配置 QQ Bot (项目: ${PROJECT_NAME}) ===${RESET}"

  info "配置文件: ${CONFIG_FILE}"
  info "数据目录: ${DATA_DIR}"

  # 如果配置已存在
  if [[ -f "$CONFIG_FILE" ]]; then
    if ! prompt_yesno "配置文件 ${CONFIG_FILE} 已存在，是否覆盖?"; then
      info "保留现有配置"
      return
    fi
  fi

  # 输入核心字段
  local app_id client_secret system_prompt
  if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
    app_id="${QQBOT_APP_ID:-}"
    client_secret="${QQBOT_CLIENT_SECRET:-}"
    system_prompt="${QQBOT_SYSTEM_PROMPT:-You are a helpful assistant.}"
    if [[ -z "$app_id" ]]; then
      error "非交互模式下请设置 QQBOT_APP_ID 环境变量"
      exit 1
    fi
    if [[ -z "$client_secret" ]]; then
      error "非交互模式下请设置 QQBOT_CLIENT_SECRET 环境变量"
      exit 1
    fi
  else
    prompt_str app_id "请输入 QQ Bot appId" "" true
    prompt_secret client_secret "请输入 QQ Bot clientSecret" true
    prompt_str system_prompt "请输入系统提示词" "You are a helpful assistant."
  fi

  # 生成配置
  mkdir -p "$CONFIG_DIR"
  mkdir -p "$DATA_DIR"
  cat > "$CONFIG_FILE" <<YAML
# QQ Bot 配置文件
# 项目: ${PROJECT_NAME}
# 由 setup-mcp.sh 生成

qqbot:
  appId: "${app_id}"
  clientSecret: "${client_secret}"
  enabled: true
  name: "My QQ Bot"
  systemPrompt: "${system_prompt}"
  dmPolicy: "open"
YAML

  info "已生成 ${CONFIG_FILE}"

  # 提示编辑完整配置
  if [[ "$OPT_NON_INTERACTIVE" == false ]]; then
    if prompt_yesno "是否编辑完整配置? (可设置图床、webhook 等选项)"; then
      local editor="${EDITOR:-vi}"
      if command -v "$editor" &>/dev/null; then
        "$editor" "$CONFIG_FILE"
      else
        warn "未找到编辑器 ${editor}，请手动编辑 ${CONFIG_FILE}"
      fi
    fi
  fi
}

# ============================================================
# 阶段 3: 自动探测 openid
# ============================================================
phase_detect_openid() {
  echo ""
  echo -e "${BOLD}=== 阶段 3/5: 探测 OpenID ===${RESET}"

  if [[ "$OPT_SKIP_OPENID" == true ]]; then
    info "跳过 openid 探测 (--skip-openid)"
    return
  fi

  # 检查 sqlite3
  if ! command -v sqlite3 &>/dev/null; then
    warn "未找到 sqlite3 命令，跳过 openid 探测"
    warn "请手动在 QQ 上给机器人发消息后，编辑 ${CONFIG_FILE} 设置 allowFrom"
    return
  fi

  # 检查 qqbot 命令
  if ! command -v qqbot &>/dev/null; then
    warn "未找到 qqbot 命令，跳过 openid 探测"
    return
  fi

  if [[ "$OPT_NON_INTERACTIVE" == false ]]; then
    if ! prompt_yesno "是否现在探测你的 QQ openid? (将临时启动 qqbot 服务)"; then
      info "跳过 openid 探测，dmPolicy 保持 open (允许所有私聊)"
      return
    fi
  fi

  # 启动 qqbot 后台进程
  # 使用项目独立的数据目录，避免与其他实例冲突
  info "临时启动 qqbot (health 端口 :${HEALTH_PORT}, 数据目录 ${DATA_DIR})..."
  QQBOT_PID=""
  trap cleanup_bg_process EXIT
  qqbot -config "$CONFIG_FILE" -health ":${HEALTH_PORT}" &>/dev/null &
  QQBOT_PID=$!

  # 等待进程启动
  local waited=0
  while ! kill -0 "$QQBOT_PID" 2>/dev/null && [[ $waited -lt 5 ]]; do
    sleep 1
    waited=$((waited + 1))
  done

  if ! kill -0 "$QQBOT_PID" 2>/dev/null; then
    error "qqbot 启动失败，请检查配置"
    return
  fi

  echo -e "${YELLOW}>>> 请在 QQ 上给机器人发一条消息 (超时 ${OPENID_TIMEOUT} 秒)...${RESET}"

  # 轮询数据库
  local elapsed=0 openid=""
  while [[ $elapsed -lt $OPENID_TIMEOUT ]]; do
    if [[ -f "$DB_FILE" ]]; then
      openid=$(sqlite3 "$DB_FILE" "SELECT open_id FROM known_users WHERE type='c2c' LIMIT 1;" 2>/dev/null || true)
      if [[ -n "$openid" ]]; then
        break
      fi
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    # 每 10 秒打印进度
    if (( elapsed % 10 == 0 )); then
      info "等待中... (${elapsed}/${OPENID_TIMEOUT}s)"
    fi
  done

  # 停止 qqbot
  cleanup_bg_process
  trap - EXIT

  if [[ -n "$openid" ]]; then
    info "检测到 openid: ${openid}"
    # 更新配置: 修改 dmPolicy，添加 allowFrom
    # 优先使用 python3+yaml，回退到 sed
    local yaml_updated=false
    if command -v python3 &>/dev/null && python3 -c "import yaml" 2>/dev/null; then
      python3 -c "
import yaml
with open('${CONFIG_FILE}', 'r') as f:
    cfg = yaml.safe_load(f)
cfg['qqbot']['dmPolicy'] = 'allowlist'
cfg['qqbot']['allowFrom'] = ['${openid}']
with open('${CONFIG_FILE}', 'w') as f:
    yaml.dump(cfg, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
" 2>/dev/null && yaml_updated=true
    fi
    if [[ "$yaml_updated" == false ]]; then
      # 回退: sed 替换 dmPolicy 并追加 allowFrom
      sed -i.bak 's/dmPolicy: "open"/dmPolicy: "allowlist"/' "$CONFIG_FILE"
      # 在 dmPolicy 行后插入 allowFrom (POSIX 兼容多行插入)
      sed -i.bak '/dmPolicy/a\
  allowFrom:\
    - "'"${openid}"'"' "$CONFIG_FILE"
      rm -f "${CONFIG_FILE}.bak"
    fi
    info "已将 openid 添加到 allowFrom 白名单"
  else
    warn "未能在 ${OPENID_TIMEOUT} 秒内检测到 openid"
    warn "dmPolicy 保持 open，允许所有私聊"
    warn "如需限制，请手动编辑 ${CONFIG_FILE} 设置 allowFrom"
  fi
}

# ============================================================
# 阶段 4: 配置 IDE MCP
# ============================================================
phase_ide_config() {
  echo ""
  echo -e "${BOLD}=== 阶段 4/5: 配置 IDE MCP ===${RESET}"

  local ide_type="$OPT_IDE"
  if [[ -z "$ide_type" ]]; then
    if [[ "$OPT_NON_INTERACTIVE" == true ]]; then
      ide_type="both"
    else
      prompt_choice ide_type "选择要配置的 IDE:" "两者都配置" "仅 CodeBuddy Code" "仅 Codex"
      case "$ide_type" in
        两者*) ide_type="both" ;;
        仅\ CodeBuddy*) ide_type="codebuddy" ;;
        仅\ Codex*) ide_type="codex" ;;
      esac
    fi
  fi

  local qqbot_cmd
  qqbot_cmd="$(command -v qqbot 2>/dev/null || echo "${INSTALL_DIR}/qqbot")"

  # --- CodeBuddy Code ---
  if [[ "$ide_type" == "codebuddy" || "$ide_type" == "both" ]]; then
    local mcp_file="${TARGET_DIR}/.mcp.json"
    local need_write=true

    if [[ -f "$mcp_file" ]]; then
      if grep -q '"qq-channel"' "$mcp_file" 2>/dev/null; then
        if [[ "$OPT_NON_INTERACTIVE" == false ]]; then
          if ! prompt_yesno ".mcp.json 中已存在 qq-channel 配置，是否覆盖?"; then
            need_write=false
          fi
        fi
      fi
    fi

    if [[ "$need_write" == true ]]; then
      if [[ -f "$mcp_file" ]] && command -v jq &>/dev/null; then
        # 用 jq 合并到已有配置
        local tmp
        tmp=$(mktemp)
        jq --arg cmd "$qqbot_cmd" --arg cfg "$CONFIG_FILE" \
          '.mcpServers["qq-channel"] = {"command": $cmd, "args": ["channel", "-config", $cfg]}' \
          "$mcp_file" > "$tmp" && mv "$tmp" "$mcp_file"
      elif [[ -f "$mcp_file" ]] && command -v python3 &>/dev/null; then
        # 用 python3 合并 (无需第三方库，只用内置 json)
        python3 -c "
import json
with open('${mcp_file}') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})
cfg['mcpServers']['qq-channel'] = {
    'command': '${qqbot_cmd}',
    'args': ['channel', '-config', '${CONFIG_FILE}']
}
with open('${mcp_file}', 'w') as f:
    json.dump(cfg, f, indent=2)
    f.write('\n')
"
      else
        # 无 jq/python3，检查文件是否已有其他配置
        if [[ -f "$mcp_file" ]] && grep -qF 'mcpServers' "$mcp_file" 2>/dev/null; then
          warn "无 jq/python3，无法智能合并 .mcp.json"
          warn "将覆盖现有 .mcp.json，原有 MCP 配置会丢失"
          if ! prompt_yesno "是否继续覆盖 .mcp.json?"; then
            need_write=false
          fi
        fi
        if [[ "$need_write" == true ]]; then
          cat > "$mcp_file" <<JSON
{
  "mcpServers": {
    "qq-channel": {
      "command": "${qqbot_cmd}",
      "args": ["channel", "-config", "${CONFIG_FILE}"]
    }
  }
}
JSON
        fi
      fi
      info "已生成 ${mcp_file} (CodeBuddy Code MCP 配置)"
    fi
  fi

  # --- Codex ---
  if [[ "$ide_type" == "codex" || "$ide_type" == "both" ]]; then
    local codex_dir="${HOME}/.codex"
    local codex_config="${codex_dir}/config.toml"
    # Codex 使用项目名作为 server 名，避免多项目冲突
    local server_name="qq-channel-${PROJECT_NAME}"
    local need_add=true

    mkdir -p "$codex_dir"

    if [[ -f "$codex_config" ]] && grep -qF "[mcp_servers.${server_name}]" "$codex_config" 2>/dev/null; then
      if [[ "$OPT_NON_INTERACTIVE" == false ]]; then
        if ! prompt_yesno "Codex 配置中已存在 ${server_name}，是否覆盖?"; then
          need_add=false
        fi
      fi
    fi

    if [[ "$need_add" == true ]]; then
      # 如果已有同名段，先删除旧段再用 sed
      if [[ -f "$codex_config" ]] && grep -qF "[mcp_servers.${server_name}]" "$codex_config" 2>/dev/null; then
        sed -i.bak "/^\[mcp_servers\\.${server_name}\]/,/^\[/{/^\[mcp_servers\\.${server_name}\]/d;/^\[/!d}" "$codex_config"
        rm -f "${codex_config}.bak"
      fi

      cat >> "$codex_config" <<TOML

[mcp_servers.${server_name}]
command = "${qqbot_cmd}"
args = ["channel", "-config", "${CONFIG_FILE}"]
startup_timeout_sec = 15.0
TOML
      info "已配置 ${codex_config} [${server_name}] (Codex MCP 配置)"
    fi
  fi
}

# ============================================================
# 阶段 5: 完成
# ============================================================
phase_done() {
  echo ""
  echo -e "${BOLD}=== 阶段 5/5: 安装完成 ===${RESET}"
  echo ""
  echo -e "${GREEN}QQ Bot MCP Channel 安装成功! (项目: ${PROJECT_NAME})${RESET}"
  echo ""
  echo "安装摘要:"
  echo "  目标项目:  ${TARGET_DIR}"
  echo "  二进制:    ${INSTALL_DIR}/qqbot"
  echo "  配置文件:  ${CONFIG_FILE}"
  echo "  数据目录:  ${DATA_DIR}"
  if [[ -f "${TARGET_DIR}/.mcp.json" ]]; then
    echo "  CodeBuddy: ${TARGET_DIR}/.mcp.json"
  fi
  local server_name="qq-channel-${PROJECT_NAME}"
  if [[ -f "${HOME}/.codex/config.toml" ]] && grep -qF "[mcp_servers.${server_name}]" "${HOME}/.codex/config.toml" 2>/dev/null; then
    echo "  Codex:     ~/.codex/config.toml [${server_name}]"
  fi
  echo ""
  echo "使用方式:"
  echo "  CodeBuddy Code:"
  echo "    codebuddy --dangerously-load-development-channels server:qq-channel"
  echo ""
  echo "  Codex:"
  echo "    直接运行 codex 即可自动加载 MCP 服务器 [${server_name}]"
  echo ""
  echo "多项目并行:"
  echo "  不同项目使用不同的配置文件和数据目录，可以同时启动多个 MCP 实例"
  echo "  只需在不同项目目录分别运行: $0 --dir /path/to/other-project"
  echo ""
  echo -e "${YELLOW}注意: 同一项目目录内只能有一个 IDE 使用 MCP channel (PID 锁机制)${RESET}"
}

# ============================================================
# 主流程
# ============================================================
main() {
  echo -e "${BOLD}QQ Bot MCP Channel 一键安装 (项目: ${PROJECT_NAME})${RESET}"
  echo ""

  phase_install
  phase_config
  phase_detect_openid
  phase_ide_config
  phase_done
}

main
