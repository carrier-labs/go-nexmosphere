package main

import (
	"bufio"
	"fmt"
	"log"

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
	// buff := make([]byte, 100)
	// for {
	// 	n, err := d.dev.Read(buff)
	// 	if err != nil {
	// 		return (err)
	// 	}
	// 	if n == 0 {
	// 		return fmt.Errorf("EOF")
	// 	}
	// 	fmt.Printf("%v", string(buff[:n]))
	// }

	scanner := bufio.NewScanner(d.dev)
	for scanner.Scan() {
		fmt.Println(scanner.Text()) // Println will add back the final '\n'
	}

	return scanner.Err()
}

func (d *device) write(cmd string) error {
	_, err := d.dev.Write([]byte(fmt.Sprintf("%s\n\r", cmd)))
	log.Println(fmt.Sprintf("%s\n\r", cmd))
	return err
}
