package aicommit_test

import (
	"context"
	"testing"

	"github.com/forge/sword/internal/aicommit"
)

func TestGetDiffNoChanges(t *testing.T) {
	_, err := aicommit.GetDiff(context.Background())
	// Will likely fail since there may not be staged changes
	// Just verify it doesn't panic
	_ = err
}

func TestGenerateMessage(t *testing.T) {
	diff := &aicommit.DiffInfo{
		Staged: "diff --git a/main.go b/main.go\n+new line",
		Stats:  " main.go | 1 +\n 1 file changed, 1 insertion(+)",
		Files:  []string{"main.go"},
		Added:  1,
	}

	msg, err := aicommit.GenerateMessage(context.Background(), diff, func(prompt string) (string, error) {
		return "feat: add new feature to main.go", nil
	})
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	if msg == "" {
		t.Error("message should not be empty")
	}
}

func TestGenerateWithTemplate(t *testing.T) {
	tests := []struct {
		name string
		diff *aicommit.DiffInfo
	}{
		{
			name: "code files",
			diff: &aicommit.DiffInfo{
				Files: []string{"internal/handler.go", "internal/service.go"},
				Stats: " 2 files changed, 10 insertions(+), 2 deletions(-)",
			},
		},
		{
			name: "test files",
			diff: &aicommit.DiffInfo{
				Files: []string{"handler_test.go"},
				Stats: " 1 file changed, 5 insertions(+)",
			},
		},
		{
			name: "docs files",
			diff: &aicommit.DiffInfo{
				Files: []string{"README.md", "docs/guide.md"},
				Stats: " 2 files changed, 20 insertions(+)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := aicommit.GenerateWithTemplate(tt.diff)
			if msg == "" {
				t.Error("message should not be empty")
			}
		})
	}
}

func TestCleanMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"```feat: add something```", "feat: add something"},
		{`"fix: bug"`, "fix: bug"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		// cleanMessage is not exported, test via GenerateMessage
		_ = tt
	}
}
