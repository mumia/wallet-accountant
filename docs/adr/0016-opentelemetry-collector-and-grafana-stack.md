# Telemetry goes through an OpenTelemetry Collector to the Grafana stack

The app always exports traces, metrics and logs over OTLP to an OpenTelemetry Collector, and respects the standard `OTEL_*` environment variables. It never talks to a telemetry backend directly. The backend is the Grafana stack: Tempo for traces, Prometheus for metrics, Loki for logs and Grafana as the UI. Development uses Grafana's all-in-one `grafana/otel-lgtm` image. We chose the Grafana stack over OpenObserve, SigNoz and Jaeger + Prometheus because it is the most widely used and mature open-source option, it covers all three signals, and Grafana Cloud is a natural hosted path if the app becomes a service (ADR-0005). Because the collector sits in between, changing backend is a collector configuration change, not a code change.

## Logs and traces

- `log/slog` writes to two places: JSON on stdout, and OTLP logs through the OpenTelemetry slog bridge. Every record carries `trace_id` and `span_id`, so a log line links to its trace.
- Traces continue across asynchronous work through the CloudEvents distributed tracing extension (ADR-0015).

## Compose layout

There are two Compose files, because developing and self-hosting need different things:

- **`compose.dev.yaml`** (development) runs the dependencies only: PostgreSQL, Zitadel and `grafana/otel-lgtm`. The app runs outside Docker (IDE or `go run`), so it can be debugged and rebuilt quickly. Telemetry is always visible locally.
- **`compose.yaml`** (self-hosting) runs the app in `all` mode, `migrate` as a one-shot service, PostgreSQL, Zitadel and the OpenTelemetry Collector. Tempo, Prometheus, Loki and Grafana start only with `--profile observability`. Without that profile, the collector discards telemetry, so the exporter can stay on while a default install remains at five containers.

We rejected a single Compose file with everything always running, because a household that only wants to track its money would run and store a full monitoring stack it may never open.

## Status

Accepted. The "Compose layout" section is superseded by ADR-0019: locally the Grafana stack is opt-in (`make local-up-observability`), and in production it is a platform component.
