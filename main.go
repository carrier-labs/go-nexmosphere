package main

import (
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

var devices = map[string]device{}

func main() {
	scanForDevices()
	ticker := time.NewTicker(time.Second * 2)
	for range ticker.C {
		scanForDevices()
	}

	// n, err := port.Write([]byte("10,20,30\n\r"))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Printf("Sent %v bytes\n", n)

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
		isNexmosphere := false
		// fmt.Printf("Found Serial port: %s\n", port.Name)
		if port.IsUSB {
			// Nexmosphere uses one of the following:
			// VID 067b: Prolific Technology, Inc
			// PID 2303: PL2303 Serial Port
			switch port.VID {
			case "067b": // Prolific Technology, Inc
				switch port.PID {
				case "2303": //PL2303 Serial Port
					isNexmosphere = true
				}
			}
		}

		if isNexmosphere {
			if _, ok := devices[port.Name]; !ok {
				d := device{}
				d.name = port.Name
				d.meta.vid = port.VID
				d.meta.pid = port.PID

				mode := &serial.Mode{
					BaudRate: 115200,
				}
				dev, err := serial.Open(port.Name, mode)
				if err != nil {
					log.Printf("%s:%s", port.Name, err)
				}
				d.dev = dev
				fmt.Printf("Adding Device %v\n", port.Name)
				devices[port.Name] = d
				go func(d *device) {
					err := d.listen()
					log.Printf("Error:  %s:%s", d.name, err)
					log.Printf("Remove: %s", d.name)
					err = d.dev.Close()
					if err != nil {
						log.Printf("Close:  %s:%s", d.name, err)
					}
					delete(devices, d.name)
				}(&d)
			}
		}
	}

}
