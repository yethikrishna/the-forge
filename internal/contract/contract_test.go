package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOpenAPI3(t *testing.T) {
	spec := `{
		"openapi": "3.0.0",
		"info": {"title": "Test API", "version": "1.0.0"},
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"parameters": [
						{"name": "limit", "in": "query", "required": false, "type": "integer"},
						{"name": "offset", "in": "query", "required": false, "type": "integer"}
					],
					"responses": {
						"200": {"description": "OK"},
						"401": {"description": "Unauthorized"}
					},
					"tags": ["users"]
				},
				"post": {
					"summary": "Create user",
					"responses": {
						"201": {"description": "Created"},
						"400": {"description": "Bad request"}
					}
				}
			},
			"/users/{id}": {
				"get": {
					"summary": "Get user",
					"parameters": [
						{"name": "id", "in": "path", "required": true, "type": "string"}
					],
					"responses": {
						"200": {"description": "OK"},
						"404": {"description": "Not found"}
					}
				},
				"delete": {
					"summary": "Delete user",
					"deprecated": true,
					"responses": {
						"204": {"description": "No content"}
					}
				}
			}
		}
	}`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "api.json")
	os.WriteFile(specPath, []byte(spec), 0o644)

	gen := NewGenerator(tmpDir)
	apiSpec, err := gen.ParseOpenAPI(specPath)
	if err != nil {
		t.Fatalf("ParseOpenAPI failed: %v", err)
	}

	if apiSpec.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", apiSpec.Title)
	}
	if apiSpec.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", apiSpec.Version)
	}
	if apiSpec.Format != SpecOpenAPI3 {
		t.Errorf("expected OpenAPI3, got %s", apiSpec.Format)
	}

	// Should have 3 non-deprecated endpoints (GET /users, POST /users, GET /users/{id})
	nonDeprecated := 0
	for _, ep := range apiSpec.Endpoints {
		if !ep.Deprecated {
			nonDeprecated++
		}
	}
	if nonDeprecated != 3 {
		t.Errorf("expected 3 non-deprecated endpoints, got %d (total: %d)", nonDeprecated, len(apiSpec.Endpoints))
	}
}

func TestParseOpenAPI2(t *testing.T) {
	spec := `{
		"swagger": "2.0",
		"info": {"title": "Swagger API", "version": "2.0.0"},
		"paths": {
			"/health": {
				"get": {
					"summary": "Health check",
					"responses": {
						"200": {"description": "OK"}
					}
				}
			}
		}
	}`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "swagger.json")
	os.WriteFile(specPath, []byte(spec), 0o644)

	gen := NewGenerator(tmpDir)
	apiSpec, err := gen.ParseOpenAPI(specPath)
	if err != nil {
		t.Fatalf("ParseOpenAPI failed: %v", err)
	}

	if apiSpec.Format != SpecOpenAPI2 {
		t.Errorf("expected OpenAPI2, got %s", apiSpec.Format)
	}
}

func TestGenerateTests(t *testing.T) {
	spec := &Spec{
		Title:   "Test API",
		Version: "1.0.0",
		Endpoints: []Endpoint{
			{
				Method:  "GET",
				Path:    "/users",
				Summary: "List users",
				Parameters: []Parameter{
					{Name: "limit", In: "query", Type: "integer"},
				},
				Responses: map[int]Response{
					200: {Description: "OK"},
				},
			},
			{
				Method:  "POST",
				Path:    "/users",
				Summary: "Create user",
				Responses: map[int]Response{
					201: {Description: "Created"},
				},
			},
		},
	}

	gen := NewGenerator(t.TempDir())
	tests, err := gen.GenerateTests(spec)
	if err != nil {
		t.Fatalf("GenerateTests failed: %v", err)
	}

	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(tests))
	}

	for _, test := range tests {
		if test.Code == "" {
			t.Error("test code should not be empty")
		}
		if test.Name == "" {
			t.Error("test name should not be empty")
		}
	}
}

func TestGenerateTestsSkipsDeprecated(t *testing.T) {
	spec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/v1/users", Deprecated: true},
			{Method: "GET", Path: "/v2/users"},
		},
	}

	gen := NewGenerator(t.TempDir())
	tests, _ := gen.GenerateTests(spec)
	if len(tests) != 1 {
		t.Errorf("expected 1 test (skipping deprecated), got %d", len(tests))
	}
}

