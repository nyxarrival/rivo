#!/usr/bin/env bash
set -euo pipefail

PROGRAM="Rivo"
DEFAULT_INSTALL_ROOT="/opt/rivo"
DEFAULT_IMAGE_OWNER="nyxarrival"
DEFAULT_IMAGE_REGISTRY="ghcr.io"
DEFAULT_IMAGE_TAG="latest"
DEFAULT_INSTALL_URL="https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh"
DEFAULT_RELEASE_VERSION="latest"

MODE=""
COMMAND="install"
AGENT_METHOD="${RIVO_AGENT_METHOD:-docker}"
AGENT_METHOD_SET=false
IMAGE_OWNER="${RIVO_IMAGE_OWNER:-$DEFAULT_IMAGE_OWNER}"
IMAGE_REGISTRY="${RIVO_IMAGE_REGISTRY:-$DEFAULT_IMAGE_REGISTRY}"
IMAGE_TAG="${RIVO_IMAGE_TAG:-$DEFAULT_IMAGE_TAG}"
MASTER_IMAGE="${RIVO_MASTER_IMAGE:-}"
AGENT_IMAGE="${RIVO_AGENT_IMAGE:-}"
RELEASE_REPO="${RIVO_RELEASE_REPO:-}"
RELEASE_VERSION="${RIVO_RELEASE_VERSION:-$DEFAULT_RELEASE_VERSION}"
INSTALL_DIR="${RIVO_INSTALL_DIR:-}"
HTTP_PORT="${RIVO_HTTP_PORT:-8080}"
TCP_PORT="${RIVO_TCP_PORT:-9443}"
ADMIN_PATH="${RIVO_ADMIN_PATH:-}"
ADMIN_PASSWORD="${RIVO_ADMIN_PASSWORD:-}"
SECRET_KEY="${RIVO_SECRET_KEY:-}"
MASTER_ADDR="${RIVO_MASTER_ADDR:-}"
INSTALL_URL="${RIVO_INSTALL_URL:-$DEFAULT_INSTALL_URL}"
FORCE=false
PURGE=false

usage() {
  cat <<'EOF'
Usage:
  install.sh master [options]
  install.sh agent --master HOST:9443 --secret SECRET [options]
  install.sh single [options]
  install.sh uninstall [master|agent|single|all] [options]

Options:
  --method METHOD          Agent install/uninstall method: docker or binary. Default: docker
  --image-owner OWNER       GitHub/GHCR owner, default: nyxarrival
  --image-registry URL      Image registry, default: ghcr.io
  --image-tag TAG           Image tag, default: latest
  --master-image IMAGE      Full master image name, optional tag allowed
  --agent-image IMAGE       Full agent image name, optional tag allowed
  --release-repo OWNER/REPO GitHub repo for binary releases, default: nyxarrival/rivo
  --release-version VERSION Binary release version, default: latest
  --install-dir DIR         Install directory. Defaults to /opt/rivo/<mode>
  --http-port PORT          Master HTTP port, default: 8080
  --tcp-port PORT           Master TCP port, default: 9443
  --admin-path PATH         Admin path. Generated when omitted
  --admin-password VALUE    Admin password. Generated when omitted
  --secret VALUE            Shared secret_key. Generated for master/single, required for agent
  --master HOST:PORT        Master TCP address for agent mode
  --force                   Overwrite generated compose/config/service files
  --purge                   Remove Docker volumes during uninstall
  -h, --help                Show this help

Examples:
  sudo bash install.sh master
  sudo bash install.sh agent --master 1.2.3.4:9443 --secret "..."
  sudo bash install.sh agent --method binary --master 1.2.3.4:9443 --secret "..."
  sudo bash install.sh single
  sudo bash install.sh uninstall
  sudo bash install.sh uninstall agent --method binary
EOF
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

info() {
  printf '%s\n' "$*"
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

parse_args() {
  local first="${1:-}"
  if [[ -z "$first" || "$first" == "-h" || "$first" == "--help" ]]; then
    usage
    exit 0
  fi

  case "$first" in
    uninstall)
      COMMAND="uninstall"
      shift
      if [[ $# -gt 0 && "${1:-}" != -* ]]; then
        MODE="$1"
        shift
      else
        MODE="all"
      fi
      case "$MODE" in
        master|agent|single|all) ;;
        *) die "unknown uninstall target: $MODE" ;;
      esac
      ;;
    master|agent|single)
      COMMAND="install"
      MODE="$first"
      shift
      ;;
    *) die "unknown mode or command: $first" ;;
  esac

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --image-owner) IMAGE_OWNER="${2:-}"; shift 2 ;;
      --image-registry) IMAGE_REGISTRY="${2:-}"; shift 2 ;;
      --image-tag) IMAGE_TAG="${2:-}"; shift 2 ;;
      --master-image) MASTER_IMAGE="${2:-}"; shift 2 ;;
      --agent-image) AGENT_IMAGE="${2:-}"; shift 2 ;;
      --method|--agent-method) AGENT_METHOD="${2:-}"; AGENT_METHOD_SET=true; shift 2 ;;
      --release-repo) RELEASE_REPO="${2:-}"; shift 2 ;;
      --release-version) RELEASE_VERSION="${2:-}"; shift 2 ;;
      --install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
      --http-port) HTTP_PORT="${2:-}"; shift 2 ;;
      --tcp-port) TCP_PORT="${2:-}"; shift 2 ;;
      --admin-path) ADMIN_PATH="${2:-}"; shift 2 ;;
      --admin-password) ADMIN_PASSWORD="${2:-}"; shift 2 ;;
      --secret) SECRET_KEY="${2:-}"; shift 2 ;;
      --master) MASTER_ADDR="${2:-}"; shift 2 ;;
      --force) FORCE=true; shift ;;
      --purge) PURGE=true; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown option: $1" ;;
    esac
  done
}

