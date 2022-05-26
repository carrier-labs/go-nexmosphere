package main

import (
	"bufio"

	"go.bug.st/serial"
)

type deviceMD struct {
	serialNo    string
	productCode string
	vid         string // Vendor ID
	pid         string // Product ID
}

type device struct {
	md    deviceMD
	IsUSB bool
	dev   serial.Port
	name  string
}

var devices = map[string]*device{}

// listen scans incoming buffer for complete commands (Terminated by "CR+LF")
func (d *device) listen() error {

	scanner := bufio.NewScanner(d.dev)
	for scanner.Scan() {
		// fmt.Printf("rx: %s\n", scanner.Text())
		sendSSE(ssEvent{
			Type:    "update",
			Message: scanner.Text(),
		})
	}

	return scanner.Err()
}

// func (d *device) write(cmd string) error {
// 	log.Printf("tx: %s", cmd)
// 	i, err := d.dev.Write([]byte(fmt.Sprintf("%s\r\n", cmd)))
// 	if err != nil {
// 		log.Printf("oop=%s", err)
// 		return err
// 	}
// 	log.Printf("Bytes Written: %d", i)
// 	return err
// }
