package main

import (
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

func watchNexmosphere() {
	scanForDevices()
	ticker := time.NewTicker(time.Second * 2)
	for range ticker.C {
		scanForDevices()
	}
}

// scanForDevices enumerates over all connected USB identifying Nexmosphere devices
// It populates the devices map when it finds one
func scanForDevices() {

	// Get all possible Serial Ports
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(ports) == 0 {
		fmt.Println("No serial ports found!")
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
		if _, ok := devices[port.Name]; ok {
			continue
		}

		// Create device and open port
		d, err := getDevice(port)
		if err != nil {
			log.Printf("serial port:%s", err)
			continue
		}
		devices[port.Name] = d

		// LIsten to port, delete on close
		log.Printf("Listening: %v\n", port.Name)
		go func(d *device) {
			err := d.listen()
			log.Printf("Closing:  %s:%s", d.name, err)
			err = d.dev.Close()
			if err != nil {
				log.Printf("close device:  %s:%s", d.name, err)
			}
			delete(devices, d.name)
		}(d)

		// Commands to device currently not responding
		// time.Sleep(time.Second * 2)
		// d.write("D001B[TYPE]")

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

func getDevice(port *enumerator.PortDetails) (*device, error) {
	var err error
	// Create new Device with details
	d := &device{
		md: deviceMD{
			serialNo:    "",
			productCode: "",
			vid:         port.VID,
			pid:         port.PID,
		},
		dev:   nil,
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
	d.dev, err = serial.Open(port.Name, mode)
	if err != nil {
		return nil, err

	}

	return d, nil
}
