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
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func Start(ctx context.Context, fallback slog.Handler) (*slog.Logger, func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return slog.New(fallback), func(context.Context) error { return nil }, nil
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
	logger := slog.New(slog.NewMultiHandler(fallback, otelslog.NewHandler("github.com/NerdsWhoFish/bookings", otelslog.WithLoggerProvider(logProvider))))
	shutdown := func(ctx context.Context) error {
		return errors.Join(logProvider.Shutdown(ctx), traceProvider.Shutdown(ctx))
	}
	return logger, shutdown, nil
}
