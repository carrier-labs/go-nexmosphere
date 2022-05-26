package main

import (
	"fmt"
	"log"
	"net/http"
)

type ssEvent struct {
	Type    string
	Message string
}

var c = make(map[string]chan ssEvent)

func handleListen(w http.ResponseWriter, r *http.Request) {

	// Check connection can stream, and get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Connection does not support streaming", http.StatusBadRequest)
		return
	}

	// get clientID from query
	clientIDs, ok := r.URL.Query()["clientID"]
	if !ok || len(clientIDs[0]) < 1 {
		http.Error(w, "Url Param 'clientID' is missing", http.StatusBadRequest)
		log.Println("Url Param 'clientID' is missing")
		return
	}
	clientID := clientIDs[0]

	// Create a new channel for this ID
	c[clientID] = make(chan ssEvent)

	// Control System is now connected and live
	log.Printf("Control System [%s] Connected\n", clientID)

	// Set HTTP Headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Start work loop
	for {
		select {

		case <-r.Context().Done(): // Watch context to tell when connection closed by client
			close(c[clientID])
			delete(c, clientID)
			log.Println("Closed")
			return

		case e := <-c[clientID]: // Watch for events for this control system
			fmt.Println("something happened", e.Type)
			w.Write([]byte("data: <stuff>\n\n"))

			w.Write([]byte(fmt.Sprintf("type: %s\n", e.Type)))
			w.Write([]byte(fmt.Sprintf("msg: %s\n", e.Message)))
			w.Write([]byte(fmt.Sprintf("\n")))

			flusher.Flush()

		}
	}

}
