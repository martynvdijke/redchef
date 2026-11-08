package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	collog "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetric "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"
)

func hasAttribute(attrs []*commonv1.KeyValue, key, value string) bool {
	for _, attr := range attrs {
		if attr.GetKey() == key && attr.GetValue().GetStringValue() == value {
			return true
		}
	}
	return false
}

func hasSpanNamed(req *coltrace.ExportTraceServiceRequest, name string) bool {
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				if span.GetName() == name {
					return true
				}
			}
		}
	}
	return false
}

func hasMetricNamed(req *colmetric.ExportMetricsServiceRequest, name string) bool {
	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			for _, metric := range sm.GetMetrics() {
				if metric.GetName() == name {
					return true
				}
			}
		}
	}
	return false
}

func hasLogBody(req *collog.ExportLogsServiceRequest, body string) bool {
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			for _, record := range sl.GetLogRecords() {
				if strings.Contains(record.GetBody().GetStringValue(), body) {
					return true
				}
			}
		}
	}
	return false
}

// otlpCollector is an in-process OTLP/HTTP receiver that records the protobuf
// bodies the exporters POST.
type otlpCollector struct {
	srv    *httptest.Server
	mu     sync.Mutex
	bodies map[string][]byte
}

func newOTLPCollector(t *testing.T) *otlpCollector {
	t.Helper()
	c := &otlpCollector{bodies: map[string][]byte{}}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		c.mu.Lock()
		c.bodies[r.URL.Path] = append(c.bodies[r.URL.Path], body...)
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(c.srv.Close)
	return c
}

func (c *otlpCollector) body(path string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodies[path]
}

func TestOtelEndpointConfigured(t *testing.T) {
	for _, k := range otelEnvKeys {
		t.Setenv(k, "")
	}
	if otelEndpointConfigured() {
		t.Fatal("expected telemetry disabled with no endpoint")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	if !otelEndpointConfigured() {
		t.Fatal("expected telemetry enabled when endpoint is set")
	}
}

func TestSetupOTelNoopWhenDisabled(t *testing.T) {
	for _, k := range otelEnvKeys {
		t.Setenv(k, "")
	}
	shutdown, err := setupOTel(context.Background())
	if err != nil {
		t.Fatalf("setupOTel: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected a non-nil shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown: %v", err)
	}
}

// TestSetupOTelExportsAllSignals runs setupOTel against an in-process OTLP/HTTP
// collector and proves that traces, metrics and logs are actually exported
// (and flushed on shutdown) — both telemetry created directly through the
// global providers and the HTTP server instrumentation used by main.go.
func TestSetupOTelExportsAllSignals(t *testing.T) {
	// Clear inherited endpoints so everything goes to the test collector.
	for _, k := range otelEnvKeys {
		t.Setenv(k, "")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_COMPRESSION", "")
	t.Setenv("OTEL_SERVICE_NAME", "redchef-otel-test")

	collector := newOTLPCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.srv.URL)

	shutdown, err := setupOTel(context.Background())
	if err != nil {
		t.Fatalf("setupOTel: %v", err)
	}

	ctx := context.Background()

	// Trace created directly through the global tracer provider.
	_, span := otel.Tracer("redchef-otel-test").Start(ctx, "unit-test-span")
	span.End()

	// Metric created directly through the global meter provider.
	counter, err := otel.Meter("redchef-otel-test").Int64Counter("redchef.test.counter")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(ctx, 1)

	// Log line picked up by the stdlib log bridge.
	log.Printf("otel integration test log line")

	// The HTTP handler wiring from main.go: server spans and HTTP metrics.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	httpSrv := httptest.NewServer(otelhttp.NewHandler(mux, "redchef"))
	defer httpSrv.Close()
	resp, err := http.Get(httpSrv.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /ping: status %d", resp.StatusCode)
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	var traceReq coltrace.ExportTraceServiceRequest
	if err := proto.Unmarshal(collector.body("/v1/traces"), &traceReq); err != nil {
		t.Fatalf("decode traces: %v (received %d bytes)", err, len(collector.body("/v1/traces")))
	}
	if !hasSpanNamed(&traceReq, "unit-test-span") {
		t.Fatal("exported traces do not contain the manual test span")
	}
	if !hasSpanNamed(&traceReq, "GET /ping") {
		t.Fatal("exported traces do not contain the HTTP server span")
	}
	if len(traceReq.ResourceSpans) == 0 || !hasAttribute(traceReq.ResourceSpans[0].GetResource().GetAttributes(), "service.name", "redchef-otel-test") {
		t.Fatal("exported traces missing resource service.name")
	}

	var metricReq colmetric.ExportMetricsServiceRequest
	if err := proto.Unmarshal(collector.body("/v1/metrics"), &metricReq); err != nil {
		t.Fatalf("decode metrics: %v (received %d bytes)", err, len(collector.body("/v1/metrics")))
	}
	if !hasMetricNamed(&metricReq, "redchef.test.counter") {
		t.Fatal("exported metrics do not contain the manual test counter")
	}
	if !hasMetricNamed(&metricReq, "http.server.request.duration") {
		t.Fatal("exported metrics do not contain HTTP server duration")
	}

	var logReq collog.ExportLogsServiceRequest
	if err := proto.Unmarshal(collector.body("/v1/logs"), &logReq); err != nil {
		t.Fatalf("decode logs: %v (received %d bytes)", err, len(collector.body("/v1/logs")))
	}
	if !hasLogBody(&logReq, "otel integration test log line") {
		t.Fatal("exported logs do not contain the test log line")
	}
}
