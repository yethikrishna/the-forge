// Package contract provides API contract testing with agents.
// Generate contract tests from OpenAPI/gRPC specs, run them
// against running services, detect breaking changes.
//
// Contract testing is tedious and critical. Perfect agent task.
package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SpecFormat represents the type of API specification.
type SpecFormat string

const (
	SpecOpenAPI2 SpecFormat = "openapi2" // Swagger
	SpecOpenAPI3 SpecFormat = "openapi3"
	SpecGraphQL  SpecFormat = "graphql"
	SpecGRPC     SpecFormat = "grpc"
	SpecAsyncAPI SpecFormat = "asyncapi"
)

// Endpoint represents a single API endpoint.
type Endpoint struct {
	Method      string            `json:"method"`       // GET, POST, PUT, DELETE, PATCH
	Path        string            `json:"path"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Parameters  []Parameter       `json:"parameters,omitempty"`
	RequestBody *RequestBody      `json:"request_body,omitempty"`
	Responses   map[int]Response  `json:"responses,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Deprecated  bool              `json:"deprecated,omitempty"`
	Security    []map[string][]string `json:"security,omitempty"`
}

// Parameter describes an API parameter.
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // query, path, header, cookie
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Example     string `json:"example,omitempty"`
}

// RequestBody describes a request body.
type RequestBody struct {
	ContentType string `json:"content_type"`
	Required    bool   `json:"required"`
	Schema      string `json:"schema,omitempty"`
	Example     string `json:"example,omitempty"`
}

// Response describes an API response.
type Response struct {
	Description string `json:"description"`
	ContentType string `json:"content_type,omitempty"`
	Schema      string `json:"schema,omitempty"`
	Example     string `json:"example,omitempty"`
}

// ContractTest represents a generated contract test.
type ContractTest struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Endpoint   Endpoint  `json:"endpoint"`
	Code       string    `json:"code"`
	Language   string    `json:"language"`
	Framework  string    `json:"framework"` // testing framework
	CreatedAt  time.Time `json:"created_at"`
}

