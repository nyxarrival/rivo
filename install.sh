#!/usr/bin/env bash
set -euo pipefail

PROGRAM="Rivo"
DEFAULT_INSTALL_ROOT="/opt/rivo"
DEFAULT_IMAGE_REGISTRY="ghcr.io"
DEFAULT_IMAGE_TAG="latest"
DEFAULT_INSTALL_URL="https://raw.githubusercontent.com/REPLACE_WITH_GITHUB_OWNER/rivo/main/install.sh"

MODE=""
IMAGE_OWNER="${RIVO_IMAGE_OWNER:-}"
IMAGE_REGISTRY="${RIVO_IMAGE_REGISTRY:-$DEFAULT_IMAGE_REGISTRY}"
IMAGE_TAG="${RIVO_IMAGE_TAG:-$DEFAULT_IMAGE_TAG}"
MASTER_IMAGE="${RIVO_MASTER_IMAGE:-}"
AGENT_IMAGE="${RIVO_AGENT_IMAGE:-}"
INSTALL_DIR="${RIVO_INSTALL_DIR:-}"
HTTP_PORT="${RIVO_HTTP_PORT:-8080}"
TCP_PORT="${RIVO_TCP_PORT:-9443}"
ADMIN_PATH="${RIVO_ADMIN_PATH:-}"
ADMIN_PASSWORD="${RIVO_ADMIN_PASSWORD:-}"
SECRET_KEY="${RIVO_SECRET_KEY:-}"
MASTER_ADDR="${RIVO_MASTER_ADDR:-}"
INSTALL_URL="${RIVO_INSTALL_URL:-$DEFAULT_INSTALL_URL}"
FORCE=false

usage() {
  cat <<'EOF'
Usage:
  install.sh master [options]
  install.sh agent --master HOST:9443 --secret SECRET [options]
  install.sh single [options]

Options:
  --image-owner OWNER       GitHub/GHCR owner for ghcr.io/OWNER/rivo-master and rivo-agent
  --image-registry URL      Image registry, default: ghcr.io
  --image-tag TAG           Image tag, default: latest
  --master-image IMAGE      Full master image name, optional tag allowed
  --agent-image IMAGE       Full agent image name, optional tag allowed
  --install-dir DIR         Install directory. Defaults to /opt/rivo/<mode>
  --http-port PORT          Master HTTP port, default: 8080
  --tcp-port PORT           Master TCP port, default: 9443
  --admin-path PATH         Admin path. Generated when omitted
  --admin-password VALUE    Admin password. Generated when omitted
  --secret VALUE            Shared secret_key. Generated for master/single, required for agent
  --master HOST:PORT        Master TCP address for agent mode
  --force                   Overwrite generated compose/config files
  -h, --help                Show this help

Examples:
  sudo bash install.sh master --image-owner your-github-user
  sudo bash install.sh agent --image-owner your-github-user --master 1.2.3.4:9443 --secret "..."
  sudo bash install.sh single --image-owner your-github-user
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
  MODE="${1:-}"
  if [[ -z "$MODE" || "$MODE" == "-h" || "$MODE" == "--help" ]]; then
    usage
    exit 0
  fi
  case "$MODE" in
    master|agent|single) ;;
    *) die "unknown mode: $MODE" ;;
  esac
  shift || true

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --image-owner) IMAGE_OWNER="${2:-}"; shift 2 ;;
      --image-registry) IMAGE_REGISTRY="${2:-}"; shift 2 ;;
      --image-tag) IMAGE_TAG="${2:-}"; shift 2 ;;
      --master-image) MASTER_IMAGE="${2:-}"; shift 2 ;;
      --agent-image) AGENT_IMAGE="${2:-}"; shift 2 ;;
      --install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
      --http-port) HTTP_PORT="${2:-}"; shift 2 ;;
      --tcp-port) TCP_PORT="${2:-}"; shift 2 ;;
      --admin-path) ADMIN_PATH="${2:-}"; shift 2 ;;
      --admin-password) ADMIN_PASSWORD="${2:-}"; shift 2 ;;
      --secret) SECRET_KEY="${2:-}"; shift 2 ;;
      --master) MASTER_ADDR="${2:-}"; shift 2 ;;
      --force) FORCE=true; shift ;;
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

ensure_tools() {
  need_command docker
  docker compose version >/dev/null 2>&1 || die "docker compose plugin is required"
  need_command base64
}

target_dir() {
  if [[ -n "$INSTALL_DIR" ]]; then
    printf '%s\n' "$INSTALL_DIR"
  else
    printf '%s/%s\n' "$DEFAULT_INSTALL_ROOT" "$MODE"
  fi
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

compose_up() {
  local dir="$1"
  docker compose -f "$dir/compose.yml" up -d
}

print_master_summary() {
  local host_ip
  host_ip="$(detect_host_ip)"
  host_ip="${host_ip:-YOUR_SERVER_IP}"
  cat <<EOF

$PROGRAM Master installed.

Dashboard: http://$host_ip:$HTTP_PORT
Admin:     http://$host_ip:$HTTP_PORT/$ADMIN_PATH
Username:  admin
Password:  $ADMIN_PASSWORD

Agent install command:
curl -fsSL $INSTALL_URL | sudo bash -s -- agent \\
  --image-owner ${IMAGE_OWNER:-YOUR_GITHUB_OWNER} \\
  --master $host_ip:$TCP_PORT \\
  --secret "$SECRET_KEY"

EOF
}

print_agent_summary() {
  cat <<EOF

$PROGRAM Agent installed.

Master: $MASTER_ADDR

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

install_agent() {
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

main() {
  parse_args "$@"
  validate_port "--http-port" "$HTTP_PORT"
  validate_port "--tcp-port" "$TCP_PORT"
  ensure_tools
  normalize_owner
  resolve_images
  resolve_install_url

  case "$MODE" in
    master) install_master ;;
    agent) install_agent ;;
    single) install_single ;;
  esac
}

main "$@"
