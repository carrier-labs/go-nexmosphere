package main

import (
	"fmt"

	"go.bug.st/serial"
)

type device struct {
	meta struct {
		serialNo    string
		productCode string
		IsUSB       string
		vid         string // Vendor ID
		pid         string // Product ID
	}
	dev  serial.Port
	name string
}

func (d *device) listen() error {
	buff := make([]byte, 100)
	for {
		n, err := d.dev.Read(buff)
		if err != nil {
			return (err)
		}
		if n == 0 {
			return fmt.Errorf("EOF")
		}
		fmt.Printf("%v", string(buff[:n]))
	}
}
