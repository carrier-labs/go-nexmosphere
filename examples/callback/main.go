package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.org/carrierlabs/go-nexmosphere/nexmosphere"
	"go.uber.org/zap"
)

func main() {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	log := logger.Sugar()
	defer log.Sync()

	log.Info("Nexmosphere Callback Example")
	log.Info("=============================")

	// Create Nexmosphere service
	service := nexmosphere.NewService(
		nexmosphere.WithLogger(log),
	)

	// Register event handlers using function adapter
	service.AddHandler(nexmosphere.EventHandlerFunc(func(e nexmosphere.Event) {
		// Optional: Customize hold interval for specific devices (default is 500ms)
		// Uncomment below and add "time" import to customize:
		// if e.Type == "device" && e.Action == "TYPE" {
		// 	service.SetDeviceHoldInterval(e.Controller, e.Address, 200*time.Millisecond)
		// }

		switch e.Type {
		case "button":
			handleButtonEvent(e)
		case "rfid-tag":
			handleRFIDTagEvent(e)
		case "rfid-antenna":
			handleRFIDAntennaEvent(e)
		case "presence":
			handlePresenceEvent(e)
		case "controller":
			handleControllerEvent(e)
		case "device":
			handleDeviceEvent(e)
		}
	}))

	// Start the service
	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start service: %s", err)
	}
	defer service.Stop()

	log.Info("Service started. Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")
}

func handleButtonEvent(e nexmosphere.Event) {
	// Display duration for hold, open, and release events
	durationStr := ""
	if e.Duration > 0 {
		durationStr = fmt.Sprintf(" [held: %s]", e.Duration)
	}

	fmt.Printf("🔘 BUTTON [%s] Address %d: %s (Data: %s)%s\n",
		e.Controller, e.Address, e.Action, e.Data, durationStr)

	// Example: Execute custom logic based on button press
	if e.Action == "press" {
		buttonNum := e.Data
		fmt.Printf("   → Button %s was pressed! Execute your logic here.\n", buttonNum)
	} else if e.Action == "hold" {
		// Hold events fire periodically if HoldTickInterval is configured on the device
		fmt.Printf("   → Button is being held for %s\n", e.Duration)
	} else if e.Action == "release" {
		// Release event (paired with open) includes total hold duration
		if e.Duration > 0 {
			fmt.Printf("   → Button was held for a total of %s\n", e.Duration)
		}
	}
}

func handleRFIDTagEvent(e nexmosphere.Event) {
	fmt.Printf("🏷️  RFID TAG Address %d: %s\n", e.Address, e.Action)

	// Example: Track inventory or trigger actions based on tag pickup/putback
	if e.Action == "pickup" {
		fmt.Printf("   → Tag %d picked up. Update inventory system.\n", e.Address)
	} else if e.Action == "putback" {
		fmt.Printf("   → Tag %d put back. Restore inventory.\n", e.Address)
	}
}

func handleRFIDAntennaEvent(e nexmosphere.Event) {
	fmt.Printf("📡 RFID ANTENNA [%s] Address %d: %s (Tag: %s)\n",
		e.Controller, e.Address, e.Action, e.Data)
}

func handlePresenceEvent(e nexmosphere.Event) {
	fmt.Printf("👤 PRESENCE [%s] Address %d: %s (Data: %s)\n",
		e.Controller, e.Address, e.Action, e.Data)

	// Example: Track customer presence in retail environment
	if e.Action == "detection-zone" {
		fmt.Printf("   → Presence detected in zone %s\n", e.Data)
	}
}

func handleControllerEvent(e nexmosphere.Event) {
	if e.Action == "ready" {
		fmt.Printf("✅ CONTROLLER READY: %s - %s\n", e.Controller, e.Data)
	} else {
		fmt.Printf("🎛️  CONTROLLER: %s (%s)\n", e.Action, e.Data)
	}
}

func handleDeviceEvent(e nexmosphere.Event) {
	fmt.Printf("🔧 DEVICE [%s] Address %d: %s (%s)\n",
		e.Controller, e.Address, e.Action, e.Data)
}
