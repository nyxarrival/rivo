#!/usr/bin/env bash
set -euo pipefail

PROGRAM="Rivo"
DEFAULT_INSTALL_ROOT="/opt/rivo"
DEFAULT_IMAGE_OWNER="nyxarrival"
DEFAULT_IMAGE_REGISTRY="ghcr.io"
DEFAULT_INSTALL_URL="https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh"

MODE=""
COMMAND="install"
INSTALL_METHOD="${RIVO_METHOD:-${RIVO_AGENT_METHOD:-docker}}"
METHOD_SET=false
METHOD_ARG_SET=false
VERSION="${RIVO_VERSION:-}"
VERSION_SET=false
IMAGE_OWNER="${RIVO_IMAGE_OWNER:-$DEFAULT_IMAGE_OWNER}"
IMAGE_REGISTRY="${RIVO_IMAGE_REGISTRY:-$DEFAULT_IMAGE_REGISTRY}"
IMAGE_TAG="${RIVO_IMAGE_TAG:-}"
IMAGE_TAG_SET=false
MASTER_IMAGE="${RIVO_MASTER_IMAGE:-}"
AGENT_IMAGE="${RIVO_AGENT_IMAGE:-}"
RELEASE_REPO="${RIVO_RELEASE_REPO:-}"
RELEASE_VERSION="${RIVO_RELEASE_VERSION:-}"
RELEASE_VERSION_SET=false
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
INTERACTIVE=false

if [[ -n "${RIVO_METHOD:-}" || -n "${RIVO_AGENT_METHOD:-}" ]]; then
  METHOD_SET=true
fi
if [[ -n "${RIVO_VERSION:-}" ]]; then
  VERSION_SET=true
fi
if [[ -n "${RIVO_IMAGE_TAG:-}" ]]; then
  IMAGE_TAG_SET=true
fi
if [[ -n "${RIVO_RELEASE_VERSION:-}" ]]; then
  RELEASE_VERSION_SET=true
fi

