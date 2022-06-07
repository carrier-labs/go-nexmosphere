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

var sseChan []chan ssEvent

func getChan() chan ssEvent {
	c := make(chan ssEvent)
	sseChan = append(sseChan, c)
	return c
}

func endChan(c chan ssEvent) {
	for i := 0; i < len(sseChan); i++ {
		if sseChan[i] == c {
			sseChan[i] = sseChan[len(sseChan)-1]
			sseChan = sseChan[:len(sseChan)-1]
			return
		}
	}
}

func sendSSE(sse ssEvent) {
	log.Printf("Sending: %+v", sse)
	for _, c := range sseChan {
		c <- sse
	}
}

func handleListen(w http.ResponseWriter, r *http.Request) {

	// Check connection can stream, and get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Connection does not support streaming", http.StatusBadRequest)
		return
	}

	// Get a new channel for this connection
	c := getChan()
	defer endChan(c)

	// Control System is now connected and live
	// log.Printf("Connection Live")

	// Set HTTP Headers for SSE
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	log.Println("Connection Established")

	// Start work loop
	for {
		select {

		case <-r.Context().Done(): // Watch context to tell when connection closed by client
			log.Println("Connection Closed")
			return

		case sse := <-c: // Watch for events for this control system
			w.Write([]byte(fmt.Sprintf("type: %s\n", sse.Type)))
			w.Write([]byte(fmt.Sprintf("data: %s\n\n", sse.Message)))
			flusher.Flush()
		}
	}

}
