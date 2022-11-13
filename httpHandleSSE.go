package main

import (
	"fmt"
	"net/http"
)

type ssEvent struct {
	Event   string
	Message string
}

type SensorData struct {
	Id      string `json:"id"`
	Command string `json:"command"`
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
	log.Debugf("Sending: %+v", sse)
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

	log.Infof("Connection Established: %s", r.RemoteAddr)

	// Send Current State (as a go routine so it can proceed to listen to this)
	go sendSystemUpdate()

	// Start work loop
	for {
		select {

		case <-r.Context().Done(): // Watch context to tell when connection closed by client
			log.Infof("Connection Closed")
			return

		case sse := <-c: // Watch for events for this control system
			w.Write([]byte(fmt.Sprintf("event: %s\n", sse.Event)))
			w.Write([]byte(fmt.Sprintf("data: %s\n\n", sse.Message)))
			flusher.Flush()
		}
	}

}
