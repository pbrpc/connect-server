//revive:disable:package-comments
package connectserver

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectinprocess"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	serviceName = "example.ExampleService"
	procedure   = "/" + serviceName + "/Echo"
)

var echoSpec = connect.Spec{
	StreamType: connect.StreamTypeUnary,
	Procedure:  procedure,
}

func echoMethod(observe func(context.Context)) connect.Method {
	return connect.Method{
		Spec: echoSpec,
		Handler: func(ctx context.Context, _ connect.Spec, stream connect.ServerStream) error {
			var request wrapperspb.StringValue
			if err := stream.Receive(&request); err != nil {
				return err
			}

			if observe != nil {
				observe(ctx)
			}

			return stream.Send(&request)
		},
	}
}

func callEcho(t *testing.T, server *connect.Server, text string) (string, error) {
	t.Helper()

	client := connect.NewClient(connectinprocess.New(server))
	var response wrapperspb.StringValue
	err := client.CallUnary(t.Context(), echoSpec, wrapperspb.String(text), &response)

	return response.GetValue(), err
}

func setConfiguration(t *testing.T) {
	t.Helper()

	t.Setenv("MAX_RECV_MSG_SIZE", "4194304")
	t.Setenv("MAX_SEND_MSG_SIZE", "4194304")
	t.Setenv("HOST_ADDRESS", ":50051")
	t.Setenv("HOST_IDLE_TIMEOUT", "5m")
	t.Setenv("HTTP2_SEND_PING_TIMEOUT", "2m")
	t.Setenv("HTTP2_PING_TIMEOUT", "20s")
	t.Setenv("TLS_CERT", "")
	t.Setenv("TLS_KEY", "")
	t.Setenv("TLS_CLIENT_CA", "")
}

func mustHost(t *testing.T, log *slog.Logger, opts ...Option) *Host {
	t.Helper()

	host, err := FromEnv(log, opts...)
	if err != nil {
		t.Fatalf("FromEnv() error = %v, want nil", err)
	}

	return host
}
