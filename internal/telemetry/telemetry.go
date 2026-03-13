package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitProvider initializes an OTel Tracer and Meter provider.
// It returns a shutdown function so main.go can cleanly flush data before exiting.
func InitProvider() (func(context.Context) error, error) {
	ctx := context.Background()

	// 1. Define the Resource (Who are we?)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("aegis-llm-gateway"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	// 2. Setup Trace Exporter (Printing to terminal for now)
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// 3. Setup Metric Exporter
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// 4. Setup Propagator (Ensures trace IDs travel across microservices)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return a combined shutdown function
	shutdown := func(c context.Context) error {
		var err error
		if err = tracerProvider.Shutdown(c); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		if err = meterProvider.Shutdown(c); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
		return err
	}

	return shutdown, nil
}