// Package metrics provides Prometheus-compatible metrics collection for Forge.
// Tracks agent performance, costs, latencies, error rates, and resource usage.
// Exposes /metrics endpoint for Prometheus scraping.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType defines the kind of metric.
type MetricType string

const (
	TypeCounter   MetricType = "counter"
	TypeGauge     MetricType = "gauge"
	TypeHistogram MetricType = "histogram"
	TypeSummary   MetricType = "summary"
)

// LabelSet is a set of key-value label pairs.
type LabelSet map[string]string

// Metric represents a single metric with labels.
type Metric struct {
	Name   string    `json:"name"`
	Help   string    `json:"help"`
	Type   MetricType `json:"type"`
	Labels LabelSet  `json:"labels,omitempty"`
	Value  float64   `json:"value"`
}

// Counter is a monotonically increasing counter.
type Counter struct {
	name   string
	labels LabelSet
	value  atomic.Int64
	help   string
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Add(delta float64) {
	c.value.Add(int64(delta))
}

func (c *Counter) Value() float64 {
	return float64(c.value.Load())
}

func (c *Counter) Name() string     { return c.name }
func (c *Counter) Labels() LabelSet { return c.labels }
func (c *Counter) Help() string     { return c.help }
func (c *Counter) Type() MetricType { return TypeCounter }

// Gauge is a metric that can go up and down.
type Gauge struct {
	name   string
	labels LabelSet
	value  float64
	help   string
	mu     sync.Mutex
}

func (g *Gauge) Set(val float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = val
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

func (g *Gauge) Name() string     { return g.name }
func (g *Gauge) Labels() LabelSet { return g.labels }
func (g *Gauge) Help() string     { return g.help }
func (g *Gauge) Type() MetricType { return TypeGauge }

// Histogram tracks distribution of values.
type Histogram struct {
	name    string
	labels  LabelSet
	help    string
	buckets []float64
	counts  []atomic.Int64
	sum     float64
	count   atomic.Int64
	mu      sync.Mutex
}

func NewHistogram(name string, labels LabelSet, help string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	sort.Float64s(buckets)
	h := &Histogram{
		name:    name,
		labels:  labels,
		help:    help,
		buckets: buckets,
		counts:  make([]atomic.Int64, len(buckets)+1), // +1 for the +Inf bucket
	}
	return h
}

// DefaultBuckets provides default histogram bucket boundaries.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func (h *Histogram) Observe(val float64) {
	h.count.Add(1)
	h.mu.Lock()
	h.sum += val
	h.mu.Unlock()

	for i, b := range h.buckets {
		if val <= b {
			h.counts[i].Add(1)
		}
	}
	// +Inf bucket
	h.counts[len(h.buckets)].Add(1)
}

func (h *Histogram) Name() string     { return h.name }
func (h *Histogram) Labels() LabelSet { return h.labels }
func (h *Histogram) Help() string     { return h.help }
func (h *Histogram) Type() MetricType { return TypeHistogram }

// HistogramStats returns statistics for the histogram.
type HistogramStats struct {
	Count   int64              `json:"count"`
	Sum     float64            `json:"sum"`
	Buckets map[float64]int64  `json:"buckets"`
	Mean    float64            `json:"mean,omitempty"`
}

func (h *Histogram) Stats() HistogramStats {
	count := h.count.Load()
	h.mu.Lock()
	sum := h.sum
	h.mu.Unlock()

	buckets := make(map[float64]int64)
	for i, b := range h.buckets {
		buckets[b] = h.counts[i].Load()
	}

	mean := float64(0)
	if count > 0 {
		mean = sum / float64(count)
	}

	return HistogramStats{
		Count:   count,
		Sum:     sum,
		Buckets: buckets,
		Mean:    mean,
	}
}

// Registry stores and manages all metrics.
type Registry struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
	mu         sync.RWMutex
}

// NewRegistry creates a new metric registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// NewCounter creates and registers a counter.
func (r *Registry) NewCounter(name, help string, labels LabelSet) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(name, labels)
	if c, ok := r.counters[key]; ok {
		return c
	}

	c := &Counter{name: name, labels: labels, help: help}
	r.counters[key] = c
	return c
}

