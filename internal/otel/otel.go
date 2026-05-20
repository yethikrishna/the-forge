// Package otel provides OpenTelemetry integration for the forge.
// Every strike of the hammer sends a signal through the mountain.
package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SpanKind describes the kind of span.
type SpanKind int

const (
	KindInternal SpanKind = iota
	KindServer
	KindClient
	KindProducer
	KindConsumer
)

// SpanStatus represents span status.
type SpanStatus int

const (
	StatusUnset SpanStatus = iota
	StatusOK
	StatusError
)

// Span is a recording of a single operation.
type Span struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Name      string            `json:"name"`
	Kind      SpanKind          `json:"kind"`
	Status    SpanStatus        `json:"status"`
	Start     time.Time         `json:"start"`
	End       time.Time         `json:"end,omitempty"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Events    []Event           `json:"events,omitempty"`
	Service   string            `json:"service"`
	Resource  map[string]string `json:"resource,omitempty"`
}

// Event is a timed event within a span.
type Event struct {
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}

// Duration returns the span's duration.
func (s *Span) Duration() time.Duration {
	if s.End.IsZero() {
		return 0
	}
	return s.End.Sub(s.Start)
}

// IsFinished returns whether the span has ended.
func (s *Span) IsFinished() bool {
	return !s.End.IsZero()
}

// Tracer creates and manages spans.
type Tracer struct {
	service  string
	resource map[string]string
	spans    map[string]*Span
	mu       sync.Mutex
	exporter Exporter
}

// NewTracer creates a new tracer.
func NewTracer(service string, exporter Exporter) *Tracer {
	return &Tracer{
		service:  service,
		resource: map[string]string{"service.name": service},
		spans:    make(map[string]*Span),
		exporter: exporter,
	}
}

// Start starts a new span.
func (t *Tracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	span := &Span{
		TraceID: generateTraceID(),
		SpanID:  generateSpanID(),
		Name:    name,
		Kind:    KindInternal,
		Start:   time.Now().UTC(),
		Attrs:   make(map[string]string),
		Service: t.service,
		Resource: copyMap(t.resource),
	}

	// Link to parent if in context
	if parent, ok := SpanFromContext(ctx); ok {
		span.ParentID = parent.SpanID
		span.TraceID = parent.TraceID
	}

	for _, o := range opts {
		o(span)
	}

	t.mu.Lock()
	t.spans[span.SpanID] = span
	t.mu.Unlock()

	ctx = ContextWithSpan(ctx, span)
	return ctx, span
}

// End ends a span.
func (t *Tracer) End(span *Span, opts ...EndOption) {
	span.End = time.Now().UTC()

	for _, o := range opts {
		o(span)
	}

	if t.exporter != nil {
		t.exporter.Export(span)
	}

	t.mu.Lock()
	delete(t.spans, span.SpanID)
	t.mu.Unlock()
}

// AddEvent adds a timed event to a span.
func (t *Tracer) AddEvent(span *Span, name string, attrs map[string]string) {
	span.Events = append(span.Events, Event{
		Name:      name,
		Timestamp: time.Now().UTC(),
		Attrs:     attrs,
	})
}

// ActiveSpans returns currently active spans.
func (t *Tracer) ActiveSpans() []*Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	spans := make([]*Span, 0, len(t.spans))
	for _, s := range t.spans {
		spans = append(spans, s)
	}
	return spans
}

// SpanOption customizes span creation.
type SpanOption func(*Span)

// WithParent sets the parent span.
func WithParent(parent *Span) SpanOption {
	return func(s *Span) {
		if parent != nil {
			s.ParentID = parent.SpanID
			s.TraceID = parent.TraceID
		}
	}
}

// WithKind sets the span kind.
func WithKind(kind SpanKind) SpanOption {
	return func(s *Span) { s.Kind = kind }
}

// WithAttrs sets span attributes.
func WithAttrs(attrs map[string]string) SpanOption {
	return func(s *Span) {
		for k, v := range attrs {
			s.Attrs[k] = v
		}
	}
}

// EndOption customizes span end.
type EndOption func(*Span)

// WithStatus sets the span status.
func WithStatus(status SpanStatus) EndOption {
	return func(s *Span) { s.Status = status }
}

// WithError marks the span as errored.
func WithError(err error) EndOption {
	return func(s *Span) {
		s.Status = StatusError
		s.Attrs["error.message"] = err.Error()
	}
}

// contextKey for span storage.
type contextKey struct{}

// ContextWithSpan adds a span to context.
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, contextKey{}, span)
}

// SpanFromContext extracts a span from context.
func SpanFromContext(ctx context.Context) (*Span, bool) {
	span, ok := ctx.Value(contextKey{}).(*Span)
	return span, ok
}

// Exporter exports spans to a backend.
type Exporter interface {
	Export(span *Span)
}

// ConsoleExporter logs spans to stdout.
type ConsoleExporter struct{}

func (e *ConsoleExporter) Export(span *Span) {
	status := "OK"
	if span.Status == StatusError {
		status = "ERROR"
	}
	fmt.Printf("[OTEL] %s %s %v %s attrs=%v\n",
		span.Name, span.SpanID, span.Duration(), status, span.Attrs)
}

// FileExporter writes spans to a JSONL file.
type FileExporter struct {
	path string
	mu   sync.Mutex
}

// NewFileExporter creates a file exporter.
func NewFileExporter(path string) *FileExporter {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0o755)
	return &FileExporter{path: path}
}

func (e *FileExporter) Export(span *Span) {
	e.mu.Lock()
	defer e.mu.Unlock()

	f, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(span)
	f.Write(data)
	f.Write([]byte("\n"))
}

// MultiExporter fans out to multiple exporters.
type MultiExporter []Exporter

func (e MultiExporter) Export(span *Span) {
	for _, exp := range e {
		exp.Export(span)
	}
}

// NoopExporter discards all spans.
type NoopExporter struct{}

func (e *NoopExporter) Export(_ *Span) {}

// Trace represents a complete trace (collection of spans).
type Trace struct {
	TraceID string
	Spans   []*Span
}

// GetTrace reconstructs a trace from exported spans.
func GetTrace(spans []*Span, traceID string) *Trace {
	var traceSpans []*Span
	for _, s := range spans {
		if s.TraceID == traceID {
			traceSpans = append(traceSpans, s)
		}
	}
	return &Trace{TraceID: traceID, Spans: traceSpans}
}

// FormatTrace formats a trace for display.
func FormatTrace(trace *Trace) string {
	var result string
	result += fmt.Sprintf("Trace %s\n", trace.TraceID)
	for _, s := range trace.Spans {
		indent := ""
		if s.ParentID != "" {
			indent = "  "
		}
		status := "OK"
		if s.Status == StatusError {
			status = "ERR"
		}
		sid := s.SpanID
		if len(sid) > 8 {
			sid = sid[:8]
		}
		result += fmt.Sprintf("%s[%s] %s (%v) %s\n", indent, sid, s.Name, s.Duration(), status)
		for _, e := range s.Events {
			result += fmt.Sprintf("%s  ↳ %s\n", indent, e.Name)
		}
	}
	return result
}

func generateTraceID() string {
	return fmt.Sprintf("%032x", time.Now().UnixNano())
}

func generateSpanID() string {
	return fmt.Sprintf("%016x", time.Now().UnixNano()%0xFFFF)
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
