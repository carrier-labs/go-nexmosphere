# go-nexmosphere

Go based server for communication with Nexmosphere sensor controllers using Go on Linux.

Searches for USB based Serial devices an assumes they are Nexmosphere controllers.

Provides SSE interface over HTTP for watching events.

## Configuration

Configuration is done via environment variables.

NX_SERVER_PORT - Port to listen on (Default: 8089)

## TODO:

- Document event structure
- Provide API endpoint for commands to controller
- Check found Serial device is Nexmosphere (difficult as no diagnostic commands for core device)

## USB Notes

### VID/PID


| Controller | VID  | PID  | Vendor                    | Product              |
| ---------- | ---- | ---- | ------------------------- | -------------------- |
| XN-185     | 067b | 2303 | Prolific Technology, Inc. | PL2303 Serial Port   |
| XN-185     | 067b | 23a3 | Prolific Technology, Inc. | ATEN Serial Bridge   |
| XN-135     | 067b | 23d3 | Prolific Technology, Inc. | PL2303GL Serial Port |