usage() {
  cat <<'EOF'
Usage:
  install.sh [--interactive]
  install.sh master [options]
  install.sh agent --master HOST:9443 --secret SECRET [options]
  install.sh single [options]
  install.sh upgrade [master|agent|single|all] [options]
  install.sh uninstall [master|agent|single|all] [options]

Options:
  -i, --interactive        Prompt for install options
  --method METHOD          Install/upgrade/uninstall method: docker or binary. Default: docker
  --version VERSION        Use one version for Docker images and binary releases. Default: latest stable release
  --image-owner OWNER       GitHub/GHCR owner, default: nyxarrival
  --image-registry URL      Image registry, default: ghcr.io
  --image-tag TAG           Docker image tag. Overrides --version for Docker. Use latest for the rolling main build
  --master-image IMAGE      Full master image name, optional tag allowed
  --agent-image IMAGE       Full agent image name, optional tag allowed
  --release-repo OWNER/REPO GitHub repo for stable version and binary releases, default: nyxarrival/rivo
  --release-version VERSION Binary release version. Overrides --version for binary installs
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
  sudo bash install.sh master --method binary
  sudo bash install.sh agent --master 1.2.3.4:9443 --secret "..."
  sudo bash install.sh agent --method binary --master 1.2.3.4:9443 --secret "..."
  sudo bash install.sh single
  sudo bash install.sh upgrade
  sudo bash install.sh upgrade master
  sudo bash install.sh upgrade agent --method binary --version v1.0.1
  sudo bash install.sh uninstall
  sudo bash install.sh uninstall master --method binary
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

need_tty() {
  [[ -r /dev/tty && -w /dev/tty ]] || die "interactive mode requires a TTY"
}

prompt_text() {
  local prompt="$1"
  local default="$2"
  local value
  need_tty
  if [[ -n "$default" ]]; then
    printf '%s [%s]: ' "$prompt" "$default" >/dev/tty
  else
    printf '%s: ' "$prompt" >/dev/tty
  fi
  IFS= read -r value </dev/tty
  if [[ -z "$value" ]]; then
    value="$default"
  fi
  printf '%s\n' "$value"
}

prompt_choice() {
  local prompt="$1"
  local default="$2"
  shift 2
  local value
  local choice
  local index
  local alias
  local candidate
  local i
  local options_text=""
  local aliases=()
  local used_aliases=""

  for choice in "$@"; do
    alias=""
    for ((i = 0; i < ${#choice}; i++)); do
      candidate="${choice:i:1}"
      if [[ "$used_aliases" != *"|$candidate|"* ]]; then
        alias="$candidate"
        used_aliases="${used_aliases}|${alias}|"
        break
      fi
    done
    [[ -n "$alias" ]] || die "cannot assign prompt alias for option: $choice"
    aliases+=("$alias")
    if [[ -n "$options_text" ]]; then
      options_text="$options_text, "
    fi
    options_text="${options_text}${alias}=${choice}"
  done

  while true; do
    value="$(prompt_text "$prompt ($options_text)" "$default")"
    value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
    index=0
    for choice in "$@"; do
      alias="${aliases[$index]}"
      if [[ "$value" == "$choice" || "$value" == "$alias" ]]; then
        printf '%s\n' "$choice"
        return
      fi
      index=$((index + 1))
    done
    printf 'Please choose: %s\n' "$options_text" >/dev/tty
  done
}

parse_args() {
  if [[ $# -eq 0 ]]; then
    INTERACTIVE=true
    return
  fi

  local first="${1:-}"
  if [[ "$first" == "-i" || "$first" == "--interactive" ]]; then
    INTERACTIVE=true
    shift
    first="${1:-}"
    if [[ -z "$first" ]]; then
      return
    fi
  fi

  if [[ "$first" == "-h" || "$first" == "--help" ]]; then
    usage
    exit 0
  fi

  case "$first" in
    upgrade)
      COMMAND="upgrade"
      shift
      if [[ $# -gt 0 && "${1:-}" != -* ]]; then
        MODE="$1"
        shift
      else
        MODE="all"
      fi
      case "$MODE" in
        master|agent|single|all) ;;
        *) die "unknown upgrade target: $MODE" ;;
      esac
      ;;
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
      --version) VERSION="${2:-}"; VERSION_SET=true; shift 2 ;;
      --image-tag) IMAGE_TAG="${2:-}"; IMAGE_TAG_SET=true; shift 2 ;;
      --master-image) MASTER_IMAGE="${2:-}"; shift 2 ;;
      --agent-image) AGENT_IMAGE="${2:-}"; shift 2 ;;
      --method|--agent-method) INSTALL_METHOD="${2:-}"; METHOD_SET=true; METHOD_ARG_SET=true; shift 2 ;;
      --release-repo) RELEASE_REPO="${2:-}"; shift 2 ;;
      --release-version) RELEASE_VERSION="${2:-}"; RELEASE_VERSION_SET=true; shift 2 ;;
      --install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
      --http-port) HTTP_PORT="${2:-}"; shift 2 ;;
      --tcp-port) TCP_PORT="${2:-}"; shift 2 ;;
      --admin-path) ADMIN_PATH="${2:-}"; shift 2 ;;
      --admin-password) ADMIN_PASSWORD="${2:-}"; shift 2 ;;
      --secret) SECRET_KEY="${2:-}"; shift 2 ;;
      --master) MASTER_ADDR="${2:-}"; shift 2 ;;
      --force) FORCE=true; shift ;;
      --purge) PURGE=true; shift ;;
      -i|--interactive) INTERACTIVE=true; shift ;;
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
  local value_lc
  [[ ${#value} -gt 5 ]] || die "admin path must be longer than 5 characters"
  [[ "$value" =~ ^[A-Za-z0-9]+$ ]] || die "admin path must contain only letters and digits"
  value_lc="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value_lc" in
    api|healthz|themes) die "admin path cannot be a reserved path: $value" ;;
  esac
}

validate_install_method() {
  INSTALL_METHOD="$(printf '%s' "$INSTALL_METHOD" | tr '[:upper:]' '[:lower:]')"
  case "$INSTALL_METHOD" in
    docker|binary) ;;
    *) die "--method must be docker or binary" ;;
  esac
  if [[ "$COMMAND" == "install" && "$MODE" == "single" && "$INSTALL_METHOD" != "docker" ]]; then
    die "--method binary is only supported for master and agent modes"
  fi
  if [[ "$COMMAND" != "install" && "$MODE" == "single" && "$INSTALL_METHOD" != "docker" ]]; then
    die "--method binary is only supported for master and agent $COMMAND"
  fi
  if [[ "$COMMAND" != "install" && "$MODE" == "all" && "$METHOD_ARG_SET" == true ]]; then
    die "--method cannot be used with $COMMAND all"
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

