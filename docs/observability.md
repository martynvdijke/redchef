# Observability (OpenTelemetry)

RedChef exports **traces, metrics and logs** over OTLP/HTTP. Telemetry is
opt-in and configured entirely with the standard `OTEL_*` environment
variables. With no endpoint configured the app uses the global no-op
providers: no exporter is created and nothing is sent.

## Enable it

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_SERVICE_NAME=redchef
```

One endpoint enables all three signals. Per-signal endpoints
(`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `..._METRICS_ENDPOINT`,
`..._LOGS_ENDPOINT`) override it.

Authentication and transport use the exporter defaults:

```bash
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20<token>
OTEL_EXPORTER_OTLP_INSECURE=false
OTEL_EXPORTER_OTLP_COMPRESSION=gzip
OTEL_EXPORTER_OTLP_TIMEOUT=10000
```

Sampling defaults to the SDK's `parentbased_always_on`. To sample:

```bash
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1
```

Metric export interval defaults to 60s and can be changed with
`OTEL_METRIC_EXPORT_INTERVAL` (milliseconds).

## What is emitted

| Signal | Source |
| --- | --- |
| Traces | Every inbound HTTP request (`otelhttp` server span), with route, method, status code and W3C `traceparent` propagation. |
| Metrics | HTTP server metrics from `otelhttp` (`http.server.request.duration`, active requests) plus Go runtime and process metrics (`go.*`, `process.*`). |
| Logs | Every stdlib `log.Printf` line, bridged through `otelslog` while still written to stderr. |

All three carry the resource attributes `service.name`, `service.version` and
the usual `telemetry.sdk.*` / process attributes. `OTEL_RESOURCE_ATTRIBUTES`
adds more (for example `deployment.environment=prod`).

## Verify locally

Any OTLP/HTTP collector works, for example:

```yaml
# docker-compose.yml
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otelcol/config.yaml"]
    volumes:
      - ./otelcol.yaml:/etc/otelcol/config.yaml
    ports:
      - "4318:4318"
```

A minimal collector config that logs everything it receives:

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      exporters: [debug]
    logs:
      receivers: [otlp]
      exporters: [debug]
```

Then start RedChef with `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318`
and hit the server; spans appear per request, HTTP/runtime metrics every
interval, and each request log line as a log record.

## Tests

`otel_test.go` includes an end-to-end test that runs `setupOTel` against an
in-process OTLP/HTTP collector and asserts that a span, a metric and a log
record are exported and flushed on shutdown.