random_hex() {
  local bytes="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$bytes"
  else
    od -An -N "$bytes" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

random_base64_32() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32
  else
    dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 | tr -d '\n'
  fi
}

normalize_owner() {
  if [[ -n "$IMAGE_OWNER" ]]; then
    IMAGE_OWNER="$(printf '%s' "$IMAGE_OWNER" | tr '[:upper:]' '[:lower:]')"
  fi
}

validate_port() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[0-9]+$ ]] || die "$name must be a port number"
  (( value >= 1 && value <= 65535 )) || die "$name must be between 1 and 65535"
}

validate_admin_path() {
  local value="$1"
  [[ ${#value} -gt 5 ]] || die "admin path must be longer than 5 characters"
  [[ "$value" =~ ^[A-Za-z0-9]+$ ]] || die "admin path must contain only letters and digits"
  case "${value,,}" in
    api|healthz|themes) die "admin path cannot be a reserved path: $value" ;;
  esac
}

validate_agent_method() {
  AGENT_METHOD="$(printf '%s' "$AGENT_METHOD" | tr '[:upper:]' '[:lower:]')"
  case "$AGENT_METHOD" in
    docker|binary) ;;
    *) die "--method must be docker or binary" ;;
  esac
  if [[ "$COMMAND" == "install" && "$MODE" != "agent" && "$AGENT_METHOD" != "docker" ]]; then
    die "--method binary is only supported for agent mode"
  fi
  if [[ "$COMMAND" == "uninstall" && "$MODE" != "agent" && "$MODE" != "all" && "$AGENT_METHOD" != "docker" ]]; then
    die "--method binary is only supported for agent uninstall"
  fi
}

image_ref() {
  local image="$1"
  local tag="$2"
  local last="${image##*/}"
  if [[ "$image" == *@sha256:* || "$last" == *:* ]]; then
    printf '%s\n' "$image"
  else
    printf '%s:%s\n' "$image" "$tag"
  fi
}

resolve_images() {
  case "$MODE" in
    master)
      if [[ -z "$MASTER_IMAGE" ]]; then
        [[ -n "$IMAGE_OWNER" ]] || die "set --image-owner, or pass --master-image"
        MASTER_IMAGE="$IMAGE_REGISTRY/$IMAGE_OWNER/rivo-master"
      fi
      MASTER_IMAGE="$(image_ref "$MASTER_IMAGE" "$IMAGE_TAG")"
      ;;
    agent)
      if [[ -z "$AGENT_IMAGE" ]]; then
        [[ -n "$IMAGE_OWNER" ]] || die "set --image-owner, or pass --agent-image"
        AGENT_IMAGE="$IMAGE_REGISTRY/$IMAGE_OWNER/rivo-agent"
      fi
      AGENT_IMAGE="$(image_ref "$AGENT_IMAGE" "$IMAGE_TAG")"
      ;;
    single)
      if [[ -z "$MASTER_IMAGE" || -z "$AGENT_IMAGE" ]]; then
        [[ -n "$IMAGE_OWNER" ]] || die "set --image-owner, or pass --master-image and --agent-image"
      fi
      MASTER_IMAGE="${MASTER_IMAGE:-$IMAGE_REGISTRY/$IMAGE_OWNER/rivo-master}"
      AGENT_IMAGE="${AGENT_IMAGE:-$IMAGE_REGISTRY/$IMAGE_OWNER/rivo-agent}"
      MASTER_IMAGE="$(image_ref "$MASTER_IMAGE" "$IMAGE_TAG")"
      AGENT_IMAGE="$(image_ref "$AGENT_IMAGE" "$IMAGE_TAG")"
      ;;
  esac
}

