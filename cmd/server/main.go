package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/taills/EasyAnyLink/common/config"
	"github.com/taills/EasyAnyLink/common/crypto"
	"github.com/taills/EasyAnyLink/common/proto"
	"github.com/taills/EasyAnyLink/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config/server.example.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("EasyAnyLink Server\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadServerConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting EasyAnyLink Server version %s", Version)
	log.Printf("Listening on %s", cfg.Listen)

	// Initialize database
	db, err := server.NewDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully")

	// Validate TLS certificate
	if err := crypto.ValidateCertificate(cfg.CertFile); err != nil {
		log.Printf("Warning: Certificate validation: %v", err)
	}

	log.Println("Using one-way TLS with QUIC transport")
	log.Println("Agents will verify server certificate using system root CAs")

	// Load TLS configuration for QUIC
	tlsConfig, err := crypto.LoadServerTLSConfig(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS configuration: %v", err)
	}

	// Create QUIC listener
	quicListener, err := crypto.NewQUICListener(cfg.Listen, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to create QUIC listener: %v", err)
	}
	defer quicListener.Close()
	log.Printf("QUIC listener started on %s", cfg.Listen)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(10000),
	)

	// Register service
	agentServer, err := server.NewServer(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	proto.RegisterAgentServiceServer(grpcServer, agentServer)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		grpcServer.GracefulStop()
	}()

	// Start server
	log.Printf("Server listening on %s with QUIC transport", cfg.Listen)
	log.Println("Press Ctrl+C to stop")

	if err := grpcServer.Serve(quicListener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	log.Println("Server stopped")
}
