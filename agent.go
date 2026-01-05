package rue

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// Agent is a test client for making requests to the engine
type Agent struct {
	engine  *Engine
	headers http.Header
	cookies []*http.Cookie
}

// NewAgent creates a new Agent for testing
func NewAgent(engine *Engine) *Agent {
	return &Agent{
		engine:  engine,
		headers: make(http.Header),
	}
}

// SetHeader sets a header for all requests
func (a *Agent) SetHeader(key, value string) *Agent {
	a.headers.Set(key, value)
	return a
}

// AddHeader adds a header value
func (a *Agent) AddHeader(key, value string) *Agent {
	a.headers.Add(key, value)
	return a
}

// SetCookie sets a cookie for all requests
func (a *Agent) SetCookie(cookie *http.Cookie) *Agent {
	a.cookies = append(a.cookies, cookie)
	return a
}

// ClearHeaders clears all headers
func (a *Agent) ClearHeaders() *Agent {
	a.headers = make(http.Header)
	return a
}

// ClearCookies clears all cookies
func (a *Agent) ClearCookies() *Agent {
	a.cookies = nil
	return a
}

// AgentResponse wraps the response from an agent request
type AgentResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Cookies    []*http.Cookie
}

// String returns the body as a string
func (r *AgentResponse) String() string {
	return string(r.Body)
}

// JSON unmarshals the body into the given object
func (r *AgentResponse) JSON(obj any) error {
	return json.Unmarshal(r.Body, obj)
}

// Do performs a request with the given method, path, and body
func (a *Agent) Do(method, path string, body io.Reader) *AgentResponse {
	req := httptest.NewRequest(method, path, body)

	// Copy headers
	for key, values := range a.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Add cookies
	for _, cookie := range a.cookies {
		req.AddCookie(cookie)
	}

	// Execute request
	w := httptest.NewRecorder()
	a.engine.ServeHTTP(w, req)

	return &AgentResponse{
		StatusCode: w.Code,
		Headers:    w.Header(),
		Body:       w.Body.Bytes(),
		Cookies:    w.Result().Cookies(),
	}
}

// Get performs a GET request
func (a *Agent) Get(path string) *AgentResponse {
	return a.Do("GET", path, nil)
}

// Post performs a POST request with the given body
func (a *Agent) Post(path string, body io.Reader) *AgentResponse {
	return a.Do("POST", path, body)
}

// PostJSON performs a POST request with JSON body
func (a *Agent) PostJSON(path string, obj any) *AgentResponse {
	data, err := json.Marshal(obj)
	if err != nil {
		return &AgentResponse{StatusCode: 0}
	}
	a.SetHeader("Content-Type", "application/json")
	return a.Post(path, bytes.NewReader(data))
}

// PostForm performs a POST request with form data
func (a *Agent) PostForm(path string, data url.Values) *AgentResponse {
	a.SetHeader("Content-Type", "application/x-www-form-urlencoded")
	return a.Post(path, strings.NewReader(data.Encode()))
}

// Put performs a PUT request with the given body
func (a *Agent) Put(path string, body io.Reader) *AgentResponse {
	return a.Do("PUT", path, body)
}

// PutJSON performs a PUT request with JSON body
func (a *Agent) PutJSON(path string, obj any) *AgentResponse {
	data, err := json.Marshal(obj)
	if err != nil {
		return &AgentResponse{StatusCode: 0}
	}
	a.SetHeader("Content-Type", "application/json")
	return a.Put(path, bytes.NewReader(data))
}

// Patch performs a PATCH request with the given body
func (a *Agent) Patch(path string, body io.Reader) *AgentResponse {
	return a.Do("PATCH", path, body)
}

// PatchJSON performs a PATCH request with JSON body
func (a *Agent) PatchJSON(path string, obj any) *AgentResponse {
	data, err := json.Marshal(obj)
	if err != nil {
		return &AgentResponse{StatusCode: 0}
	}
	a.SetHeader("Content-Type", "application/json")
	return a.Patch(path, bytes.NewReader(data))
}

// Delete performs a DELETE request
func (a *Agent) Delete(path string) *AgentResponse {
	return a.Do("DELETE", path, nil)
}

// Head performs a HEAD request
func (a *Agent) Head(path string) *AgentResponse {
	return a.Do("HEAD", path, nil)
}

// Options performs an OPTIONS request
func (a *Agent) Options(path string) *AgentResponse {
	return a.Do("OPTIONS", path, nil)
}

// ============== Request Builder ==============

// Request is a builder for creating requests
type Request struct {
	agent   *Agent
	method  string
	path    string
	headers http.Header
	body    io.Reader
	query   url.Values
}

// NewRequest creates a new request builder
func (a *Agent) NewRequest(method, path string) *Request {
	return &Request{
		agent:   a,
		method:  method,
		path:    path,
		headers: make(http.Header),
		query:   make(url.Values),
	}
}

// Header sets a header
func (r *Request) Header(key, value string) *Request {
	r.headers.Set(key, value)
	return r
}

// Query sets a query parameter
func (r *Request) Query(key, value string) *Request {
	r.query.Set(key, value)
	return r
}

// Body sets the request body
func (r *Request) Body(body io.Reader) *Request {
	r.body = body
	return r
}

// JSON sets the body as JSON
func (r *Request) JSON(obj any) *Request {
	data, _ := json.Marshal(obj)
	r.body = bytes.NewReader(data)
	r.headers.Set("Content-Type", "application/json")
	return r
}

// Form sets the body as form data
func (r *Request) Form(data url.Values) *Request {
	r.body = strings.NewReader(data.Encode())
	r.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// Send executes the request
func (r *Request) Send() *AgentResponse {
	path := r.path
	if len(r.query) > 0 {
		path += "?" + r.query.Encode()
	}

	req := httptest.NewRequest(r.method, path, r.body)

	// Copy agent headers
	for key, values := range r.agent.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy request headers (override agent headers)
	for key, values := range r.headers {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	// Add cookies
	for _, cookie := range r.agent.cookies {
		req.AddCookie(cookie)
	}

	// Execute request
	w := httptest.NewRecorder()
	r.agent.engine.ServeHTTP(w, req)

	return &AgentResponse{
		StatusCode: w.Code,
		Headers:    w.Header(),
		Body:       w.Body.Bytes(),
		Cookies:    w.Result().Cookies(),
	}
}
