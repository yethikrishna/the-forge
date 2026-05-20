// Package hat provides HTTP API testing helpers for building and
// verifying HTTP requests in tests. Test your APIs with style.
package hat

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
type RequestBuilder struct {
	method  string
	path    string
	headers map[string]string
	query   url.Values
	body    any
}

// New creates a new RequestBuilder.
func New(method, path string) *RequestBuilder {
	return &RequestBuilder{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		query:   make(url.Values),
	}
}

// Get creates a GET request builder.
func Get(path string) *RequestBuilder {
	return New(http.MethodGet, path)
}

// Post creates a POST request builder.
func Post(path string) *RequestBuilder {
	return New(http.MethodPost, path)
}

// Put creates a PUT request builder.
func Put(path string) *RequestBuilder {
	return New(http.MethodPut, path)
}

// Delete creates a DELETE request builder.
func Delete(path string) *RequestBuilder {
	return New(http.MethodDelete, path)
}

// Patch creates a PATCH request builder.
func Patch(path string) *RequestBuilder {
	return New(http.MethodPatch, path)
}

// Header adds a header to the request.
func (rb *RequestBuilder) Header(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// Query adds a query parameter.
func (rb *RequestBuilder) Query(key, value string) *RequestBuilder {
	rb.query.Add(key, value)
	return rb
}

// JSON sets the request body as JSON and adds Content-Type header.
func (rb *RequestBuilder) JSON(body any) *RequestBuilder {
	rb.body = body
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// Body sets a raw string body.
func (rb *RequestBuilder) Body(body string) *RequestBuilder {
	rb.body = body
	return rb
}

// Build constructs the http.Request.
func (rb *RequestBuilder) Build() (*http.Request, error) {
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
				return nil, fmt.Errorf("hat: marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequest(rb.method, path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("hat: build request: %w", err)
	}

	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// Response wraps an httptest.ResponseRecorder for assertions.
type Response struct {
	*httptest.ResponseRecorder
}

// Send builds the request and sends it to the handler.
func (rb *RequestBuilder) Send(handler http.Handler) (*Response, error) {
	req, err := rb.Build()
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return &Response{ResponseRecorder: rec}, nil
}

// MustSend sends the request or panics on error.
func (rb *RequestBuilder) MustSend(handler http.Handler) *Response {
	resp, err := rb.Send(handler)
	if err != nil {
		panic(err)
	}
	return resp
}

// AssertStatus checks the response status code.
func (r *Response) AssertStatus(t tester, expected int) {
	t.Helper()
	if r.Code != expected {
		t.Errorf("expected status %d, got %d", expected, r.Code)
	}
}

// AssertHeader checks a response header.
func (r *Response) AssertHeader(t tester, key, expected string) {
	t.Helper()
	got := r.Header().Get(key)
	if got != expected {
		t.Errorf("expected header %s=%q, got %q", key, expected, got)
	}
}

// AssertJSON unmarshals the response body as JSON.
func (r *Response) AssertJSON(t tester, target any) {
	t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), target); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}

// AssertBodyContains checks if the response body contains a substring.
func (r *Response) AssertBodyContains(t tester, substr string) {
	t.Helper()
	if !strings.Contains(r.Body.String(), substr) {
		t.Errorf("response body does not contain %q: %s", substr, r.Body.String())
	}
}

// tester is a minimal interface compatible with *testing.T.
type tester interface {
	Helper()
	Errorf(format string, args ...any)
}

// MustSendRequest sends a pre-built request to a handler.
func MustSendRequest(handler http.Handler, req *http.Request) *Response {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return &Response{ResponseRecorder: rec}
}

// ServeAndSend creates a test server and sends the request.
func ServeAndSend(handler http.Handler, req *http.Request) (*http.Response, error) {
	server := httptest.NewServer(handler)
	defer server.Close()

	req.URL, _ = url.Parse(server.URL + req.URL.Path)
	if req.URL.RawQuery != "" {
		req.URL.RawQuery = req.URL.RawQuery
	}

	return http.DefaultClient.Do(req)
}
