package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func Start(ctx context.Context, fallback slog.Handler) (*slog.Logger, func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	correlated := traceHandler{Handler: fallback}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return slog.New(correlated), func(context.Context) error { return nil }, nil
	}
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(resource.Default()),
	)
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		_ = traceProvider.Shutdown(ctx)
		return nil, nil, err
	}
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(resource.Default()),
	)
	otel.SetTracerProvider(traceProvider)
	logger := slog.New(slog.NewMultiHandler(correlated, otelslog.NewHandler("github.com/NerdsWhoFish/bookings", otelslog.WithLoggerProvider(logProvider))))
	shutdown := func(ctx context.Context) error {
		return errors.Join(logProvider.Shutdown(ctx), traceProvider.Shutdown(ctx))
	}
	return logger, shutdown, nil
}

type traceHandler struct{ slog.Handler }

func (handler traceHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanContextFromContext(ctx)
	if span.IsValid() {
		record.AddAttrs(slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
	}
	return handler.Handler.Handle(ctx, record)
}

func (handler traceHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return traceHandler{Handler: handler.Handler.WithAttrs(attributes)}
}

func (handler traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: handler.Handler.WithGroup(name)}
}