resolve_install_url() {
  if [[ "$INSTALL_URL" == *REPLACE_WITH_GITHUB_OWNER* && -n "$IMAGE_OWNER" ]]; then
    INSTALL_URL="${INSTALL_URL/REPLACE_WITH_GITHUB_OWNER/$IMAGE_OWNER}"
  fi
}

resolve_release_repo() {
  if [[ -z "$RELEASE_REPO" && -n "$IMAGE_OWNER" ]]; then
    RELEASE_REPO="$IMAGE_OWNER/rivo"
  fi
  if [[ -z "$RELEASE_REPO" && "$INSTALL_URL" =~ raw\.githubusercontent\.com/([^/]+)/([^/]+)/ ]]; then
    RELEASE_REPO="${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
  fi
  [[ -n "$RELEASE_REPO" ]] || die "set --image-owner or --release-repo for binary agent install"
  [[ "$RELEASE_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "--release-repo must look like OWNER/REPO"
}

detect_release_platform() {
  local os
  local arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) os="linux" ;;
    *) die "binary agent install currently supports Linux only, got: $os" ;;
  esac

  arch="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported CPU architecture for binary agent install: $arch" ;;
  esac

  printf '%s %s\n' "$os" "$arch"
}

agent_release_url() {
  local os="$1"
  local arch="$2"
  local asset="rivo-agent-${os}-${arch}.tar.gz"
  if [[ "$RELEASE_VERSION" == "latest" ]]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$RELEASE_REPO" "$asset"
  else
    printf 'https://github.com/%s/releases/download/%s/%s\n' "$RELEASE_REPO" "$RELEASE_VERSION" "$asset"
  fi
}

download_to() {
  local url="$1"
  local path="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 15 -o "$path" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$path" "$url"
  else
    die "curl or wget is required"
  fi
}

ensure_docker_compose() {
  need_command docker
  docker compose version >/dev/null 2>&1 || die "docker compose plugin is required"
}

ensure_docker_tools() {
  ensure_docker_compose
  need_command base64
}

ensure_agent_binary_tools() {
  need_command uname
  need_command tar
  need_command systemctl
  command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || die "curl or wget is required"
}

target_dir_for_mode() {
  local mode="$1"
  if [[ -n "$INSTALL_DIR" ]]; then
    printf '%s\n' "$INSTALL_DIR"
  else
    printf '%s/%s\n' "$DEFAULT_INSTALL_ROOT" "$mode"
  fi
}

target_dir() {
  target_dir_for_mode "$MODE"
}

write_file() {
  local path="$1"
  if [[ -e "$path" && "$FORCE" != true ]]; then
    die "$path already exists; rerun with --force to overwrite"
  fi
  mkdir -p "$(dirname "$path")"
  cat >"$path"
}

make_master_config() {
  local dir="$1"
  local config="$dir/config.yaml"
  write_file "$config" <<EOF
http:
  listen_addr: ":8080"
  admin_path: "$ADMIN_PATH"

tcp:
  listen_addr: ":9443"
  secret_key: "$SECRET_KEY"

database:
  driver: "sqlite"
  dsn: "data/rivo.db"
  auto_migrate: true

auth:
  username: "admin"
  password: "$ADMIN_PASSWORD"

log:
  level: "info"
  file: "logs/master.log"
  retention_days: 30
EOF
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    chown 10001:10001 "$config" 2>/dev/null || true
    chmod 0640 "$config"
  else
    chmod 0644 "$config"
  fi
}

