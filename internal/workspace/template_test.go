package workspace

import (
	"testing"
)

func TestTemplateRegistryBuiltins(t *testing.T) {
	r := NewTemplateRegistry()
	templates := r.List()
	if len(templates) < 8 {
		t.Errorf("expected at least 8 built-in templates, got %d", len(templates))
	}
}

func TestTemplateGet(t *testing.T) {
	r := NewTemplateRegistry()

	tmpl, err := r.Get("go")
	if err != nil {
		t.Fatalf("Get go failed: %v", err)
	}
	if tmpl.Image != "golang:1.23" {
		t.Errorf("expected golang:1.23, got %s", tmpl.Image)
	}
	if tmpl.DefaultCPU != 2 {
		t.Errorf("expected CPU 2, got %f", tmpl.DefaultCPU)
	}
}

func TestTemplateGetCaseInsensitive(t *testing.T) {
	r := NewTemplateRegistry()

	tmpl, err := r.Get("Go")
	if err != nil {
		t.Fatalf("Get Go failed: %v", err)
	}
	if tmpl.Name != "go" {
		t.Errorf("expected go, got %s", tmpl.Name)
	}
}

func TestTemplateGetNotFound(t *testing.T) {
	r := NewTemplateRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestTemplateRegister(t *testing.T) {
	r := NewTemplateRegistry()

	err := r.Register(Template{
		Name:       "custom",
		Type:       TemplateDocker,
		Description: "My custom template",
		Image:      "myimage:latest",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	tmpl, _ := r.Get("custom")
	if tmpl.Image != "myimage:latest" {
		t.Errorf("expected myimage:latest, got %s", tmpl.Image)
	}
}

func TestTemplateRegisterDuplicate(t *testing.T) {
	r := NewTemplateRegistry()

	err := r.Register(Template{
		Name:  "go", // already exists
		Type:  TemplateDocker,
		Image: "override:latest",
	})
	if err == nil {
		t.Error("expected error for duplicate template")
	}
}

func TestTemplateListByType(t *testing.T) {
	r := NewTemplateRegistry()

	docker := r.ListByType(TemplateDocker)
	if len(docker) == 0 {
		t.Error("expected at least one Docker template")
	}

	k8s := r.ListByType(TemplateK8s)
	if len(k8s) != 0 {
		t.Errorf("expected 0 K8s templates, got %d", len(k8s))
	}
}

func TestTemplateListByTag(t *testing.T) {
	r := NewTemplateRegistry()

	language := r.ListByTag("language")
	if len(language) < 4 {
		t.Errorf("expected at least 4 language templates, got %d", len(language))
	}

	db := r.ListByTag("database")
	if len(db) < 2 {
		t.Errorf("expected at least 2 database templates, got %d", len(db))
	}
}

func TestResolveToConfig(t *testing.T) {
	r := NewTemplateRegistry()

	cfg, err := r.ResolveToConfig("go", ProvisionConfig{
		Name: "my-go-env",
	})
	if err != nil {
		t.Fatalf("ResolveToConfig failed: %v", err)
	}

	if cfg.Name != "my-go-env" {
		t.Errorf("expected my-go-env, got %s", cfg.Name)
	}
	if cfg.Backend != BackendDocker {
		t.Errorf("expected docker backend, got %s", cfg.Backend)
	}
	if cfg.Image != "golang:1.23" {
		t.Errorf("expected golang:1.23, got %s", cfg.Image)
	}
	if cfg.CPU != 2 {
		t.Errorf("expected CPU 2, got %f", cfg.CPU)
	}
	if cfg.MemoryMB != 4096 {
		t.Errorf("expected 4096 MB, got %d", cfg.MemoryMB)
	}
}

func TestResolveToConfigWithOverrides(t *testing.T) {
	r := NewTemplateRegistry()

	cfg, err := r.ResolveToConfig("python", ProvisionConfig{
		Name:      "big-python",
		CPU:       8,
		MemoryMB:  16384,
		Ports:     []int{9000},
		Env:       map[string]string{"DEBUG": "1"},
	})
	if err != nil {
		t.Fatalf("ResolveToConfig failed: %v", err)
	}

	if cfg.CPU != 8 {
		t.Errorf("expected overridden CPU 8, got %f", cfg.CPU)
	}
	if cfg.MemoryMB != 16384 {
		t.Errorf("expected overridden 16384 MB, got %d", cfg.MemoryMB)
	}
	if len(cfg.Ports) != 1 || cfg.Ports[0] != 9000 {
		t.Errorf("expected overridden port 9000, got %v", cfg.Ports)
	}
	if cfg.Env["DEBUG"] != "1" {
		t.Error("expected DEBUG=1 env override")
	}
}

func TestResolveToConfigNotFound(t *testing.T) {
	r := NewTemplateRegistry()

	_, err := r.ResolveToConfig("nonexistent", ProvisionConfig{})
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}
