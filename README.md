# go-nexmosphere

Go based server for communication with Nexmosphere sensor controllers using Go on Raspberry Pi devices.

Searches for USB based Serial devices an assumes they are Nexmosphere controllers.

Provides a SSE interface for watching events.

## Configuration

Configuration is done via environment variables.

NX_SERVER_PORT - Port to listen on (Default: 8089)

## TODO:

- Document event structure
- Provide API endpoint for commands to controller
- Check found Serial device is Nexmosphere (difficult as no diagnostic commands for core device)

## USB Notes

XN-185 appears as:

```
ID: 067b:2303 Prolific Technology, Inc. PL2303 Serial Port
VID: 067b
PID: 2303
```
