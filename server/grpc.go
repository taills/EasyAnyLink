package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/taills/EasyAnyLink/common/config"
	"github.com/taills/EasyAnyLink/common/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Server represents the gRPC server
type Server struct {
	proto.UnimplementedAgentServiceServer

	config   *config.ServerConfig
	db       *Database
	ipPool   *IPPool
	sessions sync.Map // sessionID -> *SessionInfo
	agents   sync.Map // agentID -> *AgentInfo
}

// SessionInfo holds information about an active session
type SessionInfo struct {
	SessionID     string
	AgentID       string
	Type          proto.AgentType
	Stream        proto.AgentService_RelayDataServer
	Created       time.Time
	LastActivity  time.Time
	BytesSent     uint64
	BytesReceived uint64
	mu            sync.RWMutex
}

// AgentInfo holds cached agent information
type AgentInfo struct {
	AgentID   string
	UserID    string
	Type      proto.AgentType
	IPAddress string
	Status    proto.AgentStatus
	Metadata  *proto.AgentMetadata
	LastSeen  time.Time
}

// NewServer creates a new gRPC server instance
func NewServer(cfg *config.ServerConfig, db *Database) (*Server, error) {
	// Initialize IP pool
	ipPool, err := NewIPPool(cfg.Network.OverlayCIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP pool: %w", err)
	}

	server := &Server{
		config: cfg,
		db:     db,
		ipPool: ipPool,
	}

	return server, nil
}

// Register handles agent registration
func (s *Server) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
	log.Printf("Registration request from agent %s, type: %s", req.AgentId, req.Type)

	// Validate protocol version
	if !s.isProtocolCompatible(req.ProtocolVersion) {
		return &proto.RegisterResponse{
			Accepted:                false,
			ErrorMessage:            "Incompatible protocol version",
			ServerVersion:           "1.0.0",
			MinimumSupportedVersion: "1.0.0",
		}, nil
	}

	// Authenticate user
	user, err := s.db.GetUserByAPIKey(req.UserKey)
	if err != nil {
		log.Printf("Authentication failed for user key: %v", err)
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed")
	}

	// Get or create agent
	agent, err := s.db.GetAgentByID(req.AgentId)
	if err != nil {
		// Create new agent
		metadata, _ := json.Marshal(req.Metadata)

		// Allocate IP address
		ip, err := s.ipPool.Allocate(req.AgentId)
		if err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "failed to allocate IP: %v", err)
		}

		agent = &Agent{
			ID:                     req.AgentId,
			UserID:                 user.ID,
			Name:                   req.Metadata.Hostname,
			Type:                   req.Type.String(),
			Status:                 "online",
			IPAddress:              ip.String(),
			BandwidthLimit:         int(req.Bandwidth),
			CertificateFingerprint: req.CertificateFingerprint,
			Metadata:               string(metadata),
		}

		if err := s.db.CreateAgent(agent); err != nil {
			s.ipPool.Release(req.AgentId)
			return nil, status.Errorf(codes.Internal, "failed to create agent: %v", err)
		}
	} else {
		// Update existing agent status
		if err := s.db.UpdateAgentStatus(agent.ID, "online"); err != nil {
			log.Printf("Failed to update agent status: %v", err)
		}
	}

	// Create session
	sessionID := uuid.New().String()
	connectionID := fmt.Sprintf("%s-%d", req.AgentId, time.Now().Unix())

	session := &Session{
		ID:           sessionID,
		AgentID:      agent.ID,
		ConnectionID: connectionID,
	}

	if err := s.db.CreateSession(session); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create session: %v", err)
	}

	// Cache agent info
	s.agents.Store(agent.ID, &AgentInfo{
		AgentID:   agent.ID,
		UserID:    user.ID,
		Type:      req.Type,
		IPAddress: agent.IPAddress,
		Status:    proto.AgentStatus_ONLINE,
		Metadata:  req.Metadata,
		LastSeen:  time.Now(),
	})

	log.Printf("Agent %s registered successfully, IP: %s, Session: %s",
		agent.ID, agent.IPAddress, sessionID)

	return &proto.RegisterResponse{
		Accepted:                true,
		SessionId:               sessionID,
		AssignedIp:              agent.IPAddress,
		ServerVersion:           "1.0.0",
		MinimumSupportedVersion: "1.0.0",
		ServerConfig: &proto.ServerConfig{
			GatewayIp:         s.config.Network.GatewayIP,
			Mtu:               int32(s.config.Network.MTU),
			KeepaliveInterval: int32(s.config.Network.KeepaliveInterval),
			KeepaliveTimeout:  int32(s.config.Network.KeepaliveTimeout),
		},
	}, nil
}

