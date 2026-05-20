// Package traces provides OpenTelemetry trace viewing and export for Forge.
// See the whole mountain — every chisel strike, every hammer blow.
package traces

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TraceStore persists and queries OTel spans.
type TraceStore struct {
	dir  string
	mu   sync.Mutex
	spans []StoredSpan
	loaded bool
}

// StoredSpan is a persisted span with metadata.
type StoredSpan struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Status    string            `json:"status"`
	Start     time.Time         `json:"start"`
	End       time.Time         `json:"end,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Events    []SpanEvent       `json:"events,omitempty"`
	Service   string            `json:"service"`
}

// SpanEvent is an event within a span.
type SpanEvent struct {
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}

// TraceSummary is a summary of a single trace.
type TraceSummary struct {
	TraceID    string        `json:"trace_id"`
	RootSpan   string        `json:"root_span"`
	SpanCount  int           `json:"span_count"`
	Duration   time.Duration `json:"duration"`
	Status     string        `json:"status"`
	Start      time.Time     `json:"start"`
	Service    string        `json:"service"`
	ErrorCount int           `json:"error_count"`
}

// TraceDetail is a full trace with all spans.
type TraceDetail struct {
	TraceID   string        `json:"trace_id"`
	Summary   TraceSummary  `json:"summary"`
	Spans     []StoredSpan  `json:"spans"`
	Tree      SpanNode      `json:"tree,omitempty"`
}

// SpanNode represents a span in a tree structure.
type SpanNode struct {
	Span    StoredSpan  `json:"span"`
	Children []SpanNode `json:"children,omitempty"`
}

// NewTraceStore creates a trace store backed by a directory.
func NewTraceStore(dir string) *TraceStore {
	return &TraceStore{dir: dir}
}

// Store records a span to the store.
func (ts *TraceStore) Store(span StoredSpan) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	ts.spans = append(ts.spans, span)

	// Append to daily file
	filename := filepath.Join(ts.dir, span.Start.Format("2006-01-02")+".jsonl")
	os.MkdirAll(ts.dir, 0o755)

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open trace file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(span)
	if err != nil {
		return fmt.Errorf("marshal span: %w", err)
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

// StoreBatch records multiple spans at once.
func (ts *TraceStore) StoreBatch(spans []StoredSpan) error {
	for _, s := range spans {
		if err := ts.Store(s); err != nil {
			return err
		}
	}
	return nil
}

// ListTraces returns summaries of all traces, optionally filtered.
func (ts *TraceStore) ListTraces(opts ListOpts) ([]TraceSummary, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	// Group spans by trace ID
	traces := make(map[string][]StoredSpan)
	for _, s := range ts.spans {
		if opts.matches(s) {
			traces[s.TraceID] = append(traces[s.TraceID], s)
		}
	}

	summaries := make([]TraceSummary, 0, len(traces))
	for tid, spans := range traces {
		summary := buildSummary(tid, spans)
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Start.After(summaries[j].Start)
	})

	if opts.Limit > 0 && len(summaries) > opts.Limit {
		summaries = summaries[:opts.Limit]
	}

	return summaries, nil
}

// GetTrace returns the full detail of a trace.
func (ts *TraceStore) GetTrace(traceID string) (*TraceDetail, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	var spans []StoredSpan
	for _, s := range ts.spans {
		if s.TraceID == traceID {
			spans = append(spans, s)
		}
	}

	if len(spans) == 0 {
		return nil, fmt.Errorf("trace %s not found", traceID)
	}

	summary := buildSummary(traceID, spans)
	tree := buildTree(spans)

	return &TraceDetail{
		TraceID: traceID,
		Summary: summary,
		Spans:   spans,
		Tree:    tree,
	}, nil
}

// DeleteTrace removes a trace from the store.
func (ts *TraceStore) DeleteTrace(traceID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	filtered := make([]StoredSpan, 0, len(ts.spans))
	for _, s := range ts.spans {
		if s.TraceID != traceID {
			filtered = append(filtered, s)
		}
	}
	ts.spans = filtered

	// Rewrite files
	return ts.rewriteFiles()
}

// Stats returns trace store statistics.
func (ts *TraceStore) Stats() StoreStats {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	stats := StoreStats{
		TotalSpans:  len(ts.spans),
		ServiceCount: make(map[string]int),
	}

	traces := make(map[string][]StoredSpan)
	for _, s := range ts.spans {
		traces[s.TraceID] = append(traces[s.TraceID], s)
		stats.ServiceCount[s.Service]++
		if s.Status == "error" {
			stats.ErrorSpans++
		}
		if s.Start.After(stats.LatestSpan) {
			stats.LatestSpan = s.Start
		}
		if stats.EarliestSpan.IsZero() || s.Start.Before(stats.EarliestSpan) {
			stats.EarliestSpan = s.Start
		}
	}
	stats.TotalTraces = len(traces)

	return stats
}

// StoreStats holds trace store statistics.
type StoreStats struct {
	TotalTraces  int            `json:"total_traces"`
	TotalSpans   int            `json:"total_spans"`
	ErrorSpans   int            `json:"error_spans"`
	ServiceCount map[string]int `json:"service_count"`
	EarliestSpan time.Time      `json:"earliest_span"`
	LatestSpan   time.Time      `json:"latest_span"`
}

// ListOpts filters trace listings.
type ListOpts struct {
	Service string    `json:"service,omitempty"`
	Status  string    `json:"status,omitempty"`
	After   time.Time `json:"after,omitempty"`
	Before  time.Time `json:"before,omitempty"`
	MinDur  time.Duration `json:"min_duration,omitempty"`
	Limit   int       `json:"limit,omitempty"`
}

func (o ListOpts) matches(s StoredSpan) bool {
	if o.Service != "" && s.Service != o.Service {
		return false
	}
	if o.Status != "" && s.Status != o.Status {
		return false
	}
	if !o.After.IsZero() && s.Start.Before(o.After) {
		return false
	}
	if !o.Before.IsZero() && s.Start.After(o.Before) {
		return false
	}
	if o.MinDur > 0 && s.Duration < o.MinDur {
		return false
	}
	return true
}

// ExportJaeger exports traces in Jaeger JSON format.
func (ts *TraceStore) ExportJaeger(opts ListOpts) ([]JaegerTrace, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	traces := make(map[string][]StoredSpan)
	for _, s := range ts.spans {
		if opts.matches(s) {
			traces[s.TraceID] = append(traces[s.TraceID], s)
		}
	}

	result := make([]JaegerTrace, 0, len(traces))
	for tid, spans := range traces {
		jt := JaegerTrace{
			TraceID:   tid,
			Spans:     make([]JaegerSpan, 0, len(spans)),
		}

		services := make(map[string]string)
		for _, s := range spans {
			js := JaegerSpan{
				TraceID:   s.TraceID,
				SpanID:    s.SpanID,
				Operation: s.Name,
				StartTime: s.Start.UnixMicro(),
				Duration:  s.Duration.Microseconds(),
				Tags:      make([]JaegerTag, 0),
			}

			if s.ParentID != "" {
				js.ParentSpanID = s.ParentID
			}

			if s.Status == "error" {
				js.Tags = append(js.Tags, JaegerTag{Key: "error", Type: "bool", Value: true})
			}

			js.Tags = append(js.Tags, JaegerTag{Key: "service", Type: "string", Value: s.Service})
			for k, v := range s.Attrs {
				js.Tags = append(js.Tags, JaegerTag{Key: k, Type: "string", Value: v})
			}

			js.Flags = 1
			services[s.Service] = s.Service

			jt.Spans = append(jt.Spans, js)
		}

		for svc := range services {
			jt.Processes = append(jt.Processes, JaegerProcess{
				ServiceName: svc,
				Tags:        []JaegerTag{},
			})
		}

		result = append(result, jt)
	}

	return result, nil
}

// ExportZipkin exports traces in Zipkin JSON format.
func (ts *TraceStore) ExportZipkin(opts ListOpts) ([]ZipkinSpan, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	var result []ZipkinSpan
	for _, s := range ts.spans {
		if !opts.matches(s) {
			continue
		}

		zs := ZipkinSpan{
			ID:          s.SpanID,
			TraceID:     s.TraceID,
			Name:        s.Name,
			Timestamp:   s.Start.UnixMicro(),
			Duration:    s.Duration.Microseconds(),
			LocalEndpoint: &ZipkinEndpoint{ServiceName: s.Service},
			Tags:        s.Attrs,
		}

		if s.ParentID != "" {
			zs.ParentID = s.ParentID
		}

		if s.Status == "error" {
			if zs.Tags == nil {
				zs.Tags = make(map[string]string)
			}
			zs.Tags["error"] = "true"
		}

		result = append(result, zs)
	}

	return result, nil
}

// ExportOpenTelemetry exports in OTLP JSON format.
func (ts *TraceStore) ExportOpenTelemetry(opts ListOpts) (*OTLPExport, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.loaded {
		ts.loadAll()
	}

	export := &OTLPExport{
		ResourceSpans: []OTLPResourceSpans{},
	}

	// Group by service
	services := make(map[string][]StoredSpan)
	for _, s := range ts.spans {
		if opts.matches(s) {
			services[s.Service] = append(services[s.Service], s)
		}
	}

	for svc, spans := range services {
		rs := OTLPResourceSpans{
			Resource: OTLPResource{
				Attributes: []OTLPAttribute{
					{Key: "service.name", Value: OTLPValue{StringValue: svc}},
				},
			},
			ScopeSpans: []OTLPScopeSpans{
				{
					Scope: OTLPScope{Name: "forge"},
					Spans: make([]OTLPSpan, 0, len(spans)),
				},
			},
		}

		for _, s := range spans {
			os := OTLPSpan{
				TraceID:        s.TraceID,
				SpanID:         s.SpanID,
				ParentSpanID:   s.ParentID,
				Name:           s.Name,
				Kind:           spanKindToInt(s.Kind),
				StartTimeUnix:  s.Start.UnixNano(),
				EndTimeUnix:    s.End.UnixNano(),
				Attributes:     make([]OTLPAttribute, 0),
			}

			for k, v := range s.Attrs {
				os.Attributes = append(os.Attributes, OTLPAttribute{
					Key: k, Value: OTLPValue{StringValue: v},
				})
			}

			if s.Status == "error" {
				os.Status = OTLPSpanStatus{Code: 2, Message: "error"}
			}

			rs.ScopeSpans[0].Spans = append(rs.ScopeSpans[0].Spans, os)
		}

		export.ResourceSpans = append(export.ResourceSpans, rs)
	}

	return export, nil
}

// Jaeger types

// JaegerTrace is a Jaeger-format trace.
type JaegerTrace struct {
	TraceID   string          `json:"traceID"`
	Spans     []JaegerSpan    `json:"spans"`
	Processes []JaegerProcess `json:"processes"`
}

// JaegerSpan is a Jaeger-format span.
type JaegerSpan struct {
	TraceID      string       `json:"traceID"`
	SpanID       string       `json:"spanID"`
	ParentSpanID string       `json:"parentSpanID,omitempty"`
	Operation    string       `json:"operationName"`
	StartTime    int64        `json:"startTime"`
	Duration     int64        `json:"duration"`
	Tags         []JaegerTag  `json:"tags"`
	Flags        int          `json:"flags"`
}

// JaegerTag is a Jaeger key-value tag.
type JaegerTag struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// JaegerProcess is a Jaeger process definition.
type JaegerProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []JaegerTag `json:"tags"`
}

// Zipkin types

// ZipkinSpan is a Zipkin-format span.
type ZipkinSpan struct {
	ID            string            `json:"id"`
	TraceID       string            `json:"traceId"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	Timestamp     int64             `json:"timestamp"`
	Duration      int64             `json:"duration"`
	LocalEndpoint *ZipkinEndpoint   `json:"localEndpoint,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ZipkinEndpoint is a Zipkin service endpoint.
type ZipkinEndpoint struct {
	ServiceName string `json:"serviceName"`
}

// OTLP types

// OTLPExport is an OTLP JSON export.
type OTLPExport struct {
	ResourceSpans []OTLPResourceSpans `json:"resourceSpans"`
}

// OTLPResourceSpans groups spans by resource.
type OTLPResourceSpans struct {
	Resource   OTLPResource      `json:"resource"`
	ScopeSpans []OTLPScopeSpans  `json:"scopeSpans"`
}

// OTLPResource describes the resource.
type OTLPResource struct {
	Attributes []OTLPAttribute `json:"attributes"`
}

// OTLPAttribute is a key-value attribute.
type OTLPAttribute struct {
	Key   string   `json:"key"`
	Value OTLPValue `json:"value"`
}

// OTLPValue is an OTLP attribute value.
type OTLPValue struct {
	StringValue string `json:"stringValue,omitempty"`
	IntValue    int64  `json:"intValue,omitempty"`
}

// OTLPScopeSpans groups spans by instrumentation scope.
type OTLPScopeSpans struct {
	Scope OTLPScope  `json:"scope"`
	Spans []OTLPSpan `json:"spans"`
}

// OTLPScope is an instrumentation scope.
type OTLPScope struct {
	Name string `json:"name"`
}

// OTLPSpan is an OTLP span.
type OTLPSpan struct {
	TraceID       string          `json:"traceId"`
	SpanID        string          `json:"spanId"`
	ParentSpanID  string          `json:"parentSpanId,omitempty"`
	Name          string          `json:"name"`
	Kind          int             `json:"kind"`
	StartTimeUnix int64           `json:"startTimeUnixNano"`
	EndTimeUnix   int64           `json:"endTimeUnixNano"`
	Attributes    []OTLPAttribute `json:"attributes,omitempty"`
	Status        OTLPSpanStatus  `json:"status,omitempty"`
}

// OTLPSpanStatus is span status.
type OTLPSpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// FormatTrace renders a trace detail as a readable string.
func FormatTrace(detail *TraceDetail) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Trace: %s\n", detail.TraceID))
	sb.WriteString(fmt.Sprintf("  Root:     %s\n", detail.Summary.RootSpan))
	sb.WriteString(fmt.Sprintf("  Spans:    %d\n", detail.Summary.SpanCount))
	sb.WriteString(fmt.Sprintf("  Duration: %v\n", detail.Summary.Duration))
	sb.WriteString(fmt.Sprintf("  Status:   %s\n", detail.Summary.Status))
	sb.WriteString(fmt.Sprintf("  Service:  %s\n", detail.Summary.Service))
	sb.WriteString(fmt.Sprintf("  Errors:   %d\n", detail.Summary.ErrorCount))
	sb.WriteString("\nSpan Tree:\n")
	formatNode(&sb, detail.Tree, 0)
	return sb.String()
}

// FormatSummary renders a trace summary as a readable string.
func FormatSummary(s TraceSummary) string {
	return fmt.Sprintf("%s  %-30s  %3d spans  %8v  %-6s  %s",
		s.TraceID[:16],
		s.RootSpan,
		s.SpanCount,
		s.Duration.Round(time.Millisecond),
		s.Status,
		s.Service,
	)
}

func formatNode(sb *strings.Builder, node SpanNode, depth int) {
	indent := strings.Repeat("  ", depth)
	status := ""
	if node.Span.Status == "error" {
		status = " ⚠"
	}
	sb.WriteString(fmt.Sprintf("%s├─ %s (%v)%s\n", indent, node.Span.Name, node.Span.Duration.Round(time.Microsecond), status))
	for _, child := range node.Children {
		formatNode(sb, child, depth+1)
	}
}

func buildSummary(traceID string, spans []StoredSpan) TraceSummary {
	summary := TraceSummary{
		TraceID:   traceID,
		SpanCount: len(spans),
	}

	var minStart, maxEnd time.Time
	for _, s := range spans {
		if s.ParentID == "" {
			summary.RootSpan = s.Name
			summary.Service = s.Service
		}
		if s.Status == "error" {
			summary.ErrorCount++
		}
		if minStart.IsZero() || s.Start.Before(minStart) {
			minStart = s.Start
		}
		if s.End.After(maxEnd) {
			maxEnd = s.End
		}
	}

	summary.Start = minStart
	if !maxEnd.IsZero() {
		summary.Duration = maxEnd.Sub(minStart)
	}

	if summary.ErrorCount > 0 {
		summary.Status = "error"
	} else {
		summary.Status = "ok"
	}

	return summary
}

func buildTree(spans []StoredSpan) SpanNode {
	byID := make(map[string]*StoredSpan)
	for i := range spans {
		byID[spans[i].SpanID] = &spans[i]
	}

	var root *StoredSpan
	children := make(map[string][]*StoredSpan)
	for i := range spans {
		if spans[i].ParentID == "" {
			root = &spans[i]
		} else {
			children[spans[i].ParentID] = append(children[spans[i].ParentID], &spans[i])
		}
	}

	if root == nil && len(spans) > 0 {
		root = &spans[0]
	}

	if root == nil {
		return SpanNode{}
	}

	return buildNode(root, children)
}

func buildNode(span *StoredSpan, children map[string][]*StoredSpan) SpanNode {
	node := SpanNode{Span: *span}
	for _, child := range children[span.SpanID] {
		node.Children = append(node.Children, buildNode(child, children))
	}
	return node
}

func (ts *TraceStore) loadAll() {
	ts.loaded = true
	files, err := filepath.Glob(filepath.Join(ts.dir, "*.jsonl"))
	if err != nil {
		return
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var span StoredSpan
			if err := json.Unmarshal([]byte(line), &span); err != nil {
				continue
			}
			ts.spans = append(ts.spans, span)
		}
	}
}

func (ts *TraceStore) rewriteFiles() error {
	byDay := make(map[string][]StoredSpan)
	for _, s := range ts.spans {
		key := s.Start.Format("2006-01-02")
		byDay[key] = append(byDay[key], s)
	}

	os.RemoveAll(ts.dir)
	os.MkdirAll(ts.dir, 0o755)

	for day, spans := range byDay {
		path := filepath.Join(ts.dir, day+".jsonl")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(f)
		for _, s := range spans {
			enc.Encode(s)
		}
		f.Close()
	}

	return nil
}

func spanKindToInt(kind string) int {
	switch kind {
	case "internal":
		return 1
	case "server":
		return 2
	case "client":
		return 3
	case "producer":
		return 4
	case "consumer":
		return 5
	default:
		return 1
	}
}
