package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/taills/EasyAnyLink/common/config"
	"github.com/taills/EasyAnyLink/common/crypto"
	"github.com/taills/EasyAnyLink/common/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Agent represents the agent instance
type Agent struct {
	config       *config.AgentConfig
	client       proto.AgentServiceClient
	conn         *grpc.ClientConn
	tun          *TUNInterface
	routeManager *RouteManager
	sessionID    string
	assignedIP   string
	agentID      string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	stats   AgentStats
	statsMu sync.RWMutex
}

// AgentStats holds agent statistics
type AgentStats struct {
	BytesSent       uint64
	BytesReceived   uint64
	PacketsSent     uint64
	PacketsReceived uint64
	Errors          uint32
	Drops           uint32
}

// NewAgent creates a new agent instance
func NewAgent(cfg *config.AgentConfig) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Generate or use existing agent ID
	agentID := cfg.AgentID
	if agentID == "" {
		agentID = uuid.New().String()
	}

	agent := &Agent{
		config:       cfg,
		agentID:      agentID,
		ctx:          ctx,
		cancel:       cancel,
		routeManager: NewRouteManager(),
	}

	return agent, nil
}

// Start starts the agent
func (a *Agent) Start() error {
	log.Printf("Starting agent in %s mode", a.config.Mode)

	// Connect to server
	if err := a.connect(); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Register with server
	if err := a.register(); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	// Create TUN interface
	if err := a.setupTUN(); err != nil {
		return fmt.Errorf("failed to setup TUN: %w", err)
	}

	// Setup routing (client mode only)
	if a.config.Mode == "client" {
		if err := a.setupRouting(); err != nil {
			return fmt.Errorf("failed to setup routing: %w", err)
		}
	}

	// Start background tasks
	a.wg.Add(3)
	go a.heartbeatLoop()
	go a.readTUN()
	go a.relayData()

	log.Printf("Agent started successfully, ID: %s, IP: %s", a.agentID, a.assignedIP)

	return nil
}

// Stop stops the agent
func (a *Agent) Stop() error {
	log.Println("Stopping agent...")

	// Cancel context to stop goroutines
	a.cancel()

	// Wait for goroutines to finish
	a.wg.Wait()

	// Cleanup routing
	if err := a.routeManager.Cleanup(); err != nil {
		log.Printf("Warning: failed to cleanup routes: %v", err)
	}

	// Close TUN interface
	if a.tun != nil {
		if err := a.tun.Close(); err != nil {
			log.Printf("Warning: failed to close TUN: %v", err)
		}
	}

	// Close gRPC connection
	if a.conn != nil {
		if err := a.conn.Close(); err != nil {
			log.Printf("Warning: failed to close connection: %v", err)
		}
	}

	log.Println("Agent stopped")
	return nil
}

// connect establishes gRPC connection to server using QUIC
func (a *Agent) connect() error {
	// Extract server address and hostname
	host, _, err := net.SplitHostPort(a.config.Server)
	if err != nil {
		return fmt.Errorf("invalid server address: %w", err)
	}

	// Load TLS configuration for QUIC (one-way TLS)
	tlsConfig, err := crypto.LoadClientTLSConfig(host, a.config.InsecureSkipVerify)
	if err != nil {
		return fmt.Errorf("failed to load TLS configuration: %w", err)
	}

	// Warn if certificate verification is disabled
	if a.config.InsecureSkipVerify {
		log.Println("WARNING: TLS certificate verification is disabled. This should only be used for debugging!")
	}

	// Create QUIC dialer
	dialer := crypto.NewQUICDialer(tlsConfig)

	// Create gRPC connection with QUIC transport
	conn, err := grpc.Dial(
		a.config.Server,
		crypto.GRPCDialOption(dialer),
		grpc.WithInsecure(), // TLS is handled by QUIC layer
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	a.conn = conn
	a.client = proto.NewAgentServiceClient(conn)

	log.Printf("Connected to server at %s using QUIC transport", a.config.Server)
	return nil
}

// register registers the agent with the server
func (a *Agent) register() error {
	// Note: Certificate fingerprint is not needed for one-way TLS
	// Server authenticates agent using user_key instead

	// Determine agent type
	var agentType proto.AgentType
	if a.config.Mode == "client" {
		agentType = proto.AgentType_CLIENT
	} else {
		agentType = proto.AgentType_GATEWAY
	}

	// Create registration request
	req := &proto.RegisterRequest{
		AgentId:         a.agentID,
		UserKey:         a.config.UserKey,
		Type:            agentType,
		ProtocolVersion: "1.0.0",
		Bandwidth:       int32(a.config.Bandwidth),
		Metadata: &proto.AgentMetadata{
			Os:       "darwin", // TODO: detect actual OS
			Arch:     "amd64",  // TODO: detect actual arch
			Version:  "1.0.0",
			Hostname: "agent-" + a.agentID[:8],
		},
	}

	// Send registration
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := a.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("registration rejected: %s", resp.ErrorMessage)
	}

	a.sessionID = resp.SessionId
	a.assignedIP = resp.AssignedIp

	log.Printf("Registration successful, session: %s, IP: %s", a.sessionID, a.assignedIP)

	return nil
}

