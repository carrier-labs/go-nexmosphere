package nexmosphere

import "time"

// Event represents a device event from a Nexmosphere controller
type Event struct {
	Type       string        `json:"type"`
	Controller string        `json:"controller"`
	Address    int           `json:"address"`
	Action     string        `json:"action"`
	Data       string        `json:"data,omitempty"`
	Raw        string        `json:"raw,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// EventHandler handles events from Nexmosphere controllers
type EventHandler interface {
	HandleEvent(event Event)
}

// EventHandlerFunc is a function adapter for EventHandler interface
type EventHandlerFunc func(Event)

// HandleEvent calls the function
func (f EventHandlerFunc) HandleEvent(e Event) {
	f(e)
}
