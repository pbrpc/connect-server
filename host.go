//revive:disable:package-comments
package connectserver

import (
	"log/slog"
	"sync"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/caarlos0/env/v11"

	httpserver "github.com/pbrpc/http-server"
)

// options collects what the caller adds to the standard host.
type options struct {
	interceptors []connect.ServerInterceptor
	transport    []connecthttp.Option
	middleware   []httpserver.Middleware
}

// Option customizes FromEnv.
type Option func(*options)

// WithInterceptors appends interceptors to the standard chain. They run after
// the standard ones, in the order given.
func WithInterceptors(interceptors ...connect.ServerInterceptor) Option {
	return func(o *options) {
		o.interceptors = append(o.interceptors, interceptors...)
	}
}

// WithTransportOptions appends options for connecthttp.Mount. They follow the
// standard ones, so a caller's message-size limit wins over the environment's.
func WithTransportOptions(transport ...connecthttp.Option) Option {
	return func(o *options) {
		o.transport = append(o.transport, transport...)
	}
}

// WithRouteMiddleware appends middleware for the HTTP host. The first
// middleware given is the outermost.
func WithRouteMiddleware(middleware ...httpserver.Middleware) Option {
	return func(o *options) {
		o.middleware = append(o.middleware, middleware...)
	}
}

// Host combines a Connect server with the HTTP host that serves its mounted
// procedures.
type Host struct {
	// Server is the Connect server. Generated registration functions add
	// methods to it.
	Server *connect.Server

	// HTTPHost owns the route mux and HTTP server that listens.
	HTTPHost *httpserver.Host

	transport []connecthttp.Option
	mounted   sync.Once
}

// FromEnv creates a Connect host with production-grade defaults.
//
// Standard interceptors, outermost first: the procedure's service and method
// on the span the HTTP layer started, and panic recovery answering Internal,
// logged to log. Message-size limits come from MAX_RECV_MSG_SIZE and
// MAX_SEND_MSG_SIZE. HTTP configuration is owned by HTTPHost.
func FromEnv(log *slog.Logger, opts ...Option) (*Host, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	configured, err := env.ParseAs[configuration]()
	if err != nil {
		return nil, err
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	interceptors := append([]connect.ServerInterceptor{
		makeSpanInterceptor(),
		makeRecoveryInterceptor(log),
	}, o.interceptors...)

	transport := append([]connecthttp.Option{
		connecthttp.WithReadMaxBytes(configured.MaxRecvMsgSize),
		connecthttp.WithSendMaxBytes(configured.MaxSendMsgSize),
	}, o.transport...)

	httpHost, err := httpserver.FromEnv(o.middleware...)
	if err != nil {
		return nil, err
	}

	return &Host{
		Server:    connect.NewServer(interceptors...),
		HTTPHost:  httpHost,
		transport: transport,
	}, nil
}
