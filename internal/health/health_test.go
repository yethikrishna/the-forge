package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	server := NewServer("test-version")

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	server.Healthz().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "alive" {
		t.Errorf("expected alive, got %v", body["status"])
	}
	if body["version"] != "test-version" {
		t.Errorf("expected test-version, got %v", body["version"])
	}
}

func TestReadyzHealthy(t *testing.T) {
	server := NewServer("test-version")
	server.Register("test", AlwaysHealthy("test"))

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Readyz().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != string(StatusHealthy) {
		t.Errorf("expected healthy, got %v", body["status"])
	}
}

func TestReadyzUnhealthy(t *testing.T) {
	server := NewServer("test-version")
	server.Register("failing", func() CheckResult {
		return CheckResult{
			Name:    "failing",
			Status:  StatusUnhealthy,
			Message: "something is broken",
		}
	})

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Readyz().ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestReadyzDegraded(t *testing.T) {
	server := NewServer("test-version")
	server.Register("degraded", func() CheckResult {
		return CheckResult{
			Name:    "degraded",
			Status:  StatusDegraded,
			Message: "slightly broken",
		}
	})

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Readyz().ServeHTTP(w, req)

	// Degraded should still return 200 (service is available)
	if w.Code != 200 {
		t.Errorf("expected 200 for degraded, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != string(StatusDegraded) {
		t.Errorf("expected degraded, got %v", body["status"])
	}
}

func TestLivez(t *testing.T) {
	server := NewServer("test-version")
	server.Register("test", AlwaysHealthy("test"))

	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()

	server.Livez().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLivezUnhealthy(t *testing.T) {
	server := NewServer("test-version")
	server.Register("dead", func() CheckResult {
		return CheckResult{Name: "dead", Status: StatusUnhealthy}
	})

	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()

	server.Livez().ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestRegisterUnregister(t *testing.T) {
	server := NewServer("test-version")
	server.Register("test", AlwaysHealthy("test"))
	server.Unregister("test")

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Readyz().ServeHTTP(w, req)

	// No checkers = healthy
	if w.Code != 200 {
		t.Errorf("expected 200 with no checkers, got %d", w.Code)
	}
}

func TestAPIChecker(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	checker := APIChecker("test-api", ts.URL)
	result := checker()

	if result.Status != StatusHealthy {
		t.Errorf("expected healthy for reachable API, got %s", result.Status)
	}
}

func TestAPICheckerUnreachable(t *testing.T) {
	checker := APIChecker("dead-api", "http://localhost:1")
	result := checker()

	if result.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy for unreachable API, got %s", result.Status)
	}
}

func TestAPIChecker500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	checker := APIChecker("error-api", ts.URL)
	result := checker()

	if result.Status != StatusDegraded {
		t.Errorf("expected degraded for 500 API, got %s", result.Status)
	}
}

func TestAlwaysHealthy(t *testing.T) {
	checker := AlwaysHealthy("test")
	result := checker()

	if result.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", result.Status)
	}
	if result.Name != "test" {
		t.Errorf("expected name test, got %s", result.Name)
	}
}

func TestMultipleCheckers(t *testing.T) {
	server := NewServer("test-version")
	server.Register("ok1", AlwaysHealthy("ok1"))
	server.Register("ok2", AlwaysHealthy("ok2"))
	server.Register("degraded", func() CheckResult {
		return CheckResult{Name: "degraded", Status: StatusDegraded}
	})

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	server.Readyz().ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	checks := body["checks"].([]interface{})
	if len(checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(checks))
	}
}

func TestUptimeInResponse(t *testing.T) {
	server := NewServer("test-version")
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	server.Healthz().ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["uptime"] == nil {
		t.Error("expected uptime in response")
	}
}
