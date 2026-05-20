package envbuilder_test

import (
	"testing"

	"github.com/forge/sword/internal/envbuilder"
)

func TestNewBuilder(t *testing.T) {
	b := envbuilder.NewBuilder()
	if b == nil {
		t.Fatal("builder should not be nil")
	}
}

func TestBuildWithoutDocker(t *testing.T) {
	b := envbuilder.NewBuilder()
	if !b.DockerAvailable() {
		_, err := b.Build(nil, envbuilder.BuildConfig{Name: "test", Image: "alpine"})
		if err == nil {
			t.Error("should error without Docker")
		}
	}
}

func TestListEmpty(t *testing.T) {
	b := envbuilder.NewBuilder()
	list := b.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestGetNonExistent(t *testing.T) {
	b := envbuilder.NewBuilder()
	_, err := b.Get("nonexistent")
	if err == nil {
		t.Error("should error for non-existent environment")
	}
}

func TestGenerateDockerfile(t *testing.T) {
	languages := []string{"go", "python", "node", "rust"}
	for _, lang := range languages {
		df, err := envbuilder.GenerateDockerfile(lang)
		if err != nil {
			t.Errorf("GenerateDockerfile(%s) error: %v", lang, err)
		}
		if df == "" {
			t.Errorf("GenerateDockerfile(%s) returned empty", lang)
		}
	}
}

func TestGenerateDockerfileUnsupported(t *testing.T) {
	_, err := envbuilder.GenerateDockerfile("cobol")
	if err == nil {
		t.Error("should error for unsupported language")
	}
}
