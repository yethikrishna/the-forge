package aicommit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSmartCommitAnalyzeNoStaged(t *testing.T) {
	sc := NewSmartCommit(DefaultSmartCommitConfig())
	// Outside a git repo or no staged changes — may error or return empty
	summary, err := sc.Analyze()
	if err != nil {
		// Expected in test environment without staged changes
		return
	}
	if summary == nil {
		t.Error("expected non-nil summary")
	}
}

func TestSmartCommitGenerateConventional(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{
		Style:       StyleConventional,
		MaxSubject:  72,
		IncludeBody: true,
	})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "internal/workspace/provision.go", Status: "A", Additions: 150},
		},
		TotalAdded: 150,
		IsNewFiles: true,
		Scope:      "workspace",
	}

	msg := sc.Generate(summary)
	if !strings.HasPrefix(msg.Header, "feat") {
		t.Errorf("expected feat prefix, got %s", msg.Header)
	}
	if !strings.Contains(msg.Header, "provision.go") {
		t.Errorf("expected file name in header, got %s", msg.Header)
	}
	if msg.Full == "" {
		t.Error("expected non-empty full message")
	}
}

func TestSmartCommitGenerateTestChange(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{Style: StyleConventional})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "internal/workspace/provision_test.go", Status: "M", Additions: 20, Deletions: 5},
		},
		TotalAdded:   20,
		TotalDeleted:  5,
		IsTestChange: true,
	}

	msg := sc.Generate(summary)
	if !strings.HasPrefix(msg.Header, "test") {
		t.Errorf("expected test prefix, got %s", msg.Header)
	}
}

func TestSmartCommitGenerateDocChange(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{Style: StyleConventional})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "README.md", Status: "M", Additions: 10},
		},
		TotalAdded:   10,
		IsDocChange:  true,
	}

	msg := sc.Generate(summary)
	if !strings.HasPrefix(msg.Header, "docs") {
		t.Errorf("expected docs prefix, got %s", msg.Header)
	}
}

func TestSmartCommitGenerateDescriptive(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{Style: StyleDescriptive})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "main.go", Status: "M", Additions: 5, Deletions: 2},
		},
		TotalAdded:    5,
		TotalDeleted:   2,
	}

	msg := sc.Generate(summary)
	if !strings.Contains(msg.Header, "main.go") {
		t.Errorf("expected file name, got %s", msg.Header)
	}
}

func TestSmartCommitGenerateOneLine(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{Style: StyleOneLine})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "main.go", Status: "M", Additions: 5},
		},
		TotalAdded: 5,
	}

	msg := sc.Generate(summary)
	if strings.Contains(msg.Full, "\n\n") {
		t.Error("expected no body in oneline mode")
	}
}

func TestSmartCommitHeaderTruncation(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{
		Style:      StyleOneLine,
		MaxSubject: 20,
	})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "very/long/path/to/a/file/that/exceeds/max/subject/length.go", Status: "A"},
		},
		IsNewFiles: true,
	}

	msg := sc.Generate(summary)
	if len(msg.Header) > 20 {
		t.Errorf("header too long (%d): %s", len(msg.Header), msg.Header)
	}
}

func TestSmartCommitCoAuthor(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{
		Style:       StyleConventional,
		IncludeBody: true,
		CoAuthor:    "Forge Agent <agent@forge.dev>",
	})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{{Path: "main.go", Status: "M", Additions: 1}},
		TotalAdded:   1,
	}

	msg := sc.Generate(summary)
	if !strings.Contains(msg.Full, "Co-authored-by:") {
		t.Error("expected co-author trailer")
	}
	if !strings.Contains(msg.Trailer, "Forge Agent") {
		t.Errorf("expected Forge Agent in trailer, got %s", msg.Trailer)
	}
}

func TestSmartCommitMultipleFiles(t *testing.T) {
	sc := NewSmartCommit(SmartCommitConfig{
		Style:       StyleConventional,
		IncludeBody: true,
	})

	summary := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "internal/workspace/provision.go", Status: "A", Additions: 100},
			{Path: "internal/workspace/template.go", Status: "A", Additions: 80},
			{Path: "internal/workspace/autostart.go", Status: "A", Additions: 60},
		},
		TotalAdded: 240,
		IsNewFiles: true,
		Scope:      "workspace",
	}

	msg := sc.Generate(summary)
	if !strings.Contains(msg.Header, "workspace") {
		t.Errorf("expected scope in header, got %s", msg.Header)
	}
	if !strings.Contains(msg.Body, "3 files") {
		t.Errorf("expected file count in body, got %s", msg.Body)
	}
}

func TestDescribeChanges(t *testing.T) {
	tests := []struct {
		name     string
		summary  *ChangeSummary
		contains string
	}{
		{
			"empty",
			&ChangeSummary{},
			"update project files",
		},
		{
			"single added",
			&ChangeSummary{FilesChanged: []FileChange{{Path: "main.go", Status: "A"}}},
			"add main.go",
		},
		{
			"single deleted",
			&ChangeSummary{FilesChanged: []FileChange{{Path: "old.go", Status: "D"}}},
			"remove old.go",
		},
		{
			"single modified",
			&ChangeSummary{FilesChanged: []FileChange{{Path: "main.go", Status: "M"}}},
			"update main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := describeChanges(tt.summary)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected %q in %q", tt.contains, result)
			}
		})
	}
}

func TestInferScope(t *testing.T) {
	tests := []struct {
		files    []FileChange
		expected string
	}{
		{[]FileChange{{Path: "internal/workspace/a.go"}, {Path: "internal/workspace/b.go"}}, "workspace"},
		{[]FileChange{{Path: "cmd/main.go"}}, ""},
		{[]FileChange{{Path: "docs/readme.md"}, {Path: "docs/guide.md"}}, "docs"},
		{nil, ""},
	}

	for _, tt := range tests {
		got := inferScope(tt.files)
		if got != tt.expected {
			t.Errorf("inferScope(%v) = %q, want %q", tt.files, got, tt.expected)
		}
	}
}

func TestChangeSummarySerialization(t *testing.T) {
	s := &ChangeSummary{
		FilesChanged: []FileChange{
			{Path: "main.go", Status: "A", Additions: 100},
		},
		TotalAdded:   100,
		IsNewFiles:   true,
		IsTestChange: false,
		Scope:        "core",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var s2 ChangeSummary
	json.Unmarshal(data, &s2)
	if s2.TotalAdded != 100 {
		t.Errorf("expected 100, got %d", s2.TotalAdded)
	}
	if s2.Scope != "core" {
		t.Errorf("expected core, got %s", s2.Scope)
	}
}

func TestCommitMessageSerialization(t *testing.T) {
	msg := CommitMessage{
		Header:  "feat: add smart commits",
		Body:    "Detailed description",
		Trailer: "Co-authored-by: Agent",
		Full:    "feat: add smart commits\n\nDetailed description\n\nCo-authored-by: Agent",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var msg2 CommitMessage
	json.Unmarshal(data, &msg2)
	if msg2.Header != "feat: add smart commits" {
		t.Errorf("unexpected header: %s", msg2.Header)
	}
}

func TestDefaultSmartCommitConfig(t *testing.T) {
	cfg := DefaultSmartCommitConfig()
	if cfg.Style != StyleConventional {
		t.Errorf("expected conventional style")
	}
	if cfg.MaxSubject != 72 {
		t.Errorf("expected 72 max subject")
	}
	if !cfg.IncludeBody {
		t.Error("expected include body")
	}
}
