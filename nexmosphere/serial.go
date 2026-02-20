package nexmosphere

import (
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// scanForControllers enumerates over all connected USB devices identifying Nexmosphere controllers
func (s *Service) scanForControllers() {
	// Get all possible Serial Ports
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		s.logger.Errorf("Failed to enumerate ports: %s", err)
		return
	}

	if len(ports) == 0 {
		s.logger.Debugf("No serial ports found")
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
		s.mu.RLock()
		_, exists := s.controllers[port.Name]
		s.mu.RUnlock()

		if exists {
			continue
		}

		// Create controller and open port
		c, err := s.openController(port)
		if err != nil {
			s.logger.Debugf("Failed to open controller %s: %s", port.Name, err)
			continue
		}

		s.mu.Lock()
		s.controllers[port.Name] = c
		s.mu.Unlock()

		s.sendSystemUpdate()

		// Listen to port, cleanup on close
		s.logger.Infof("Listening: %v", port.Name)
		go func(c *Controller) {
			err := c.listen()
			s.logger.Errorf("Closing: %s: %s", c.name, err)

			// Close the controller
			if closeErr := c.close(); closeErr != nil {
				s.logger.Errorf("Error closing controller %s: %s", c.name, closeErr)
			}

			// Remove from map
			s.mu.Lock()
			delete(s.controllers, c.name)
			s.mu.Unlock()

			s.sendSystemUpdate()
		}(c)

		// Pause before starting comms ticker
		time.Sleep(10 * time.Second)

		// Send commands to get device information
		c.pendingDeviceQueries = 8
		for i := 1; i <= 8; i++ {
			s.logger.Debugf("Sending info request to address %d", i)
			c.addToQueue(systemQueue, fmt.Sprintf("D%03dB[TYPE]", i))
		}

		// Timeout for ready state if devices don't respond
		go func(ctrl *Controller) {
			time.Sleep(5 * time.Second)
			if !ctrl.ready {
				ctrl.ready = true
				devicesFound := 8 - ctrl.pendingDeviceQueries
				s.logger.Infof("Controller %s ready - %d device(s) found", ctrl.name, devicesFound)
				s.dispatch(Event{
					Type:       "controller",
					Controller: ctrl.name,
					Action:     "ready",
					Data:       fmt.Sprintf("%d device(s) found", devicesFound),
				})
			}
		}(c)

		// Start command queue processor
		go func(c *Controller) {
			c.qTimer = time.NewTicker(250 * time.Millisecond)
			defer c.qTimer.Stop()

			for range c.qTimer.C {
				cmd := c.getFromQueue()
				if cmd != "" {
					c.write(cmd)
				}
			}
		}(c)
	}
}

// checkForUSB returns true if a port matches Nexmosphere USB profile
func checkForUSB(port *enumerator.PortDetails) bool {
	// Nexmosphere uses Prolific Technology devices:
	// VID 067b: Prolific Technology, Inc
	// PID 2303: PL2303 Serial Port
	// PID 23a3: ATEN Serial Bridge
	// PID 23d3: PL2303GL Serial Port
	switch strings.ToLower(port.VID) {
	case "067b": // Prolific Technology, Inc
		switch strings.ToLower(port.PID) {
		case "2303", "23a3", "23d3":
			return true
		}
	}

	return false
}

// checkForRS232 returns false always, placeholder for future RS232 support
func checkForRS232(_ *enumerator.PortDetails) bool {
	return false
}

// openController opens and configures a controller
func (s *Service) openController(port *enumerator.PortDetails) (*Controller, error) {
	c := &Controller{
		md: controllerMD{
			serialNo:    "",
			productCode: "",
			vid:         port.VID,
			pid:         port.PID,
		},
		name:    port.Name,
		isUSB:   port.IsUSB,
		service: s,
	}

	// Configure Serial (RS232) Mode
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	// Open the port
	var err error
	c.port, err = serial.Open(port.Name, mode)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// sendSystemUpdate dispatches a system update event
func (s *Service) sendSystemUpdate() {
	s.mu.RLock()
	controllerCount := len(s.controllers)
	handlerCount := len(s.handlers)
	s.mu.RUnlock()

	event := Event{
		Type:   "controller",
		Action: "system-update",
		Data:   fmt.Sprintf("controllers=%d,handlers=%d", controllerCount, handlerCount),
	}

	s.dispatch(event)

	// Send device type updates for all devices
	s.mu.RLock()
	controllers := make([]*Controller, 0, len(s.controllers))
	for _, c := range s.controllers {
		controllers = append(controllers, c)
	}
	s.mu.RUnlock()

	for _, c := range controllers {
		for i, d := range c.devices {
			if d != nil && d.Type != "" {
				deviceEvent := Event{
					Type:       "device",
					Controller: c.name,
					Address:    i,
					Action:     "update",
					Data:       fmt.Sprintf("TYPE=%s", d.Type),
				}
				s.dispatch(deviceEvent)
			}
		}
	}
}