make_agent_config() {
  local dir="$1"
  local config="$dir/config.yaml"
  write_file "$config" <<EOF
master_addr: "$MASTER_ADDR"
secret_key: "$SECRET_KEY"

agent:
  node_id: ""
  state_file: "/app/data/agent-state.json"

public_ip:
  enabled: true
  timeout_ms: 3000
  refresh_interval_seconds: 600
  ipv4_enabled: true
  ipv6_enabled: true
  ipv4_endpoints:
    - "https://api.ipify.org"
    - "https://v4.ident.me"
    - "https://ipv4.icanhazip.com"
    - "https://ifconfig.me/ip"
  ipv6_endpoints:
    - "https://api6.ipify.org"
    - "https://v6.ident.me"
    - "https://ipv6.icanhazip.com"
    - "https://ifconfig.me/ip"

log:
  level: "info"
  file: "logs/agent.log"
  retention_days: 30
EOF
  chmod 0600 "$config"
}

make_agent_binary_config() {
  local dir="$1"
  local config="$dir/config.yaml"
  mkdir -p "$dir/data" "$dir/logs"
  write_file "$config" <<EOF
master_addr: "$MASTER_ADDR"
secret_key: "$SECRET_KEY"

agent:
  node_id: ""
  state_file: "$dir/data/agent-state.json"

public_ip:
  enabled: true
  timeout_ms: 3000
  refresh_interval_seconds: 600
  ipv4_enabled: true
  ipv6_enabled: true
  ipv4_endpoints:
    - "https://api.ipify.org"
    - "https://v4.ident.me"
    - "https://ipv4.icanhazip.com"
    - "https://ifconfig.me/ip"
  ipv6_endpoints:
    - "https://api6.ipify.org"
    - "https://v6.ident.me"
    - "https://ipv6.icanhazip.com"
    - "https://ifconfig.me/ip"

log:
  level: "info"
  file: "$dir/logs/agent.log"
  retention_days: 30
EOF
  chmod 0600 "$config"
}

install_agent_binary_files() {
  local dir="$1"
  local os
  local arch
  local url
  local archive
  local tmp_dir
  local extracted_bin

  read -r os arch < <(detect_release_platform)
  url="$(agent_release_url "$os" "$arch")"
  archive="$dir/rivo-agent-${os}-${arch}.tar.gz"
  tmp_dir="$dir/.release-tmp"
  extracted_bin="$tmp_dir/rivo-agent-${os}-${arch}/rivo-agent"

  mkdir -p "$dir/bin" "$tmp_dir"
  info "Downloading $url"
  download_to "$url" "$archive"

  rm -rf "$tmp_dir"
  mkdir -p "$tmp_dir"
  tar -xzf "$archive" -C "$tmp_dir"
  [[ -f "$extracted_bin" ]] || die "release archive does not contain $extracted_bin"

  cp "$extracted_bin" "$dir/bin/rivo-agent"
  chmod 0755 "$dir/bin/rivo-agent"
  rm -rf "$tmp_dir"
}

make_agent_systemd_service() {
  local dir="$1"
  local service="/etc/systemd/system/rivo-agent.service"
  write_file "$service" <<EOF
[Unit]
Description=Rivo Agent
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$dir
ExecStart=$dir/bin/rivo-agent -config $dir/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
}

start_agent_systemd_service() {
  systemctl daemon-reload
  systemctl enable rivo-agent.service
  systemctl restart rivo-agent.service
}

make_master_compose() {
  local dir="$1"
  write_file "$dir/compose.yml" <<EOF
services:
  master:
    image: "$MASTER_IMAGE"
    container_name: "rivo-master"
    restart: unless-stopped
    command: ["-config", "/app/config.yaml"]
    ports:
      - "$HTTP_PORT:8080"
      - "$TCP_PORT:9443"
    volumes:
      - type: bind
        source: "$dir/config.yaml"
        target: /app/config.yaml
        read_only: true
        bind:
          create_host_path: false
      - master-data:/app/data
      - master-logs:/app/logs

volumes:
  master-data:
  master-logs:
EOF
}

make_agent_compose() {
  local dir="$1"
  write_file "$dir/compose.yml" <<EOF
services:
  agent:
    image: "$AGENT_IMAGE"
    container_name: "rivo-agent"
    restart: unless-stopped
    command: ["-config", "/app/config.yaml"]
    network_mode: host
    pid: host
    user: "0:0"
    cap_add:
      - NET_RAW
    volumes:
      - type: bind
        source: "$dir/config.yaml"
        target: /app/config.yaml
        read_only: true
        bind:
          create_host_path: false
      - agent-data:/app/data
      - agent-logs:/app/logs
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro

volumes:
  agent-data:
  agent-logs:
EOF
}

