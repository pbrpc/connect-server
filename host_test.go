//revive:disable:package-comments
package connectserver

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect/v2"
)

func TestFromEnv(t *testing.T) {
	t.Run("builds the Connect and HTTP hosts", func(t *testing.T) {
		setConfiguration(t)

		host := mustHost(t, nil)

		if host.Server == nil {
			t.Fatal("Connect server is nil")
		}
		if host.HTTPHost == nil || host.HTTPHost.Mux == nil || host.HTTPHost.Server == nil {
			t.Fatalf("HTTP host = %+v, want the host, mux, and server", host.HTTPHost)
		}
	})

	t.Run("applies caller interceptors", func(t *testing.T) {
		setConfiguration(t)
		called := false
		observe := func(next connect.ServerFunc) connect.ServerFunc {
			return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				called = true

				return next(ctx, spec, stream)
			}
		}

		host := mustHost(t, slog.New(slog.DiscardHandler), WithInterceptors(observe))
		host.Server.Register(echoMethod(nil))

		if got, err := callEcho(t, host.Server, "hello"); err != nil || got != "hello" {
			t.Fatalf("echo = %q, %v; want hello, nil", got, err)
		}
		if !called {
			t.Fatal("caller interceptor was not invoked")
		}
	})

	t.Run("returns invalid Connect configuration", func(t *testing.T) {
		setConfiguration(t)
		t.Setenv("MAX_RECV_MSG_SIZE", "not-an-integer")

		if _, err := FromEnv(slog.New(slog.DiscardHandler)); err == nil {
			t.Fatal("FromEnv() error = nil, want the Connect configuration error")
		}
	})

	t.Run("returns invalid HTTP configuration", func(t *testing.T) {
		setConfiguration(t)
		t.Setenv("HOST_IDLE_TIMEOUT", "not-a-duration")

		if _, err := FromEnv(slog.New(slog.DiscardHandler)); err == nil {
			t.Fatal("FromEnv() error = nil, want the HTTP configuration error")
		}
	})
}