image_has_tag() {
  local image="$1"
  local last="${image##*/}"
  [[ "$image" == *@sha256:* || "$last" == *:* ]]
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
  [[ -n "$RELEASE_REPO" ]] || die "set --image-owner or --release-repo"
  [[ "$RELEASE_REPO" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "--release-repo must look like OWNER/REPO"
}

fetch_url_stdout() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --connect-timeout 15 "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
  else
    die "curl or wget is required"
  fi
}

latest_release_tag() {
  local api_url="https://api.github.com/repos/$RELEASE_REPO/releases/latest"
  local tag
  tag="$(fetch_url_stdout "$api_url" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  [[ -n "$tag" ]] || die "could not resolve latest stable release from $api_url"
  printf '%s\n' "$tag"
}

resolve_install_versions() {
  local needs_image=false
  local needs_release=false
  local stable_tag=""

  if [[ "$VERSION_SET" == true ]]; then
    [[ "$IMAGE_TAG_SET" == true ]] || IMAGE_TAG="$VERSION"
    [[ "$RELEASE_VERSION_SET" == true ]] || RELEASE_VERSION="$VERSION"
  fi

  case "$MODE" in
    master|agent)
      if [[ "$INSTALL_METHOD" == "docker" ]]; then
        case "$MODE" in
          master)
            if [[ -z "$MASTER_IMAGE" ]] || ! image_has_tag "$MASTER_IMAGE"; then
              needs_image=true
            fi
            ;;
          agent)
            if [[ -z "$AGENT_IMAGE" ]] || ! image_has_tag "$AGENT_IMAGE"; then
              needs_image=true
            fi
            ;;
        esac
      else
        needs_release=true
      fi
      ;;
    single)
      if [[ -z "$MASTER_IMAGE" || -z "$AGENT_IMAGE" ]] || ! image_has_tag "$MASTER_IMAGE" || ! image_has_tag "$AGENT_IMAGE"; then
        needs_image=true
      fi
      ;;
  esac

  if { [[ "$needs_image" == true && -z "$IMAGE_TAG" ]] || [[ "$needs_release" == true && -z "$RELEASE_VERSION" ]]; }; then
    resolve_release_repo
    stable_tag="$(latest_release_tag)"
  fi

  if [[ -n "$stable_tag" && -z "$IMAGE_TAG" ]]; then
    IMAGE_TAG="$stable_tag"
  fi
  if [[ -n "$stable_tag" && -z "$RELEASE_VERSION" ]]; then
    RELEASE_VERSION="$stable_tag"
  fi
  if [[ -z "$IMAGE_TAG" && -n "$RELEASE_VERSION" && "$RELEASE_VERSION" != "latest" ]]; then
    IMAGE_TAG="$RELEASE_VERSION"
  fi
  if [[ -z "$RELEASE_VERSION" && -n "$IMAGE_TAG" && "$IMAGE_TAG" != "latest" ]]; then
    RELEASE_VERSION="$IMAGE_TAG"
  fi
}

resolve_upgrade_versions() {
  local stable_tag=""

  if [[ "$VERSION_SET" == true ]]; then
    [[ "$IMAGE_TAG_SET" == true ]] || IMAGE_TAG="$VERSION"
    [[ "$RELEASE_VERSION_SET" == true ]] || RELEASE_VERSION="$VERSION"
  fi

  if [[ -z "$IMAGE_TAG" || -z "$RELEASE_VERSION" ]]; then
    resolve_release_repo
    stable_tag="$(latest_release_tag)"
  fi

  if [[ -n "$stable_tag" && -z "$IMAGE_TAG" ]]; then
    IMAGE_TAG="$stable_tag"
  fi
  if [[ -n "$stable_tag" && -z "$RELEASE_VERSION" ]]; then
    RELEASE_VERSION="$stable_tag"
  fi
  if [[ -z "$IMAGE_TAG" && -n "$RELEASE_VERSION" && "$RELEASE_VERSION" != "latest" ]]; then
    IMAGE_TAG="$RELEASE_VERSION"
  fi
  if [[ -z "$RELEASE_VERSION" && -n "$IMAGE_TAG" && "$IMAGE_TAG" != "latest" ]]; then
    RELEASE_VERSION="$IMAGE_TAG"
  fi
}

