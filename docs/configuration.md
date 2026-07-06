# 配置说明

本文说明 Rivo Master 和 Agent 使用的运行配置。

## 通信密钥

Master 和 Agent 必须使用同一个 `secret_key`。该值必须是标准 Base64，并且解码后不少于 32 字节随机数据。

生产环境建议使用：

```bash
openssl rand -base64 32
```

不要把普通密码转成 Base64 使用，也不要把生产密钥提交到公开仓库。

## Master 配置

示例：

```yaml
http:
  listen_addr: ":8080"
  admin_path: "replaceWithRandomPath"

tcp:
  listen_addr: ":9443"
  secret_key: "replace-with-generated-secret"

database:
  driver: "sqlite"
  dsn: "data/rivo.db"
  auto_migrate: true

auth:
  username: "admin"
  password: "change-me-admin-password"

log:
  level: "info"
  file: "logs/master.log"
  retention_days: 30
```

### `http`

- `listen_addr`：HTTP API 和面板监听地址。`":8080"` 表示监听所有网卡。
- `admin_path`：后台管理路径。必须超过 5 个字符，并且只能包含英文字母和数字。

前台展示页挂载在 `/`。后台面板挂载在 `/<admin_path>`，Master 启动时会在控制台输出完整后台地址。

### `tcp`

- `listen_addr`：Agent 连接 Master 使用的 TCP 地址。
- `secret_key`：Master 和 Agent 共享的 Base64 密钥，用于注册认证和会话密钥派生。

### `database`

SQLite 适合快速体验和单机部署：

```yaml
database:
  driver: "sqlite"
  dsn: "data/rivo.db"
  auto_migrate: true
```

MySQL 示例：

```yaml
database:
  driver: "mysql"
  dsn: "rivo:replace-with-database-password@tcp(127.0.0.1:3306)/rivo?charset=utf8mb4&parseTime=True&loc=Local"
  auto_migrate: true
```

### `auth`

后台使用单个配置账号登录，没有独立用户体系。

- `username`：后台用户名。
- `password`：后台密码。

修改用户名或密码后，浏览器里旧 token 会失效，需要重新登录。

### `log`

- `level`：`debug`、`info`、`warn` 或 `error`。
- `file`：本地日志文件路径。
- `retention_days`：本地日志和数据库系统日志的保留天数。

## Agent 配置

示例：

```yaml
master_addr: "127.0.0.1:9443"
secret_key: "replace-with-generated-secret"

agent:
  node_id: ""
  state_file: "data/agent-state.json"

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
```

### 根字段

- `master_addr`：Master TCP 地址。配置文件里建议显式写端口。
- `secret_key`：必须和 Master 的 `tcp.secret_key` 完全一致。

### `agent`

- `node_id`：固定节点 ID。留空时 Agent 首次启动会自动生成 16 位稳定 ID。
- `state_file`：本地 JSON 状态文件，用于保存自动生成的 `node_id` 和命令行启动时传入的连接参数。

### `public_ip`

Agent 可以请求外部接口识别公网 IPv4 和 IPv6 出口地址。

- `enabled`：是否启用公网 IP 嗅探。
- `timeout_ms`：单个外部接口请求超时时间。
- `refresh_interval_seconds`：公网 IP 刷新间隔，刷新结果会随心跳上报。
- `ipv4_enabled` / `ipv6_enabled`：分别控制 IPv4 和 IPv6 嗅探。
- `ipv4_endpoints` / `ipv6_endpoints`：返回纯 IP 文本的外部接口列表。

启用后，Agent 会对 IPv4 接口强制使用 `tcp4`，对 IPv6 接口强制使用 `tcp6`。

## 运行时设置

显示选项、告警阈值、通知渠道、主题、数据保留周期和节点级配置主要存储在数据库里，并通过后台管理面板修改。多数设置保存后不需要重启 Master。
