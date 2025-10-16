package main

import (
	"flag"
	"fmt"
	"log"
	"net"
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

	// Validate TLS certificates
	if err := crypto.ValidateCertificate(cfg.TLS.CertFile); err != nil {
		log.Printf("Warning: Certificate validation: %v", err)
	}

	if err := crypto.VerifyCertificateChain(cfg.TLS.CertFile, cfg.TLS.CAFile); err != nil {
		log.Printf("Warning: Certificate chain verification: %v", err)
	}

	// Load TLS credentials
	tlsConfig := crypto.TLSConfig{
		CertFile:   cfg.TLS.CertFile,
		KeyFile:    cfg.TLS.KeyFile,
		CAFile:     cfg.TLS.CAFile,
		MinVersion: crypto.GetTLSVersion(cfg.TLS.MinVersion),
	}

	creds, err := crypto.LoadServerTLSCredentials(tlsConfig)
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}
	log.Println("TLS/mTLS configured successfully")

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
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

	// Start listening
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		grpcServer.GracefulStop()
	}()

	// Start server
	log.Printf("Server listening on %s", cfg.Listen)
	log.Println("Press Ctrl+C to stop")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	log.Println("Server stopped")
}