detect_release_platform() {
  local os
  local arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) os="linux" ;;
    *) die "binary install currently supports Linux only, got: $os" ;;
  esac

  arch="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported CPU architecture for binary install: $arch" ;;
  esac

  printf '%s %s\n' "$os" "$arch"
}

component_release_url() {
  local component="$1"
  local os="$2"
  local arch="$3"
  local asset="rivo-${component}-${os}-${arch}.tar.gz"
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

ensure_binary_tools() {
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

make_master_binary_config() {
  local dir="$1"
  local config="$dir/config.yaml"
  mkdir -p "$dir/data" "$dir/logs"
  write_file "$config" <<EOF
http:
  listen_addr: ":$HTTP_PORT"
  admin_path: "$ADMIN_PATH"

tcp:
  listen_addr: ":$TCP_PORT"
  secret_key: "$SECRET_KEY"

database:
  driver: "sqlite"
  dsn: "$dir/data/rivo.db"
  auto_migrate: true

auth:
  username: "admin"
  password: "$ADMIN_PASSWORD"

log:
  level: "info"
  file: "$dir/logs/master.log"
  retention_days: 30
EOF
  chmod 0600 "$config"
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

install_binary_files() {
  local component="$1"
  local dir="$2"
  local os
  local arch
  local url
  local archive
  local tmp_dir
  local extracted_bin

  read -r os arch < <(detect_release_platform)
  url="$(component_release_url "$component" "$os" "$arch")"
  archive="$dir/rivo-${component}-${os}-${arch}.tar.gz"
  tmp_dir="$dir/.release-tmp"
  extracted_bin="$tmp_dir/rivo-${component}-${os}-${arch}/rivo-${component}"

  mkdir -p "$dir/bin" "$tmp_dir"
  info "Downloading $url"
  download_to "$url" "$archive"

  rm -rf "$tmp_dir"
  mkdir -p "$tmp_dir"
  tar -xzf "$archive" -C "$tmp_dir"
  [[ -f "$extracted_bin" ]] || die "release archive does not contain $extracted_bin"

  cp "$extracted_bin" "$dir/bin/rivo-${component}"
  chmod 0755 "$dir/bin/rivo-${component}"
  rm -rf "$tmp_dir"
}

make_master_systemd_service() {
  local dir="$1"
  local service="/etc/systemd/system/rivo-master.service"
  write_file "$service" <<EOF
[Unit]
Description=Rivo Master
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$dir
ExecStart=$dir/bin/rivo-master -config $dir/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
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

start_systemd_service() {
  local service="$1"
  systemctl daemon-reload
  systemctl enable "$service"
  systemctl restart "$service"
}

start_master_systemd_service() {
  start_systemd_service rivo-master.service
}

start_agent_systemd_service() {
  start_systemd_service rivo-agent.service
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
  local ip_lc
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

  ip_lc="$(printf '%s' "$ip" | tr '[:upper:]' '[:lower:]')"
  case "$ip_lc" in
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

timestamp_utc() {
  date -u +%Y%m%d%H%M%S
}

backup_file() {
  local path="$1"
  local backup="$path.bak.$(timestamp_utc).$$"
  cp -p "$path" "$backup"
  printf '%s\n' "$backup"
}

replace_compose_service_image() {
  local compose_file="$1"
  local service="$2"
  local image="$3"
  local tmp="$compose_file.tmp.$$"
  local status

  set +e
  awk -v service="$service" -v image="$image" '
    /^  [A-Za-z0-9_-]+:[[:space:]]*$/ {
      current = $0
      sub(/^  /, "", current)
      sub(/:[[:space:]]*$/, "", current)
      in_service = current == service
    }
    in_service && /^    image:[[:space:]]*/ {
      print "    image: \"" image "\""
      replaced = 1
      next
    }
    { print }
    END {
      if (!replaced) {
        exit 42
      }
    }
  ' "$compose_file" >"$tmp"
  status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    rm -f "$tmp"
    if [[ "$status" -eq 42 ]]; then
      die "compose file does not contain service image for: $service"
    fi
    die "failed to update compose file: $compose_file"
  fi

  mv "$tmp" "$compose_file"
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

remove_systemd_service() {
  local service_name="$1"
  local service="/etc/systemd/system/$service_name"
  local touched=false

  if ! command -v systemctl >/dev/null 2>&1; then
    if [[ -e "$service" ]]; then
      die "systemctl is required to remove $service"
    fi
    info "systemctl not found; skipping binary service cleanup."
    return
  fi

  if systemctl is-active --quiet "$service_name" || systemctl is-enabled --quiet "$service_name" || [[ -e "$service" ]]; then
    systemctl stop "$service_name" >/dev/null 2>&1 || true
    systemctl disable "$service_name" >/dev/null 2>&1 || true
    touched=true
  else
    info "$service_name not found; skipping systemd cleanup."
  fi

  if [[ -e "$service" ]]; then
    rm -f "$service"
    info "Removed systemd service: $service"
    touched=true
  fi

  if [[ "$touched" == true ]]; then
    systemctl daemon-reload
    systemctl reset-failed "$service_name" >/dev/null 2>&1 || true
  fi
}

remove_master_systemd_service() {
  remove_systemd_service rivo-master.service
}

remove_agent_systemd_service() {
  remove_systemd_service rivo-agent.service
}

print_master_summary() {
  local host_ip
  local master_addr
  host_ip="$(detect_host_ip)"
  host_ip="${host_ip:-YOUR_SERVER_IP}"
  master_addr="$host_ip:$TCP_PORT"
  cat <<EOF

$PROGRAM Master installed.

Method:    $INSTALL_METHOD
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
EOF
  if [[ -n "$IMAGE_TAG" ]]; then
    printf '  --image-tag %s \\\n' "$IMAGE_TAG"
  fi
  cat <<EOF
  --master $master_addr \\
  --secret "$SECRET_KEY"

Agent install command (binary):
curl -fsSL $INSTALL_URL | sudo bash -s -- agent \\
  --method binary \\
EOF
  if [[ -n "$RELEASE_VERSION" ]]; then
    printf '  --release-version %s \\\n' "$RELEASE_VERSION"
  fi
  cat <<EOF
  --master $master_addr \\
  --secret "$SECRET_KEY"

EOF
}

print_agent_summary() {
  cat <<EOF

$PROGRAM Agent installed.

Master: $MASTER_ADDR
Method: $INSTALL_METHOD

EOF
}

install_master_docker() {
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

install_master_binary() {
  [[ -n "$ADMIN_PATH" ]] || ADMIN_PATH="rivo$(random_hex 8)"
  [[ -n "$ADMIN_PASSWORD" ]] || ADMIN_PASSWORD="$(random_hex 12)"
  [[ -n "$SECRET_KEY" ]] || SECRET_KEY="$(random_base64_32)"
  validate_admin_path "$ADMIN_PATH"

  local dir
  dir="$(target_dir)"
  mkdir -p "$dir"
  make_master_binary_config "$dir"
  install_binary_files master "$dir"
  make_master_systemd_service "$dir"
  start_master_systemd_service
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
  install_binary_files agent "$dir"
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

uninstall_master_binary() {
  local dir
  dir="$(target_dir_for_mode master)"
  validate_install_dir_for_uninstall "$dir"
  remove_master_systemd_service
  remove_install_dir "$dir"
}

uninstall_master_all_methods() {
  local dir
  dir="$(target_dir_for_mode master)"
  validate_install_dir_for_uninstall "$dir"
  compose_down "$dir" "$PURGE"
  remove_master_systemd_service
  remove_install_dir "$dir"
}

uninstall_master() {
  if [[ "$METHOD_SET" != true ]]; then
    uninstall_master_all_methods
    return
  fi

  case "$INSTALL_METHOD" in
    docker)
      uninstall_docker_mode master
      ;;
    binary)
      uninstall_master_binary
      ;;
  esac
}

uninstall_agent_binary() {
  local dir
  dir="$(target_dir_for_mode agent)"
  validate_install_dir_for_uninstall "$dir"
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
  if [[ "$METHOD_SET" != true ]]; then
    uninstall_agent_all_methods
    return
  fi

  case "$INSTALL_METHOD" in
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

  uninstall_master_all_methods
  uninstall_docker_mode single
  uninstall_agent_all_methods
}

uninstall() {
  case "$MODE" in
    master)
      uninstall_master
      ;;
    single)
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

mode_install_exists() {
  local mode="$1"
  local dir
  dir="$(target_dir_for_mode "$mode")"
  case "$mode" in
    master|agent)
      [[ -f "$dir/compose.yml" || -x "$dir/bin/rivo-$mode" || -f "/etc/systemd/system/rivo-$mode.service" ]]
      ;;
    single)
      [[ -f "$dir/compose.yml" ]]
      ;;
    *)
      return 1
      ;;
  esac
}

upgrade_method_for_mode() {
  local mode="$1"
  local dir
  local has_docker=false
  local has_binary=false

  if [[ "$METHOD_SET" == true ]]; then
    printf '%s\n' "$INSTALL_METHOD"
    return
  fi

  if [[ "$mode" == "single" ]]; then
    printf 'docker\n'
    return
  fi

  dir="$(target_dir_for_mode "$mode")"
  [[ -f "$dir/compose.yml" ]] && has_docker=true
  [[ -x "$dir/bin/rivo-$mode" || -f "/etc/systemd/system/rivo-$mode.service" ]] && has_binary=true

  if [[ "$has_docker" == true && "$has_binary" == true ]]; then
    die "both Docker and binary installs were found for $mode; rerun with --method docker or --method binary"
  fi
  if [[ "$has_docker" == true ]]; then
    printf 'docker\n'
    return
  fi
  if [[ "$has_binary" == true ]]; then
    printf 'binary\n'
    return
  fi

  die "$mode install not found under $(target_dir_for_mode "$mode")"
}

print_upgrade_summary() {
  local target="$1"
  local method="$2"
  local version="$3"
  local backup="$4"

  cat <<EOF

$PROGRAM $target upgraded.

Method:  $method
Version: $version
Backup:  $backup

EOF
}

upgrade_docker_mode() {
  local mode="$1"
  local old_mode="$MODE"
  local dir
  local compose_file
  local backup
  local version

  MODE="$mode"
  dir="$(target_dir_for_mode "$mode")"
  compose_file="$dir/compose.yml"
  [[ -f "$compose_file" ]] || die "Docker install not found: $compose_file"

  ensure_docker_compose
  resolve_images
  backup="$(backup_file "$compose_file")"

  case "$mode" in
    master)
      replace_compose_service_image "$compose_file" master "$MASTER_IMAGE"
      version="$MASTER_IMAGE"
      ;;
    agent)
      replace_compose_service_image "$compose_file" agent "$AGENT_IMAGE"
      version="$AGENT_IMAGE"
      ;;
    single)
      replace_compose_service_image "$compose_file" master "$MASTER_IMAGE"
      replace_compose_service_image "$compose_file" agent "$AGENT_IMAGE"
      version="$IMAGE_TAG"
      ;;
    *)
      die "unknown Docker upgrade target: $mode"
      ;;
  esac

  if docker compose -f "$compose_file" pull && docker compose -f "$compose_file" up -d; then
    print_upgrade_summary "$mode" docker "${version:-$IMAGE_TAG}" "$backup"
  else
    cp -p "$backup" "$compose_file"
    docker compose -f "$compose_file" up -d >/dev/null 2>&1 || true
    die "Docker upgrade failed; restored compose file from $backup"
  fi

  MODE="$old_mode"
}

