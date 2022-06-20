package main

import (
	"fmt"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

func watchNexmosphere() {
	scanForControllers()
	ticker := time.NewTicker(time.Second * 2)
	for range ticker.C {
		scanForControllers()
	}
}

// scanForControllers enumerates over all connected USB identifying Nexmosphere controlers
// It populates the controllers map when it finds one
func scanForControllers() {

	// Get all possible Serial Ports
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(ports) == 0 {
		log.Debugf("No serial ports found!")
		return
	}

	for _, port := range ports {
		var isNexmosphere bool

		// Check port for Nexmosphere device
		switch port.IsUSB {
		case true:
			isNexmosphere = checkForUSB(port)
		case false:
			isNexmosphere = checkForRS232(port)
		}

		// If not Nexmosphere, bail
		if !isNexmosphere {
			continue
		}

		// If port already added, bail
		if _, ok := controllers[port.Name]; ok {
			continue
		}

		// Create controller and open port
		c, err := getController(port)
		if err != nil {
			log.Infof("Found serial port: %s", err)
			continue
		}

		controllers[port.Name] = c

		// Listen to port, delete on close
		log.Infof("Listening: %v\n", port.Name)
		go func(c *controller) {
			err := c.listen()
			log.Errorf("Closing:  %s:%s", c.name, err)

			err = c.port.Close()
			if err != nil {
				log.Errorf("close controller:  %s:%s", c.name, err)
			}

			if c.qTimer != nil {
				c.qTimer.Stop()
				c.qTimer = nil
			}

			delete(controllers, c.name)
		}(c)

		// Send commands to get data
		for i := 1; i <= 8; i++ {
			c.addToQueue(system, fmt.Sprintf("D%03dB[TYPE]", i))
		}

		// Pause before starting comms ticker
		time.Sleep(5 * time.Second)
		go func(c *controller) {
			c.qTimer = time.NewTicker(250 * time.Millisecond)
			for range c.qTimer.C {
				cmd := c.getFromQueue()
				if cmd != "" {
					c.write(cmd)
				}
			}
		}(c)

	}
}

// checkForUSB will return true if a port matches Nexmosphere profile
func checkForUSB(port *enumerator.PortDetails) bool {

	// Nexmosphere uses one of the following:
	// VID 067b: Prolific Technology, Inc
	// PID 2303: PL2303 Serial Port
	switch port.VID {
	case "067b": // Prolific Technology, Inc
		switch port.PID {
		case "2303": //PL2303 Serial Port
			return true
		}
	}

	// Return false by default
	return false
}

// checkForRS232 returns false always, is placeholder for future RS232 link code via passed config
func checkForRS232(port *enumerator.PortDetails) bool {
	return false
}

func getController(port *enumerator.PortDetails) (*controller, error) {
	var err error
	// Create new Device with details
	d := &controller{
		md: controllerMD{
			serialNo:    "",
			productCode: "",
			vid:         port.VID,
			pid:         port.PID,
		},
		port:  nil,
		name:  port.Name,
		IsUSB: port.IsUSB,
	}

	// Configure Serial (RS232) Mode
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,   // Default
		StopBits: serial.OneStopBit, // Default
	}

	// Open the port
	d.port, err = serial.Open(port.Name, mode)
	if err != nil {
		return nil, err

	}

	return d, nil
}
