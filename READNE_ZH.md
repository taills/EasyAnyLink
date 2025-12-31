# EasyAnyLink

中文文档 | [English](./README.md)

**状态**: 🚧 核心后端实现已完成 - 准备测试  
**版本**: 1.0.0-dev

EasyAnyLink 是一个由两部分组成的覆盖网（overlay networking）系统，用于将分散的私有网络统一为一个可达的空间。它包含一个面向公网的 Server（服务器）以及可插拔的 Agent（代理），Agent 承担两种角色：Client（客户端）和 Gateway（网关）。

## 🎯 功能特性

- **安全连接** 将分散的私有网络连接到统一的覆盖网络
- **访问资源** 从任何位置访问私有网络中的资源
- **流量路由** 通过指定的网关路由流量（类似 VPN 功能）
- **简单部署** 一个公网服务器，多个代理节点

## 🏗️ 架构

```
┌─────────┐         ┌────────────┐         ┌─────────┐
│ Client  │ ◄─────► │   Server   │ ◄─────► │ Gateway │
│ Agent   │  TLS    │  (Public)  │  TLS    │ Agent   │
└─────────┘         └────────────┘         └─────────┘
     │                                           │
     │              覆盖网络                      │
     │            (10.200.0.0/16)                │
     │                                           │
     └──────────► 访问私有网络或 ◄───────────────┘
                  通过网关访问互联网
```

## 组件说明

### Server（服务器）
- 面向互联网的协调与数据中继节点
- 负责代理注册、认证和流量中继
- 内置会话管理和 IP 地址分配
- 基于 gRPC，强制使用 mTLS 认证

### Agent（代理 - 双角色）
- **Client 模式**：创建本地 TUN 接口，安装路由规则，将流量发送到覆盖网络
- **Gateway 模式**：接收来自客户端的数据包，转发到本地网络或互联网
- 相同的二进制文件，根据配置运行不同的角色
- 平台特定的 TUN/路由实现（Linux、macOS）

## ✨ 功能特性

### 已实现 ✅
- [x] 基于 gRPC 的通信，使用 mTLS 认证
- [x] 代理注册和认证
- [x] 双向数据包中继
- [x] TUN 接口管理（Linux、macOS）
- [x] 动态 IP 地址分配
- [x] 灵活的路由策略（转发、直连、拒绝）
- [x] 会话跟踪和统计
- [x] MariaDB 后端持久化存储
- [x] 基于证书的安全机制
- [x] 优雅关闭和清理

### 进行中 🚧
- [ ] Web 管理界面（Vue 3）
- [ ] Windows TUN 支持
- [ ] 综合测试套件
- [ ] 监控和指标（Prometheus）

### 计划中 📋
- [ ] IPv6 支持
- [ ] NAT 穿透（STUN/TURN）
- [ ] 多租户隔离
- [ ] 证书轮换
- [ ] 管理 REST API

## 🚀 快速开始

### 前置要求
- Go 1.21+
- MariaDB 10.5+
- protoc（Protocol Buffer 编译器）
- Root/sudo 权限（用于创建 TUN 接口）

### 构建

```bash
# 克隆仓库
git clone https://github.com/taills/EasyAnyLink.git
cd EasyAnyLink

# 安装依赖
go mod download

# 安装 protoc Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成 Protocol Buffer 代码
make proto

# 构建二进制文件
make build
```

构建完成后，你会得到：
- `bin/server` - 服务器二进制文件
- `bin/agent` - 代理二进制文件

### 设置

```bash
# 1. 初始化数据库
mysql -u root -p < scripts/init_db.sql

# 默认创建的凭证：
# 用户名: admin
# 密码: admin123
# API Key: dev_admin_key_change_in_production_00000000

# 2. 生成开发环境证书
./scripts/generate_certs.sh

# 生成的证书文件：
# - certs/ca.crt, certs/ca.key - 证书颁发机构
# - certs/server.crt, certs/server.key - 服务器证书
# - certs/client.crt, certs/client.key - 客户端证书
# - certs/gateway.crt, certs/gateway.key - 网关证书
```

⚠️ **安全警告**：在生产环境中更改默认密码和 API 密钥！

### 运行

#### 1. 启动 Server

编辑 `config/server.example.json`，更新数据库密码：

