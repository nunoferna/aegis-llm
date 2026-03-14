package telemetry

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/nunoferna/aegis-llm/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type Options struct {
	Exporter         string
	OTLPEndpoint     string
	OTLPInsecure     bool
	MetricInterval   time.Duration
	TraceSampleRatio float64
	ServiceName      string
	ServiceVersion   string
}

func defaultOptions() Options {
	return Options{
		Exporter:         config.DefaultTelemetryExporter,
		OTLPEndpoint:     config.DefaultOTLPEndpoint,
		OTLPInsecure:     config.DefaultOTLPInsecure,
		MetricInterval:   config.DefaultMetricInterval,
		TraceSampleRatio: config.DefaultTraceSampleRatio,
		ServiceName:      config.DefaultServiceName,
		ServiceVersion:   config.DefaultServiceVersion,
	}
}

// InitProvider initializes an OTel Tracer and Meter provider.
// It returns a shutdown function so main.go can cleanly flush data before exiting.
func InitProvider(opts Options) (func(context.Context) error, error) {
	ctx := context.Background()
	if opts.Exporter == "" {
		opts = defaultOptions()
	} else {
		def := defaultOptions()
		if opts.OTLPEndpoint == "" {
			opts.OTLPEndpoint = def.OTLPEndpoint
		}
		if opts.MetricInterval <= 0 {
			opts.MetricInterval = def.MetricInterval
		}
		if opts.TraceSampleRatio < 0 || opts.TraceSampleRatio > 1 {
			opts.TraceSampleRatio = def.TraceSampleRatio
		}
		if opts.ServiceName == "" {
			opts.ServiceName = def.ServiceName
		}
		if opts.ServiceVersion == "" {
			opts.ServiceVersion = def.ServiceVersion
		}
	}

	// 1. Define the Resource (Who are we?)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(opts.ServiceName),
			semconv.ServiceVersion(opts.ServiceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	var traceExporter trace.SpanExporter
	var metricReader metric.Reader
	exporter := strings.ToLower(opts.Exporter)

	if exporter == "otlp" {
		traceOpts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(opts.OTLPEndpoint),
		}
		if opts.OTLPInsecure {
			traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
		}

		traceExporter, err = otlptracegrpc.New(ctx, traceOpts...)
		if err != nil {
			return nil, err
		}

		metricOpts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(opts.OTLPEndpoint),
		}
		if opts.OTLPInsecure {
			metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
		}

		metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
		if err != nil {
			return nil, err
		}

		metricReader = metric.NewPeriodicReader(metricExporter, metric.WithInterval(opts.MetricInterval))
	} else {
		traceExporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}

		metricExporter, err := stdoutmetric.New()
		if err != nil {
			return nil, err
		}

		metricReader = metric.NewPeriodicReader(metricExporter, metric.WithInterval(opts.MetricInterval))
	}

	sampler := trace.ParentBased(trace.TraceIDRatioBased(opts.TraceSampleRatio))
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithSampler(sampler),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metricReader),
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
