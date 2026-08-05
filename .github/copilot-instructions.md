# AI Agent Instructions: go-nexmosphere

## Project Overview

Go library and HTTP server for Nexmosphere sensor controllers (USB serial devices). Can be used directly as a library with event callbacks OR as a standalone HTTP/SSE server. Auto-discovers controllers, parses proprietary protocol, dispatches events.

## Architecture Components

### Package Structure

- **nexmosphere/** - Importable library (core functionality)
- **cmd/server/** - HTTP/SSE server implementation
- **examples/callback/** - Direct library usage example

### Core Data Flow

1. **Serial Discovery** ([nexmosphere/serial.go](../nexmosphere/serial.go)): `Service.scanForControllers()` scans USB ports every 2s for Prolific VID 067b with specific PIDs
2. **Protocol Parsing** ([nexmosphere/controller.go](../nexmosphere/controller.go)): Each controller spawns `listen()` goroutine that scans serial buffer, decodes feedback, routes to device handlers
3. **Event Dispatching** ([nexmosphere/service.go](../nexmosphere/service.go)): `Service.dispatch()` sends events to all registered `EventHandler` implementations
4. **HTTP/SSE Bridge** ([cmd/server/sse.go](../cmd/server/sse.go)): `SSEHandler` implements `EventHandler`, broadcasts to HTTP SSE clients

### Key Data Structures

- `Service`: Main entry point, manages controllers map, event handlers slice, scanning lifecycle
- `Controller`: Wraps serial port, maintains `devices[1000]` array indexed by protocol address, dual queue (system/command priority)
- `Event`: Public event structure (`Type`, `Controller`, `Address`, `Action`, `Data`, `Raw`, `Timestamp`)
- `feedback`: Internal protocol message parsing (`Type`, `Address`, `Format`, `Action`, `Data`, `Raw`)
- `Device`: Type-specific state (e.g., `Button[4]` array for XTB4N6, debounced via goroutine delays)

## Nexmosphere Protocol

### Message Format

`<Type><Address:3><Format:1>[Command]` terminated by CR+LF

**Examples:**

- `X005A[1]` → X-Talk from address 5, format A, command "1"
- `D008B[TYPE=XTB4N6]` → Diagnostic response with device type
- `XR[PU123]` → RFID tag 123 picked up (special XR format)

### Device-Specific Parsing

Add new device types in [nexmosphere/controller.go](../nexmosphere/controller.go) `doXfb()` switch:

```go
case "XTNEW":
    return "new-event-type", d.processFbXTNEW(fb, c)
```

Implement processor in [nexmosphere/device.go](../nexmosphere/device.go) following patterns like `processFbXTB4N6()`.

## Critical Patterns

### Service-Centric Design

- No global state - everything encapsulated in `Service` struct
- Controllers stored in `Service.controllers` map keyed by port name
- Event handlers in `Service.handlers` slice, dispatched via `Service.dispatch()`
- Service lifecycle: `NewService()` → `Start()` → `Stop()`

### Controller Lifecycle

- Auto-discovered by `scanForControllers()` and added to service's controllers map
- On disconnect, `listen()` returns error → cleanup via `controller.close()` and map delete
- 10s delay after discovery before querying devices (`D###B[TYPE]` for addresses 1-8)
- Queue processed every 250ms via ticker goroutine (system queue has priority over command queue)

### Event Dispatching

`Service.dispatch(Event)` calls `HandleEvent()` on all registered handlers **non-blockingly** (via goroutine). Event types:

- `"controller"` - system status updates
- `"device"` - TYPE/SERIAL updates from diagnostics
- `"button"`, `"rfid-antenna"`, `"rfid-tag"`, `"presence"` - device-specific events
  Using as Library

```go
import "github.org/carrierlabs/go-nexmosphere/nexmosphere"

service := nexmosphere.NewService()
service.AddHandler(nexmosphere.EventHandlerFunc(func(e nexmosphere.Event) {
    // Handle event
}))
service.Start()
defer service.Stop()
```

See [examples/callback/main.go](../examples/callback/main.go) for complete example.

### Running HTTP/SSE Server

```bash
cd cmd/server
NX_SERVER_PORT=8089 go run .
```

Version defaults to "develop" (enables debug logging). Production build:

````bash
go build -ldflags "-X main.version=1.0.0" -o server ./cmd/server, lazy-initialized in `getDevice()`
- `lastFB` on controller used by RFID antenna to correlate tag address with pickup/putback actions
- USB check requires exact VID/PID match (067b with 2303/23a3/23d3) - no way to verify it's Nexmosphere without connecting
- Event handlers called non-blockingly (goroutine spawned per handler per event) - don't assume sequential delivery
- `Service` owns all state - safe to create multiple independent Service instances

## Adding New Features

### New Event Handler
Implement `EventHandler` interface or use `EventHandlerFunc` adapter:
```go
type MyHandler struct{}
func (h *MyHandler) HandleEvent(e nexmosphere.Event) { /* ... */ }

service.AddHandler(&MyHandler{})
````

### New Device Type

1. Add case in `Controller.doXfb()` switch
2. Implement `processFbXXX()` in device.go
3. Return `*Event` or dispatch directly via `c.service.dispatch()`

### Sending Commands to Controllers

Use `Service.SendCommand()` in library mode:

```go
service.SendCommand("controllerName", "X005A[1]")
```

Commands queued via `Controller.addToQueue()`:

````go
c.addToQueue(commandQueue, "X005A[1]")   // user commands
c.addToQueue(systemQueue, "D005B[TYPE]") // diagnostic queries (priority)me device events.

### Adding Commands to Controller
Controllers respond to X-Talk commands (not currently exposed via API). Use queue system:
```go
c.addToQueue(command, "X005A[1]")  // user commands
c.addToQueue(system, "D005B[TYPE]") // diagnostic queries take priority
````

## Common Gotchas

- **Linux permissions**: Serial ports require user in `dialout` group or udev rules (see README). "Permission denied" errors are common without this
- Device array is fixed size 1000, directly indexed by protocol address (1-999 valid), lazy-initialized in `getDevice()`
- `lastFB` on controller used by RFID antenna to correlate tag address with pickup/putback actions
- USB check requires exact VID/PID match (067b with 2303/23a3/23d3) - no way to verify it's Nexmosphere without connecting
- Event handlers called non-blockingly (goroutine spawned per handler per event) - don't assume sequential delivery
- `Service` owns all state - safe to create multiple independent Service instances

## TODOs from README

- Provide `/action` endpoint for sending commands to controllers
- Validate detected serial device is truly Nexmosphere (protocol lacks diagnostic ping)
