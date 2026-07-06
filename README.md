# Rivo

Rivo 是一套面向私有服务器、VPS、LXC 容器和小规模基础设施的轻量级自托管监控面板。

项目由 Go Master、Go Agent 和 Vue 管理面板组成。Agent 通过加密 TCP 长连接接入 Master，负责采集系统指标、执行网络探测、上报快照数据，并接收后台下发的运行配置。

## 快速部署

推荐使用已经发布到 GHCR 的 Docker 镜像部署。Master 负责面板、API、数据库和 Agent 连接管理；Agent 运行在需要监控的服务器上。

默认镜像名：

- `ghcr.io/nyxarrival/rivo-master:latest`
- `ghcr.io/nyxarrival/rivo-agent:latest`

仓库第一次发布到 GitHub 后，`main` 分支和 `v*` 标签会通过 GitHub Actions 自动构建并推送镜像。

Master、单机模式和 Docker Agent 需要先安装 Docker 与 Docker Compose 插件。二进制 Agent 不需要 Docker，但需要 Linux、systemd、`curl` 或 `wget`，以及 `tar`。

### 一键安装

安装脚本会自动生成 `admin_path`、后台密码、`secret_key` 和运行配置，默认安装到 `/opt/rivo/<mode>`。Docker 方式会生成 `compose.yml`，二进制 Agent 会生成 systemd service。

安装 Master：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- master
```

安装完成后控制台会输出：

- 前台地址：`http://服务器IP:8080`
- 后台地址：`http://服务器IP:8080/<admin_path>`
- 后台账号和随机密码
- Agent 安装命令

如果输出的 `--master` 是 `172.x`、`10.x`、`192.168.x` 等内网地址，跨服务器部署 Agent 时需要替换成 Agent 能访问到的公网 IP、域名、VPC/VPN 内网地址。

在需要监控的服务器上安装 Agent，默认使用 Docker：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- agent \
  --master MASTER_IP:9443 \
  --secret "MASTER输出的secret_key"
```

也可以使用二进制 Agent。脚本会自动识别 Linux `amd64` / `arm64`，从最新 GitHub Release 下载对应的 `rivo-agent-linux-*.tar.gz`，并注册为 `rivo-agent.service`：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- agent \
  --method binary \
  --master MASTER_IP:9443 \
  --secret "MASTER输出的secret_key"
```

如果 Master 和 Agent 在同一台机器上，也可以单机安装：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- single
```

常用参数：

- `--http-port 8080`：Master HTTP 端口。
- `--tcp-port 9443`：Agent 接入端口。
- `--admin-path value`：自定义后台路径，必须超过 5 个字符，只能包含英文字母和数字。
- `--admin-password value`：自定义后台密码。
- `--secret value`：自定义 Master 和 Agent 共用密钥。
- `--image-tag v0.1.0`：使用指定镜像标签。
- `--method docker|binary`：Agent 安装方式，默认 `docker`。
- `--release-version v0.1.0`：二进制 Agent 使用指定 Release；不传则使用最新 Release。
- `--image-owner owner`：覆盖默认镜像和 Release owner，默认 `nyxarrival`。
- `--force`：覆盖脚本生成的配置、Compose 或 systemd service 文件。

安装指定版本时，Docker 方式使用镜像标签：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- agent \
  --image-tag v0.1.0 \
  --master MASTER_IP:9443 \
  --secret "MASTER输出的secret_key"
```

二进制方式使用 Release 版本：

```bash
curl -fsSL https://raw.githubusercontent.com/nyxarrival/rivo/main/install.sh | sudo bash -s -- agent \
  --method binary \
  --release-version v0.1.0 \
  --master MASTER_IP:9443 \
  --secret "MASTER输出的secret_key"
```

Agent 报 `master closed connection during register handshake` 时，优先检查 Master 和 Agent 使用的 `secret_key` 是否一致。更多说明见 [Agent 说明](docs/agent.md)。

### 手动 Docker Compose

如果不想使用安装脚本，可以复制 `deploy/` 里的模板手动部署。

Master：

```bash
mkdir -p /opt/rivo/master
cp deploy/config.master.example.yaml /opt/rivo/master/config.yaml
cp deploy/compose.master.yml /opt/rivo/master/compose.yml
cd /opt/rivo/master
RIVO_MASTER_IMAGE=ghcr.io/nyxarrival/rivo-master:latest docker compose up -d
```