```json
{
    "listen": ":8228",
    "database": {
        "type": "mariadb",
        "host": "localhost",
        "port": 3306,
        "user": "root",
        "password": "YOUR_MYSQL_PASSWORD",
        "database": "easy_any_link"
    }
}
```

启动服务器：

```bash
./bin/server -config config/server.example.json
```

#### 2. 启动 Gateway Agent

生成 Gateway UUID：

```bash
# Linux/macOS
uuidgen
# 或
python3 -c "import uuid; print(uuid.uuid4())"
```

编辑 `config/agent-gateway.example.json`：

```json
{
    "mode": "gateway",
    "server": "YOUR_SERVER_IP:8228",
    "id": "YOUR_GATEWAY_UUID",
    "bandwidth": 1000
}
```

启动网关（需要 root 权限）：

```bash
sudo ./bin/agent -config config/agent-gateway.example.json
```

#### 3. 启动 Client Agent

编辑 `config/agent-client.example.json`：

```json
{
    "mode": "client",
    "server": "YOUR_SERVER_IP:8228",
    "user_key": "dev_admin_key_change_in_production_00000000",
    "rules": [
        {
            "action": "forward",
            "destination": "192.168.1.0/24",
            "gateway": "YOUR_GATEWAY_UUID",
            "priority": 10
        },
        {
            "action": "direct",
            "destination": "0.0.0.0/0",
            "priority": 100
        }
    ]
}
```

启动客户端（需要 root 权限）：

```bash
sudo ./bin/agent -config config/agent-client.example.json
```

### 验证连接

```bash
# 在客户端机器上，测试连接到网关
ping 10.200.251.1  # 网关的覆盖网络 IP

# 测试通过网关访问私有网络资源
ping 192.168.1.1  # 网关后面的私有网络 IP
```

## 📖 配置说明

### Server 配置

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `listen` | gRPC 监听地址 | `:8228` |
| `database.host` | MariaDB 主机 | `localhost` |
| `database.port` | MariaDB 端口 | `3306` |
| `database.database` | 数据库名称 | `easy_any_link` |
| `network.overlay_cidr` | 覆盖网络 CIDR | `10.200.0.0/16` |
| `network.mtu` | TUN 接口 MTU | `1400` |
| `tls.cert_file` | TLS 证书文件 | - |
| `tls.key_file` | TLS 私钥文件 | - |
| `tls.ca_file` | CA 证书文件 | - |

### Agent 配置

#### Client 模式

| 配置项 | 说明 | 必需 |
|--------|------|------|
| `mode` | 运行模式（"client"） | ✅ |
| `server` | Server 地址 | ✅ |
| `user_key` | 用户 API 密钥 | ✅ |
| `rules` | 路由规则列表 | ✅ |
| `tls.cert_file` | 客户端证书 | ✅ |
| `tls.key_file` | 客户端私钥 | ✅ |
| `tls.ca_file` | CA 证书 | ✅ |

#### Gateway 模式

| 配置项 | 说明 | 必需 |
|--------|------|------|
| `mode` | 运行模式（"gateway"） | ✅ |
| `server` | Server 地址 | ✅ |
| `id` | Gateway UUID | ✅ |
| `bandwidth` | 带宽限制（KB/s，0 为无限制） | ❌ |
| `tls.cert_file` | 网关证书 | ✅ |
| `tls.key_file` | 网关私钥 | ✅ |
| `tls.ca_file` | CA 证书 | ✅ |

### 路由规则

路由规则按优先级（priority）排序，数字越小优先级越高。

| 动作 | 说明 | 示例 |
|------|------|------|
| `forward` | 通过指定网关转发 | `{"action": "forward", "destination": "10.0.0.0/8", "gateway": "uuid"}` |
| `direct` | 直接连接（不通过覆盖网络） | `{"action": "direct", "destination": "0.0.0.0/0"}` |
| `deny` | 拒绝访问 | `{"action": "deny", "destination": "1.2.3.4/32"}` |

## 🔧 开发

### 项目结构

```
EasyAnyLink/
├── cmd/                    # 主程序入口
│   ├── server/            # Server 主程序
│   └── agent/             # Agent 主程序
├── server/                # Server 实现
│   ├── grpc.go           # gRPC 服务实现
│   ├── database.go       # 数据库操作
│   └── ippool.go         # IP 地址池管理
├── agent/                 # Agent 实现
│   ├── agent.go          # 核心逻辑
│   ├── tun_linux.go      # Linux TUN 实现
│   ├── tun_darwin.go     # macOS TUN 实现
│   ├── route_linux.go    # Linux 路由实现
│   └── route_darwin.go   # macOS 路由实现
├── common/                # 共享代码
│   ├── proto/            # gRPC Protocol Buffer 定义
│   ├── config/           # 配置解析
│   └── crypto/           # TLS 工具
├── config/                # 配置示例文件
├── scripts/               # 脚本工具
├── docs/                  # 文档
└── web/                   # Web 界面（开发中）
```

