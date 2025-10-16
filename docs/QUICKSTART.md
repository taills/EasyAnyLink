# EasyAnyLink Quick Start

This guide will help you get EasyAnyLink up and running in under 10 minutes.

## What is EasyAnyLink?

EasyAnyLink is an overlay networking system that connects scattered private networks into one unified space. Think of it as a VPN solution where:

- **Server**: Central coordinator running on a public server
- **Client Agents**: Connect to the overlay from anywhere
- **Gateway Agents**: Provide access to private networks or act as VPN egress

**Data Flow**: `client ↔ server ↔ gateway`

---

## Prerequisites

### On All Machines
- Go 1.21 or later
- Root/sudo access (for TUN interface)

### On Server Machine
- Public IP address or domain name
- MariaDB 10.5+
- Open port 8228 (or your chosen port)

### On Your Development Machine
- protoc (Protocol Buffer compiler)
- OpenSSL (for certificate generation)

---

## Step 1: Install Dependencies

### macOS
```bash
# Install dependencies
brew install go protobuf mariadb openssl

# Start MariaDB
brew services start mariadb
```

### Linux (Ubuntu/Debian)
```bash
# Install dependencies
sudo apt update
sudo apt install -y golang-go protobuf-compiler mariadb-server openssl

# Start MariaDB
sudo systemctl start mariadb
sudo systemctl enable mariadb
```

---

## Step 2: Clone and Build

```bash
# Clone repository
git clone https://github.com/taills/EasyAnyLink.git
cd EasyAnyLink

# Install Go dependencies
go mod download

# Install protoc Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Protocol Buffer code
make proto

# Build binaries
make build
```

You should now have:
- `bin/server` - Server binary
- `bin/agent` - Agent binary

---

## Step 3: Setup Database

```bash
# Create database and tables
mysql -u root -p < scripts/init_db.sql

# Default credentials created:
# Username: admin
# Password: admin123
# API Key: dev_admin_key_change_in_production_00000000
```

⚠️ **Security Warning**: Change the default password in production!

---

## Step 4: Generate Certificates

```bash
# Generate CA and certificates for development
./scripts/generate_certs.sh
```

This creates:
- `certs/ca.crt`, `certs/ca.key` - Certificate Authority
- `certs/server.crt`, `certs/server.key` - Server certificate
- `certs/client.crt`, `certs/client.key` - Client certificate
- `certs/gateway.crt`, `certs/gateway.key` - Gateway certificate

For production, generate certificates with proper hostnames/IPs in SAN field.

---

## Step 5: Configure Server

Edit `config/server.example.json`:

```json
{
    "listen": ":8228",
    "database": {
        "type": "mariadb",
        "host": "localhost",
        "port": 3306,
        "user": "root",
        "password": "YOUR_MYSQL_PASSWORD",
        "database": "easy_any_link",
        "charset": "utf8mb4"
    },
    "network": {
        "overlay_cidr": "10.200.0.0/16",
        "gateway_ip": "10.200.0.1",
        "mtu": 1400
    }
}
```

**Important**: Update the database password!

---

## Step 6: Start Server

```bash
# Start server
./bin/server -config config/server.example.json
```

You should see:
```
Server listening on :8228
Press Ctrl+C to stop
```

---

## Step 7: Configure Gateway Agent

On the machine you want to use as a gateway (can be same as server for testing):

Edit `config/agent-gateway.example.json`:

```json
{
    "mode": "gateway",
    "server": "YOUR_SERVER_IP:8228",
    "id": "GENERATE_A_UUID_HERE",
    "bandwidth": 1000,
    "tls": {
        "cert_file": "./certs/gateway.crt",
        "key_file": "./certs/gateway.key",
        "ca_file": "./certs/ca.crt"
    }
}
```

Generate a UUID for the gateway:
```bash
python3 -c "import uuid; print(uuid.uuid4())"
# or
uuidgen
```

---

## Step 8: Start Gateway Agent

```bash
# Start gateway agent (requires root)
sudo ./bin/agent -config config/agent-gateway.example.json
```

You should see:
```
Agent started successfully, ID: <uuid>, IP: 10.200.251.1
```

---

## Step 9: Configure Client Agent

On the client machine:

Edit `config/agent-client.example.json`:

