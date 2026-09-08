package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestErrorLogsRemainCorrelatedAndFlushOnShutdown(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]int)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "60000")
	t.Setenv("OTEL_BLRP_SCHEDULE_DELAY", "60000")
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})
	var output bytes.Buffer
	logger, shutdown, err := Start(context.Background(), slog.NewJSONHandler(&output, nil))
	if err != nil {
		t.Fatal(err)
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier{
		"traceparent": "00-11111111111111111111111111111111-2222222222222222-01",
		"baggage":     "synthetic=test",
	})
	if span := trace.SpanContextFromContext(ctx); span.TraceID().String() != "11111111111111111111111111111111" {
		t.Fatal("upstream trace context was not extracted")
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if carrier.Get("baggage") != "synthetic=test" {
		t.Fatal("baggage was not propagated")
	}
	ctx, span := otel.Tracer("synthetic-test").Start(ctx, "synthetic.operation")
	logger.With("operation", "synthetic").ErrorContext(ctx, "synthetic failure")
	span.End()
	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["trace_id"] != "11111111111111111111111111111111" || record["span_id"] != span.SpanContext().SpanID().String() {
		t.Fatalf("error log missing trace correlation: %v", record)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests["/v1/traces"] == 0 || requests["/v1/logs"] == 0 {
		t.Fatalf("shutdown failed to deliver both queued signals: %v", requests)
	}
}
