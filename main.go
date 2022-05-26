package main

import (
	"io"
	"log"
	"net/http"
)

func main() {

	// Start the Nexmosphere watching process
	go watchNexmosphere()

	// Set routing rules
	// http.HandleFunc("/action", handleAction) // HTTP GET/POST to control devices
	http.HandleFunc("/listen", handleListen) // HTTP SSE Stream of Device Events
	http.HandleFunc("/", handleAnythingElse) // Essentially a 404 catch-all
	log.Println("Starting Server")
	//Use the default DefaultServeMux.

	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal(err)

	}
}

func handleAnythingElse(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Nothing to see here...")
}
