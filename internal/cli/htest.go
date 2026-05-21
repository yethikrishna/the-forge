package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// RequestBuilder constructs HTTP requests for testing.
// It provides a fluent API for building and sending test requests
// to http.Handler implementations.
type RequestBuilder struct {
	method  string
	path    string
	headers map[string]string
	query   url.Values
	body    any
}

// NewHTTPRequest creates a new RequestBuilder.
func NewHTTPRequest(method, path string) *RequestBuilder {
	return &RequestBuilder{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		query:   make(url.Values),
	}
}

// GetRequest creates a GET request builder.
func GetRequest(path string) *RequestBuilder {
	return NewHTTPRequest(http.MethodGet, path)
}

// PostRequest creates a POST request builder.
func PostRequest(path string) *RequestBuilder {
	return NewHTTPRequest(http.MethodPost, path)
}

// PutRequest creates a PUT request builder.
func PutRequest(path string) *RequestBuilder {
	return NewHTTPRequest(http.MethodPut, path)
}

// DeleteRequest creates a DELETE request builder.
func DeleteRequest(path string) *RequestBuilder {
	return NewHTTPRequest(http.MethodDelete, path)
}

// PatchRequest creates a PATCH request builder.
func PatchRequest(path string) *RequestBuilder {
	return NewHTTPRequest(http.MethodPatch, path)
}

// Header adds a header to the request.
func (rb *RequestBuilder) Header(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// QueryParam adds a query parameter.
func (rb *RequestBuilder) QueryParam(key, value string) *RequestBuilder {
	rb.query.Add(key, value)
	return rb
}

// JSONBody sets the request body as JSON and adds Content-Type header.
func (rb *RequestBuilder) JSONBody(body any) *RequestBuilder {
	rb.body = body
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// RawBody sets a raw string body.
func (rb *RequestBuilder) RawBody(body string) *RequestBuilder {
	rb.body = body
	return rb
}

// BuildRequest constructs the http.Request.
func (rb *RequestBuilder) BuildRequest() (*http.Request, error) {
	path := rb.path
	if len(rb.query) > 0 {
		path = path + "?" + rb.query.Encode()
	}

	var bodyReader io.Reader
	if rb.body != nil {
		switch v := rb.body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			data, err := json.Marshal(rb.body)
			if err != nil {
				return nil, fmt.Errorf("cli: marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequest(rb.method, path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("cli: build request: %w", err)
	}

	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// TestResponse wraps an httptest.ResponseRecorder for assertions.
type TestResponse struct {
	*httptest.ResponseRecorder
}

// SendRequest builds the request and sends it to the handler.
func (rb *RequestBuilder) SendRequest(handler http.Handler) (*TestResponse, error) {
	req, err := rb.BuildRequest()
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return &TestResponse{ResponseRecorder: rec}, nil
}

// MustSendRequest sends the request or panics on error.
func (rb *RequestBuilder) MustSendRequest(handler http.Handler) *TestResponse {
	resp, err := rb.SendRequest(handler)
	if err != nil {
		panic(err)
	}
	return resp
}

// AssertStatus checks the response status code.
func (r *TestResponse) AssertStatus(t TestReporter, expected int) {
	t.Helper()
	if r.Code != expected {
		t.Errorf("expected status %d, got %d", expected, r.Code)
	}
}

// AssertHeader checks a response header.
func (r *TestResponse) AssertHeader(t TestReporter, key, expected string) {
	t.Helper()
	got := r.Header().Get(key)
	if got != expected {
		t.Errorf("expected header %s=%q, got %q", key, expected, got)
	}
}

// AssertJSON unmarshals the response body as JSON.
func (r *TestResponse) AssertJSON(t TestReporter, target any) {
	t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), target); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}

// AssertBodyContains checks if the response body contains a substring.
func (r *TestResponse) AssertBodyContains(t TestReporter, substr string) {
	t.Helper()
	if !strings.Contains(r.Body.String(), substr) {
		t.Errorf("response body does not contain %q: %s", substr, r.Body.String())
	}
}

// TestReporter is a minimal interface compatible with *testing.T.
type TestReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// SendHTTPToHandler sends a pre-built request to a handler.
func SendHTTPToHandler(handler http.Handler, req *http.Request) *TestResponse {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return &TestResponse{ResponseRecorder: rec}
}

// ServeAndRequest creates a test server and sends the request.
func ServeAndRequest(handler http.Handler, req *http.Request) (*http.Response, error) {
	server := httptest.NewServer(handler)
	defer server.Close()

	req.URL, _ = url.Parse(server.URL + req.URL.Path)

	return http.DefaultClient.Do(req)
}
