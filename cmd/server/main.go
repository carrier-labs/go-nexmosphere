package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.org/carrierlabs/go-nexmosphere/nexmosphere"
	"go.uber.org/zap"
)

var version = "develop"

func isProduction() bool {
	return version != "develop"
}

func main() {
	// Setup logger
	var logger *zap.Logger
	if isProduction() {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	log := logger.Sugar()
	defer log.Sync()

	log.Infof("go-nexmosphere Server Started")
	log.Info("-------------------------------------")
	log.Infof("%-15s: %s", "Version", version)
	log.Infof("%-15s: %v", "Production", isProduction())

	// Get server port from environment
	serverPort := os.Getenv("NX_SERVER_PORT")
	if serverPort == "" {
		serverPort = "8089"
	}

	log.Infof("%-15s: %s", "Server Port", serverPort)
	log.Info("-------------------------------------")

	// Create Nexmosphere service
	service := nexmosphere.NewService(
		nexmosphere.WithLogger(log),
	)

	// Create SSE handler
	sseHandler := NewSSEHandler(log)
	service.AddHandler(sseHandler)

	// Start the service
	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start service: %s", err)
	}
	defer service.Stop()

	// Setup HTTP routes
	http.HandleFunc("/sse", sseHandler.HandleHTTP)
	http.HandleFunc("/", handleRoot)

	// Start HTTP server
	log.Infof("Starting HTTP server on port %s", serverPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", serverPort), nil); err != nil {
		log.Fatalf("Server stopped: %s", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Nexmosphere SSE Server\n\nConnect to /sse for event stream")
}