// NewGauge creates and registers a gauge.
func (r *Registry) NewGauge(name, help string, labels LabelSet) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(name, labels)
	if g, ok := r.gauges[key]; ok {
		return g
	}

	g := &Gauge{name: name, labels: labels, help: help}
	r.gauges[key] = g
	return g
}

// NewHistogram creates and registers a histogram.
func (r *Registry) NewHistogram(name, help string, labels LabelSet, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(name, labels)
	if h, ok := r.histograms[key]; ok {
		return h
	}

	h := NewHistogram(name, labels, help, buckets)
	r.histograms[key] = h
	return h
}

// AllMetrics returns all registered metrics.
func (r *Registry) AllMetrics() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metrics []Metric

	for _, c := range r.counters {
		metrics = append(metrics, Metric{
			Name:   c.Name(),
			Help:   c.Help(),
			Type:   c.Type(),
			Labels: c.Labels(),
			Value:  c.Value(),
		})
	}

	for _, g := range r.gauges {
		metrics = append(metrics, Metric{
			Name:   g.Name(),
			Help:   g.Help(),
			Type:   g.Type(),
			Labels: g.Labels(),
			Value:  g.Value(),
		})
	}

	for _, h := range r.histograms {
		stats := h.Stats()
		metrics = append(metrics, Metric{
			Name:   h.Name() + "_count",
			Help:   h.Help(),
			Type:   TypeCounter,
			Labels: h.Labels(),
			Value:  float64(stats.Count),
		})
		metrics = append(metrics, Metric{
			Name:   h.Name() + "_sum",
			Help:   h.Help(),
			Type:   TypeGauge,
			Labels: h.Labels(),
			Value:  stats.Sum,
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})

	return metrics
}

// PrometheusFormat outputs all metrics in Prometheus exposition format.
func (r *Registry) PrometheusFormat() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder

	// Counters
	for _, c := range r.counters {
		b.WriteString(fmt.Sprintf("# HELP %s %s\n", c.Name(), c.Help()))
		b.WriteString(fmt.Sprintf("# TYPE %s counter\n", c.Name()))
		b.WriteString(fmt.Sprintf("%s%s %v\n", c.Name(), formatLabels(c.Labels()), c.Value()))
	}

	// Gauges
	for _, g := range r.gauges {
		b.WriteString(fmt.Sprintf("# HELP %s %s\n", g.Name(), g.Help()))
		b.WriteString(fmt.Sprintf("# TYPE %s gauge\n", g.Name()))
		b.WriteString(fmt.Sprintf("%s%s %v\n", g.Name(), formatLabels(g.Labels()), g.Value()))
	}

	// Histograms
	for _, h := range r.histograms {
		stats := h.Stats()
		b.WriteString(fmt.Sprintf("# HELP %s %s\n", h.Name(), h.Help()))
		b.WriteString(fmt.Sprintf("# TYPE %s histogram\n", h.Name()))

		for bucket, count := range stats.Buckets {
			b.WriteString(fmt.Sprintf("%s_bucket{le=\"%v\"%s} %d\n",
				h.Name(), bucket, extraLabels(h.Labels()), count))
		}
		b.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"%s} %d\n",
			h.Name(), extraLabels(h.Labels()), stats.Count))
		b.WriteString(fmt.Sprintf("%s_sum%s %v\n", h.Name(), formatLabels(h.Labels()), stats.Sum))
		b.WriteString(fmt.Sprintf("%s_count%s %d\n", h.Name(), formatLabels(h.Labels()), stats.Count))
	}

	return b.String()
}

// Summary returns a human-readable metrics summary.
func (r *Registry) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString("Forge Metrics Summary\n")
	b.WriteString("=====================\n\n")

	b.WriteString(fmt.Sprintf("Counters: %d | Gauges: %d | Histograms: %d\n\n",
		len(r.counters), len(r.gauges), len(r.histograms)))

	if len(r.counters) > 0 {
		b.WriteString("Counters:\n")
		for _, c := range r.counters {
			b.WriteString(fmt.Sprintf("  %-40s %v\n", c.Name()+formatLabels(c.Labels()), c.Value()))
		}
		b.WriteString("\n")
	}

	if len(r.gauges) > 0 {
		b.WriteString("Gauges:\n")
		for _, g := range r.gauges {
			b.WriteString(fmt.Sprintf("  %-40s %v\n", g.Name()+formatLabels(g.Labels()), g.Value()))
		}
		b.WriteString("\n")
	}

	if len(r.histograms) > 0 {
		b.WriteString("Histograms:\n")
		for _, h := range r.histograms {
			stats := h.Stats()
			b.WriteString(fmt.Sprintf("  %-40s count=%d mean=%.4f\n",
				h.Name()+formatLabels(h.Labels()), stats.Count, stats.Mean))
		}
	}

	return b.String()
}