upgrade_binary_component() {
  local component="$1"
  local dir="$2"
  local service="rivo-$component.service"
  local bin="$dir/bin/rivo-$component"
  local backup

  [[ -x "$bin" ]] || die "binary install not found: $bin"
  ensure_binary_tools
  resolve_release_repo

  backup="$(backup_file "$bin")"
  if ! ( install_binary_files "$component" "$dir" ); then
    cp -p "$backup" "$bin"
    die "binary download or install failed; restored $bin from $backup"
  fi

  systemctl daemon-reload
  if systemctl restart "$service"; then
    print_upgrade_summary "$component" binary "$RELEASE_VERSION" "$backup"
  else
    cp -p "$backup" "$bin"
    chmod 0755 "$bin"
    systemctl restart "$service" >/dev/null 2>&1 || true
    die "binary service restart failed; restored $bin from $backup"
  fi
}

upgrade_master_binary() {
  upgrade_binary_component master "$(target_dir_for_mode master)"
}

upgrade_agent_binary() {
  upgrade_binary_component agent "$(target_dir_for_mode agent)"
}

upgrade_target() {
  local method

  case "$MODE" in
    master)
      method="$(upgrade_method_for_mode master)"
      INSTALL_METHOD="$method"
      case "$method" in
        docker) upgrade_docker_mode master ;;
        binary) upgrade_master_binary ;;
      esac
      ;;
    agent)
      method="$(upgrade_method_for_mode agent)"
      INSTALL_METHOD="$method"
      case "$method" in
        docker) upgrade_docker_mode agent ;;
        binary) upgrade_agent_binary ;;
      esac
      ;;
    single)
      INSTALL_METHOD="docker"
      upgrade_docker_mode single
      ;;
    *)
      die "unknown upgrade target: $MODE"
      ;;
  esac
}

