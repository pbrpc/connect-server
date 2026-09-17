//revive:disable:package-comments
package connectserver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/pbrpc/otel-testing/mocks/tracer"
)

func TestMakeRecoveryInterceptor(t *testing.T) {
	t.Run("passes a handler's answer through", func(t *testing.T) {
		want := errors.New("handler failed")
		serve := makeRecoveryInterceptor(slog.New(slog.DiscardHandler))(
			func(context.Context, connect.Spec, connect.ServerStream) error { return want },
		)

		if err := serve(t.Context(), echoSpec, nil); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("answers Internal and logs when the handler panics", func(t *testing.T) {
		var out bytes.Buffer
		log := slog.New(slog.NewTextHandler(&out, nil))
		serve := makeRecoveryInterceptor(log)(
			func(context.Context, connect.Spec, connect.ServerStream) error {
				panic("test panic value")
			},
		)

		err := serve(t.Context(), echoSpec, nil)
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("code = %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}

		var connectErr *connect.Error
		if !errors.As(err, &connectErr) || connectErr.Message() != "internal server error" {
			t.Errorf("error = %v, want the fixed internal message", err)
		}
		if !strings.Contains(out.String(), "test panic value") {
			t.Errorf("log = %q, want it to carry the panic value", out.String())
		}
	})
}

func TestSplitProcedure(t *testing.T) {
	cases := []struct {
		procedure, service, method string
	}{
		{"/example.ExampleService/Echo", "example.ExampleService", "Echo"},
		{"example.ExampleService/Echo", "example.ExampleService", "Echo"},
		{"/example.ExampleService/", "example.ExampleService", ""},
		{"/example.ExampleService", "example.ExampleService", ""},
		{"", "", ""},
	}

	for _, c := range cases {
		t.Run(c.procedure, func(t *testing.T) {
			service, method := splitProcedure(c.procedure)

			if service != c.service || method != c.method {
				t.Errorf("got (%q, %q), want (%q, %q)", service, method, c.service, c.method)
			}
		})
	}
}

// spanAttributes answers with the attributes of the one span the mock recorded.
func spanAttributes(t *testing.T, tt *tracer.Mock) []attribute.KeyValue {
	t.Helper()

	tt.EndSpan()

	spans := tt.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}

	return spans[0].Attributes
}

// hasAttribute reports whether attrs carries want.
func hasAttribute(attrs []attribute.KeyValue, want attribute.KeyValue) bool {
	for _, attr := range attrs {
		if attr.Key == want.Key && attr.Value == want.Value {
			return true
		}
	}

	return false
}

func TestMakeSpanInterceptor(t *testing.T) {
	t.Run("names the RPC on the span", func(t *testing.T) {
		tt, ctx := tracer.New(t)
		defer tt.Shutdown(t)

		if err := makeSpanInterceptor()(
			func(context.Context, connect.Spec, connect.ServerStream) error { return nil },
		)(ctx, echoSpec, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		attrs := spanAttributes(t, tt)

		for _, want := range []attribute.KeyValue{
			semconv.RPCSystemConnectRPC,
			semconv.RPCService(serviceName),
			semconv.RPCMethod("Echo"),
		} {
			if !hasAttribute(attrs, want) {
				t.Errorf("attributes = %v, want %v", attrs, want)
			}
		}
		if hasAttribute(attrs, semconv.RPCConnectRPCErrorCodeKey.String("internal")) {
			t.Error("a successful call must carry no error code")
		}
	})

	t.Run("records the error code and status when the handler fails", func(t *testing.T) {
		tt, ctx := tracer.New(t)
		defer tt.Shutdown(t)

		want := connect.NewError(connect.CodeNotFound, "missing")
		err := makeSpanInterceptor()(
			func(context.Context, connect.Spec, connect.ServerStream) error { return want },
		)(ctx, echoSpec, nil)
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}

		tt.EndSpan()

		span := tt.GetSpans()[0]

		if !hasAttribute(span.Attributes, semconv.RPCConnectRPCErrorCodeKey.String("not_found")) {
			t.Errorf("attributes = %v, want the not_found code", span.Attributes)
		}
		if span.Status.Code != codes.Error {
			t.Errorf("status = %v, want %v", span.Status.Code, codes.Error)
		}
	})
}
