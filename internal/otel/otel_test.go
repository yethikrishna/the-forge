package otel_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/forge/sword/internal/otel"
)

func TestStartEnd(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	ctx, span := tracer.Start(context.Background(), "test-operation")
	if span == nil {
		t.Fatal("span should not be nil")
	}
	if span.Name != "test-operation" {
		t.Errorf("expected name 'test-operation', got %s", span.Name)
	}
	if span.Service != "test" {
		t.Errorf("expected service 'test', got %s", span.Service)
	}

	tracer.End(span)
	if !span.IsFinished() {
		t.Error("span should be finished")
	}
	if span.Duration() == 0 {
		t.Error("span should have duration")
	}

	_ = ctx
}

func TestParentChild(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	ctx, parent := tracer.Start(context.Background(), "parent")
	childCtx, child := tracer.Start(ctx, "child")

	if child.ParentID != parent.SpanID {
		t.Errorf("child should have parent ID %s, got %s", parent.SpanID, child.ParentID)
	}
	if child.TraceID != parent.TraceID {
		t.Error("child should share parent trace ID")
	}

	tracer.End(child)
	tracer.End(parent)

	_ = childCtx
}

func TestSpanAttrs(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, span := tracer.Start(context.Background(), "op",
		otel.WithAttrs(map[string]string{"key": "value"}),
	)

	if span.Attrs["key"] != "value" {
		t.Error("should have attr")
	}

	tracer.End(span)
}

func TestSpanStatus(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span, otel.WithStatus(otel.StatusOK))

	if span.Status != otel.StatusOK {
		t.Error("should be OK")
	}
}

func TestSpanError(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span, otel.WithError(fmt.Errorf("boom")))

	if span.Status != otel.StatusError {
		t.Error("should be error")
	}
	if span.Attrs["error.message"] != "boom" {
		t.Error("should have error message")
	}
}

func TestAddEvent(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, span := tracer.Start(context.Background(), "op")
	tracer.AddEvent(span, "checkpoint", map[string]string{"step": "1"})
	tracer.End(span)

	if len(span.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "checkpoint" {
		t.Errorf("expected 'checkpoint', got %s", span.Events[0].Name)
	}
}

func TestSpanKind(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, span := tracer.Start(context.Background(), "op", otel.WithKind(otel.KindServer))
	if span.Kind != otel.KindServer {
		t.Error("should be server kind")
	}
	tracer.End(span)
}

func TestActiveSpans(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	_, s1 := tracer.Start(context.Background(), "op1")
	_, s2 := tracer.Start(context.Background(), "op2")

	active := tracer.ActiveSpans()
	if len(active) != 2 {
		t.Errorf("expected 2 active, got %d", len(active))
	}

	tracer.End(s1)
	tracer.End(s2)

	active = tracer.ActiveSpans()
	if len(active) != 0 {
		t.Errorf("expected 0 active, got %d", len(active))
	}
}

func TestConsoleExporter(t *testing.T) {
	exporter := &otel.ConsoleExporter{}
	tracer := otel.NewTracer("test", exporter)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span)
}

func TestFileExporter(t *testing.T) {
	dir := t.TempDir()
	exporter := otel.NewFileExporter(dir + "/traces.jsonl")
	tracer := otel.NewTracer("test", exporter)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span)

	time.Sleep(50 * time.Millisecond)

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Error("file should exist")
	}
}

func TestMultiExporter(t *testing.T) {
	exporter := otel.MultiExporter{
		&otel.ConsoleExporter{},
		&otel.NoopExporter{},
	}
	tracer := otel.NewTracer("test", exporter)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span)
}

func TestNoopExporter(t *testing.T) {
	exporter := &otel.NoopExporter{}
	tracer := otel.NewTracer("test", exporter)

	_, span := tracer.Start(context.Background(), "op")
	tracer.End(span)
}

func TestGetTrace(t *testing.T) {
	tracer := otel.NewTracer("test", nil)

	ctx, parent := tracer.Start(context.Background(), "parent")
	_, child := tracer.Start(ctx, "child")

	tracer.End(child)
	tracer.End(parent)

	spans := []*otel.Span{parent, child}
	trace := otel.GetTrace(spans, parent.TraceID)
	if len(trace.Spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(trace.Spans))
	}
}

func TestFormatTrace(t *testing.T) {
	trace := &otel.Trace{
		TraceID: "abc123",
		Spans: []*otel.Span{
			{SpanID: "span1", Name: "parent", TraceID: "abc123"},
		},
	}

	formatted := otel.FormatTrace(trace)
	if formatted == "" {
		t.Error("formatted trace should not be empty")
	}
}