func TestDiffSpecs(t *testing.T) {
	oldSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/users", Parameters: []Parameter{{Name: "limit", In: "query", Type: "integer"}}},
			{Method: "GET", Path: "/posts"},
			{Method: "DELETE", Path: "/users/{id}"},
		},
	}

	newSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/users", Parameters: []Parameter{{Name: "limit", In: "query", Type: "string"}}},
			{Method: "GET", Path: "/comments"},
			{Method: "GET", Path: "/users/{id}", Parameters: []Parameter{{Name: "id", In: "path", Required: true}}},
		},
	}

	gen := NewGenerator(t.TempDir())
	diff := gen.DiffSpecs(oldSpec, newSpec)

	if len(diff.RemovedEndpoints) != 2 {
		t.Errorf("expected 2 removed endpoints, got %d", len(diff.RemovedEndpoints))
	}
	if len(diff.AddedEndpoints) != 2 {
		t.Errorf("expected 2 added endpoints, got %d", len(diff.AddedEndpoints))
	}
	if len(diff.BreakingChanges) == 0 {
		t.Error("expected breaking changes")
	}
}

func TestDiffSpecsIdentical(t *testing.T) {
	spec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/users"},
		},
	}

	gen := NewGenerator(t.TempDir())
	diff := gen.DiffSpecs(spec, spec)

	if len(diff.BreakingChanges) != 0 {
		t.Errorf("expected 0 breaking changes for identical specs, got %d", len(diff.BreakingChanges))
	}
}

func TestSanitizeTestName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/users", "Users"},
		{"/users/{id}", "UsersId"},
		{"/api/v1/health", "ApiV1Health"},
	}

	for _, tt := range tests {
		result := sanitizeTestName(tt.path)
		if result != tt.expected {
			t.Errorf("sanitizeTestName(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestSaveTests(t *testing.T) {
	tmpDir := t.TempDir()
	tests := []ContractTest{
		{
			Name:     "TestGetUsers",
			Endpoint: Endpoint{Method: "GET", Path: "/users"},
			Code:     "func TestGetUsers(t *testing.T) {\n\t// test\n}\n",
		},
	}

	err := SaveTests(tests, tmpDir)
	if err != nil {
		t.Fatalf("SaveTests failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "contract_test.go")); os.IsNotExist(err) {
		t.Error("contract_test.go should exist")
	}
}

func TestFormatDiff(t *testing.T) {
	diff := &SpecDiff{
		AddedEndpoints:   []Endpoint{{Method: "GET", Path: "/new"}},
		RemovedEndpoints: []Endpoint{{Method: "POST", Path: "/old"}},
		BreakingChanges: []BreakingChange{
			{Type: "removed_endpoint", Severity: "breaking", Endpoint: "POST /old", Description: "Endpoint removed"},
		},
		Summary: "1 added, 1 removed, 0 modified, 1 breaking change(s)",
	}

	output := FormatDiff(diff)
	if !strings.Contains(output, "Breaking") {
		t.Error("expected breaking changes section")
	}
	if !strings.Contains(output, "POST /old") {
		t.Error("expected removed endpoint")
	}
}

func TestContractTestSerialization(t *testing.T) {
	test := ContractTest{
		ID:        "test-123",
		Name:      "TestGetUsers",
		Endpoint:  Endpoint{Method: "GET", Path: "/users", Summary: "List users"},
		Code:      "func TestGetUsers(t *testing.T) {}",
		Language:  "go",
		Framework: "testing",
	}

	data, err := json.Marshal(test)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var test2 ContractTest
	if err := json.Unmarshal(data, &test2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if test2.Name != "TestGetUsers" {
		t.Errorf("expected name TestGetUsers, got %s", test2.Name)
	}
}

func TestNewRequiredParameterBreaking(t *testing.T) {
	oldSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/users"},
		},
	}

	newSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/users", Parameters: []Parameter{
				{Name: "email", In: "query", Required: true},
			}},
		},
	}

	gen := NewGenerator(t.TempDir())
	diff := gen.DiffSpecs(oldSpec, newSpec)

	found := false
	for _, bc := range diff.BreakingChanges {
		if bc.Type == "new_required_parameter" {
			found = true
		}
	}
	if !found {
		t.Error("expected new_required_parameter breaking change")
	}
}

func TestParameterTypeChangeBreaking(t *testing.T) {
	oldSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/search", Parameters: []Parameter{
				{Name: "q", In: "query", Required: true, Type: "string"},
			}},
		},
	}

	newSpec := &Spec{
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/search", Parameters: []Parameter{
				{Name: "q", In: "query", Required: true, Type: "integer"},
			}},
		},
	}

	gen := NewGenerator(t.TempDir())
	diff := gen.DiffSpecs(oldSpec, newSpec)

	found := false
	for _, bc := range diff.BreakingChanges {
		if bc.Type == "changed_parameter_type" {
			found = true
		}
	}
	if !found {
		t.Error("expected changed_parameter_type breaking change")
	}
}
