package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/taills/EasyAnyLink/agent"
	"github.com/taills/EasyAnyLink/common/config"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config/agent-client.example.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("EasyAnyLink Agent\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Check if running as root
	if os.Geteuid() != 0 {
		log.Fatal("Agent must run as root (or with sudo) to create TUN interface and modify routes")
	}

	// Load configuration
	cfg, err := config.LoadAgentConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting EasyAnyLink Agent version %s", Version)
	log.Printf("Mode: %s", cfg.Mode)
	log.Printf("Server: %s", cfg.Server)

	// Create agent
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Start agent
	if err := ag.Start(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	// Stop agent
	if err := ag.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Agent stopped")
}