// TestResult holds the result of running a contract test.
type TestResult struct {
	TestID     string    `json:"test_id"`
	Endpoint   string    `json:"endpoint"`
	Passed     bool      `json:"passed"`
	StatusCode int       `json:"status_code"`
	Expected   int       `json:"expected"`
	Duration   string    `json:"duration"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Body       string    `json:"body,omitempty"`
}

// BreakingChange represents a detected breaking change.
type BreakingChange struct {
	Type        string `json:"type"` // removed_endpoint, changed_type, removed_field, etc.
	Severity    string `json:"severity"` // breaking, warning, info
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
}

// SpecDiff holds the result of comparing two API specs.
type SpecDiff struct {
	AddedEndpoints    []Endpoint      `json:"added_endpoints,omitempty"`
	RemovedEndpoints  []Endpoint      `json:"removed_endpoints,omitempty"`
	ModifiedEndpoints []Endpoint      `json:"modified_endpoints,omitempty"`
	BreakingChanges   []BreakingChange `json:"breaking_changes,omitempty"`
	Summary           string          `json:"summary"`
}

// Spec represents a parsed API specification.
type Spec struct {
	Format     SpecFormat `json:"format"`
	Title      string     `json:"title"`
	Version    string     `json:"version"`
	Endpoints  []Endpoint `json:"endpoints"`
	SourcePath string     `json:"source_path,omitempty"`
}

// Generator creates contract tests from API specs.
type Generator struct {
	WorkDir    string
	Language   string
	Framework  string
	BaseURL    string
}

// NewGenerator creates a contract test generator.
func NewGenerator(workDir string) *Generator {
	return &Generator{
		WorkDir:   workDir,
		Language:  "go",
		Framework: "testing",
	}
}

// ParseOpenAPI parses an OpenAPI/Swagger spec file.
func (g *Generator) ParseOpenAPI(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	spec := &Spec{
		SourcePath: path,
		Format:     SpecOpenAPI3,
	}

	// Extract info
	if info, ok := raw["info"].(map[string]interface{}); ok {
		spec.Title, _ = info["title"].(string)
		spec.Version, _ = info["version"].(string)
	}

	// Detect version
	if swagger, ok := raw["swagger"].(string); ok && strings.HasPrefix(swagger, "2") {
		spec.Format = SpecOpenAPI2
	}

	// Extract paths
	if paths, ok := raw["paths"].(map[string]interface{}); ok {
		for path, methods := range paths {
			methodsMap, ok := methods.(map[string]interface{})
			if !ok {
				continue
			}

			for method, details := range methodsMap {
				if method == "parameters" || method == "summary" {
					continue
				}

				method = strings.ToUpper(method)
				if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" && method != "HEAD" && method != "OPTIONS" {
					continue
				}

				endpoint := Endpoint{
					Method: method,
					Path:   path,
				}

				if detailsMap, ok := details.(map[string]interface{}); ok {
					endpoint.Summary, _ = detailsMap["summary"].(string)
					endpoint.Description, _ = detailsMap["description"].(string)
					endpoint.Deprecated, _ = detailsMap["deprecated"].(bool)

					// Parse parameters
					if params, ok := detailsMap["parameters"].([]interface{}); ok {
						for _, p := range params {
							if paramMap, ok := p.(map[string]interface{}); ok {
								param := Parameter{
									Name:     fmt.Sprintf("%v", paramMap["name"]),
									In:       fmt.Sprintf("%v", paramMap["in"]),
									Required: paramMap["required"] == true,
									Type:     fmt.Sprintf("%v", paramMap["type"]),
								}
								param.Description, _ = paramMap["description"].(string)
								param.Default = fmt.Sprintf("%v", paramMap["default"])
								endpoint.Parameters = append(endpoint.Parameters, param)
							}
						}
					}

					// Parse responses
					if responses, ok := detailsMap["responses"].(map[string]interface{}); ok {
						endpoint.Responses = make(map[int]Response)
						for code, resp := range responses {
							var statusCode int
							fmt.Sscanf(code, "%d", &statusCode)
							if statusCode > 0 {
								respMap, _ := resp.(map[string]interface{})
								r := Response{
									Description: fmt.Sprintf("%v", respMap["description"]),
								}
								endpoint.Responses[statusCode] = r
							}
						}
					}

					// Parse tags
					if tags, ok := detailsMap["tags"].([]interface{}); ok {
						for _, tag := range tags {
							if t, ok := tag.(string); ok {
								endpoint.Tags = append(endpoint.Tags, t)
							}
						}
					}
				}

				spec.Endpoints = append(spec.Endpoints, endpoint)
			}
		}
	}

	sort.Slice(spec.Endpoints, func(i, k int) bool {
		if spec.Endpoints[i].Path == spec.Endpoints[k].Path {
			return spec.Endpoints[i].Method < spec.Endpoints[k].Method
		}
		return spec.Endpoints[i].Path < spec.Endpoints[k].Path
	})

	return spec, nil
}

// GenerateTests creates contract tests for all endpoints in a spec.
func (g *Generator) GenerateTests(spec *Spec) ([]ContractTest, error) {
	var tests []ContractTest

	for _, ep := range spec.Endpoints {
		if ep.Deprecated {
			continue
		}

		test := g.generateTest(ep)
		tests = append(tests, test)
	}

	return tests, nil
}

// DiffSpecs compares two API specs and finds breaking changes.
func (g *Generator) DiffSpecs(oldSpec, newSpec *Spec) *SpecDiff {
	diff := &SpecDiff{}

	// Build endpoint maps
	oldMap := make(map[string]Endpoint)
	newMap := make(map[string]Endpoint)

	for _, ep := range oldSpec.Endpoints {
		key := ep.Method + " " + ep.Path
		oldMap[key] = ep
	}
	for _, ep := range newSpec.Endpoints {
		key := ep.Method + " " + ep.Path
		newMap[key] = ep
	}

	// Find removed endpoints (breaking)
	for key, ep := range oldMap {
		if _, exists := newMap[key]; !exists {
			diff.RemovedEndpoints = append(diff.RemovedEndpoints, ep)
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_endpoint",
				Severity:    "breaking",
				Endpoint:    key,
				Description: fmt.Sprintf("Endpoint %s was removed", key),
				OldValue:    key,
			})
		}
	}

	// Find added endpoints (non-breaking)
	for key, ep := range newMap {
		if _, exists := oldMap[key]; !exists {
			diff.AddedEndpoints = append(diff.AddedEndpoints, ep)
		}
	}

	// Find modified endpoints
	for key, newEp := range newMap {
		oldEp, exists := oldMap[key]
		if !exists {
			continue
		}

		// Check for breaking changes in parameters
		g.diffParameters(key, oldEp, newEp, diff)
		g.diffResponses(key, oldEp, newEp, diff)
	}

	// Build summary
	totalBreaking := len(diff.BreakingChanges)
	diff.Summary = fmt.Sprintf("%d added, %d removed, %d modified, %d breaking change(s)",
		len(diff.AddedEndpoints), len(diff.RemovedEndpoints), len(diff.ModifiedEndpoints), totalBreaking)

	return diff
}

func (g *Generator) diffParameters(key string, oldEp, newEp Endpoint, diff *SpecDiff) {
	oldParams := make(map[string]Parameter)
	for _, p := range oldEp.Parameters {
		oldParams[p.Name] = p
	}

	for _, newP := range newEp.Parameters {
		if newP.Required {
			oldP, exists := oldParams[newP.Name]
			if !exists {
				// New required parameter = breaking change
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "new_required_parameter",
					Severity:    "breaking",
					Endpoint:    key,
					Description: fmt.Sprintf("New required parameter '%s' added", newP.Name),
					NewValue:    newP.Name,
				})
			} else if oldP.Type != newP.Type {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_parameter_type",
					Severity:    "breaking",
					Endpoint:    key,
					Description: fmt.Sprintf("Parameter '%s' type changed from %s to %s", newP.Name, oldP.Type, newP.Type),
					OldValue:    oldP.Type,
					NewValue:    newP.Type,
				})
			}
		}
	}

	// Check for removed parameters (warning, not breaking)
	newParams := make(map[string]Parameter)
	for _, p := range newEp.Parameters {
		newParams[p.Name] = p
	}
	for _, oldP := range oldEp.Parameters {
		if _, exists := newParams[oldP.Name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_parameter",
				Severity:    "warning",
				Endpoint:    key,
				Description: fmt.Sprintf("Parameter '%s' was removed", oldP.Name),
				OldValue:    oldP.Name,
			})
		}
	}
}

func (g *Generator) diffResponses(key string, oldEp, newEp Endpoint, diff *SpecDiff) {
	// Check for removed status codes
	for code := range oldEp.Responses {
		if _, exists := newEp.Responses[code]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_response_code",
				Severity:    "warning",
				Endpoint:    key,
				Description: fmt.Sprintf("Response code %d was removed", code),
				OldValue:    fmt.Sprintf("%d", code),
			})
		}
	}
}

func (g *Generator) generateTest(ep Endpoint) ContractTest {
	testName := fmt.Sprintf("Test%s%s",
		strings.Title(strings.ToLower(ep.Method)),
		sanitizeTestName(ep.Path))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", testName))
	sb.WriteString(fmt.Sprintf("\t// %s %s — %s\n", ep.Method, ep.Path, ep.Summary))
	sb.WriteString(fmt.Sprintf("\tbaseURL := os.Getenv(\"API_BASE_URL\")\n"))
	sb.WriteString(fmt.Sprintf("\tif baseURL == \"\" {\n\t\tbaseURL = \"%s\"\n\t}\n", g.BaseURL))

	// Build URL with path parameters
	path := ep.Path
	for _, p := range ep.Parameters {
		if p.In == "path" {
			path = strings.ReplaceAll(path, "{"+p.Name+"}", fmt.Sprintf("\" + %s + \"", p.Name))
		}
	}

	sb.WriteString(fmt.Sprintf("\turl := baseURL + \"%s\"\n", path))

	// Add parameter setup
	for _, p := range ep.Parameters {
		if p.In == "path" {
			defaultVal := p.Default
			if defaultVal == "" {
				defaultVal = "test-" + p.Name
			}
			sb.WriteString(fmt.Sprintf("\t%s := \"%s\"\n", p.Name, defaultVal))
		}
	}

	// Create request
	sb.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(\"%s\", url, nil)\n", ep.Method))
	sb.WriteString("\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\n")

	// Add headers
	for _, p := range ep.Parameters {
		if p.In == "header" {
			defaultVal := p.Default
			if defaultVal == "" {
				defaultVal = "test-value"
			}
			sb.WriteString(fmt.Sprintf("\treq.Header.Set(\"%s\", \"%s\")\n", p.Name, defaultVal))
		}
	}

	// Execute request
	sb.WriteString("\tclient := &http.Client{Timeout: 10 * time.Second}\n")
	sb.WriteString("\tresp, err := client.Do(req)\n")
	sb.WriteString("\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n")
	sb.WriteString("\tdefer resp.Body.Close()\n\n")

	// Check status code
	expectedCode := 200
	for code := range ep.Responses {
		if code >= 200 && code < 300 {
			expectedCode = code
			break
		}
	}

	sb.WriteString(fmt.Sprintf("\tif resp.StatusCode != %d {\n", expectedCode))
	sb.WriteString(fmt.Sprintf("\t\tt.Errorf(\"expected status %d, got %%d\", resp.StatusCode)\n", expectedCode))
	sb.WriteString("\t}\n")

	sb.WriteString("}\n")

	return ContractTest{
		ID:        fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Name:      testName,
		Endpoint:  ep,
		Code:      sb.String(),
		Language:  g.Language,
		Framework: g.Framework,
		CreatedAt: time.Now(),
	}
}

func sanitizeTestName(path string) string {
	parts := strings.Split(path, "/")
	var name string
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Remove path parameter markers
		p = strings.Trim(p, "{}")
		p = strings.ReplaceAll(p, "-", "")
		p = strings.ReplaceAll(p, "_", "")
		if p != "" {
			name += strings.Title(strings.ToLower(p))
		}
	}
	return name
}

// SaveTests writes generated tests to files.
func SaveTests(tests []ContractTest, dir string) error {
	os.MkdirAll(dir, 0o755)

	var sb strings.Builder
	sb.WriteString("package contract_test\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"net/http\"\n")
	sb.WriteString("\t\"os\"\n")
	sb.WriteString("\t\"testing\"\n")
	sb.WriteString("\t\"time\"\n")
	sb.WriteString(")\n\n")

	for _, test := range tests {
		sb.WriteString(test.Code)
		sb.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(dir, "contract_test.go"), []byte(sb.String()), 0o644)
}

// FormatDiff renders a spec diff for display.
func FormatDiff(diff *SpecDiff) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("API Diff: %s\n\n", diff.Summary))

	if len(diff.BreakingChanges) > 0 {
		sb.WriteString("Breaking Changes:\n")
		for _, bc := range diff.BreakingChanges {
			icon := "⚠"
			if bc.Severity == "breaking" {
				icon = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] %s: %s\n", icon, bc.Severity, bc.Endpoint, bc.Description))
		}
		sb.WriteString("\n")
	}

	if len(diff.AddedEndpoints) > 0 {
		sb.WriteString("Added:\n")
		for _, ep := range diff.AddedEndpoints {
			sb.WriteString(fmt.Sprintf("  + %s %s\n", ep.Method, ep.Path))
		}
		sb.WriteString("\n")
	}

	if len(diff.RemovedEndpoints) > 0 {
		sb.WriteString("Removed:\n")
		for _, ep := range diff.RemovedEndpoints {
			sb.WriteString(fmt.Sprintf("  - %s %s\n", ep.Method, ep.Path))
		}
	}

	return sb.String()
}