### 编译目标

```bash
# 查看所有可用目标
make help

# 生成 Protocol Buffer 代码
make proto

# 构建所有二进制文件
make build

# 运行测试
make test

# 生成证书
make certs

# 清理构建产物
make clean

# 跨平台编译
make build-linux    # Linux AMD64
make build-darwin   # macOS ARM64/AMD64
```

### 测试

```bash
# 运行所有测试
go test ./...

# 带覆盖率的测试
go test -cover ./...

# 集成测试（需要 root 权限）
sudo go test -tags=integration ./...
```

## 🔒 安全性

### TLS/mTLS
- 所有 gRPC 通信强制使用 TLS 1.3+
- 双向认证（mTLS）：服务器和代理互相验证
- 证书指纹验证
- 不支持明文连接

### 认证
- 基于证书的代理认证
- API 密钥用于用户识别
- 会话超时机制
- 失败认证尝试限制

### 网络隔离
- 覆盖网络 IP 地址独立分配
- 用户级别的代理隔离
- 网关访问控制
- 路由规则验证

## 📊 IP 地址管理

### 覆盖网络设计
- 默认覆盖网络 CIDR：`10.200.0.0/16`（65,534 个地址）
- 服务器保留：`10.200.0.1`（网关）
- Client 范围：`10.200.1.0/24` - `10.200.250.0/24`
- Gateway 范围：`10.200.251.0/24` - `10.200.255.0/24`

### 分配策略
- **顺序分配**：从池中分配下一个可用 IP
- **粘性分配**：记住代理之前的 IP，重连时重新分配
- **预留**：允许为特定代理手动分配 IP
- **冲突检测**：分配前验证 IP 未被使用

## 🐳 Docker 部署

### 使用 Docker Compose

```bash
# 复制并编辑配置
cp config/server.example.json config/server.json
# 编辑 config/server.json 更新数据库密码

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 手动构建 Docker 镜像

```bash
# 构建 Server 镜像
docker build -t easyanylink/server:latest -f Dockerfile.server .

# 构建 Agent 镜像
docker build -t easyanylink/agent:latest -f Dockerfile.agent .
```

## 🔍 故障排查

### Server 问题

**数据库连接错误**
- 验证凭证和连接字符串
- 检查连接池设置
- 确保 MariaDB max_connections > 连接池大小

**代理注册失败**
- 检查证书有效性和 mTLS 配置
- 验证代理协议版本兼容性
- 检查服务器防火墙规则

### Agent 问题

**TUN 接口未创建**
- 验证是否以足够的权限运行
- Linux：检查内核模块是否加载 `lsmod | grep tun`
- macOS：确认 TUN/TAP 驱动已安装

**无网络连接**
- 验证路由已安装：`ip route` / `route print`
- 检查 TUN 接口状态：`ip link show tun0`
- 测试连接：`ping 10.200.0.1`（服务器覆盖网络 IP）

**频繁断开连接**
- 检查网络稳定性
- 验证 keepalive 设置
- 查看服务器日志错误
- 调整重连退避参数

## 📚 文档

- [快速开始指南](docs/QUICKSTART.md) - 10 分钟上手教程
- [开发指南](docs/DEVELOPMENT.md) - 开发者文档
- [.github/copilot-instructions.md](.github/copilot-instructions.md) - 详细的项目说明和设计文档

## 🤝 贡献

欢迎贡献！请查看我们的贡献指南。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🔗 链接

- GitHub: https://github.com/taills/EasyAnyLink
- 问题反馈: https://github.com/taills/EasyAnyLink/issues

## 💡 使用场景

1. **远程办公**：从家中安全访问公司内网资源
2. **跨地域协作**：连接分布在不同地域的办公室网络
3. **安全浏览**：通过受信任的网关访问互联网
4. **开发测试**：为开发环境提供统一的网络访问
5. **IoT 设备管理**：集中管理分散的 IoT 设备网络
