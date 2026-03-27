# netmgr
## Purpose
TCP connection manager that owns client/server queues, event handlers, and message multiplexing.
## Use When
Initiating or accepting TCP connections for gate/logic/public services and binding protocol handlers.
## Avoid When
You only need HTTP/grpc or lightweight goroutine-based messaging; netmgr expects long-lived queues.
## Key Entry Points
- `NewNetMgr`, `NetMgr` lifecycle (Start/Stop not listed but implied).
- `NewConnectParams`, `NewListenParams`, `NetOptions`, `MergeOptions` to configure queues.
- `MsgQue` interfaces and handlers (bind via `msghandler.go` / `msgque.go`).
- `ConnAgt` representing a connection agent with `netmgr.IMsgQue`.
## Notes
Message ordering and session handling depend on `netmgr` queue semantics; refer to `netmgr/options.go` for tuning.

## Business Usage
- `gate`, `public`, and `logic` use `NetMgr` as the real long-connection transport; business handlers assume the queue/session exists before they run and access it through `netmgr.IMsgQue`.
- `netmgr/options` in `serverlisten.go` and `state_auth.go` tunes queue behavior for service listeners and robot clients. Do not misread those options as optional cosmetics; they shape reconnect and message handling behavior.
- Robot code uses the same transport path as production handlers. That means bot flows are a strong reference for expected queue semantics, not a separate fake transport layer.
