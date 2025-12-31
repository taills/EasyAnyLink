# QUIC传输和单向TLS验证说明

## 概述

EasyAnyLink现已升级为使用QUIC协议作为传输层，并采用单向TLS验证机制。这些变更带来更好的性能、更简化的部署和更强的安全性。

## 主要变更

### 1. QUIC传输协议

**之前**: 使用传统的TCP作为gRPC传输层
**现在**: 强制使用QUIC (Quick UDP Internet Connections) 作为传输层

#### QUIC的优势

- **更快的连接建立**: QUIC集成了TLS 1.3握手，0-RTT连接恢复
- **更好的多路复用**: 避免TCP的队头阻塞问题
- **连接迁移**: 支持IP地址变更时保持连接（适合移动设备）
- **内置加密**: 所有数据包默认加密
- **更好的拥塞控制**: 改进的丢包恢复算法

### 2. 单向TLS验证

**之前**: 双向TLS (mTLS) - Server和Agent都需要证书
**现在**: 单向TLS - 仅Server需要证书，Agent验证Server

#### 单向TLS的优势

- **简化部署**: Agent不再需要证书配置
- **兼容Let's Encrypt**: Server可使用免费的公共CA证书
- **更容易扩展**: 新增Agent无需生成和分发证书
- **标准化**: 使用系统根CA池，与浏览器HTTPS一致
- **用户认证**: 通过user_key API密钥进行身份验证

## 配置变更

### Server配置

**旧配置** (已废弃):
```json
{
    "listen": ":8228",
    "tls": {
        "cert_file": "./certs/server.crt",
        "key_file": "./certs/server.key",
        "ca_file": "./certs/ca.crt",
        "min_version": "TLS1.3"
    },
    "security": {
        "require_client_certs": true
    }
}
```

**新配置**:
```json
{
    "listen": ":8228",
    "cert_file": "./certs/server.crt",
    "key_file": "./certs/server.key",
    "security": {
        "session_timeout": 1440,
        "max_failed_auth": 5
    }
}
```

### Agent配置

**旧配置** (已废弃):
```json
{
    "mode": "client",
    "server": "server.example.com:8228",
    "user_key": "your-api-key",
    "tls": {
        "cert_file": "./certs/client.crt",
        "key_file": "./certs/client.key",
        "ca_file": "./certs/ca.crt"
    }
}
```

**新配置**:
```json
{
    "mode": "client",
    "server": "server.example.com:8228",
    "user_key": "your-api-key"
}
```

## 证书要求

### Server端

Server需要有效的TLS证书，可以选择：

1. **Let's Encrypt证书** (推荐用于公网部署)
   ```bash
   # 使用certbot自动获取
   certbot certonly --standalone -d your-server.example.com
   
   # 配置文件指向
   "cert_file": "/etc/letsencrypt/live/your-server.example.com/fullchain.pem",
   "key_file": "/etc/letsencrypt/live/your-server.example.com/privkey.pem"
   ```

2. **自签名证书** (仅用于测试或内网部署)
   ```bash
   # 生成自签名证书
   openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
     -subj "/CN=your-server.example.com"
   
   # 注意: 使用自签名证书时，Agent需要添加证书到系统信任库
   ```

### Agent端

Agent不再需要证书文件，直接使用系统的根CA证书池来验证Server证书。

**如果Server使用自签名证书**，需要将Server证书添加到系统信任库：

- **Linux**:
  ```bash
  sudo cp server.crt /usr/local/share/ca-certificates/
  sudo update-ca-certificates
  ```

- **macOS**:
  ```bash
  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain server.crt
  ```

- **Windows**:
  ```powershell
  Import-Certificate -FilePath server.crt -CertStoreLocation Cert:\LocalMachine\Root
  ```

## 迁移指南

### 从旧版本升级

1. **更新配置文件**
   - Server: 移除`tls`对象，添加`cert_file`和`key_file`字段
   - Agent: 移除整个`tls`对象

2. **更新Server证书** (可选但推荐)
   - 如果之前使用自签名CA，考虑迁移到Let's Encrypt
   - 确保证书的Common Name (CN)或Subject Alternative Name (SAN)匹配域名

3. **移除Agent证书** (可选)
   - Agent端不再需要证书文件
   - 清理旧的证书配置

4. **重新编译**
   ```bash
   go mod tidy  # 更新依赖
   make build   # 重新编译
   ```

5. **测试连接**
   - 先启动Server，检查日志确认QUIC监听成功
   - 启动Agent，验证连接建立

## 端口变更说明

**重要**: QUIC使用UDP协议，而不是TCP

- **旧版本**: 需要开放TCP 8228端口
- **新版本**: 需要开放UDP 8228端口

### 防火墙配置

