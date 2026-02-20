# go-nexmosphere

Go library and server for communication with Nexmosphere sensor controllers on Linux.

Automatically discovers USB-based Nexmosphere controllers, parses their proprietary protocol, and dispatches device events via callbacks or Server-Sent Events (SSE).

## Features

- **Dual usage modes**: Use as a Go library with callback handlers, or as a standalone HTTP/SSE server
- **Auto-discovery**: Automatically detects Nexmosphere controllers on USB ports
- **Protocol parsing**: Handles X-Talk, XR (RFID), and diagnostic protocols
- **Device support**: Buttons (XTB4N6), RFID readers (XRDR1), presence sensors (XY240)
- **Event-driven**: Non-blocking event dispatch to multiple handlers

## Linux Permissions

Serial ports require proper permissions. Choose one option:

**Option 1: Add user to dialout group (recommended)**

```bash
sudo usermod -a -G dialout $USER
# Log out and back in, or run: newgrp dialout
```

**Option 2: Create udev rules**

```bash
# Create /etc/udev/rules.d/99-nexmosphere.rules with:
SUBSYSTEM=="tty", ATTRS{idVendor}=="067b", ATTRS{idProduct}=="2303", MODE="0666"
SUBSYSTEM=="tty", ATTRS{idVendor}=="067b", ATTRS{idProduct}=="23a3", MODE="0666"
SUBSYSTEM=="tty", ATTRS{idVendor}=="067b", ATTRS{idProduct}=="23d3", MODE="0666"

# Then reload:
sudo udevadm control --reload-rules && sudo udevadm trigger
```

## Usage

### As a Library (Direct Integration)

Import the package and register event handlers:

```go
package main

import (
    "fmt"
    "github.org/carrierlabs/go-nexmosphere/nexmosphere"
)

func main() {
    // Create service
    service := nexmosphere.NewService()

    // Register callback handler
    service.AddHandler(nexmosphere.EventHandlerFunc(func(e nexmosphere.Event) {
        if e.Type == "button" && e.Action == "press" {
            fmt.Printf("Button %d pressed on %s\n", e.Address, e.Controller)
            // Your business logic here
        }
    }))

    // Start monitoring controllers
    service.Start()
    defer service.Stop()

    // Your application continues...
    select {}
}
```

See [examples/callback/main.go](examples/callback/main.go) for a complete example.

### As a Standalone HTTP/SSE Server

Build and run the server:

```bash
cd cmd/server
go build
NX_SERVER_PORT=8089 ./server
```

Connect to the SSE stream:

```bash
curl -N http://localhost:8089/sse
```

Events are streamed as Server-Sent Events with named event types (`button`, `rfid-tag`, `presence`, etc.).

## Event Types

| Event Type     | Description           | Example Actions                              |
| -------------- | --------------------- | -------------------------------------------- |
| `controller`   | System status updates | `system-update`, `ready`                     |
| `device`       | Device discovery/info | `update`                                     |
| `button`       | Button events         | `press`, `release`, `hold`, `closed`, `open` |
| `rfid-tag`     | RFID tag events       | `pickup`, `putback`                          |
| `rfid-antenna` | RFID antenna events   | `pickup`, `putback`, `status`                |
| `presence`     | Presence detection    | `detection-zone`                             |

### Event Structure

```go
type Event struct {
    Type       string        // Event type (see table above)
    Controller string        // Controller port name
    Address    int           // Device address (0 for system events)
    Action     string        // Event action
    Data       string        // Additional data (optional)
    Raw        string        // Raw protocol message (optional)
    Duration   time.Duration // Hold duration for button events (optional)
    Timestamp  time.Time     // Event timestamp
}
```

### Controller Ready Event

When a Nexmosphere controller is discovered, there's an initialization period where device information is queried. A **"ready"** event is emitted when initialization is complete:

```go
service.AddHandler(nexmosphere.EventHandlerFunc(func(e nexmosphere.Event) {
    if e.Type == "controller" && e.Action == "ready" {
        fmt.Printf("Controller %s is ready\n", e.Controller)
        // Safe to start using buttons and other devices now
    }
}))
```

The ready event fires either:

- When all device info queries complete (typically 1-2 seconds after discovery)
- After a 5-second timeout if some devices don't respond

Button and device events won't be reliable until the ready event is received.

### Button Hold Duration

Button events include hold duration tracking. Events come in pairs for clarity:

**When button is pressed:**

- **"closed"** - Physical state change (no duration)
- **"press"** - Logical press event (no duration)

**While button is held:**

- **"hold"** - Emitted periodically (includes `Duration`)
  - Default interval: 500ms (emits at 500ms, 1000ms, 1500ms, etc.)
  - Configurable per-device via `SetDeviceHoldInterval()`

**When button is released:**

- **"open"** - Physical state change (includes total hold `Duration`)
- **"release"** - Logical release event (includes total hold `Duration`)

**Example:**

```go
service.AddHandler(nexmosphere.EventHandlerFunc(func(e nexmosphere.Event) {
    if e.Type == "button" {
        switch e.Action {
        case "hold":
            // Periodic updates while button is held
            fmt.Printf("Button held for %s\n", e.Duration) // "500ms", "1s", "1.5s"...
        case "open":
            // Final duration when released
            fmt.Printf("Button held for total of %s\n", e.Duration)
        }
    }
}))
```

**Configure hold tick interval:**

```go
// Set custom interval (e.g., 200ms instead of default 500ms)
service.SetDeviceHoldInterval("controllerName", deviceAddress, 200*time.Millisecond)

// Disable hold events for a device
service.SetDeviceHoldInterval("controllerName", deviceAddress, 0)
```

## Configuration

### Library Options

```go
service := nexmosphere.NewService(
    nexmosphere.WithLogger(customLogger),        // Custom zap logger
    nexmosphere.WithScanInterval(2*time.Second), // USB scan interval
)
```

### Server Environment Variables

- `NX_SERVER_PORT` - HTTP server port (default: `8089`)

## Supported Controllers & Devices

### USB Controllers

| Controller | VID  | PID  | Vendor                    | Product              |
| ---------- | ---- | ---- | ------------------------- | -------------------- |
| XN-185     | 067b | 2303 | Prolific Technology, Inc. | PL2303 Serial Port   |
| XN-185     | 067b | 23a3 | Prolific Technology, Inc. | ATEN Serial Bridge   |
| XN-135     | 067b | 23d3 | Prolific Technology, Inc. | PL2303GL Serial Port |

### Supported Devices

- **XTB4N6** - 4-button interface with debouncing
- **XRDR1** - RFID reader/antenna
- **XY240** - X-Eye presence & air-button sensor

## Architecture

```
nexmosphere/           # Core library
├── service.go         # Main service with event dispatch
├── controller.go      # Controller management
├── device.go          # Device-specific protocol handlers
├── serial.go          # USB discovery and connections
└── events.go          # Event types and interfaces

cmd/server/            # HTTP/SSE server
└── main.go            # Server implementation

examples/callback/     # Direct usage example
└── main.go
```

## Development

### Building the Server

```bash
go build -o server ./cmd/server
```

### Building with Version

```bash
go build -ldflags "-X main.version=1.0.0" -o server ./cmd/server
```

### Running Tests

```bash
go test ./...
```

## TODO

- Provide API endpoint for sending commands to controllers
- Validate detected serial device is truly Nexmosphere
- Add RS232 support (placeholder exists)
- Unit tests for protocol parsing
