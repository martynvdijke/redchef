package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// OtelSettingsResponse is the public, unauthenticated OTLP configuration the
// Android app reads at startup. It intentionally exposes only the collector
// endpoint and per-signal switches, never exporter headers or credentials.
type OtelSettingsResponse struct {
	Endpoint       string `json:"endpoint"`
	Enabled        bool   `json:"enabled"`
	TracesEnabled  bool   `json:"traces_enabled"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	LogsEnabled    bool   `json:"logs_enabled"`
}

// PublicGetOtelSettings reports whether the server is configured to export
// telemetry, so clients can point their own exporters at the same collector.
// With no OTLP endpoint configured everything is reported disabled.
func PublicGetOtelSettings(w http.ResponseWriter, r *http.Request) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	enabled := endpoint != ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OtelSettingsResponse{
		Endpoint:       endpoint,
		Enabled:        enabled,
		TracesEnabled:  otelSignalEnabled("TRACES", enabled),
		MetricsEnabled: otelSignalEnabled("METRICS", enabled),
		LogsEnabled:    otelSignalEnabled("LOGS", enabled),
	})
}

// otelSignalEnabled resolves one signal's switch. An explicit exporter setting
// wins (OTEL_TRACES_EXPORTER=none disables it), then a per-signal endpoint
// enables it, otherwise the signal follows the global enabled state.
func otelSignalEnabled(signal string, fallback bool) bool {
	if v, ok := os.LookupEnv("OTEL_" + signal + "_EXPORTER"); ok && strings.TrimSpace(v) != "" {
		return !strings.EqualFold(strings.TrimSpace(v), "none")
	}
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT")) != "" {
		return true
	}
	return fallback
}
