# Agent 说明

## 启动 Agent

本地开发推荐：

```bash
go run ./cmd/agent -config configs/agent.example.yaml
```

也可以完全使用命令行参数：

```bash
go run ./cmd/agent \
  --master 127.0.0.1:9443 \
  --secret-key "replace-with-generated-secret" \
  --node-id "agent-01"
```

如果 `--master` 不带端口，Agent 默认补 `9443`。

## Node ID

Agent 支持两种节点 ID 模式：

- 固定 ID：设置 `agent.node_id` 或传入 `--node-id`。
- 自动 ID：保持 `agent.node_id` 为空，Agent 首次启动会生成 16 位稳定 ID，并写入 `state_file`。

同一台宿主机运行多个 Agent 时，每个 Agent 必须使用不同的固定 `node_id`，或使用不同的 `state_file`。

## State File

默认配置：

```yaml
agent:
  state_file: "data/agent-state.json"
```

状态文件会保存：

- 自动生成的 `node_id`
- 命令行传入的 `master_addr`
- 命令行传入的 `secret_key`

这对命令行启动很方便，但本地调试时也可能造成混淆。例如 `data/agent-state.json` 里保留了旧 `secret_key`，而配置文件已经换成新密钥，Agent 仍可能拿旧密钥去注册。

如果 Master 在注册阶段关闭连接，按顺序检查：

1. Master 的 `tcp.secret_key`
2. Agent 配置里的 `secret_key`
3. `data/agent-state.json`
4. 命令行传入的 `--secret-key`

## 公网 IP 嗅探

Agent 可以请求外部 IPv4 / IPv6 接口，并把观测到的公网出口地址上报给 Master。这适合多网卡、NAT、隧道或 Master 看到的 TCP 来源 IP 不是你希望展示的公网 IP 的场景。

如果运行环境不允许 Agent 访问外部 HTTP 接口，可以关闭：

```yaml
public_ip:
  enabled: false
```

## ICMP Ping 权限

TCP Ping 不需要特殊权限。ICMP Ping 需要原始套接字权限。

Docker Agent 部署文件已经配置 `NET_RAW`。直接在宿主机运行时，需要给予足够权限，或者只使用 TCP Ping 任务。

## 运行时配置下发

Master 可以向在线 Agent 下发运行配置：

- 心跳间隔
- 指标上报间隔
- Ping 任务分配
- 快照开关
- 进程和连接采集数量上限

后台保存相关设置后，在线 Agent 会收到配置更新，不需要重启 Agent。