// setupTUN creates and configures the TUN interface
func (a *Agent) setupTUN() error {
	// Create TUN interface
	tun, err := NewTUNInterface("tun0", 1400)
	if err != nil {
		return err
	}

	a.tun = tun

	// Set IP address
	if err := tun.SetIP(a.assignedIP, "255.255.0.0"); err != nil {
		return err
	}

	// Bring interface up
	if err := tun.Up(); err != nil {
		return err
	}

	log.Printf("TUN interface %s created with IP %s", tun.Name(), a.assignedIP)

	return nil
}

// setupRouting configures routing rules
func (a *Agent) setupRouting() error {
	if len(a.config.Rules) == 0 {
		log.Println("No routing rules configured")
		return nil
	}

	for _, rule := range a.config.Rules {
		switch rule.Action {
		case "forward":
			// Route through overlay
			if err := a.routeManager.AddRoute(rule.Destination, "", a.tun.Name()); err != nil {
				return fmt.Errorf("failed to add forward route: %w", err)
			}
			log.Printf("Added route: %s via %s", rule.Destination, a.tun.Name())

		case "direct":
			// Direct routing (no action needed, uses existing default route)
			log.Printf("Direct route configured for %s", rule.Destination)

		case "deny":
			// TODO: Implement deny rules via firewall
			log.Printf("Deny rule configured for %s (not yet implemented)", rule.Destination)
		}
	}

	return nil
}

// heartbeatLoop sends periodic heartbeats
func (a *Agent) heartbeatLoop() {
	defer a.wg.Done()

	stream, err := a.client.Heartbeat(a.ctx)
	if err != nil {
		log.Printf("Failed to create heartbeat stream: %v", err)
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.statsMu.RLock()
			stats := &proto.AgentStats{
				BytesSent:       a.stats.BytesSent,
				BytesReceived:   a.stats.BytesReceived,
				PacketsSent:     uint64(a.stats.PacketsSent),
				PacketsReceived: uint64(a.stats.PacketsReceived),
				Errors:          a.stats.Errors,
				Drops:           a.stats.Drops,
			}
			a.statsMu.RUnlock()

			req := &proto.HeartbeatRequest{
				SessionId: a.sessionID,
				Stats:     stats,
			}

			if err := stream.Send(req); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				return
			}

			// Receive response (optional)
			_, err := stream.Recv()
			if err != nil {
				log.Printf("Failed to receive heartbeat response: %v", err)
				return
			}
		}
	}
}

// readTUN reads packets from TUN and sends to server
func (a *Agent) readTUN() {
	defer a.wg.Done()

	buf := make([]byte, 2048)

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			n, err := a.tun.Read(buf)
			if err != nil {
				log.Printf("Failed to read from TUN: %v", err)
				return
			}

			// Send packet to server
			// This will be implemented in relayData

			a.statsMu.Lock()
			a.stats.BytesSent += uint64(n)
			a.stats.PacketsSent++
			a.statsMu.Unlock()
		}
	}
}

// relayData handles data relay with server
func (a *Agent) relayData() {
	defer a.wg.Done()

	stream, err := a.client.RelayData(a.ctx)
	if err != nil {
		log.Printf("Failed to create relay stream: %v", err)
		return
	}

	// Send first packet with session info
	initialPacket := &proto.DataPacket{
		SessionId:     a.sessionID,
		SourceAgentId: a.agentID,
	}

	if err := stream.Send(initialPacket); err != nil {
		log.Printf("Failed to send initial packet: %v", err)
		return
	}

	// Receive packets from server and write to TUN
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			packet, err := stream.Recv()
			if err != nil {
				log.Printf("Failed to receive packet: %v", err)
				return
			}

			// Write to TUN
			if _, err := a.tun.Write(packet.Payload); err != nil {
				log.Printf("Failed to write to TUN: %v", err)
				a.statsMu.Lock()
				a.stats.Drops++
				a.statsMu.Unlock()
				continue
			}

			a.statsMu.Lock()
			a.stats.BytesReceived += uint64(len(packet.Payload))
			a.stats.PacketsReceived++
			a.statsMu.Unlock()
		}
	}
}

// GetStats returns current agent statistics
func (a *Agent) GetStats() AgentStats {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()
	return a.stats
}