**Linux (iptables)**:
```bash
# 移除旧的TCP规则
sudo iptables -D INPUT -p tcp --dport 8228 -j ACCEPT

# 添加新的UDP规则
sudo iptables -A INPUT -p udp --dport 8228 -j ACCEPT
```

**Linux (firewalld)**:
```bash
sudo firewall-cmd --remove-port=8228/tcp --permanent
sudo firewall-cmd --add-port=8228/udp --permanent
sudo firewall-cmd --reload
```

**云服务商安全组**:
- AWS Security Group: 将规则从TCP改为UDP
- Azure NSG: 更新规则协议为UDP
- Google Cloud Firewall: 修改允许规则为UDP

## 性能优化

### QUIC配置参数

当前QUIC配置（硬编码在代码中）:
- `MaxIdleTimeout`: 300秒
- `KeepAlivePeriod`: 30秒
- `EnableDatagrams`: false

如需调整，修改 `common/crypto/quic_transport.go`

### MTU设置

QUIC通过UDP传输，建议MTU设置：
- 互联网环境: 1280-1400字节
- 内网环境: 1400-1500字节

## 故障排查

### 连接问题

1. **Agent无法连接Server**
   ```
   错误: failed to dial QUIC: context deadline exceeded
   ```
   
   检查项:
   - Server防火墙是否允许UDP 8228
   - Server是否正在运行
   - DNS解析是否正确

2. **TLS握手失败**
   ```
   错误: x509: certificate signed by unknown authority
   ```
   
   原因: Server使用自签名证书但Agent系统未信任
   
   解决: 将Server证书添加到Agent系统的信任库

3. **证书域名不匹配**
   ```
   错误: x509: certificate is valid for example.com, not 192.168.1.100
   ```
   
   解决: Agent配置中的server地址必须使用证书中的域名，而非IP

### 性能问题

1. **高延迟**
   - 检查网络RTT: `ping server-address`
   - 验证QUIC连接: 检查日志中的连接建立时间
   - 调整keepalive参数

2. **高丢包率**
   - QUIC有内建重传机制
   - 检查网络质量
   - 考虑降低MTU设置

## 安全考虑

### 认证机制

单向TLS + API Key认证提供多层安全:

1. **传输层安全**: QUIC内置TLS 1.3加密
2. **Server身份验证**: Agent验证Server证书
3. **用户身份验证**: 通过user_key API密钥

### 证书管理

**推荐实践**:
- 使用Let's Encrypt的自动续期
- 监控证书过期时间（提前30天告警）
- 使用强密钥长度（RSA 4096或ECDSA P-256）

### API密钥安全

- 每个用户使用唯一的API Key
- 定期轮换API Key
- 在数据库中存储密钥的哈希值
- 使用HTTPS/安全通道分发密钥

## 技术实现细节

### 依赖库

- `github.com/quic-go/quic-go`: QUIC协议实现
- `google.golang.org/grpc`: gRPC框架
- `crypto/tls`: Go标准库TLS支持

### 架构变更

```
旧架构:
Agent --[TCP/TLS/mTLS]--> Server

新架构:
Agent --[UDP/QUIC/TLS1.3]--> Server
```

### 数据流

1. Agent发起QUIC连接到Server
2. QUIC层完成TLS 1.3握手（Agent验证Server证书）
3. gRPC在QUIC stream上建立通道
4. Agent通过gRPC Register调用提供user_key进行认证
5. Server验证user_key并分配overlay IP
6. 建立双向stream用于数据中继

## 参考资源

- [QUIC协议 (RFC 9000)](https://www.rfc-editor.org/rfc/rfc9000.html)
- [TLS 1.3 (RFC 8446)](https://www.rfc-editor.org/rfc/rfc8446.html)
- [Let's Encrypt文档](https://letsencrypt.org/docs/)
- [gRPC over QUIC](https://github.com/grpc/grpc/blob/master/doc/core/transport_explainer.md)

## 常见问题

**Q: 为什么不继续使用mTLS？**
A: 单向TLS简化了部署和证书管理，同时通过API Key提供等效的认证安全性。对于大规模部署，不需要为每个Agent生成和分发证书。

**Q: QUIC是否比TCP更可靠？**
A: QUIC基于UDP但实现了可靠传输机制（类似TCP的重传），同时避免了TCP的一些限制，在丢包网络环境下性能更好。

**Q: 能否回退到TCP？**
A: 当前版本强制使用QUIC。如需TCP支持，需要修改代码添加传输协议选项。

**Q: 如何监控QUIC连接质量？**
A: 可以通过日志查看连接统计，或使用Wireshark抓包分析QUIC流量（需要提供TLS密钥）。

**Q: 能否使用IP地址而非域名？**
A: 可以，但证书必须包含该IP在SAN字段中。Let's Encrypt不签发IP证书，需要自签名或私有CA。
