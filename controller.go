package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

type controllerMD struct {
	serialNo    string // Serial number of the controller
	productCode string // Product code from USB descriptor
	vid         string // VendorID from USB descriptor
	pid         string // ProductID from USB descriptor
}

type Queue int

const (
	system Queue = iota
	command
)

type controller struct {
	IsUSB   bool         // Is this a USB controller?
	port    serial.Port  // Serial port
	name    string       // Controller name
	md      controllerMD // Controller metadata
	devices [1000]device // Array of devices
	lastFB  feedback     // Last feedback received
	queue   [2][]string  // Command queue
	qTimer  *time.Ticker // Queue timer
}

type feedback struct {
	Type    string `json:"-"`              // Protocol Type
	Format  string `json:"-"`              // Protocol Command Format
	Command string `json:"-"`              // Protocol Command String
	Address int    `json:"address"`        // Address or ID of cause device
	Action  string `json:"action"`         // Action of event
	Data    string `json:"data,omitempty"` // Additional Data
	Raw     string `json:"raw"`            // Raw protocol command
}

var controllers = map[string]*controller{}

// sendSysyetemUpdate sends a system update to the controller
func sendSystemUpdate() {

	// local type for system
	s := struct {
		SensorConnection bool `json:"deviceConnection"`
		ControllerCount  int  `json:"controllerCount"`
		ClientCount      int  `json:"clientCount"`
	}{
		SensorConnection: len(controllers) > 0,
		ControllerCount:  len(controllers),
		ClientCount:      len(sseChan),
	}

	// Send System Update
	sendSSE("controller", s)

	// Set device types
	for _, c := range controllers {
		for i, d := range c.devices {
			if d.Type != "" {
				fb := &feedback{
					Address: i,
					Action:  "update",
					Data:    fmt.Sprintf("TYPE=%s", d.Type),
				}
				sendSSE("device", fb)
			}
		}
	}

}

// addToQueue adds a command to the queue
func (c *controller) addToQueue(q Queue, cmd string) {
	c.queue[q] = append(c.queue[q], cmd)
}

// getFromQueue returns the next command from the queue
func (c *controller) getFromQueue() string {
	var q Queue
	var x string
	switch {
	case len(c.queue[system]) > 0:
		q = system
	case len(c.queue[command]) > 0:
		q = command
	default:
		return ""
	}
	x, c.queue[q] = c.queue[q][0], c.queue[q][1:]
	return x
}

func (c *controller) getDevice(i int) *device {
	if i > 0 || i < 1000 {
		return &c.devices[i]
	}
	return nil
}

// decodeFeedback decodes a raw feedback string into a feedback struct
func (c *controller) decodeFeedback(data string) *feedback {

	// New feedback
	fb := &feedback{
		Raw:     data,
		Command: strings.TrimSpace(data[strings.Index(data, `[`)+1 : strings.Index(data, `]`)]),
	}

	// Handle XR Sensors
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

// listen scans incoming buffer for complete commands (Terminated by "CR+LF")
func (c *controller) listen() error {

	scanner := bufio.NewScanner(c.port)

	for scanner.Scan() {
		// log.Printf("rx: %s\n", scanner.Text())

		fb := c.decodeFeedback(scanner.Text())

		event := "unhandled"

		switch fb.Type {
		case "XR": // XR Antenna
			event, fb = c.doXRfb(fb)
		case "X": // X-Talk Command
			event, fb = c.doXfb(fb)
		case "D": // Diagnostic Command
			event, fb = c.doDiagnosticfb(fb)
		}

		if fb != nil {
			sendSSE(event, fb)
			c.lastFB = *fb
		}
	}

	return scanner.Err()
}

// write sends a command to controller
func (c *controller) write(cmd string) error {
	// log.Printf("tx: %s", cmd)
	_, err := c.port.Write([]byte(fmt.Sprintf("%s\r\n", cmd)))
	if err != nil {
		log.Errorf("can't write to serial: %s", err)
		return err
	}
	return err
}

func (c *controller) doXfb(fb *feedback) (string, *feedback) {

	// Set default to unknown
	fb.Action = "unknown"

	d := c.getDevice(fb.Address)
	if d == nil {
		return "unknown", fb
	}

	switch d.Type {

	case "XTB4N6": // 4 Button XT-B4
		return "button", d.processFbXTB4N6(fb)

	case "XRDR1": // RFID Reader
		return "rfid-antenna", c.processFbXRDR1(fb)

	default:
		return "unknown", fb

	}
}

func (c *controller) doXRfb(fb *feedback) (string, *feedback) {

	switch fb.Command[0:2] {
	case "PU":
		fb.Action = "pickup"
	case "PB":
		fb.Action = "putback"
	default:
		fb.Action = "unknown"
	}

	return "rfid-tag", fb
}

func (c *controller) doDiagnosticfb(fb *feedback) (string, *feedback) {

	// Split up the command
	s := strings.Split(fb.Command, "=")

	// Return if the command is out of scope
	if len(s) < 2 && fb.Address > 999 {
		return "system-unhandled", fb
	}

	// Save diagnstics data against correct port
	switch s[0] {
	case "TYPE":
		c.devices[fb.Address].Type = s[1]
		switch s[1] {
		case "XRDR1":
			c.addToQueue(system, fmt.Sprintf("X%03dB[]", fb.Address))
		}
	case "SERIAL":
		c.devices[fb.Address].Serial = s[1]
	}

	// Set feedback
	fb.Data = fb.Command
	fb.Action = "update"

	return "device", fb
}
