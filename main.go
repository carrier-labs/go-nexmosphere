package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"
)

var serverPort string = "8089"
var version = "develop"
var log *zap.SugaredLogger

func isProduction() bool {
	return version != "develop"
}

func init() {

	logger, _ := zap.NewProduction()
	log = logger.Sugar()

	log.Infof("go-nexmosphere Started")
	log.Info("-------------------------------------------------------------")

	log.Infof("%20s: %s", "Version", version)
	log.Infof("%20s: %s", "Production", isProduction())

	// Check for ENV to override default Port
	if e := os.Getenv("NX_SERVER_PORT"); e != "" {
		serverPort = e
	}

	log.Info("-------------------------------------------------------------")
}

func main() {

	// Start the Nexmosphere watching process
	go watchNexmosphere()

	// Set routing rules
	// http.HandleFunc("/action", handleAction) // HTTP GET/POST to control devices
	http.HandleFunc("/listen", handleListen) // HTTP SSE Stream of Device Events
	http.HandleFunc("/", handleAnythingElse) // Essentially a 404 catch-all

	// Get default port number
	port := os.Getenv("NX_SERVER_PORT")
	if port == "" {
		port = "8089"
	}

	// Start HTTP Server
	log.Infof("Starting Server on port %s", serverPort)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)

	log.Sync()

	if err != nil {
		log.Fatal("Server Stopped: %s", err)
	}
}

func handleAnythingElse(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Nothing to see here...")
}