make_single_compose() {
  local dir="$1"
  write_file "$dir/compose.yml" <<EOF
services:
  master:
    image: "$MASTER_IMAGE"
    container_name: "rivo-master"
    restart: unless-stopped
    command: ["-config", "/app/master-config.yaml"]
    ports:
      - "$HTTP_PORT:8080"
      - "$TCP_PORT:9443"
    volumes:
      - type: bind
        source: "$dir/master-config.yaml"
        target: /app/master-config.yaml
        read_only: true
        bind:
          create_host_path: false
      - master-data:/app/data
      - master-logs:/app/logs

  agent:
    image: "$AGENT_IMAGE"
    container_name: "rivo-agent"
    restart: unless-stopped
    command: ["-config", "/app/agent-config.yaml"]
    network_mode: host
    pid: host
    user: "0:0"
    cap_add:
      - NET_RAW
    volumes:
      - type: bind
        source: "$dir/agent-config.yaml"
        target: /app/agent-config.yaml
        read_only: true
        bind:
          create_host_path: false
      - agent-data:/app/data
      - agent-logs:/app/logs
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro

volumes:
  master-data:
  master-logs:
  agent-data:
  agent-logs:
EOF
}

make_single_configs() {
  local dir="$1"
  local old_install_dir="$INSTALL_DIR"
  INSTALL_DIR="$dir"

  local master_config="$dir/master-config.yaml"
  write_file "$master_config" <<EOF
http:
  listen_addr: ":8080"
  admin_path: "$ADMIN_PATH"

tcp:
  listen_addr: ":9443"
  secret_key: "$SECRET_KEY"

database:
  driver: "sqlite"
  dsn: "data/rivo.db"
  auto_migrate: true

auth:
  username: "admin"
  password: "$ADMIN_PASSWORD"

log:
  level: "info"
  file: "logs/master.log"
  retention_days: 30
EOF
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    chown 10001:10001 "$master_config" 2>/dev/null || true
    chmod 0640 "$master_config"
  else
    chmod 0644 "$master_config"
  fi

  local agent_config="$dir/agent-config.yaml"
  write_file "$agent_config" <<EOF
master_addr: "127.0.0.1:$TCP_PORT"
secret_key: "$SECRET_KEY"

agent:
  node_id: ""
  state_file: "/app/data/agent-state.json"

public_ip:
  enabled: true
  timeout_ms: 3000
  refresh_interval_seconds: 600
  ipv4_enabled: true
  ipv6_enabled: true
  ipv4_endpoints:
    - "https://api.ipify.org"
    - "https://v4.ident.me"
    - "https://ipv4.icanhazip.com"
    - "https://ifconfig.me/ip"
  ipv6_endpoints:
    - "https://api6.ipify.org"
    - "https://v6.ident.me"
    - "https://ipv6.icanhazip.com"
    - "https://ifconfig.me/ip"

log:
  level: "info"
  file: "logs/agent.log"
  retention_days: 30
EOF
  chmod 0600 "$agent_config"
  INSTALL_DIR="$old_install_dir"
}

detect_host_ip() {
  if command -v ip >/dev/null 2>&1; then
    ip route get 1.1.1.1 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}'
  elif command -v hostname >/dev/null 2>&1; then
    hostname -I 2>/dev/null | awk '{print $1}'
  fi
}

is_private_or_local_address() {
  local ip="$1"
  local o1
  local o2
  local o3
  local o4

  [[ "$ip" == "YOUR_SERVER_IP" ]] && return 0

  if [[ "$ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
    IFS=. read -r o1 o2 o3 o4 <<<"$ip"
    o1=$((10#$o1))
    o2=$((10#$o2))
    case "$o1" in
      0|10|127) return 0 ;;
      169) (( o2 == 254 )) && return 0 ;;
      172) (( o2 >= 16 && o2 <= 31 )) && return 0 ;;
      192) (( o2 == 168 )) && return 0 ;;
      100) (( o2 >= 64 && o2 <= 127 )) && return 0 ;;
    esac
    return 1
  fi

  case "${ip,,}" in
    localhost|::1|fe80:*|fc*|fd*) return 0 ;;
  esac
  return 1
}

