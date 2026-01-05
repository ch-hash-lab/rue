package rue

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Property 11: Error Handling Consistency
// For any error returned by the framework, the error handler should produce
// a consistent response format with appropriate status code.
// Validates: Requirements 8.2, 8.3, 8.4

func TestError_Error(t *testing.T) {
	err := NewError(http.StatusBadRequest, "Bad Request")
	expected := "code=400, message=Bad Request"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// With wrapped error
	err = err.WithError(errors.New("underlying error"))
	if !strings.Contains(err.Error(), "underlying error") {
		t.Errorf("Error() should contain wrapped error, got: %s", err.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := NewError(http.StatusBadRequest, "Bad Request").WithError(underlying)

	if err.Unwrap() != underlying {
		t.Error("Unwrap() should return the underlying error")
	}
}

func TestError_Is(t *testing.T) {
	err := NewError(http.StatusBadRequest, "Bad Request")

	// Same code should match
	if !errors.Is(err, ErrBadRequest) {
		t.Error("errors.Is should match errors with same code")
	}

	// Different code should not match
	if errors.Is(err, ErrNotFound) {
		t.Error("errors.Is should not match errors with different code")
	}
}

func TestError_WithDetails(t *testing.T) {
	err := NewError(http.StatusBadRequest, "Bad Request")
	details := map[string]string{"field": "name", "reason": "required"}

	newErr := err.WithDetails(details)

	// Should return a new error
	if newErr == err {
		t.Error("WithDetails should return a new error")
	}

	// Original should not be modified
	if err.Details != nil {
		t.Error("Original error should not be modified")
	}

	// New error should have details
	if newErr.Details == nil {
		t.Error("New error should have details")
	}
}

func TestError_WithMessage(t *testing.T) {
	err := NewError(http.StatusBadRequest, "Bad Request")
	newErr := err.WithMessage("Custom message")

	// Should return a new error
	if newErr == err {
		t.Error("WithMessage should return a new error")
	}

	// Original should not be modified
	if err.Message != "Bad Request" {
		t.Error("Original error should not be modified")
	}

	// New error should have new message
	if newErr.Message != "Custom message" {
		t.Error("New error should have custom message")
	}
}

func TestError_StatusCode(t *testing.T) {
	err := NewError(http.StatusNotFound, "Not Found")
	if err.StatusCode() != http.StatusNotFound {
		t.Errorf("StatusCode() = %d, want %d", err.StatusCode(), http.StatusNotFound)
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		err  *Error
		code int
	}{
		{ErrBadRequest, http.StatusBadRequest},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrNotFound, http.StatusNotFound},
		{ErrMethodNotAllowed, http.StatusMethodNotAllowed},
		{ErrConflict, http.StatusConflict},
		{ErrUnprocessableEntity, http.StatusUnprocessableEntity},
		{ErrTooManyRequests, http.StatusTooManyRequests},
		{ErrInternalServerError, http.StatusInternalServerError},
		{ErrServiceUnavailable, http.StatusServiceUnavailable},
		{ErrGatewayTimeout, http.StatusGatewayTimeout},
		{ErrRequestTimeout, http.StatusRequestTimeout},
		{ErrPayloadTooLarge, http.StatusRequestEntityTooLarge},
		{ErrUnsupportedMedia, http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		if tt.err.Code != tt.code {
			t.Errorf("%s.Code = %d, want %d", tt.err.Message, tt.err.Code, tt.code)
		}
	}
}

func TestDefaultErrorHandler_RueError(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	err := ErrNotFound.WithDetails(map[string]string{"resource": "user"})
	DefaultErrorHandler(c, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Not Found") {
		t.Errorf("Body should contain error message, got: %s", body)
	}
	if !strings.Contains(body, "resource") {
		t.Errorf("Body should contain details, got: %s", body)
	}
}

func TestDefaultErrorHandler_ValidationErrors(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	errs := ValidationErrors{
		{Field: "name", Tag: "required", Message: "field is required"},
		{Field: "age", Tag: "min", Message: "value must be at least 18"},
	}
	DefaultErrorHandler(c, errs)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Validation failed") {
		t.Errorf("Body should contain validation message, got: %s", body)
	}
}

func TestDefaultErrorHandler_GenericError(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	err := errors.New("something went wrong")
	DefaultErrorHandler(c, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestDefaultErrorHandler_AlreadyWritten(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	// Write something first
	c.Text(http.StatusOK, "OK")

	// Error handler should not write again
	DefaultErrorHandler(c, ErrNotFound)

	// Status should still be 200
	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d (should not change)", w.Code, http.StatusOK)
	}
}

func TestJSONErrorHandler_Debug(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	underlying := errors.New("database connection failed")
	err := ErrInternalServerError.WithError(underlying)

	handler := JSONErrorHandler(true)
	handler(c, err)

	body := w.Body.String()
	if !strings.Contains(body, "database connection failed") {
		t.Errorf("Debug mode should include underlying error, got: %s", body)
	}
}

func TestJSONErrorHandler_NoDebug(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	underlying := errors.New("database connection failed")
	err := ErrInternalServerError.WithError(underlying)

	handler := JSONErrorHandler(false)
	handler(c, err)

	body := w.Body.String()
	if strings.Contains(body, "database connection failed") {
		t.Errorf("Non-debug mode should not include underlying error, got: %s", body)
	}
}

func TestContext_Error(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	// Add errors
	c.Error(errors.New("error 1"))
	c.Error(errors.New("error 2"))
	c.Error(nil) // Should be ignored

	if len(c.Errors) != 2 {
		t.Errorf("Errors count = %d, want 2", len(c.Errors))
	}
}

func TestContext_AbortWithError(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	c := &Context{engine: engine}
	c.reset(w, req)

	c.AbortWithError(http.StatusBadRequest, ErrBadRequest)

	if !c.IsAborted() {
		t.Error("Context should be aborted")
	}

	if len(c.Errors) != 1 {
		t.Errorf("Errors count = %d, want 1", len(c.Errors))
	}
}

// Property-based test: Error codes should match HTTP status codes
// Feature: rue-framework, Property 11: Error Handling Consistency
// Validates: Requirements 8.2, 8.3, 8.4
func TestError_Property_StatusCodeConsistency(t *testing.T) {
	statusCodes := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}

	for _, code := range statusCodes {
		err := NewError(code, "Test")
		if err.StatusCode() != code {
			t.Errorf("StatusCode() = %d, want %d", err.StatusCode(), code)
		}
	}
}

// Property-based test: WithDetails should not modify original error
// Feature: rue-framework, Property 11: Error Handling Consistency
// Validates: Requirements 8.2, 8.3, 8.4
func TestError_Property_Immutability(t *testing.T) {
	original := NewError(http.StatusBadRequest, "Bad Request")
	originalCode := original.Code
	originalMessage := original.Message

	// Create variations
	_ = original.WithDetails("details")
	_ = original.WithError(errors.New("wrapped"))
	_ = original.WithMessage("new message")

	// Original should be unchanged
	if original.Code != originalCode {
		t.Error("Original code was modified")
	}
	if original.Message != originalMessage {
		t.Error("Original message was modified")
	}
	if original.Details != nil {
		t.Error("Original details was modified")
	}
	if original.Err != nil {
		t.Error("Original wrapped error was modified")
	}
}

// Property-based test: Error handler should always set status code
// Feature: rue-framework, Property 11: Error Handling Consistency
// Validates: Requirements 8.2, 8.3, 8.4
func TestError_Property_HandlerSetsStatus(t *testing.T) {
	testCases := []struct {
		err          error
		expectedCode int
	}{
		{ErrBadRequest, http.StatusBadRequest},
		{ErrNotFound, http.StatusNotFound},
		{ErrInternalServerError, http.StatusInternalServerError},
		{errors.New("generic error"), http.StatusInternalServerError},
		{ValidationErrors{{Field: "test"}}, http.StatusBadRequest},
	}

	for _, tc := range testCases {
		engine := New()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		c := &Context{engine: engine}
		c.reset(w, req)

		DefaultErrorHandler(c, tc.err)

		if w.Code != tc.expectedCode {
			t.Errorf("For error %T, Status = %d, want %d", tc.err, w.Code, tc.expectedCode)
		}
	}
}
