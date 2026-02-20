package nexmosphere

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

type controllerMD struct {
	serialNo    string
	productCode string
	vid         string
	pid         string
}

type queue int

const (
	systemQueue queue = iota
	commandQueue
)

// Controller manages communication with a Nexmosphere controller
type Controller struct {
	isUSB                bool
	port                 serial.Port
	name                 string
	md                   controllerMD
	devices              [1000]*Device
	lastFB               *feedback
	queue                [2][]string
	qTimer               *time.Ticker
	service              *Service
	ready                bool
	pendingDeviceQueries int
}

type feedback struct {
	Type    string
	Format  string
	Command string
	Address int
	Action  string
	Data    string
	Raw     string
}

// addToQueue adds a command to the queue
func (c *Controller) addToQueue(q queue, cmd string) {
	c.queue[q] = append(c.queue[q], cmd)
}

// getFromQueue returns the next command from the queue
func (c *Controller) getFromQueue() string {
	var q queue
	var x string
	switch {
	case len(c.queue[systemQueue]) > 0:
		q = systemQueue
	case len(c.queue[commandQueue]) > 0:
		q = commandQueue
	default:
		return ""
	}
	x, c.queue[q] = c.queue[q][0], c.queue[q][1:]
	return x
}

// getDevice returns a device by address
func (c *Controller) getDevice(i int) *Device {
	if i >= 0 && i < 1000 {
		if c.devices[i] == nil {
			c.devices[i] = &Device{
				HoldTickInterval: defaultHoldTickInterval,
			}
		}
		return c.devices[i]
	}
	return nil
}

// decodeFeedback decodes a raw feedback string into a feedback struct
func (c *Controller) decodeFeedback(data string) *feedback {
	fb := &feedback{
		Raw:     data,
		Command: strings.TrimSpace(data[strings.Index(data, `[`)+1 : strings.Index(data, `]`)]),
	}

	// Handle XR Sensors (RFID tags)
	if strings.HasPrefix(data, "XR") {
		fb.Type = "XR"
		fb.Address, _ = strconv.Atoi(fb.Command[2:])
		return fb
	}

	fb.Type = data[0:1]
	fb.Address, _ = strconv.Atoi(data[1:4])
	fb.Format = data[4:5]

	return fb
}

// listen scans incoming buffer for complete commands (terminated by CR+LF)
func (c *Controller) listen() error {
	scanner := bufio.NewScanner(c.port)

	for scanner.Scan() {
		fb := c.decodeFeedback(scanner.Text())

		eventType := "unhandled"
		var event *Event

		switch fb.Type {
		case "XR": // XR Antenna (RFID tag events)
			eventType, event = c.doXRfb(fb)
		case "X": // X-Talk Command (device events)
			eventType, event = c.doXfb(fb)
		case "D": // Diagnostic Command
			eventType, event = c.doDiagnosticfb(fb)
		}

		if event != nil {
			event.Type = eventType
			event.Controller = c.name
			c.service.dispatch(*event)
			c.lastFB = fb
		}
	}

	return scanner.Err()
}

// write sends a command to controller
func (c *Controller) write(cmd string) error {
	_, err := c.port.Write([]byte(fmt.Sprintf("%s\r\n", cmd)))
	if err != nil {
		c.service.logger.Errorf("can't write to serial %s: %s", c.name, err)
		return err
	}
	return nil
}

// close closes the controller port and stops the queue timer
func (c *Controller) close() error {
	if c.qTimer != nil {
		c.qTimer.Stop()
		c.qTimer = nil
	}
	if c.port != nil {
		return c.port.Close()
	}
	return nil
}

// getInfo returns controller information
func (c *Controller) getInfo() ControllerInfo {
	deviceCount := 0
	for _, d := range c.devices {
		if d != nil && d.Type != "" {
			deviceCount++
		}
	}

	return ControllerInfo{
		Name:        c.name,
		IsUSB:       c.isUSB,
		VID:         c.md.vid,
		PID:         c.md.pid,
		DeviceCount: deviceCount,
	}
}

// doXfb handles X-Talk feedback (device events)
func (c *Controller) doXfb(fb *feedback) (string, *Event) {
	d := c.getDevice(fb.Address)
	if d == nil {
		return "unknown", nil
	}

	switch d.Type {
	case "XTB4N6": // 4 Button XT-B4
		return "button", d.processFbXTB4N6(fb, c)

	case "XRDR1": // RFID Reader
		return "rfid-antenna", c.processFbXRDR1(fb)

	case "XY240": // X-Eye Presence & Airbutton
		return "presence", d.processFbXY240(fb)

	default:
		return "unknown", nil
	}
}

// doXRfb handles XR feedback (RFID tag events)
func (c *Controller) doXRfb(fb *feedback) (string, *Event) {
	event := &Event{
		Address: fb.Address,
		Raw:     fb.Raw,
	}

	switch fb.Command[0:2] {
	case "PU":
		event.Action = "pickup"
	case "PB":
		event.Action = "putback"
	default:
		event.Action = "unknown"
	}

	return "rfid-tag", event
}

// doDiagnosticfb handles diagnostic feedback
func (c *Controller) doDiagnosticfb(fb *feedback) (string, *Event) {
	// Split up the command
	s := strings.Split(fb.Command, "=")

	// Return if the command is out of scope
	if len(s) < 2 || fb.Address > 999 {
		return "system-unhandled", nil
	}

	d := c.getDevice(fb.Address)
	if d == nil {
		return "system-unhandled", nil
	}

	// Save diagnostics data against correct device
	switch s[0] {
	case "TYPE":
		d.Type = s[1]
		// For RFID readers, request status
		if s[1] == "XRDR1" {
			c.addToQueue(systemQueue, fmt.Sprintf("X%03dB[]", fb.Address))
		}
		// Track device query completion
		if c.pendingDeviceQueries > 0 {
			c.pendingDeviceQueries--
			if c.pendingDeviceQueries == 0 && !c.ready {
				c.ready = true
				c.service.logger.Infof("Controller %s ready - all devices initialized", c.name)
				c.service.dispatch(Event{
					Type:       "controller",
					Controller: c.name,
					Action:     "ready",
					Data:       "All devices initialized",
				})
			}
		}
	case "SERIAL":
		d.Serial = s[1]
	}

	// Create device update event
	event := &Event{
		Address: fb.Address,
		Action:  "update",
		Data:    fb.Command,
		Raw:     fb.Raw,
	}

	return "device", event
}
