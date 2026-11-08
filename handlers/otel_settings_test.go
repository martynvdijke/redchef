package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// clearOtelEnv neutralises every env var the settings handler reads so a test
// starts from a known state regardless of the developer's shell.
func clearOtelEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
		"OTEL_LOGS_EXPORTER",
	} {
		t.Setenv(key, "")
	}
}

func getOtelSettings(t *testing.T) (*httptest.ResponseRecorder, OtelSettingsResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/otel-settings", nil)
	resp := httptest.NewRecorder()
	PublicGetOtelSettings(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json, got %q", got)
	}
	var body OtelSettingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp, body
}

func TestPublicGetOtelSettings_Disabled(t *testing.T) {
	clearOtelEnv(t)

	resp, body := getOtelSettings(t)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if body.Endpoint != "" || body.Enabled || body.TracesEnabled ||
		body.MetricsEnabled || body.LogsEnabled {
		t.Fatalf("expected everything disabled, got %+v", body)
	}
}

func TestPublicGetOtelSettings_FollowsGlobalEndpoint(t *testing.T) {
	clearOtelEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")

	_, body := getOtelSettings(t)
	if !body.Enabled {
		t.Fatal("expected enabled with a global endpoint")
	}
	if body.Endpoint != "http://collector:4318" {
		t.Fatalf("endpoint = %q", body.Endpoint)
	}
	if !body.TracesEnabled || !body.MetricsEnabled || !body.LogsEnabled {
		t.Fatalf("per-signal flags must default to enabled, got %+v", body)
	}
}

func TestPublicGetOtelSettings_PerSignalOverrides(t *testing.T) {
	clearOtelEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "otlp")

	_, body := getOtelSettings(t)
	if body.TracesEnabled {
		t.Error("OTEL_TRACES_EXPORTER=none must disable traces")
	}
	if !body.MetricsEnabled {
		t.Error("an explicit metrics exporter must keep metrics enabled")
	}
	if !body.LogsEnabled {
		t.Error("logs should follow the global enabled state")
	}
}

func TestPublicGetOtelSettings_AcceptsAnonymousRequest(t *testing.T) {
	clearOtelEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")

	// No session cookie, Authorization header or user context is provided:
	// the handler is public by construction.
	req := httptest.NewRequest(http.MethodGet, "/api/otel-settings", nil)
	resp := httptest.NewRecorder()
	PublicGetOtelSettings(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("anonymous request rejected: %d", resp.Code)
	}
}
