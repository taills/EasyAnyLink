# EasyAnyLink

[中文文档](./READNE_ZH.md) | English

EasyAnyLink is a two-component overlay networking system that unifies scattered private networks into one reachable space. It consists of a public-facing Server and pluggable Agents that assume two roles: client and gateway.


## Components

Server: Internet-exposed coordinator and data relay. It handles agent registration, session orchestration, authentication, and relays traffic between agents.
Agent:
Client: Creates a local TUN interface and installs a default (or optional split) route to send traffic into the overlay.
Gateway: Receives packets from client TUNs over the overlay and forwards them to its local network or the Internet, functioning like a VPN egress/proxy.
Data Path

client ↔ server ↔ gateway
What it enables

Securely access resources inside the gateway’s private network from anywhere.
Use the gateway for egress to the public Internet, enabling proxy-like browsing or service access.
Design goals

Simple deployment: one public server, many agents.
Layer-3 transparency with TUN interfaces.
Security-first architecture with mutual authentication and encrypted tunnels.
Flexible routing policies (default or split-tunnel) and multi-tenant readiness.

## Implementation Details

### Server

The server is the central component of the EasyAnyLink architecture. It is responsible for:

- Agent registration and management
- Session orchestration and state management
- Authentication and authorization of agents
- Relaying traffic between clients and gateways
- Monitoring and logging of network activity

### Agent

The agent is a crucial component that interacts with both the server and the local network. It has two main roles:

- Client: Creates a local TUN interface and installs a default (or optional split) route to send traffic into the overlay.
- Gateway: Receives packets from client TUNs over the overlay and forwards them to its local network or the Internet, functioning like a VPN egress/proxy.