upgrade_all() {
  local old_mode="$MODE"
  local upgraded=false
  local mode
  [[ -z "$INSTALL_DIR" ]] || die "--install-dir cannot be used with upgrade all; choose master, agent, or single"

  for mode in master single agent; do
    if mode_install_exists "$mode"; then
      MODE="$mode"
      upgrade_target
      upgraded=true
    else
      info "No $mode install found at $(target_dir_for_mode "$mode"); skipping."
    fi
  done

  MODE="$old_mode"
  [[ "$upgraded" == true ]] || die "no $PROGRAM installation found under $DEFAULT_INSTALL_ROOT"
}

upgrade() {
  case "$MODE" in
    master|agent|single)
      upgrade_target
      ;;
    all)
      upgrade_all
      ;;
  esac
}

prompt_action_choice() {
  local value
  while true; do
    value="$(prompt_text "Action (i=install, u=uninstall, g=upgrade)" "install")"
    value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
    case "$value" in
      install|i) printf 'install\n'; return ;;
      uninstall|u) printf 'uninstall\n'; return ;;
      upgrade|g) printf 'upgrade\n'; return ;;
    esac
    printf 'Please choose: i=install, u=uninstall, g=upgrade\n' >/dev/tty
  done
}

run_interactive_prompts() {
  if [[ "$INTERACTIVE" != true ]]; then
    return 0
  fi

  need_tty
  if [[ -z "$MODE" ]]; then
    COMMAND="$(prompt_action_choice)"
    if [[ "$COMMAND" == "install" ]]; then
      MODE="$(prompt_choice "Install target (master/agent/single)" "master" master agent single)"
    elif [[ "$COMMAND" == "uninstall" ]]; then
      MODE="$(prompt_choice "Uninstall target (master/agent/single/all)" "all" master agent single all)"
    else
      MODE="$(prompt_choice "Upgrade target (master/agent/single/all)" "all" master agent single all)"
    fi
  fi

  if [[ "$COMMAND" == "install" ]]; then
    case "$MODE" in
      master|agent)
        if [[ "$METHOD_SET" != true ]]; then
          INSTALL_METHOD="$(prompt_choice "Install method (docker/binary)" "docker" docker binary)"
          METHOD_SET=true
        fi
        ;;
      single)
        INSTALL_METHOD="docker"
        ;;
    esac

    if [[ "$MODE" == "agent" ]]; then
      if [[ -z "$MASTER_ADDR" ]]; then
        MASTER_ADDR="$(prompt_text "Master TCP address" "")"
      fi
      if [[ -z "$SECRET_KEY" ]]; then
        SECRET_KEY="$(prompt_text "Shared secret_key" "")"
      fi
    fi
  fi
}

main() {
  parse_args "$@"
  run_interactive_prompts
  validate_install_method

  if [[ "$COMMAND" == "uninstall" ]]; then
    uninstall
    exit 0
  fi

  [[ "$PURGE" != true ]] || die "--purge is only supported for uninstall"
  normalize_owner
  resolve_install_url

  if [[ "$COMMAND" == "upgrade" ]]; then
    resolve_upgrade_versions
    upgrade
    exit 0
  fi

  validate_port "--http-port" "$HTTP_PORT"
  validate_port "--tcp-port" "$TCP_PORT"
  resolve_install_versions

  case "$MODE" in
    master)
      case "$INSTALL_METHOD" in
        docker)
          ensure_docker_tools
          resolve_images
          install_master_docker
          ;;
        binary)
          ensure_binary_tools
          resolve_release_repo
          install_master_binary
          ;;
      esac
      ;;
    agent)
      case "$INSTALL_METHOD" in
        docker)
          ensure_docker_tools
          resolve_images
          install_agent_docker
          ;;
        binary)
          ensure_binary_tools
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
