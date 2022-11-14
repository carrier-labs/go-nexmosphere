package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type device struct {
	Type   string
	Serial string
	State  struct {
		ButtonPressed [4]bool
	}
}

// setButtonPressed(buttonID int,state) sets the state of a button
func (d *device) setButtonPressed(buttonID int, state bool, fb *feedback) {
	// If state hasn't changed, return
	if d.State.ButtonPressed[buttonID-1] == state {
		return
	}

	// Set state
	d.State.ButtonPressed[buttonID-1] = state
	// Button Update
	f := feedback{
		Address: fb.Address,
		Data:    fmt.Sprintf("%02d", buttonID),
		Raw:     fb.Raw,
	}
	// Set actual state
	if state {
		f.Action = "press"
	} else {
		f.Action = "release"
	}
	// Send button update
	b, _ := json.Marshal(f)
	sse := ssEvent{
		Event:   "button",
		Message: string(b),
	}
	sendSSE(sse)
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
			d.setButtonPressed(b, i&(int(math.Pow(2, float64(b)))) > 0, fb)
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
			b, _ := json.Marshal(f)
			sse := ssEvent{
				Event:   "rfid-antenna",
				Message: string(b),
			}
			sendSSE(sse)
		}
	}

	return fb
}