启动前需要编辑 `/opt/rivo/master/config.yaml`，至少修改：

- `tcp.secret_key`：和 Agent 保持一致。
- `auth.password`：后台登录密码。
- `http.admin_path`：后台访问路径，必须超过 5 个字符，只能包含英文字母和数字。

Agent：

```bash
mkdir -p /opt/rivo/agent
cp deploy/config.agent.example.yaml /opt/rivo/agent/config.yaml
cp deploy/compose.agent.yml /opt/rivo/agent/compose.yml
cd /opt/rivo/agent
RIVO_AGENT_IMAGE=ghcr.io/nyxarrival/rivo-agent:latest docker compose up -d
```

启动前需要编辑 `/opt/rivo/agent/config.yaml`，把 `master_addr` 改成 `MASTER_IP:9443`，并把 `secret_key` 改成和 Master 一致。

手动 Compose 模板可以通过环境变量覆盖镜像名；下面命令使用 `nyxarrival` 发布的 GHCR 镜像：

```bash
RIVO_MASTER_IMAGE=ghcr.io/nyxarrival/rivo-master:latest docker compose up -d
RIVO_AGENT_IMAGE=ghcr.io/nyxarrival/rivo-agent:latest docker compose up -d
```

更完整的本地构建、MySQL 和 Makefile 部署方式见 [部署说明](depoly/README.md)。

### 二进制 Release

如果不使用 Docker，可以从 GitHub Release 下载二进制包。Master 二进制已经内嵌后台和默认主题静态资源。

Release 会包含：

- `rivo-master-linux-amd64.tar.gz`
- `rivo-master-linux-arm64.tar.gz`
- `rivo-agent-linux-amd64.tar.gz`
- `rivo-agent-linux-arm64.tar.gz`
- `rivo-master-darwin-amd64.tar.gz`
- `rivo-master-darwin-arm64.tar.gz`
- `rivo-agent-darwin-amd64.tar.gz`
- `rivo-agent-darwin-arm64.tar.gz`
- `rivo-master-windows-amd64.tar.gz`
- `rivo-agent-windows-amd64.tar.gz`
- `checksums.txt`

Master：

```bash
VERSION=v0.1.0
OS=linux # linux / darwin / windows
ARCH=amd64 # linux/darwin 可用 arm64；windows 当前仅 amd64
curl -LO "https://github.com/nyxarrival/rivo/releases/download/${VERSION}/rivo-master-${OS}-${ARCH}.tar.gz"
tar -xzf "rivo-master-${OS}-${ARCH}.tar.gz"
cd "rivo-master-${OS}-${ARCH}"
cp config.example.yaml config.yaml
./rivo-master -config config.yaml
```

Agent：

```bash
VERSION=v0.1.0
OS=linux # linux / darwin / windows
ARCH=amd64 # linux/darwin 可用 arm64；windows 当前仅 amd64
curl -LO "https://github.com/nyxarrival/rivo/releases/download/${VERSION}/rivo-agent-${OS}-${ARCH}.tar.gz"
tar -xzf "rivo-agent-${OS}-${ARCH}.tar.gz"
cd "rivo-agent-${OS}-${ARCH}"
cp config.example.yaml config.yaml
./rivo-agent -config config.yaml
```

Windows 包内的可执行文件是 `rivo-master.exe` 和 `rivo-agent.exe`；macOS 运行下载的二进制时，可能需要按系统提示允许来自终端的可执行文件。

## 界面预览

<p>
  <img src="screenshot/ScreenShot_2026-07-04_180759_122.png" alt="默认主题首页" width="100%">
</p>
<p>
  <img src="screenshot/ScreenShot_2026-07-04_180826_757.png" alt="后台节点列表" width="100%">
</p>
<p>
  <img src="screenshot/ScreenShot_2026-07-04_181347_779.png" alt="Ping 节点管理" width="100%">
</p>
<p>
  <img src="screenshot/ScreenShot_2026-07-04_181400_985.png" alt="主题管理" width="100%">
</p>
<p>
  <img src="screenshot/ScreenShot_2026-07-04_181415_098.png" alt="通知设置" width="100%">
</p>
<p>
  <img src="screenshot/ScreenShot_2026-07-04_181452_670.png" alt="Cyberpunk 主题总览" width="100%">
</p>

## 核心功能

