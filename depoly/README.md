# Rivo 部署说明

`depoly` 目录保存 Rivo 的正式部署脚本，支持两种部署方式：

- Docker / Docker Compose：适合服务器部署，Master、Agent、MySQL 可按需拆分运行。
- 单二进制：把 Panel 编译进 Master，直接运行 `rivo-master` 和 `rivo-agent`。

## 目录结构

```text
depoly/
  Makefile                 # 统一构建和部署入口
  README.md                # 部署说明
  master/
    Dockerfile             # Master 镜像构建文件，包含 Panel 构建和 Go 编译
    compose.yml            # Master + 可选 MySQL 的 compose 配置
    .env.example           # Master 部署参数示例
    .env                   # Master 实际部署参数，本地使用，不建议提交
    config.yaml            # Master 运行配置，挂载到容器 /app/config.yaml
  agent/
    Dockerfile             # Agent 镜像构建文件
    compose.yml            # Agent compose 配置
    .env.example           # Agent 部署参数示例
    .env                   # Agent 实际部署参数，本地使用，不建议提交
```

## 准备工作

首次使用建议先复制示例配置：

```bash
cp depoly/master/.env.example depoly/master/.env
cp depoly/master/config.example.yaml depoly/master/config.yaml
cp depoly/agent/.env.example depoly/agent/.env
```

如果你在 devcontainer 内执行 Docker 命令，但 Docker daemon 实际运行在宿主机，`depoly/master/.env` 里的 `RIVO_MASTER_CONFIG_FILE` 必须写宿主机绝对路径，例如：

```dotenv
RIVO_MASTER_CONFIG_FILE=/path/to/rivo/depoly/master/config.yaml
```

原因是 Docker bind mount 由宿主机 Docker daemon 解析，`/workspaces/rivo/...` 这类 devcontainer 内路径在宿主机侧通常不存在。

## Master Docker 部署

启动 Master：

```bash
make -f depoly/Makefile compose-master-up
```

停止 Master：

```bash
make -f depoly/Makefile compose-master-down
```

只构建 Master 镜像，不启动容器：

```bash
make -f depoly/Makefile docker-master
```

Master 镜像不会内置 `config.yaml` 这类运行配置；`depoly/master/.env` 里的 `RIVO_MASTER_CONFIG_FILE` 会把宿主机配置文件挂载到容器内 `/app/config.yaml`。

后台管理面板的访问路径来自 Master 配置里的 `http.admin_path`，必须超过 5 个字符，并且只能包含英文字母和数字。前台展示页继续访问根路径，后台完整地址会在 Master 启动时输出到控制台。

## 数据库模式

Master 支持 `sqlite` 和 `mysql`，通过 `depoly/master/.env` 控制。

SQLite 模式：

```dotenv
RIVO_DATABASE_DRIVER=sqlite
RIVO_DATABASE_DSN=data/rivo.db
```

这种模式只启动 Master 容器，数据库文件保存在 Docker volume `master-data` 里。

MySQL 模式：

```dotenv
RIVO_DATABASE_DRIVER=mysql
RIVO_DATABASE_DSN=rivo:replace-with-database-password@tcp(mysql:3306)/rivo?charset=utf8mb4&parseTime=True&loc=Local
```

这种模式执行 `compose-master-up` 时会自动启用 compose profile，并同时启动 `master` 和 `mysql` 两个服务。

也可以显式启动 MySQL profile：

```bash
make -f depoly/Makefile compose-mysql-up
```

## Agent Docker 部署

编辑 Agent 参数：

```bash
vim depoly/agent/.env
```

至少需要设置：

```dotenv
RIVO_MASTER_ADDR=你的Master地址:9443
RIVO_SECRET_KEY=和Master一致的secret_key
```

`RIVO_SECRET_KEY` 必须是标准 Base64，且解码后不少于 32 字节。它应该是 32 字节随机数据的 Base64 表示，不是普通密码，也不建议把自定义文本手动转成 Base64。

推荐生成方式：

```bash
openssl rand -base64 32
```

生成后同步写入 Master 配置的 `tcp.secret_key` 和 Agent 的 `RIVO_SECRET_KEY`。

Docker Agent 默认会启用公网 IP 嗅探：启动后分别用 IPv4/IPv6 请求外部查询接口，并把观测到的公网出口 IP 上报给 Master。Master 会保留同一个 Agent 历史上报过的多个公网 IPv4/IPv6，首页默认展示首次观测到的公网 IPv4；如果有公网 IPv6，也会同时展示。

