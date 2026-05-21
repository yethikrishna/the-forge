package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/forge/sword/internal/cli"
)

func TestNewSpinner(t *testing.T) {
	s := cli.NewSpinner("loading")
	if s == nil {
		t.Fatal("spinner should not be nil")
	}
}

func TestNewForgeSpinner(t *testing.T) {
	s := cli.NewForgeSpinner("forging")
	if s == nil {
		t.Fatal("spinner should not be nil")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := cli.NewSpinner("test").WithWriter(&buf)
	s.Start()
	s.Stop()
	// Should not panic, output may or may not have content depending on timing
}

func TestStepTracker(t *testing.T) {
	tracker := cli.NewStepTracker([]string{"step1", "step2", "step3"})
	if tracker == nil {
		t.Fatal("tracker should not be nil")
	}
	tracker.Start(0)
	tracker.Done(0)
	tracker.Start(1)
	tracker.Fail(1)
	tracker.Skip(2)
}

func TestStepTrackerOutOfBounds(t *testing.T) {
	tracker := cli.NewStepTracker([]string{"only"})
	// Should not panic
	tracker.Start(-1)
	tracker.Start(5)
	tracker.Done(-1)
	tracker.Done(5)
}

func TestGetRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	resp := cli.GetRequest("/test").MustSendRequest(handler)
	resp.AssertStatus(t, http.StatusOK)
	resp.AssertBodyContains(t, "ok")
}

func TestPostJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content type")
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "forge" {
			t.Errorf("expected name=forge, got %v", body)
		}
		w.WriteHeader(http.StatusCreated)
	})

	resp := cli.PostRequest("/test").JSONBody(map[string]string{"name": "forge"}).MustSendRequest(handler)
	resp.AssertStatus(t, http.StatusCreated)
}

func TestQueryParams(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "sword" {
			t.Errorf("expected q=sword, got %s", r.URL.Query().Get("q"))
		}
		w.WriteHeader(http.StatusOK)
	})

	resp := cli.GetRequest("/search").QueryParam("q", "sword").MustSendRequest(handler)
	resp.AssertStatus(t, http.StatusOK)
}

func TestAssertJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"forge","version":"3.0"}`))
	})

	resp := cli.GetRequest("/info").MustSendRequest(handler)
	var result map[string]string
	resp.AssertJSON(t, &result)
	if result["name"] != "forge" {
		t.Errorf("expected name=forge, got %v", result)
	}
}

func TestBuildRequest(t *testing.T) {
	req, err := cli.PutRequest("/resource/1").
		Header("Authorization", "Bearer token").
		JSONBody(map[string]string{"action": "update"}).
		BuildRequest()
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if req.Method != http.MethodPut {
		t.Errorf("expected PUT, got %s", req.Method)
	}
	if req.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("authorization header not set")
	}
}
