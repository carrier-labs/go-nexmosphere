package nexmosphere

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Button represents the state of a physical button
type Button struct {
	Closed     bool          // Physical button state (wire closed)
	Pressed    bool          // Logical button state (debounced)
	PressedAt  time.Time     // Timestamp when button was pressed
	holdCancel chan struct{} // Channel to stop hold ticker goroutine
}

const buttonCount int = 4
const defaultHoldTickInterval = 500 * time.Millisecond

// Device represents a Nexmosphere device connected to a controller
type Device struct {
	Type             string
	Serial           string
	Button           [buttonCount]Button
	HoldTickInterval time.Duration // Interval for emitting hold events (default: 500ms, set to 0 to disable)
}

// setButton sets the state of a button and dispatches events
func (d *Device) setButton(buttonID int, state bool, fb *feedback, c *Controller) {
	// Check if button exists
	if buttonID > buttonCount || buttonID < 1 {
		return
	}

	// Get pointer to button for easier access
	b := &d.Button[buttonID-1]

	// If state hasn't changed, return
	if b.Closed == state {
		return
	}

	// Set state
	b.Closed = state

	// Send raw switch update
	event := Event{
		Address: fb.Address,
		Data:    fmt.Sprintf("%02d", buttonID),
		Raw:     fb.Raw,
	}

	if state {
		event.Action = "closed"
		// Record press time
		b.PressedAt = time.Now()
	} else {
		event.Action = "open"
		// Calculate hold duration
		if !b.PressedAt.IsZero() {
			event.Duration = time.Since(b.PressedAt)
			b.PressedAt = time.Time{} // Reset
		}
		// Stop hold ticker if running
		if b.holdCancel != nil {
			close(b.holdCancel)
			b.holdCancel = nil
		}
		// Reset pressed state
		b.Pressed = false
	}

	event.Type = "button"
	event.Controller = c.name
	c.service.dispatch(event)

	// If button was opened (released), also send logical "release" event
	if !state {
		releaseEvent := event
		releaseEvent.Action = "release"
		c.service.dispatch(releaseEvent)
	}

	// If switch is closed, send pressed update
	if b.Closed && !b.Pressed {
		b.Pressed = true
		event.Action = "press"
		event.Type = "button"
		event.Controller = c.name
		c.service.dispatch(event)

		// Start hold ticker if interval configured
		if d.HoldTickInterval > 0 {
			b.holdCancel = make(chan struct{})
			go func(ev Event, cancel chan struct{}) {
				ticker := time.NewTicker(d.HoldTickInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if !b.PressedAt.IsZero() {
							holdEvent := ev
							holdEvent.Action = "hold"
							holdEvent.Duration = time.Since(b.PressedAt)
							c.service.dispatch(holdEvent)
						}
					case <-cancel:
						return
					}
				}
			}(event, b.holdCancel)
		}
	}
}

// processFbXTB4N6 processes feedback from XTB4N6 Push Button Interface
func (d *Device) processFbXTB4N6(fb *feedback, c *Controller) *Event {
	switch fb.Format {
	case "A":
		// Get raw button state
		i, _ := strconv.Atoi(fb.Command)
		// If greater than 0 adjust
		if i > 0 {
			i = i - 1
		}
		// Set button states
		for b := 1; b <= 4; b++ {
			d.setButton(b, i&(int(math.Pow(2, float64(b)))) > 0, fb, c)
		}
	}
	return nil
}

// processFbXY240 processes feedback from XY240 X-Eye Presence & AirButton Sensor
func (d *Device) processFbXY240(fb *feedback) *Event {
	event := &Event{
		Address: fb.Address,
		Raw:     fb.Raw,
	}

	switch fb.Format {
	case "B":
		// Split command into parts
		parts := strings.Split(fb.Command, "=")
		if len(parts) != 2 {
			return nil
		}
		switch parts[0] {
		case "Dz": // Detection Zone
			event.Action = "detection-zone"
			event.Data = parts[1]
			return event
		}
	}

	return nil
}

// processFbXRDR1 processes feedback from XRDR1 RFID Reader
func (c *Controller) processFbXRDR1(fb *feedback) *Event {
	event := &Event{
		Address: fb.Address,
		Raw:     fb.Raw,
	}

	switch fb.Format {
	case "A":
		switch fb.Command {
		case "1":
			event.Action = "pickup"
			if c.lastFB != nil {
				event.Data = fmt.Sprintf("%03d", c.lastFB.Address)
			}
		case "0":
			event.Action = "putback"
			if c.lastFB != nil {
				event.Data = fmt.Sprintf("%03d", c.lastFB.Address)
			}
		default:
			return nil
		}
		return event

	case "B":
		event.Action = "status"

		// Send additional updates for each tag
		tags := strings.Split(fb.Command, " ")
		for _, tag := range tags {
			add, _ := strconv.Atoi(strings.TrimPrefix(tag, "d"))
			if add == 0 {
				continue
			}

			// Dispatch additional putback event for each tag
			tagEvent := Event{
				Type:       "rfid-antenna",
				Controller: c.name,
				Address:    fb.Address,
				Action:     "putback",
				Data:       fmt.Sprintf("%03d", add),
				Raw:        fb.Raw,
			}
			c.service.dispatch(tagEvent)
		}

		return event
	}

	return nil
}