启动 Agent：

```bash
make -f depoly/Makefile compose-agent-up
```

停止 Agent：

```bash
make -f depoly/Makefile compose-agent-down
```

只构建 Agent 镜像，不启动容器：

```bash
make -f depoly/Makefile docker-agent
```

Agent 使用 `network_mode: host`、`pid: host`、`NET_RAW`，目的是采集宿主机指标、进程连接信息，并支持 ICMP Ping。

## 单二进制构建

构建 Master：

```bash
make -f depoly/Makefile master
```

这个命令会先构建 Panel，再把 `panel/dist` 同步到 Go embed 目录，最后生成：

```text
dist/rivo-master
```

构建 Agent：

```bash
make -f depoly/Makefile agent
```

生成：

```text
dist/rivo-agent
```

同时构建 Master 和 Agent：

```bash
make -f depoly/Makefile build
```

## 交叉编译

当前 devcontainer 基于 Debian Go 镜像。Master 使用 SQLite 时会引入 `github.com/mattn/go-sqlite3`，因此交叉编译 Master 需要开启 cgo，并为目标平台准备对应的 C 交叉编译器。

Agent 不依赖 SQLite，交叉编译时可以关闭 cgo。注意 `master` 目标的输出名由 `MASTER_BIN` 控制，`agent` 目标的输出名由 `AGENT_BIN` 控制。

先确认当前容器平台：

```bash
go env GOOS GOARCH
```

如果只编译当前 devcontainer 平台的 Linux 二进制，直接执行：

```bash
make -f depoly/Makefile master MASTER_BIN=rivo-master-linux-amd64
```

如果 `go env GOARCH` 显示 `arm64`，输出名建议改成：

```bash
make -f depoly/Makefile master MASTER_BIN=rivo-master-linux-arm64
```

从 devcontainer 编译 Linux arm64：

```bash
sudo apt-get update
sudo apt-get install -y gcc-aarch64-linux-gnu
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc \
  make -f depoly/Makefile master MASTER_BIN=rivo-master-linux-arm64
```

编译 Agent Linux arm64：

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  make -f depoly/Makefile agent AGENT_BIN=rivo-agent-linux-arm64
```

从 devcontainer 编译 Linux amd64：

```bash
sudo apt-get update
sudo apt-get install -y gcc-x86-64-linux-gnu
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc \
  make -f depoly/Makefile master MASTER_BIN=rivo-master-linux-amd64
```

编译 Agent Linux amd64：

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  make -f depoly/Makefile agent AGENT_BIN=rivo-agent-linux-amd64
```

从 devcontainer 编译 Windows amd64：

```bash
sudo apt-get update
sudo apt-get install -y gcc-mingw-w64-x86-64
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  make -f depoly/Makefile master MASTER_BIN=rivo-master-windows-amd64.exe
```

编译 Agent Windows amd64：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  make -f depoly/Makefile agent AGENT_BIN=rivo-agent-windows-amd64.exe
```

Linux devcontainer 内不建议交叉编译 macOS 版本，因为 cgo 需要 macOS SDK 和对应 toolchain。需要 macOS 产物时，建议在 macOS 本机或 macOS CI runner 上执行：

```bash
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 CC="clang -arch arm64" \
  make -f depoly/Makefile master MASTER_BIN=rivo-master-darwin-arm64
