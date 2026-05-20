package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("test_counter", "A test counter", nil)

	c.Inc()
	if c.Value() != 1 {
		t.Errorf("expected 1, got %v", c.Value())
	}

	c.Inc()
	if c.Value() != 2 {
		t.Errorf("expected 2, got %v", c.Value())
	}

	c.Add(5)
	if c.Value() != 7 {
		t.Errorf("expected 7, got %v", c.Value())
	}
}

func TestCounterWithLabels(t *testing.T) {
	r := NewRegistry()
	c1 := r.NewCounter("http_requests", "HTTP requests", LabelSet{"method": "GET", "path": "/api"})
	c2 := r.NewCounter("http_requests", "HTTP requests", LabelSet{"method": "POST", "path": "/api"})

	c1.Inc()
	c2.Inc()
	c2.Inc()

	if c1.Value() != 1 {
		t.Errorf("expected 1 for GET, got %v", c1.Value())
	}
	if c2.Value() != 2 {
		t.Errorf("expected 2 for POST, got %v", c2.Value())
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("test_gauge", "A test gauge", nil)

	g.Set(42.5)
	if g.Value() != 42.5 {
		t.Errorf("expected 42.5, got %v", g.Value())
	}

	g.Inc()
	if g.Value() != 43.5 {
		t.Errorf("expected 43.5, got %v", g.Value())
	}

	g.Dec()
	if g.Value() != 42.5 {
		t.Errorf("expected 42.5, got %v", g.Value())
	}
}

func TestHistogram(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("test_histogram", "A test histogram", nil, []float64{0.1, 0.5, 1, 5})

	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.7)
	h.Observe(3.0)
	h.Observe(10.0)

	stats := h.Stats()
	if stats.Count != 5 {
		t.Errorf("expected 5 observations, got %d", stats.Count)
	}

	if stats.Buckets[0.1] != 1 {
		t.Errorf("expected 1 observation in <=0.1 bucket, got %d", stats.Buckets[0.1])
	}
	if stats.Buckets[0.5] != 2 {
		t.Errorf("expected 2 cumulative observations in <=0.5 bucket, got %d", stats.Buckets[0.5])
	}
	if stats.Buckets[1.0] != 3 {
		t.Errorf("expected 3 cumulative observations in <=1.0 bucket, got %d", stats.Buckets[1.0])
	}
}

func TestHistogramStats(t *testing.T) {
	h := NewHistogram("test", nil, "test", []float64{1, 5, 10})

	h.Observe(2.0)
	h.Observe(4.0)

	stats := h.Stats()
	if stats.Count != 2 {
		t.Errorf("expected 2, got %d", stats.Count)
	}
	if stats.Sum != 6.0 {
		t.Errorf("expected 6.0, got %v", stats.Sum)
	}
	if stats.Mean != 3.0 {
		t.Errorf("expected 3.0, got %v", stats.Mean)
	}
}

func TestPrometheusFormat(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("http_requests_total", "Total HTTP requests", LabelSet{"method": "GET"})
	g := r.NewGauge("active_connections", "Active connections", nil)
	h := r.NewHistogram("request_duration_seconds", "Request duration", nil, []float64{0.1, 0.5})

	c.Inc()
	c.Inc()
	g.Set(5)
	h.Observe(0.3)

	output := r.PrometheusFormat()

	if !strings.Contains(output, "http_requests_total") {
		t.Error("expected counter in Prometheus output")
	}
	if !strings.Contains(output, "# TYPE http_requests_total counter") {
		t.Error("expected counter type declaration")
	}
	if !strings.Contains(output, "active_connections") {
		t.Error("expected gauge in Prometheus output")
	}
	if !strings.Contains(output, "request_duration_seconds") {
		t.Error("expected histogram in Prometheus output")
	}
	if !strings.Contains(output, "request_duration_seconds_bucket") {
		t.Error("expected histogram buckets in output")
	}
}

func TestSummary(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("test_counter", "Test", nil)
	c.Inc()

	summary := r.Summary()
	if !strings.Contains(summary, "Counters:") {
		t.Error("expected Counters section")
	}
	if !strings.Contains(summary, "test_counter") {
		t.Error("expected counter name in summary")
	}
}

func TestReset(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("test_counter", "Test", nil)
	g := r.NewGauge("test_gauge", "Test", nil)

	c.Inc()
	c.Inc()
	g.Set(100)

	r.Reset()

	if c.Value() != 0 {
		t.Errorf("expected 0 after reset, got %v", c.Value())
	}
	if g.Value() != 0 {
		t.Errorf("expected 0 after reset, got %v", g.Value())
	}
}

func TestAllMetrics(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("counter1", "Test counter", nil)
	g := r.NewGauge("gauge1", "Test gauge", nil)
	h := r.NewHistogram("hist1", "Test histogram", nil, []float64{1})

	c.Inc()
	g.Set(42)
	h.Observe(0.5)

	metrics := r.AllMetrics()
	if len(metrics) < 3 { // counter + gauge + histogram (count + sum)
		t.Errorf("expected at least 3 metrics, got %d", len(metrics))
	}
}

func TestTimer(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("operation_duration", "Operation duration", nil, []float64{0.01, 0.1, 1})

	timer := NewTimer(h)
	time.Sleep(10 * time.Millisecond)
	timer.ObserveDuration()

	stats := h.Stats()
	if stats.Count != 1 {
		t.Errorf("expected 1 observation, got %d", stats.Count)
	}
	if stats.Sum < 0.01 {
		t.Errorf("expected duration >= 0.01s, got %v", stats.Sum)
	}
}

func TestDefaultForgeRegistry(t *testing.T) {
	if AgentRequestsTotal == nil {
		t.Error("expected AgentRequestsTotal to be initialized")
	}
	if ActiveAgents == nil {
		t.Error("expected ActiveAgents to be initialized")
	}
	if AgentDuration == nil {
		t.Error("expected AgentDuration to be initialized")
	}

	// Test using default registry
	AgentRequestsTotal.Inc()
	if AgentRequestsTotal.Value() < 1 {
		t.Error("expected AgentRequestsTotal to be incremented")
	}
}

func TestMetricKey(t *testing.T) {
	tests := []struct {
		name   string
		labels LabelSet
		want   string
	}{
		{"test", nil, "test"},
		{"test", LabelSet{}, "test"},
		{"test", LabelSet{"a": "1"}, "test{a=1}"},
	}

	for _, tt := range tests {
		got := metricKey(tt.name, tt.labels)
		if got != tt.want {
			t.Errorf("metricKey(%q, %v) = %q, want %q", tt.name, tt.labels, got, tt.want)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		labels LabelSet
		want   string
	}{
		{nil, ""},
		{LabelSet{}, ""},
		{LabelSet{"method": "GET"}, `{method="GET"}`},
	}

	for _, tt := range tests {
		got := formatLabels(tt.labels)
		if got != tt.want {
			t.Errorf("formatLabels(%v) = %q, want %q", tt.labels, got, tt.want)
		}
	}
}

func TestCounterDuplicateRegistration(t *testing.T) {
	r := NewRegistry()
	c1 := r.NewCounter("test", "Test", LabelSet{"a": "1"})
	c2 := r.NewCounter("test", "Test", LabelSet{"a": "1"})

	c1.Inc()
	if c2.Value() != 1 {
		t.Error("expected same counter instance for duplicate registration")
	}
}
