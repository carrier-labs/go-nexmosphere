package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"encoding/json"
)

type ssEvent struct {
	Type    string
	Message string
}

type SensorData struct {
    Id string `json:"id"`
    Command  string `json:"command"`
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

	// Start work loop
	for {
		select {

		case <-r.Context().Done(): // Watch context to tell when connection closed by client
			log.Println("Connection Closed")
			return

		case sse := <-c: // Watch for events for this control system
			w.Write([]byte(fmt.Sprintf("event: %s\n", sse.Type)))
			// w.Write([]byte(fmt.Sprintf("data: %s\n\n", sse.Message)))
			
			antenna_event, _ := regexp.Compile(`X([0-9]+)A\[([0-9])\]`)
			tag_event, _ := regexp.Compile(`XR\[([A-Z][A-Z])([0-9][0-9][0-9])\]`)

    		if tag_event.MatchString(sse.Message) {
				id := tag_event.FindStringSubmatch(sse.Message)[2]
				action := tag_event.FindStringSubmatch(sse.Message)[1]
				//fmt.Println("Tag event! ID: ", id, "Action:", action)
				sd := SensorData{id, action}
				b, _ := json.Marshal(sd)
				w.Write([]byte(fmt.Sprintf("data: %s\n\n", string(b))))
			} else if antenna_event.MatchString(sse.Message) {
				id := antenna_event.FindStringSubmatch(sse.Message)[1]
				action := antenna_event.FindStringSubmatch(sse.Message)[2] // 0 == tag put back, 1 == tag removed
        		//fmt.Println("Antenna event! ID: ", id, "Action:", action)
        		sd := SensorData{id, action}
				b, _ := json.Marshal(sd)
				w.Write([]byte(fmt.Sprintf("data: %s\n\n", string(b))))
			} else {
        		fmt.Println("Unhandled message: ", sse.Message)
			}
			flusher.Flush()
		}
	}

}
