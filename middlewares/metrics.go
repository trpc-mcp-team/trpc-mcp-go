package middlewares

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	. "trpc.group/trpc-go/trpc-mcp-go"
)

type MetricsRecorder interface {
	// IncRequests 记录一个请求总数（按方法维度）
	IncRequests(method string)
	// IncErrors 记录一个错误总数（按方法+错误码维度）
	IncErrors(method string, code int)
	// ObserveLatency 记录一个请求时延（毫秒，按方法维度）
	ObserveLatency(method string, durationMs float64, success bool)

	// IncInFlight 记录并发中请求的数量 (进入+1, 退出-1)
	IncInFlight(method string)
	DecInFlight(method string)
}

// MetricsConfig 自定义组件及控制指标采集的行为
type MetricsConfig struct {
	recorder MetricsRecorder

	// Filter 返回 true 则对该方法采样
	Filter func(method string) bool

	// 是否记录各类指标（默认全部开启）
	EnableLatency  bool
	EnableCounters bool
	EnableErrors   bool
	EnableInFlight bool

	Logger Logger
}

func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		recorder:       NewInMemoryMetricsRecorder(),
		Filter:         nil,
		EnableLatency:  true,
		EnableCounters: true,
		EnableErrors:   true,
		EnableInFlight: true,
		Logger:         GetDefaultLogger(),
	}
}

func NewMetricsMiddleware(opts ...MetricsOption) MiddlewareFunc {
	cfg := DefaultMetricsConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return func(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (JSONRPCMessage, error) {
		method := req.Method
		if cfg.Filter != nil && !cfg.Filter(method) {
			return next(ctx, req, session)
		}

		var start time.Time
		if cfg.EnableLatency {
			start = time.Now()
		}
		if cfg.EnableInFlight {
			cfg.recorder.IncInFlight(method)
			defer cfg.recorder.DecInFlight(method)
		}
		if cfg.EnableCounters {
			cfg.recorder.IncRequests(method)
		}

		resp, err := next(ctx, req, session)

		success := err == nil
		errorCode := 0
		if err != nil {
			// 运行期错误统一映射到内部错误码
			errorCode = ErrCodeInternal
		} else {
			// 如果业务返回的是 JSON-RPC 错误响应，也视为失败并上报相应错误码
			if rpcErr, ok := resp.(*JSONRPCError); ok {
				success = false
				errorCode = rpcErr.Error.Code
			}
		}

		if !success && cfg.EnableErrors {
			cfg.recorder.IncErrors(method, errorCode)
		}
		if cfg.EnableLatency {
			durationMs := float64(time.Since(start).Milliseconds())
			cfg.recorder.ObserveLatency(method, durationMs, success)
		}

		return resp, err
	}
}

// InMemoryMetricsRecorder 是一个无外部依赖的内存实现，便于测试断言
type InMemoryMetricsRecorder struct {
	mu             sync.Mutex
	Requests       map[string]int
	Errors         map[string]map[int]int
	LatencyMs      map[string][]float64
	LatencySuccess map[string][]bool
	InFlight       map[string]int
}

func NewInMemoryMetricsRecorder() *InMemoryMetricsRecorder {
	return &InMemoryMetricsRecorder{
		Requests:       map[string]int{},
		Errors:         map[string]map[int]int{},
		LatencyMs:      map[string][]float64{},
		LatencySuccess: map[string][]bool{},
		InFlight:       map[string]int{},
	}
}

func (m *InMemoryMetricsRecorder) IncRequests(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests[method]++
}

func (m *InMemoryMetricsRecorder) IncErrors(method string, code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Errors[method]; !ok {
		m.Errors[method] = map[int]int{}
	}
	m.Errors[method][code]++
}

func (m *InMemoryMetricsRecorder) ObserveLatency(method string, durationMs float64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LatencyMs[method] = append(m.LatencyMs[method], durationMs)
	m.LatencySuccess[method] = append(m.LatencySuccess[method], success)
}

func (m *InMemoryMetricsRecorder) IncInFlight(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InFlight[method]++
}

func (m *InMemoryMetricsRecorder) DecInFlight(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InFlight[method]--
}

// PromRecorderConfig 与prometheus集成所需配置
type PromRecorderConfig struct {
	Namespace string
	Subsystem string
	Buckets   []float64 // 毫秒桶，留空用默认
}

func DefaultPromRecorderConfig() *PromRecorderConfig {
	return &PromRecorderConfig{
		Namespace: "mcp",
		Subsystem: "server",
		Buckets:   []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000},
	}
}