```json
{
    "mode": "client",
    "server": "YOUR_SERVER_IP:8228",
    "user_key": "dev_admin_key_change_in_production_00000000",
    "tls": {
        "cert_file": "./certs/client.crt",
        "key_file": "./certs/client.key",
        "ca_file": "./certs/ca.crt"
    },
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

Replace `YOUR_GATEWAY_UUID` with the gateway ID from Step 7.

---

## Step 10: Start Client Agent

```bash
# Start client agent (requires root)
sudo ./bin/agent -config config/agent-client.example.json
```

You should see:
```
Agent started successfully, ID: <uuid>, IP: 10.200.1.1
TUN interface tun0 created with IP 10.200.1.1
```

---

## Step 11: Test Connectivity

From the client machine:

```bash
# Ping the gateway's overlay IP
ping 10.200.251.1

# Check TUN interface
ifconfig tun0  # macOS
ip addr show tun0  # Linux

# Check routes
route -n get 192.168.1.0  # macOS
ip route show  # Linux
```

If you can ping the gateway, congratulations! Your overlay network is working.

---

## Troubleshooting

### Server won't start
- Check if port 8228 is already in use: `lsof -i :8228`
- Verify database credentials
- Check certificate files exist

### Agent won't connect
- Verify server is reachable: `telnet SERVER_IP 8228`
- Check firewall rules
- Verify certificates are valid
- Check server logs for authentication errors

### TUN interface not created
- Make sure you're running with `sudo`
- On macOS: Check System Preferences > Security & Privacy
- On Linux: Verify TUN module is loaded: `lsmod | grep tun`

### Can't ping gateway
- Check if both agents are connected (check server logs)
- Verify routing rules are installed: `route -n` or `ip route`
- Check firewall rules on gateway

---

## Next Steps

1. **Secure Your Setup**
   - Change default admin password
   - Generate production certificates with proper CN/SAN
   - Use strong API keys

2. **Configure Routing**
   - Add more specific routes for your private networks
   - Set up split-tunnel vs full-tunnel

3. **Add More Agents**
   - Deploy additional clients and gateways
   - Create separate users and API keys

4. **Monitor Performance**
   - Check agent statistics (feature to be added)
   - Monitor server logs
   - Review database for session history

---

## Common Use Cases

### Use Case 1: Access Home Network from Anywhere
- **Gateway**: At home, behind your router
- **Client**: On your laptop when traveling
- **Benefit**: Access home devices securely

### Use Case 2: Connect Multiple Office Networks
- **Gateways**: One at each office location
- **Clients**: Remote workers
- **Benefit**: Unified network across locations

### Use Case 3: VPN Egress/Proxy
- **Gateway**: In desired geographic location
- **Client**: Your machine
- **Route**: Default route (0.0.0.0/0) through gateway
- **Benefit**: Browse as if you're in gateway location

---

## Configuration Tips

### Split-Tunnel Configuration
Only route specific networks through overlay:

```json
"rules": [
    {
        "action": "forward",
        "destination": "10.0.0.0/8",
        "gateway": "gateway-uuid",
        "priority": 10
    },
    {
        "action": "direct",
        "destination": "0.0.0.0/0",
        "priority": 100
    }
]
```

### Full-Tunnel Configuration
Route all traffic through gateway:

```json
"rules": [
    {
        "action": "forward",
        "destination": "0.0.0.0/0",
        "gateway": "gateway-uuid",
        "priority": 10
    }
]
```

### Deny Specific Networks
Block access to certain IPs:

```json
"rules": [
    {
        "action": "deny",
        "destination": "192.168.99.0/24",
        "priority": 1
    }
]
```

---

## Getting Help

- **Documentation**: `/docs` directory
- **Issues**: https://github.com/taills/EasyAnyLink/issues
- **Architecture**: See `.github/copilot-instructions.md`

---

## Development Mode vs Production

This guide uses development settings. For production:

1. **Use Real Database Credentials**
2. **Generate Production Certificates** with proper hostnames
3. **Change All Default Passwords and API Keys**
4. **Enable HTTPS for Web UI** (when implemented)
5. **Set Up Log Aggregation**
6. **Configure Backups** for database
7. **Use Systemd/Docker** for service management
8. **Monitor Resource Usage**

---

Happy Networking! 🚀