// Reset resets all metric values.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.counters {
		c.value.Store(0)
	}
	for _, g := range r.gauges {
		g.Set(0)
	}
	for _, h := range r.histograms {
		for i := range h.counts {
			h.counts[i].Store(0)
		}
		h.mu.Lock()
		h.sum = 0
		h.mu.Unlock()
		h.count.Store(0)
	}
}

// DefaultForgeRegistry is the global default metrics registry.
var DefaultForgeRegistry = NewRegistry()

// Pre-registered Forge metrics
var (
	AgentRequestsTotal    *Counter
	AgentErrorsTotal      *Counter
	AgentDuration         *Histogram
	TokensUsedTotal       *Counter
	CostTotal             *Counter
	ActiveAgents          *Gauge
	QueueDepth            *Gauge
	ProviderLatency       *Histogram
	CircuitBreakerOpen    *Gauge
	BuildDuration         *Histogram
	TestResultsTotal      *Counter
	NotificationsSent     *Counter
	PipelineStageDuration *Histogram
)

func init() {
	r := DefaultForgeRegistry

	AgentRequestsTotal = r.NewCounter("forge_agent_requests_total", "Total number of agent requests", nil)
	AgentErrorsTotal = r.NewCounter("forge_agent_errors_total", "Total number of agent errors", nil)
	AgentDuration = r.NewHistogram("forge_agent_duration_seconds", "Agent request duration", nil, []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60})
	TokensUsedTotal = r.NewCounter("forge_tokens_used_total", "Total tokens consumed", nil)
	CostTotal = r.NewCounter("forge_cost_total", "Total cost in dollars", nil)
	ActiveAgents = r.NewGauge("forge_active_agents", "Number of currently active agents", nil)
	QueueDepth = r.NewGauge("forge_queue_depth", "Current task queue depth", nil)
	ProviderLatency = r.NewHistogram("forge_provider_latency_seconds", "Provider API latency", nil, []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5})
	CircuitBreakerOpen = r.NewGauge("forge_circuit_breaker_open", "Whether circuit breaker is open (1=open)", nil)
	BuildDuration = r.NewHistogram("forge_build_duration_seconds", "Build duration", nil, []float64{1, 5, 10, 30, 60, 120})
	TestResultsTotal = r.NewCounter("forge_test_results_total", "Total test results", nil)
	NotificationsSent = r.NewCounter("forge_notifications_sent_total", "Total notifications sent", nil)
	PipelineStageDuration = r.NewHistogram("forge_pipeline_stage_duration_seconds", "Pipeline stage duration", nil, []float64{0.1, 0.5, 1, 5, 10, 30, 60})
}

// Helper functions

func metricKey(name string, labels LabelSet) string {
	if len(labels) == 0 {
		return name
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return name + "{" + strings.Join(parts, ",") + "}"
}

func formatLabels(labels LabelSet) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%q", k, v))
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ",") + "}"
}

func extraLabels(labels LabelSet) string {
	if len(labels) == 0 {
		return ""
	}
	return "," + strings.Join(func() []string {
		parts := make([]string, 0, len(labels))
		for k, v := range labels {
			parts = append(parts, fmt.Sprintf("%s=%q", k, v))
		}
		sort.Strings(parts)
		return parts
	}(), ",")
}

// Timer is a helper for timing operations.
type Timer struct {
	histogram *Histogram
	start     time.Time
}

// NewTimer creates a new timer.
func NewTimer(h *Histogram) *Timer {
	return &Timer{histogram: h, start: time.Now()}
}

// ObserveDuration records the duration since the timer was created.
func (t *Timer) ObserveDuration() {
	elapsed := time.Since(t.start).Seconds()
	t.histogram.Observe(elapsed)
}