```

每次执行 `master` 目标都会先构建 Panel 并同步到 Go embed 目录。连续编译多个平台时，务必设置不同的 `MASTER_BIN` 或 `DIST_DIR`，避免产物互相覆盖。

编译后可以检查产物平台：

```bash
file dist/rivo-master-linux-amd64
```

## Make 命令说明

| 命令 | 作用 |
| --- | --- |
| `make -f depoly/Makefile help` | 显示可用命令列表。 |
| `make -f depoly/Makefile panel-build` | 进入 `panel` 目录执行 `pnpm install --frozen-lockfile` 和 `pnpm build`，只构建前端面板。 |
| `make -f depoly/Makefile panel-embed` | 先执行 `panel-build`，再把 `panel/dist` 同步到 `internal/master/web/dist`，用于 Go embed。 |
| `make -f depoly/Makefile master` | 构建单二进制 Master，输出到 `dist/rivo-master`。会自动执行 `panel-embed`。 |
| `make -f depoly/Makefile agent` | 构建单二进制 Agent，输出到 `dist/rivo-agent`。 |
| `make -f depoly/Makefile build` | 同时构建 Master 和 Agent 两个单二进制文件。 |
| `make -f depoly/Makefile docker-master` | 构建 Master Docker 镜像，不启动容器。 |
| `make -f depoly/Makefile docker-agent` | 构建 Agent Docker 镜像，不启动容器。 |
| `make -f depoly/Makefile docker` | 同时构建 Master 和 Agent Docker 镜像。 |
| `make -f depoly/Makefile compose-master-up` | 构建并启动 Master。若 `RIVO_DATABASE_DRIVER=mysql`，会同时启动 MySQL。 |
| `make -f depoly/Makefile compose-mysql-up` | 显式以 MySQL profile 启动 Master + MySQL。 |
| `make -f depoly/Makefile compose-agent-up` | 构建并启动 Agent。 |
| `make -f depoly/Makefile compose-master-down` | 停止并移除 Master compose 服务。若当前数据库模式是 MySQL，也会处理 MySQL profile。 |
| `make -f depoly/Makefile compose-agent-down` | 停止并移除 Agent compose 服务。 |
| `make -f depoly/Makefile clean` | 删除 `dist` 和 `panel/dist` 构建产物。 |

## Make 变量覆盖

Makefile 支持通过命令行覆盖部分变量：

```bash
make -f depoly/Makefile master VERSION=v1.0.0
make -f depoly/Makefile agent CGO_ENABLED=1
make -f depoly/Makefile docker-master MASTER_ENV=/path/to/master.env
```

常用变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DIST_DIR` | `项目根目录/dist` | 单二进制输出目录。 |
| `MASTER_ENV` | `depoly/master/.env` | Master compose 使用的环境变量文件。 |
| `AGENT_ENV` | `depoly/agent/.env` | Agent compose 使用的环境变量文件。 |
| `GO` | `go` | Go 命令路径。 |
| `PNPM` | `pnpm` | pnpm 命令路径。 |
| `DOCKER_COMPOSE` | `docker compose` | Docker Compose 命令。 |
| `VERSION` | `git describe --tags --always --dirty` 或 `dev` | 构建版本号，Agent 会写入 `agent_version`。 |
| `CGO_ENABLED` | `1` | 是否开启 cgo。SQLite 需要开启。 |
| `GOOS` | 当前 Go 默认值 | Go 目标操作系统，例如 `linux`、`windows`、`darwin`。 |
| `GOARCH` | 当前 Go 默认值 | Go 目标架构，例如 `amd64`、`arm64`。 |
| `CC` | 系统默认 C 编译器 | cgo 使用的 C 编译器。交叉编译 Master 时需要设置为目标平台交叉编译器。 |
| `MASTER_BIN` | `rivo-master` | Master 单二进制输出文件名。 |
| `AGENT_BIN` | `rivo-agent` | Agent 单二进制输出文件名。 |

## 镜像构建说明

Master 和 Agent Docker 镜像都使用 Alpine 作为构建和运行基础镜像。

Master 构建流程：

```text
node:22-alpine 构建 Panel
golang:1.24-alpine 编译 rivo-master，并嵌入 Panel dist
alpine:3.21 作为最终运行镜像
```

Agent 构建流程：

```text
golang:1.24-alpine 编译 rivo-agent
alpine:3.21 作为最终运行镜像
```

Go 二进制是在 Docker build 的 builder 阶段编译，再复制到最终运行层，不依赖宿主机提前编译。

## 常见问题

### `/app/config.yaml: is a directory`

通常是 `RIVO_MASTER_CONFIG_FILE` 指向的宿主机文件不存在，Docker 自动创建了同名目录。当前 compose 已设置 `create_host_path: false`，后续会直接报挂载错误。

处理方式：

```bash
cp depoly/master/config.example.yaml depoly/master/config.yaml
```

如果在 devcontainer 内执行 Docker 命令，需要把 `RIVO_MASTER_CONFIG_FILE` 改成宿主机绝对路径。

### `127.0.0.1:8080` 无响应，但 `localhost:8080` 正常

可能是 VS Code 端口转发进程占用了 `127.0.0.1:8080`。前台根路径可以直接访问：

```text
http://localhost:8080/
```

后台页面地址以 Master 启动日志输出为准。

或者修改 `depoly/master/.env`：

```dotenv
RIVO_HTTP_PORT=18080
```

然后重新启动：

```bash
make -f depoly/Makefile compose-master-up
```