- Agent 首次连接自动注册，支持固定或自动生成 `node_id`。
- Master 可从后台下发 Agent 运行配置，包含心跳、指标、Ping 任务和快照采集策略。
- 采集 CPU、负载、内存、Swap、磁盘、运行时长、网络速率和累计流量。
- 支持 TCP Ping、ICMP Ping，支持 IPv4、IPv6 和 Auto 模式，以及按 Agent 选择探测任务。
- 支持进程与连接快照，可按需开启敏感信息脱敏。
- 支持节点地区、服务商、线路、Tag、公网 IPv4/IPv6 和多公网 IP 记录。
- 支持 VPS 套餐资产管理：付费周期、价格、币种、服务周期、套餐流量、已用流量、剩余流量和剩余价值。
- 支持离线、流量、CPU、内存、磁盘、负载、服务到期等告警。
- 支持企业微信、Telegram 和 SMTP 邮件通知，SMTP 支持无加密、STARTTLS 和 SSL/TLS。
- 支持前台主题上传与切换，内置默认主题和 Cyberpunk 主题。
- 支持自定义后台访问路径，首次使用配置文件，后续可在后台修改。
- 支持系统日志、指标数据和 Ping 数据保留周期配置。
- 支持 SQLite 快速部署，也支持 MySQL 持久化部署。

## 架构

```text
Browser
  |
  | HTTP :8080
  v
Master
  |-- API + Panel
  |-- SQLite / MySQL
  |-- alerts, logs, assets, retention
  |
  | encrypted TCP :9443
  v
Agent
  |-- metrics collector
  |-- TCP / ICMP probe worker
  |-- process and connection snapshot collector
```

Master 负责面板、API、数据库、告警、日志、配置下发和 Agent 会话管理。Agent 负责采集本机数据，并通过 ChaCha20-Poly1305 加密协议上报给 Master。

## 源码运行

源码运行更适合开发和调试。正式部署建议优先使用 Docker Compose。

```bash
go run ./cmd/master -config configs/master.example.yaml
go run ./cmd/agent -config configs/agent.example.yaml
```

使用源码运行前，同样需要把 `configs/master.example.yaml` 和 `configs/agent.example.yaml` 里的 `secret_key` 改成同一个随机密钥。

## 开发

前端开发：

```bash
cd panel
pnpm install
pnpm dev
```

常用命令：

```bash
go test ./...
make build
make -f depoly/Makefile build
```

正式构建的 Master 二进制会通过 Go embed 内嵌 Panel 静态资源。前台主题 ZIP 包需要包含 `theme.json` 和 `dist/index.html`。

## 文档

- [部署说明](depoly/README.md)：Docker、Compose、Makefile、MySQL 和正式打包。
- [配置说明](docs/configuration.md)：Master 和 Agent 配置项。
- [Agent 说明](docs/agent.md)：`node_id`、`state_file`、公网 IP 嗅探和多 Agent 运行。
- [通信协议](docs/protocol.md)：注册握手、密钥派生、消息加密和数据流。

## 安全注意事项

- 生产环境必须替换示例 `secret_key`。
- `http.admin_path` 建议使用随机字符串，且长度必须超过 5 个字符。
- 必须修改默认后台密码。
- 不要提交运行时配置、数据库文件、日志文件或 Agent 状态文件。
- 尽量通过防火墙或反向代理规则限制 `8080` 和 `9443` 的访问范围。
- 进程与连接快照可能包含敏感进程名、端口和地址；多人查看面板时建议开启脱敏。

## 目录结构

```text
cmd/master                 Master 入口
cmd/agent                  Agent 入口
internal/master/api         HTTP API 和 Panel 路由
internal/master/tcp         Master TCP 服务
internal/agent              Agent 客户端、配置和采集器
internal/protocol           加密通信协议
panel                       Vue 后台面板和默认前台
themes/cyberpunk            Cyberpunk 前台主题
configs                     配置示例
migrations                  初始化 SQL
depoly                      Docker 和正式构建文件
```

## 项目状态

Rivo 仍处于快速开发阶段，数据库结构、后台设置和 Agent 运行配置可能继续调整。

## License

Rivo 使用 [GNU Affero General Public License v3.0 or later](LICENSE) 开源。

这意味着如果你修改 Rivo 并通过网络服务提供给用户使用，也需要向这些用户提供对应修改版本的源代码。
