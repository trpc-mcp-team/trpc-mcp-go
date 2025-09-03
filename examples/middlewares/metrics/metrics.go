// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package metrics

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"time"
	. "trpc.group/trpc-go/trpc-mcp-go"
)

type OtelExporterType string

const (
	ExporterStdout OtelExporterType = "stdout"
	ExporterOTLP   OtelExporterType = "otlp"
)

type OtelMetricsRecorder interface {
	RecordRequest(ctx context.Context, method string)

	RecordError(ctx context.Context, method string, code int)

	RecordLatency(ctx context.Context, method string, latencyMs float64)

	RecordInFlight(ctx context.Context, method string, count int64)
}

type OtelMetricsConfig struct {
	recorder OtelMetricsRecorder

	EnableRequests bool

	EnableErrors bool

	EnableLatency bool

	EnableInFlight bool

	Filter func(method string) bool

	ServiceName string

	ServiceVersion string

	OtelExporterType OtelExporterType
}

func DefaultOtelMetricsConfig() *OtelMetricsConfig {
	return &OtelMetricsConfig{
		EnableRequests:   true,
		EnableErrors:     true,
		EnableLatency:    true,
		EnableInFlight:   true,
		Filter:           nil,
		ServiceName:      "trpc-mcp-server",
		ServiceVersion:   "1.0.0",
		OtelExporterType: ExporterStdout,
	}
}

type OtelMetricsOption func(*OtelMetricsConfig)

func WithOtelRecorder(recorder OtelMetricsRecorder) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.recorder = recorder
	}
}

func WithOtelEnableRequests(enableRequests bool) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.EnableRequests = enableRequests
	}
}

func WithOtelEnableErrors(enableErrors bool) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.EnableErrors = enableErrors
	}
}

func WithOtelEnableLatency(enableLatency bool) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.EnableLatency = enableLatency
	}
}

func WithOtelEnableInFlight(enableInFlight bool) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.EnableInFlight = enableInFlight
	}
}

func WithOtelFilter(filter func(method string) bool) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.Filter = filter
	}
}

func WithOtelServiceName(serviceName string) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.ServiceName = serviceName
	}
}

func WithOtelServiceVersion(serviceVersion string) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.ServiceVersion = serviceVersion
	}
}

func WithOtelExporterType(exporterType OtelExporterType) OtelMetricsOption {
	return func(o *OtelMetricsConfig) {
		o.OtelExporterType = exporterType
	}
}

type DefaultOtelMetricsRecorder struct {
	meter          metric.Meter
	tracer         trace.Tracer
	requestCounter metric.Int64Counter
	errorCounter   metric.Int64Counter
	latencyHist    metric.Float64Histogram
	inFlightGauge  metric.Int64UpDownCounter
}

func NewOtelMetricsRecorder(serviceName string, exporter OtelExporterType) (*DefaultOtelMetricsRecorder, error) {
	var meterProvider *sdkmetric.MeterProvider
	var tracerProvider trace.TracerProvider

	switch exporter {
	case ExporterStdout:
		traceExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("stdouttrace.New failed: %w", err)
		}
		tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp))
		otel.SetTracerProvider(tracerProvider)

		metricExp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("stdoutmetric.New failed: %w", err)
		}
		meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(2*time.Second))))
		otel.SetMeterProvider(meterProvider)

	case ExporterOTLP:
		ctx := context.Background()
		traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint("localhost:4317"))
		if err != nil {
			return nil, fmt.Errorf("otlptracegrpc.New failed: %w", err)
		}
		tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp))
		otel.SetTracerProvider(tracerProvider)

		metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint("localhost:4317"))
		if err != nil {
			return nil, fmt.Errorf("stdoutmetric.New failed: %w", err)
		}
		meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)))
		otel.SetMeterProvider(meterProvider)

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporter)
	}

	meter := meterProvider.Meter(serviceName)
	tracer := tracerProvider.Tracer(serviceName)

	requestCounter, _ := meter.Int64Counter("mcp_requests_total", metric.WithDescription("Total number of MCP requests"))
	errorCounter, _ := meter.Int64Counter("mcp_errors_total", metric.WithDescription("Total number of MCP errors"))
	latencyHist, _ := meter.Float64Histogram("mcp_request_duration_ms", metric.WithDescription("MCP request latency in ms"))
	inflightGauge, _ := meter.Int64UpDownCounter("mcp_requests_in_flight", metric.WithDescription("Number of MCP requests in flight"))

	return &DefaultOtelMetricsRecorder{
		meter:          meter,
		tracer:         tracer,
		requestCounter: requestCounter,
		errorCounter:   errorCounter,
		latencyHist:    latencyHist,
		inFlightGauge:  inflightGauge,
	}, nil
}

func (r *DefaultOtelMetricsRecorder) RecordRequest(ctx context.Context, method string) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
	}
	r.requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *DefaultOtelMetricsRecorder) RecordError(ctx context.Context, method string, code int) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.Int("code", code),
	}
	r.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *DefaultOtelMetricsRecorder) RecordLatency(ctx context.Context, method string, latencyMs float64) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.Float64("latency_ms", latencyMs),
	}
	r.latencyHist.Record(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *DefaultOtelMetricsRecorder) RecordInFlight(ctx context.Context, method string, count int64) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
	}
	r.inFlightGauge.Add(ctx, count, metric.WithAttributes(attrs...))
}

type NoOpOtelMetricsRecorder struct{}

func (r *NoOpOtelMetricsRecorder) RecordRequest(ctx context.Context, method string)         {}
func (r *NoOpOtelMetricsRecorder) RecordError(ctx context.Context, method string, code int) {}
func (r *NoOpOtelMetricsRecorder) RecordLatency(ctx context.Context, method string, latencyMs float64) {
}
func (r *NoOpOtelMetricsRecorder) RecordInFlight(ctx context.Context, method string, count int64) {}

func NewOtelMetricsMiddleware(opts ...OtelMetricsOption) MiddlewareFunc {
	cfg := DefaultOtelMetricsConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	if cfg.recorder == nil {
		var err error
		cfg.recorder, err = NewOtelMetricsRecorder(cfg.ServiceName, cfg.OtelExporterType)
		if err != nil {
			cfg.recorder = &NoOpOtelMetricsRecorder{}
		}
	}

	return func(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (JSONRPCMessage, error) {
		method := req.Method

		if cfg.Filter != nil && !cfg.Filter(method) {
			return next(ctx, req, session)
		}

		if cfg.EnableInFlight {
			cfg.recorder.RecordInFlight(ctx, method, 1)
			defer cfg.recorder.RecordInFlight(ctx, method, -1)
		}

		var startTime time.Time
		if cfg.EnableLatency {
			startTime = time.Now()
		}

		if cfg.EnableRequests {
			cfg.recorder.RecordRequest(ctx, method)
		}

		resp, err := next(ctx, req, session)

		success := err == nil
		errorCode := 0
		if err != nil {
			errorCode = ErrCodeInternal
		} else {
			if rpcError, ok := resp.(*JSONRPCError); ok {
				success = false
				errorCode = rpcError.Error.Code
			}
		}

		if !success && cfg.EnableErrors {
			cfg.recorder.RecordError(ctx, method, errorCode)
		}
		if cfg.EnableLatency {
			latencyMs := float64(time.Since(startTime).Milliseconds())
			cfg.recorder.RecordLatency(ctx, method, latencyMs)
		}

		return resp, err
	}
}
