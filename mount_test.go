//revive:disable:package-comments
package connectserver

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect/v2/connecthttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/pbrpc/testing/mocks/listener"
)

func recordSpans(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(t.Context())
	})

	return exporter
}

func echo(host *Host, text string) *httptest.ResponseRecorder {
	return serve(host, httptest.NewRequest(http.MethodPost, procedure, strings.NewReader(`"`+text+`"`)))
}

func serve(host *Host, request *http.Request) *httptest.ResponseRecorder {
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	host.HTTPHost.Server.Handler.ServeHTTP(recorder, request)

	return recorder
}

func TestMount(t *testing.T) {
	t.Run("routes a procedure through the interceptors under an HTTP span", func(t *testing.T) {
		setConfiguration(t)
		exporter := recordSpans(t)
		host := mustHost(t, slog.New(slog.DiscardHandler))
		host.Server.Register(echoMethod(nil))
		host.Mount()

		recorder := echo(host, "hello")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %q, want 200", recorder.Code, recorder.Body.String())
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != `"hello"` {
			t.Errorf("body = %q, want the echo", got)
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("recorded %d spans, want 1", len(spans))
		}
		if spans[0].Name != http.MethodPost+" "+procedure {
			t.Errorf("span name = %q, want the method and procedure", spans[0].Name)
		}
		if !hasAttribute(spans[0].Attributes, semconv.RPCService(serviceName)) {
			t.Errorf("attributes = %v, want the RPC service", spans[0].Attributes)
		}
	})

	t.Run("applies caller transport options", func(t *testing.T) {
		setConfiguration(t)
		host := mustHost(t, slog.New(slog.DiscardHandler),
			WithTransportOptions(connecthttp.WithReadMaxBytes(1)))
		host.Server.Register(echoMethod(nil))
		host.Mount()

		if recorder := echo(host, "hello"); recorder.Code == http.StatusOK {
			t.Fatalf("status = %d, want the oversized message refused", recorder.Code)
		}
	})

	t.Run("applies transport options from the environment", func(t *testing.T) {
		setConfiguration(t)
		t.Setenv("MAX_RECV_MSG_SIZE", "1")
		t.Setenv("MAX_SEND_MSG_SIZE", "1")
		host := mustHost(t, slog.New(slog.DiscardHandler))
		host.Server.Register(echoMethod(nil))
		host.Mount()

		if recorder := echo(host, "hello"); recorder.Code == http.StatusOK {
			t.Fatalf("status = %d, want the oversized message refused", recorder.Code)
		}
	})

	t.Run("forwards route middleware to the HTTP host", func(t *testing.T) {
		setConfiguration(t)
		var calls []string
		wrap := func(name string) func(string, http.Handler) http.Handler {
			return func(pattern string, next http.Handler) http.Handler {
				return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					calls = append(calls, name+":"+pattern)
					next.ServeHTTP(writer, request)
				})
			}
		}

		host := mustHost(t, slog.New(slog.DiscardHandler),
			WithRouteMiddleware(wrap("outer"), wrap("inner")))
		host.Server.Register(echoMethod(nil))
		host.Mount()

		if recorder := echo(host, "hello"); recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if want := []string{"outer:" + procedure, "inner:" + procedure}; !slices.Equal(calls, want) {
			t.Errorf("middleware calls = %v, want %v", calls, want)
		}
	})

	t.Run("mounts once", func(t *testing.T) {
		setConfiguration(t)
		host := mustHost(t, slog.New(slog.DiscardHandler))
		host.Server.Register(echoMethod(nil))

		host.Mount()
		host.Mount()
	})
}

func TestServe(t *testing.T) {
	setConfiguration(t)
	host := mustHost(t, slog.New(slog.DiscardHandler))
	host.Server.Register(echoMethod(nil))
	serveErr := errors.New("listener failed")

	if err := host.Serve(listener.NewFailing(serveErr)); !errors.Is(err, serveErr) {
		t.Fatalf("Serve() error = %v, want %v", err, serveErr)
	}
	if recorder := echo(host, "hello"); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the procedure mounted before serving", recorder.Code)
	}
}