compose_up() {
  local dir="$1"
  docker compose -f "$dir/compose.yml" up -d
}

compose_down() {
  local dir="$1"
  local remove_volumes="$2"
  if [[ ! -f "$dir/compose.yml" ]]; then
    info "No compose file found at $dir/compose.yml; skipping Docker cleanup."
    return
  fi

  ensure_docker_compose
  local args=(compose -f "$dir/compose.yml" down --remove-orphans)
  if [[ "$remove_volumes" == true ]]; then
    args+=(--volumes)
  fi
  docker "${args[@]}"
}

validate_install_dir_for_uninstall() {
  local dir="$1"
  [[ -n "$dir" ]] || die "refusing to remove an empty install directory"
  case "$dir" in
    /|.|..|"$DEFAULT_INSTALL_ROOT"|/bin|/boot|/dev|/etc|/home|/lib|/lib64|/opt|/proc|/root|/run|/sbin|/sys|/tmp|/usr|/var)
      die "refusing to remove unsafe install directory: $dir"
      ;;
  esac
}

remove_install_dir() {
  local dir="$1"
  validate_install_dir_for_uninstall "$dir"

  if [[ -d "$dir" ]]; then
    rm -rf -- "$dir"
    info "Removed install directory: $dir"
  else
    info "Install directory not found: $dir"
  fi
}

remove_agent_systemd_service() {
  local service="/etc/systemd/system/rivo-agent.service"
  local touched=false

  if ! command -v systemctl >/dev/null 2>&1; then
    if [[ -e "$service" ]]; then
      die "systemctl is required to remove $service"
    fi
    info "systemctl not found; skipping binary agent service cleanup."
    return
  fi

  if systemctl is-active --quiet rivo-agent.service || systemctl is-enabled --quiet rivo-agent.service || [[ -e "$service" ]]; then
    systemctl stop rivo-agent.service >/dev/null 2>&1 || true
    systemctl disable rivo-agent.service >/dev/null 2>&1 || true
    touched=true
  else
    info "rivo-agent.service not found; skipping systemd cleanup."
  fi

  if [[ -e "$service" ]]; then
    rm -f "$service"
    info "Removed systemd service: $service"
    touched=true
  fi

  if [[ "$touched" == true ]]; then
    systemctl daemon-reload
    systemctl reset-failed rivo-agent.service >/dev/null 2>&1 || true
  fi
}

print_master_summary() {
  local host_ip
  local master_addr
  host_ip="$(detect_host_ip)"
  host_ip="${host_ip:-YOUR_SERVER_IP}"
  master_addr="$host_ip:$TCP_PORT"
  cat <<EOF

$PROGRAM Master installed.

Dashboard: http://$host_ip:$HTTP_PORT
Admin:     http://$host_ip:$HTTP_PORT/$ADMIN_PATH
Username:  admin
Password:  $ADMIN_PASSWORD

Detected Master TCP address: $master_addr
EOF

  if is_private_or_local_address "$host_ip"; then
    cat <<EOF
Notice: the detected Master address looks private or local. If the Agent is not in the same LAN/VPC/VPN, replace --master $master_addr with a public IP or domain that the Agent can reach.

EOF
  fi

  cat <<EOF
Agent install command (Docker):
curl -fsSL $INSTALL_URL | sudo bash -s -- agent \\
  --master $master_addr \\
  --secret "$SECRET_KEY"

Agent install command (binary):
curl -fsSL $INSTALL_URL | sudo bash -s -- agent \\
  --method binary \\
  --master $master_addr \\
  --secret "$SECRET_KEY"

EOF
}

print_agent_summary() {
  cat <<EOF

$PROGRAM Agent installed.

Master: $MASTER_ADDR
Method: $AGENT_METHOD

EOF
}

install_master() {
  [[ -n "$ADMIN_PATH" ]] || ADMIN_PATH="rivo$(random_hex 8)"
  [[ -n "$ADMIN_PASSWORD" ]] || ADMIN_PASSWORD="$(random_hex 12)"
  [[ -n "$SECRET_KEY" ]] || SECRET_KEY="$(random_base64_32)"
  validate_admin_path "$ADMIN_PATH"

  local dir
  dir="$(target_dir)"
  mkdir -p "$dir"
  make_master_config "$dir"
  make_master_compose "$dir"
  compose_up "$dir"
  print_master_summary
}

