package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Button struct {
	Closed  bool // Physical button state (wire closed)
	Pressed bool // Logical button state (debounced)
}

const BUTTON_COUNT int = 4 // Number of buttons

var DEBOUNCE int = 1 // Debounce time in seconds

type device struct {
	Type   string
	Serial string
	Button [BUTTON_COUNT]Button
}

// setButton(buttonID int,state) sets the state of a button
func (d *device) setButton(buttonID int, state bool, fb *feedback) {

	// Check if button exists
	if buttonID > BUTTON_COUNT {
		return
	}

	// get pointer to button for easier access
	b := &d.Button[buttonID-1]

	// If state hasn't changed, return
	if b.Closed == state {
		return
	}

	// Set state
	b.Closed = state

	// Send raw switch update
	f := feedback{
		Address: fb.Address,
		Data:    fmt.Sprintf("%02d", buttonID),
		Raw:     fb.Raw,
	}
	if state {
		f.Action = "closed"
	} else {
		f.Action = "open"
	}
	sendSSE("button", f)

	// If switch is closed, send pressed update
	if b.Closed && !b.Pressed {
		b.Pressed = true
		f.Action = "press"
		sendSSE("button", f)
		go func(f feedback) {
			// Wait for debounce time
			time.Sleep(time.Second * time.Duration(DEBOUNCE))
			b.Pressed = false
			f.Action = "release"
			sendSSE("button", f)
		}(f)
	}

}

// processFbXTB4N6 processes feedback from XTB4N6 Push Button Interface
func (d *device) processFbXTB4N6(fb *feedback) *feedback {
	switch fb.Format {
	case "A":
		// Get raw button state
		i, _ := strconv.Atoi(fb.Command)
		// if greater than 0 adjust
		if i > 0 {
			i = i - 1
		}
		// Set button states
		for b := 1; b <= 4; b++ {
			d.setButton(b, i&(int(math.Pow(2, float64(b)))) > 0, fb)
		}
	}
	return nil
}

// processFbXRDR1 processes feedback from XRDR1 RFID Reader
func (c *controller) processFbXRDR1(fb *feedback) *feedback {
	switch fb.Format {
	case "A":
		switch fb.Command {
		case "1":
			fb.Action = "pickup"
			fb.Data = fmt.Sprintf("%03d", c.lastFB.Address)
		case "0":
			fb.Action = "putback"
			fb.Data = fmt.Sprintf("%03d", c.lastFB.Address)
		default:
		}
	case "B":
		fb.Action = "status"
		// Send additional updates
		tags := strings.Split(fb.Command, " ")
		for _, tag := range tags {
			add, _ := strconv.Atoi(strings.TrimPrefix(tag, "d"))
			if add == 0 {
				continue
			}
			// Send rfid-antenna update
			f := feedback{
				Address: fb.Address,
				Action:  "putback",
				Data:    fmt.Sprintf("%03d", add),
				Raw:     fb.Raw,
			}

			sendSSE("rfid-antenna", f)
		}
	}

	return fb
}
