package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.org/carrierlabs/go-nexmosphere/nexmosphere"
	"go.uber.org/zap"
)

// SSEHandler implements nexmosphere.EventHandler and provides HTTP SSE streaming
type SSEHandler struct {
	clients map[chan sseEvent]bool
	mu      sync.RWMutex
	logger  *zap.SugaredLogger
}

type sseEvent struct {
	Event   string
	Message string
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(logger *zap.SugaredLogger) *SSEHandler {
	return &SSEHandler{
		clients: make(map[chan sseEvent]bool),
		logger:  logger,
	}
}

// HandleEvent implements nexmosphere.EventHandler
func (h *SSEHandler) HandleEvent(event nexmosphere.Event) {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Errorf("Failed to marshal event: %s", err)
		return
	}

	sse := sseEvent{
		Event:   event.Type,
		Message: string(data),
	}

	h.logger.Debugf("SSE: type=%s action=%s", event.Type, event.Action)

	// Broadcast to all connected clients
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client <- sse:
		default:
			// Client buffer full, skip
			h.logger.Warnf("Client buffer full, dropping event")
		}
	}
}

// HandleHTTP handles HTTP SSE connections
func (h *SSEHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	// Check connection supports streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Connection does not support streaming", http.StatusBadRequest)
		return
	}

	// Create channel for this client
	clientChan := make(chan sseEvent, 100)

	h.mu.Lock()
	h.clients[clientChan] = true
	clientCount := len(h.clients)
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, clientChan)
		h.mu.Unlock()
		close(clientChan)
	}()

	// Set SSE headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	h.logger.Infof("SSE client connected from %s (total: %d)", r.RemoteAddr, clientCount)

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			h.logger.Infof("SSE client disconnected: %s", r.RemoteAddr)
			return

		case sse := <-clientChan:
			fmt.Fprintf(w, "event: %s\n", sse.Event)
			fmt.Fprintf(w, "data: %s\n\n", sse.Message)
			flusher.Flush()
		}
	}
}