install_agent_docker() {
  [[ -n "$MASTER_ADDR" ]] || die "--master is required for agent mode"
  [[ -n "$SECRET_KEY" ]] || die "--secret is required for agent mode"

  local dir
  dir="$(target_dir)"
  mkdir -p "$dir"
  make_agent_config "$dir"
  make_agent_compose "$dir"
  compose_up "$dir"
  print_agent_summary
}

install_agent_binary() {
  [[ -n "$MASTER_ADDR" ]] || die "--master is required for agent mode"
  [[ -n "$SECRET_KEY" ]] || die "--secret is required for agent mode"

  local dir
  dir="$(target_dir)"
  mkdir -p "$dir"
  make_agent_binary_config "$dir"
  install_agent_binary_files "$dir"
  make_agent_systemd_service "$dir"
  start_agent_systemd_service
  print_agent_summary
}

install_single() {
  [[ -n "$ADMIN_PATH" ]] || ADMIN_PATH="rivo$(random_hex 8)"
  [[ -n "$ADMIN_PASSWORD" ]] || ADMIN_PASSWORD="$(random_hex 12)"
  [[ -n "$SECRET_KEY" ]] || SECRET_KEY="$(random_base64_32)"
  validate_admin_path "$ADMIN_PATH"

  local dir
  dir="$(target_dir)"
  mkdir -p "$dir"
  make_single_configs "$dir"
  make_single_compose "$dir"
  compose_up "$dir"
  print_master_summary
}

print_uninstall_summary() {
  local target="$1"
  local volume_status="preserved"
  if [[ "$PURGE" == true ]]; then
    volume_status="removed when compose files were present"
  fi

  cat <<EOF

$PROGRAM $target uninstalled.

Docker volumes: $volume_status

EOF
}

uninstall_docker_mode() {
  local mode="$1"
  local dir
  dir="$(target_dir_for_mode "$mode")"
  validate_install_dir_for_uninstall "$dir"
  compose_down "$dir" "$PURGE"
  remove_install_dir "$dir"
}

uninstall_agent_binary() {
  local dir
  dir="$(target_dir_for_mode agent)"
  validate_install_dir_for_uninstall "$dir"
  need_command systemctl
  remove_agent_systemd_service
  remove_install_dir "$dir"
}

uninstall_agent_all_methods() {
  local dir
  dir="$(target_dir_for_mode agent)"
  validate_install_dir_for_uninstall "$dir"
  compose_down "$dir" "$PURGE"
  remove_agent_systemd_service
  remove_install_dir "$dir"
}

uninstall_agent() {
  if [[ "$AGENT_METHOD_SET" != true ]]; then
    uninstall_agent_all_methods
    return
  fi

  case "$AGENT_METHOD" in
    docker)
      uninstall_docker_mode agent
      ;;
    binary)
      uninstall_agent_binary
      ;;
  esac
}

uninstall_all() {
  [[ -z "$INSTALL_DIR" ]] || die "--install-dir cannot be used with uninstall all; choose master, agent, or single"

  uninstall_docker_mode master
  uninstall_docker_mode single
  uninstall_agent_all_methods
}

uninstall() {
  case "$MODE" in
    master|single)
      uninstall_docker_mode "$MODE"
      ;;
    agent)
      uninstall_agent
      ;;
    all)
      uninstall_all
      ;;
  esac
  print_uninstall_summary "$MODE"
}

main() {
  parse_args "$@"
  validate_agent_method

  if [[ "$COMMAND" == "uninstall" ]]; then
    uninstall
    exit 0
  fi

  [[ "$PURGE" != true ]] || die "--purge is only supported for uninstall"
  validate_port "--http-port" "$HTTP_PORT"
  validate_port "--tcp-port" "$TCP_PORT"
  normalize_owner
  resolve_install_url

  case "$MODE" in
    master)
      ensure_docker_tools
      resolve_images
      install_master
      ;;
    agent)
      case "$AGENT_METHOD" in
        docker)
          ensure_docker_tools
          resolve_images
          install_agent_docker
          ;;
        binary)
          ensure_agent_binary_tools
          resolve_release_repo
          install_agent_binary
          ;;
      esac
      ;;
    single)
      ensure_docker_tools
      resolve_images
      install_single
      ;;
  esac
}

main "$@"
