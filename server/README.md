# Server package

HTTP and WebSocket entry points.

- `HandleSubscribe` (`/subscribe`) accepts a WebSocket, refuses it beyond 512 concurrent
  connections, and runs a `ConnectionHandlerActor` for it until the connection ends.
- `HandleGetRooms` (`/rooms/`) asks the room manager for room ids and player counts. It
  answers 504 if the manager does not reply within 2 seconds.
- `HandleHealthCheck` (`/` and `/health-check/`) returns `{"status": "ok"}`.

## Connection handler

`connection_handler.go` owns one socket:

- Reads frames on its own goroutine. Frames over 4 KiB, or more than 60 messages per
  second (burst 120), close the connection. Input reaches the actor through the bounded
  ingress queue, so an overloaded handler closes rather than queueing without limit.
- Expects `createRoom`, `joinRoom` or `quickPlay` within 10 seconds of connecting and an
  admission answer within 5 seconds; otherwise it closes. A second room request while
  one is pending or assigned is ignored.
- Forwards `direction` and `playerReady` to its room and tells the room when the socket
  closes.
- Writes through a `transport.Client`: one ordered writer and a 64-frame queue per socket.

`main.go` wraps `/subscribe` with a scheme check suited to Cloud Run's TLS termination.
The `Origin` header is required: any well-formed origin is accepted, and an upgrade
without one is refused with 403.

See [the game package](../game/README.md).
