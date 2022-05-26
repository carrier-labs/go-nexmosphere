package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {

	// Start the Nexmosphere watching process
	go watchNexmosphere()

	// Set routing rules
	// http.HandleFunc("/action", handleAction) // HTTP GET/POST to control devices
	http.HandleFunc("/listen", handleListen) // HTTP SSE Stream of Device Events
	http.HandleFunc("/", handleAnythingElse) // Essentially a 404 catch-all

	// Get default port number
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8089"
	}

	// Start HTTP Server
	log.Printf("Starting Server on :%s", port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Fatal(err)

	}
}

func handleAnythingElse(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Nothing to see here...")
}
