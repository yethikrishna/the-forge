package hat_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/forge/sword/internal/hat"
)

func TestGetRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	resp := hat.Get("/test").MustSend(handler)
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

	resp := hat.Post("/test").JSON(map[string]string{"name": "forge"}).MustSend(handler)
	resp.AssertStatus(t, http.StatusCreated)
}

func TestQueryParams(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "sword" {
			t.Errorf("expected q=sword, got %s", r.URL.Query().Get("q"))
		}
		w.WriteHeader(http.StatusOK)
	})

	resp := hat.Get("/search").Query("q", "sword").MustSend(handler)
	resp.AssertStatus(t, http.StatusOK)
}

func TestAssertJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"forge","version":"3.0"}`))
	})

	resp := hat.Get("/info").MustSend(handler)
	var result map[string]string
	resp.AssertJSON(t, &result)
	if result["name"] != "forge" {
		t.Errorf("expected name=forge, got %v", result)
	}
}

func TestRequestBuilder(t *testing.T) {
	req, err := hat.Put("/resource/1").
		Header("Authorization", "Bearer token").
		JSON(map[string]string{"action": "update"}).
		Build()
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
