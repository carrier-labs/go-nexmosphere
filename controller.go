package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

type controllerMD struct {
	serialNo    string
	productCode string
	vid         string // Vendor ID
	pid         string // Product ID
}

type Queue int

const (
	system Queue = iota
	command
)

type controller struct {
	IsUSB   bool
	port    serial.Port
	name    string
	devices [1000]device
	md      controllerMD
	lastFB  feedback
	queue   [2][]string
	qTimer  *time.Ticker
}

type device struct {
	Type   string
	Serial string
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

func (c *controller) addToQueue(q Queue, cmd string) {
	c.queue[q] = append(c.queue[q], cmd)
}

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

// listen scans incoming buffer for complete commands (Terminated by "CR+LF")
func (c *controller) listen() error {

	scanner := bufio.NewScanner(c.port)

	for scanner.Scan() {
		// log.Printf("rx: %s\n", scanner.Text())

		fb := c.decodeFeedback(scanner.Text())

		sse := ssEvent{
			Event: "unhandled",
		}
		switch fb.Type {
		case "XR":
			sse.Event, fb = c.doXRfb(fb)
		case "X":
			sse.Event, fb = c.doXfb(fb)
		case "D":
			sse.Event, fb = c.doDfb(fb)
		}

		b, _ := json.Marshal(fb)
		sse.Message = string(b)
		sendSSE(sse)

		c.lastFB = fb
	}

	return scanner.Err()
}

func (c *controller) write(cmd string) error {
	// log.Printf("tx: %s", cmd)
	_, err := c.port.Write([]byte(fmt.Sprintf("%s\r\n", cmd)))
	if err != nil {
		log.Errorf("can't write to serial: %s", err)
		return err
	}
	return err
}

func (c *controller) decodeFeedback(data string) feedback {

	// New feedback
	fb := feedback{
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

func (c *controller) doXfb(fb feedback) (string, feedback) {

	// Set default to unknown
	fb.Action = "unknown"

	d := c.getDevice(fb.Address)
	if d == nil {
		return "", fb
	}

	switch d.Type {
	case "XRDR1":
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
	}

	return "rfid-antenna", fb
}

func (c *controller) doXRfb(fb feedback) (string, feedback) {

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

func (c *controller) doDfb(fb feedback) (string, feedback) {

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

	return "system", fb
}