// Heartbeat handles agent heartbeat messages
func (s *Server) Heartbeat(stream proto.AgentService_HeartbeatServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		// Update session activity
		if sessionInfo, ok := s.sessions.Load(req.SessionId); ok {
			si := sessionInfo.(*SessionInfo)
			si.mu.Lock()
			si.LastActivity = time.Now()
			if req.Stats != nil {
				si.BytesSent = req.Stats.BytesSent
				si.BytesReceived = req.Stats.BytesReceived
			}
			si.mu.Unlock()
		}

		// Send response
		resp := &proto.HeartbeatResponse{
			Alive:     true,
			Timestamp: req.Timestamp,
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// RelayData handles data packet relay between agents
func (s *Server) RelayData(stream proto.AgentService_RelayDataServer) error {
	// Get session from first packet
	firstPacket, err := stream.Recv()
	if err != nil {
		return err
	}

	sessionID := firstPacket.SessionId
	sessionInfo, ok := s.sessions.Load(sessionID)
	if !ok {
		return status.Errorf(codes.NotFound, "session not found")
	}

	si := sessionInfo.(*SessionInfo)
	si.Stream = stream

	// Register session stream
	s.sessions.Store(sessionID, si)

	log.Printf("Data relay started for session %s, agent %s", sessionID, si.AgentID)

	// Handle incoming packets
	for {
		packet, err := stream.Recv()
		if err != nil {
			log.Printf("Stream ended for session %s: %v", sessionID, err)
			s.sessions.Delete(sessionID)
			return err
		}

		// Update statistics
		si.mu.Lock()
		si.BytesReceived += uint64(len(packet.Payload))
		si.LastActivity = time.Now()
		si.mu.Unlock()

		// Route packet to destination
		if err := s.routePacket(packet); err != nil {
			log.Printf("Failed to route packet: %v", err)
		}
	}
}

// GetRoutes handles routing configuration requests
func (s *Server) GetRoutes(ctx context.Context, req *proto.RouteRequest) (*proto.RouteResponse, error) {
	// Get routing rules from database
	rules, err := s.db.GetRoutingRulesByAgentID(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get routing rules: %v", err)
	}

	// Convert to proto format
	protoRules := make([]*proto.RoutingRule, 0, len(rules))
	for _, rule := range rules {
		protoRule := &proto.RoutingRule{
			RuleId:      int32(rule.ID),
			Destination: rule.Destination,
			GatewayId:   rule.GatewayID,
			Priority:    int32(rule.Priority),
			Enabled:     rule.Enabled,
		}

		switch rule.Action {
		case "forward":
			protoRule.Action = proto.RouteAction_FORWARD
		case "direct":
			protoRule.Action = proto.RouteAction_DIRECT
		case "deny":
			protoRule.Action = proto.RouteAction_DENY
		}

		protoRules = append(protoRules, protoRule)
	}

	return &proto.RouteResponse{
		Rules: protoRules,
	}, nil
}

// UpdateStatus handles agent status updates
func (s *Server) UpdateStatus(ctx context.Context, req *proto.StatusUpdate) (*proto.StatusResponse, error) {
	// Update agent status in database
	var statusStr string
	switch req.Status {
	case proto.AgentStatus_ONLINE:
		statusStr = "online"
	case proto.AgentStatus_OFFLINE:
		statusStr = "offline"
	case proto.AgentStatus_ERROR:
		statusStr = "error"
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid status")
	}

	if err := s.db.UpdateAgentStatus(req.AgentId, statusStr); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update status: %v", err)
	}

	// Update cached agent info
	if agentInfo, ok := s.agents.Load(req.AgentId); ok {
		ai := agentInfo.(*AgentInfo)
		ai.Status = req.Status
		ai.LastSeen = time.Now()
		s.agents.Store(req.AgentId, ai)
	}

	return &proto.StatusResponse{
		Acknowledged: true,
		Message:      "Status updated successfully",
	}, nil
}

// routePacket routes a packet to the destination agent
func (s *Server) routePacket(packet *proto.DataPacket) error {
	// Find destination session
	var destSession *SessionInfo

	if packet.DestinationAgentId != "" {
		// Direct routing to specific agent
		s.sessions.Range(func(key, value interface{}) bool {
			si := value.(*SessionInfo)
			if si.AgentID == packet.DestinationAgentId {
				destSession = si
				return false
			}
			return true
		})
	} else {
		// Route to gateway (for client packets)
		// Find any online gateway
		s.sessions.Range(func(key, value interface{}) bool {
			si := value.(*SessionInfo)
			if si.Type == proto.AgentType_GATEWAY {
				destSession = si
				return false
			}
			return true
		})
	}

	if destSession == nil {
		return fmt.Errorf("no route to destination")
	}

	// Send packet to destination
	if err := destSession.Stream.Send(packet); err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	// Update statistics
	destSession.mu.Lock()
	destSession.BytesSent += uint64(len(packet.Payload))
	destSession.mu.Unlock()

	return nil
}

// isProtocolCompatible checks if the client protocol version is compatible
func (s *Server) isProtocolCompatible(version string) bool {
	// Simple version check - in production, use proper semver comparison
	return version == "1.0.0"
}

// GetClientIP extracts client IP from gRPC context
func (s *Server) GetClientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	return p.Addr.String()
}

// GetMetadata extracts metadata from gRPC context
func (s *Server) GetMetadata(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
