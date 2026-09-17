# connect-server

`connect-server` builds a Connect host on the HTTP host provided by
`http-server`. It owns Connect interceptors, message limits, procedure
mounting, and the relationship between the Connect server and its HTTP host.

## Installation

```bash
go get github.com/pbrpc/connect-server
```

## Configuration

`FromEnv` reads the Connect message limits in addition to the configuration
owned by `http-server`:

| Variable            | Default   | Meaning |
| ------------------- | --------- | ------- |
| `MAX_RECV_MSG_SIZE` | `4194304` | Maximum bytes accepted for one message |
| `MAX_SEND_MSG_SIZE` | `4194304` | Maximum bytes sent for one message |

## Host

Register generated handlers on `Server`, add plain HTTP routes to the mux
owned by `HTTPHost`, and serve through the Connect host:

```go
host, err := connectserver.FromEnv(log)
if err != nil {
	return err
}

exampleconnect.RegisterExampleServiceHandler(host.Server, handler)
host.HTTPHost.Mux.Handle("/healthz", healthHandler)

listener, err := net.Listen("tcp", host.HTTPHost.Address)
if err != nil {
	return err
}

return host.Serve(listener)
```

`Serve` mounts every procedure registered on `Server` onto `HTTPHost.Mux`
before delegating to `HTTPHost.Serve`. `Mount` is also available when mounting
must be completed before serving and is safe to call more than once.
