// Package telemetry sets up logs, traces and metrics (ADR-0016). slog writes
// JSON to stdout and, through the OpenTelemetry slog bridge, OTLP logs.
// Traces and metrics are exported over OTLP to an OpenTelemetry Collector.
// Exporters follow the standard OTEL_* environment variables.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ServiceName is reported as service.name unless OTEL_SERVICE_NAME overrides it.
const ServiceName = "wallet-accountant"

// Options overrides where telemetry goes. The zero value exports everything
// as the OTEL_* environment variables say (OTLP by default) and writes JSON
// logs to nothing; production passes os.Stdout.
type Options struct {
	// Stdout receives one JSON object per log record.
	Stdout io.Writer
	// SpanExporter, when set, receives every span synchronously instead of
	// the exporter chosen by OTEL_TRACES_EXPORTER.
	SpanExporter sdktrace.SpanExporter
	// MetricReader, when set, replaces the reader chosen by OTEL_METRICS_EXPORTER.
	MetricReader sdkmetric.Reader
	// LogExporter, when set, receives every OTLP log record synchronously
	// instead of the exporter chosen by OTEL_LOGS_EXPORTER.
	LogExporter sdklog.Exporter
}

// Telemetry holds the providers the application reports through.
type Telemetry struct {
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Propagator     propagation.TextMapPropagator

	shutdown []func(context.Context) error
}

// New builds the logger and the trace, metric and log providers.
func New(ctx context.Context, opts Options) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", ServiceName)),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, res, opts.SpanExporter)
	if err != nil {
		return nil, err
	}

	mp, err := newMeterProvider(ctx, res, opts.MetricReader)
	if err != nil {
		return nil, errors.Join(err, tp.Shutdown(ctx))
	}

	lp, err := newLoggerProvider(ctx, res, opts.LogExporter)
	if err != nil {
		return nil, errors.Join(err, tp.Shutdown(ctx), mp.Shutdown(ctx))
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	logger := slog.New(slog.NewMultiHandler(
		traceContextHandler{slog.NewJSONHandler(stdout, nil)},
		otelslog.NewHandler(ServiceName, otelslog.WithLoggerProvider(lp)),
	))

	return &Telemetry{
		Logger:         logger,
		TracerProvider: tp,
		MeterProvider:  mp,
		Propagator:     propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		shutdown:       []func(context.Context) error{tp.Shutdown, mp.Shutdown, lp.Shutdown},
	}, nil
}

// Shutdown flushes and stops every provider.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, f := range t.shutdown {
		errs = append(errs, f(ctx))
	}

	return errors.Join(errs...)
}

func newTracerProvider(ctx context.Context, res *resource.Resource, exp sdktrace.SpanExporter) (*sdktrace.TracerProvider, error) {
	if exp != nil {
		return sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithSyncer(exp)), nil
	}

	exp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("span exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(exp)), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, reader sdkmetric.Reader) (*sdkmetric.MeterProvider, error) {
	if reader == nil {
		var err error

		reader, err = autoexport.NewMetricReader(ctx)
		if err != nil {
			return nil, fmt.Errorf("metric reader: %w", err)
		}
	}

	return sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(reader)), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource, exp sdklog.Exporter) (*sdklog.LoggerProvider, error) {
	if exp != nil {
		return sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp))), nil
	}

	exp, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("log exporter: %w", err)
	}

	return sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(exp))), nil
}

// traceContextHandler adds trace_id and span_id to every record logged with a
// context that carries a span, so a stdout log line links to its trace.
type traceContextHandler struct {
	slog.Handler
}

func (h traceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, r) //nolint:wrapcheck // decorator passes the inner handler's error through
}

func (h traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceContextHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceContextHandler) WithGroup(name string) slog.Handler {
	return traceContextHandler{h.Handler.WithGroup(name)}
}
