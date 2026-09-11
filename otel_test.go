package main

import (
	"context"
	"testing"
)

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
