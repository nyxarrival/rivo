# 通信协议

Rivo 在 Agent 和 Master 之间使用自定义 TCP 协议。

## 消息帧

每条消息格式为：

```text
4 字节大端长度前缀
JSON 消息体
```

连接开始时会先进行明文注册握手。注册成功后，业务消息都走加密封装。

## 注册流程

1. Agent 建立到 Master 的 TCP 连接。
2. Agent 发送明文 `register` 消息。
3. 消息中包含 `agent_nonce`，以及基于共享 `secret_key` 生成的 HMAC。
4. Master 校验 HMAC。
5. 校验通过后，Master 返回明文 `register_ack`，其中包含 `master_nonce`。

如果校验失败，Master 会关闭连接。Agent 侧通常会看到注册握手 EOF 或连接被关闭。

## 会话密钥

注册完成后，双方使用以下材料派生会话密钥：

```text
secret_key + agent_nonce + master_nonce
```

HKDF 会派生两把方向性密钥：

- `AgentToMaster`
- `MasterToAgent`

## 加密消息

业务消息会封装成 `encrypted` 消息，并使用 ChaCha20-Poly1305 加密和认证。

加密内容包括：

- 初始运行配置
- 心跳
- 指标上报
- Ping 探测结果
- 进程与连接快照
- 配置更新
- 主动拉取指标请求

## 运维注意事项

- Master 和 Agent 的 `secret_key` 必须完全一致。
- `secret_key` 应该是 32 字节随机数据的 Base64 表示。
- 轮换密钥时需要同时更新 Master 和 Agent，并让 Agent 重新连接。
- TCP 传输本身不依赖 HTTPS 或 TLS，因为注册后的应用层消息已经加密。