type PromRecorderOption func(*PromRecorderConfig)

func WithNamespace(ns string) PromRecorderOption {
	return func(cfg *PromRecorderConfig) {
		cfg.Namespace = ns
	}
}

func WithSubsystem(subsystem string) PromRecorderOption {
	return func(cfg *PromRecorderConfig) {
		cfg.Subsystem = subsystem
	}
}

func WithBuckets(buckets []float64) PromRecorderOption {
	return func(cfg *PromRecorderConfig) {
		cfg.Buckets = buckets
	}
}

type PrometheusMetricsRecorder struct {
	requests *prometheus.CounterVec
	errors   *prometheus.CounterVec
	latency  *prometheus.HistogramVec
	inFlight *prometheus.GaugeVec
}

func NewPrometheusMetricsRecorder(opts ...PromRecorderOption) (*PrometheusMetricsRecorder, error) {
	cfg := DefaultPromRecorderConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	req := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
		Name:      "requests_total",
		Help:      "Total MCP requests",
	}, []string{"method"})

	errs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
		Name:      "errors_total",
		Help:      "Total MCP errors",
	}, []string{"method", "code"})

	lat := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
		Name:      "latency_ms",
		Help:      "Latency of MCP requests (ms)",
		Buckets:   cfg.Buckets,
	}, []string{"method", "success"})

	inf := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
		Name:      "in_flight_requests",
		Help:      "Number of in-flight MCP requests",
	}, []string{"method"})

	for _, c := range []prometheus.Collector{req, errs, lat, inf} {
		if err := prometheus.Register(c); err != nil {
			var alreadyRegisteredError prometheus.AlreadyRegisteredError
			if errors.As(err, &alreadyRegisteredError) {
				continue
			}
			return nil, err
		}
	}

	return &PrometheusMetricsRecorder{requests: req, errors: errs, latency: lat, inFlight: inf}, nil
}

func (p *PrometheusMetricsRecorder) IncRequests(method string) {
	p.requests.WithLabelValues(method).Inc()
}

func (p *PrometheusMetricsRecorder) IncErrors(method string, code int) {
	p.errors.WithLabelValues(method, fmt.Sprintf("%d", code)).Inc()
}

func (p *PrometheusMetricsRecorder) ObserveLatency(method string, durationMs float64, success bool) {
	p.latency.WithLabelValues(method, fmt.Sprintf("%t", success)).Observe(durationMs)
}

func (p *PrometheusMetricsRecorder) IncInFlight(method string) {
	p.inFlight.WithLabelValues(method).Inc()
}

func (p *PrometheusMetricsRecorder) DecInFlight(method string) {
	p.inFlight.WithLabelValues(method).Dec()
}

func (p *PrometheusMetricsRecorder) RequestsCollector() prometheus.Collector {
	return p.requests
}
func (p *PrometheusMetricsRecorder) ErrorsCollector() prometheus.Collector {
	return p.errors
}

func (p *PrometheusMetricsRecorder) LatencyCollector() prometheus.Collector {
	return p.latency
}

func (p *PrometheusMetricsRecorder) InFlightCollector() prometheus.Collector {
	return p.inFlight
}

type MetricsOption func(*MetricsConfig)

func WithRecorder(recorder MetricsRecorder) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.recorder = recorder
	}
}

func WithFilter(filter func(method string) bool) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.Filter = filter
	}
}

func WithEnableLatency(enabled bool) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.EnableLatency = enabled
	}
}

func WithEnableCounters(enabled bool) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.EnableCounters = enabled
	}
}

func WithEnableErrors(enabled bool) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.EnableErrors = enabled
	}
}

func WithEnableInFlight(enabled bool) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.EnableInFlight = enabled
	}
}

func WithLogger(logger Logger) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.Logger = logger
	}
}
