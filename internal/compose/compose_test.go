package compose

import (
	"strings"
	"testing"
)

func TestPresetPostgres(t *testing.T) {
	env := Preset("postgres")
	if env.Name != "postgres" {
		t.Errorf("expected postgres, got %s", env.Name)
	}
	if _, ok := env.Services["postgres"]; !ok {
		t.Error("expected postgres service")
	}
	if env.Services["postgres"].Image != "postgres:16-alpine" {
		t.Errorf("unexpected image: %s", env.Services["postgres"].Image)
	}
}

func TestPresetRedis(t *testing.T) {
	env := Preset("redis")
	if _, ok := env.Services["redis"]; !ok {
		t.Error("expected redis service")
	}
}

func TestPresetMySQL(t *testing.T) {
	env := Preset("mysql")
	if _, ok := env.Services["mysql"]; !ok {
		t.Error("expected mysql service")
	}
}

func TestPresetFullstack(t *testing.T) {
	env := Preset("fullstack")
	if len(env.Services) != 3 {
		t.Errorf("expected 3 services, got %d", len(env.Services))
	}
	if _, ok := env.Services["postgres"]; !ok {
		t.Error("expected postgres in fullstack")
	}
	if _, ok := env.Services["redis"]; !ok {
		t.Error("expected redis in fullstack")
	}
	if _, ok := env.Services["minio"]; !ok {
		t.Error("expected minio in fullstack")
	}
}

func TestPresetUnknown(t *testing.T) {
	env := Preset("custom")
	if env.Name != "custom" {
		t.Errorf("expected custom, got %s", env.Name)
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	services := map[string]*Service{
		"web": {
			Name:  "web",
			Image: "nginx:alpine",
			Ports: []string{"80:80"},
		},
	}

	env, err := mgr.Create("test", services)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if env.Name != "test" {
		t.Errorf("expected test, got %s", env.Name)
	}
	if env.Status != "created" {
		t.Errorf("expected created, got %s", env.Status)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	created, _ := mgr.Create("test", Preset("postgres").Services)
	found, err := mgr.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Name != "test" {
		t.Errorf("expected test, got %s", found.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent environment")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	mgr.Create("env1", Preset("postgres").Services)
	mgr.Create("env2", Preset("redis").Services)

	envs, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(envs) != 2 {
		t.Errorf("expected 2 environments, got %d", len(envs))
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	env, _ := mgr.Create("test", Preset("redis").Services)
	if err := mgr.Remove(env.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := mgr.Get(env.ID); err == nil {
		t.Error("expected error after removal")
	}
}

func TestGenerateComposeYAML(t *testing.T) {
	env := Preset("fullstack")
	yaml := GenerateComposeYAML(env)

	if !strings.Contains(yaml, "version:") {
		t.Error("expected version in YAML")
	}
	if !strings.Contains(yaml, "postgres:16-alpine") {
		t.Error("expected postgres image in YAML")
	}
	if !strings.Contains(yaml, "redis:7-alpine") {
		t.Error("expected redis image in YAML")
	}
	if !strings.Contains(yaml, "ports:") {
		t.Error("expected ports in YAML")
	}
}

func TestGenerateComposeYAMLSingle(t *testing.T) {
	env := Preset("postgres")
	yaml := GenerateComposeYAML(env)

	if !strings.Contains(yaml, "postgres:") {
		t.Error("expected postgres service in YAML")
	}
	if !strings.Contains(yaml, "5432:5432") {
		t.Error("expected port mapping in YAML")
	}
}

func TestFormatEnvironment(t *testing.T) {
	env := Preset("fullstack")
	output := FormatEnvironment(env)

	if !strings.Contains(output, "fullstack") {
		t.Error("expected name in output")
	}
	if !strings.Contains(output, "postgres") {
		t.Error("expected postgres in output")
	}
}

func TestHealthCheck(t *testing.T) {
	env := Preset("postgres")
	svc := env.Services["postgres"]
	if svc.HealthCheck == nil {
		t.Fatal("expected health check on postgres")
	}
	if svc.HealthCheck.Interval != "5s" {
		t.Errorf("expected 5s interval, got %s", svc.HealthCheck.Interval)
	}
	if svc.HealthCheck.Retries != 5 {
		t.Errorf("expected 5 retries, got %d", svc.HealthCheck.Retries)
	}
}
