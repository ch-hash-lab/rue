package rue

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/quick"
)

// Property 3: Context Response Tracking
// For any response written through Context (JSON, XML, String, etc.),
// the tracked Status() and Size() should match the actual HTTP response status and body length.
// Validates: Requirements 2.6, 2.7

func TestResponseWriter_StatusTracking(t *testing.T) {
	// Test that status is correctly tracked
	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"BadRequest", http.StatusBadRequest},
		{"NotFound", http.StatusNotFound},
		{"InternalServerError", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

			w.WriteHeader(tt.statusCode)
			w.WriteHeaderNow()

			if w.Status() != tt.statusCode {
				t.Errorf("Status() = %d, want %d", w.Status(), tt.statusCode)
			}
			if rec.Code != tt.statusCode {
				t.Errorf("Recorder.Code = %d, want %d", rec.Code, tt.statusCode)
			}
		})
	}
}

func TestResponseWriter_SizeTracking(t *testing.T) {
	// Test that size is correctly tracked
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	data := []byte("Hello, World!")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != len(data) {
		t.Errorf("Write() returned %d, want %d", n, len(data))
	}
	if w.Size() != len(data) {
		t.Errorf("Size() = %d, want %d", w.Size(), len(data))
	}
}

func TestResponseWriter_WrittenFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	if w.Written() {
		t.Error("Written() should be false before writing")
	}

	w.Write([]byte("test"))

	if !w.Written() {
		t.Error("Written() should be true after writing")
	}
}

func TestResponseWriter_WriteString(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	s := "Hello, World!"
	n, err := w.WriteString(s)
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}

	if n != len(s) {
		t.Errorf("WriteString() returned %d, want %d", n, len(s))
	}
	if w.Size() != len(s) {
		t.Errorf("Size() = %d, want %d", w.Size(), len(s))
	}
	if rec.Body.String() != s {
		t.Errorf("Body = %q, want %q", rec.Body.String(), s)
	}
}

func TestResponseWriter_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	data1 := []byte("Hello, ")
	data2 := []byte("World!")

	w.Write(data1)
	w.Write(data2)

	expectedSize := len(data1) + len(data2)
	if w.Size() != expectedSize {
		t.Errorf("Size() = %d, want %d", w.Size(), expectedSize)
	}
}

func TestResponseWriter_WriteHeaderOnlyOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	w.WriteHeader(http.StatusCreated)
	w.WriteHeaderNow()

	// Try to change status after writing
	w.WriteHeader(http.StatusBadRequest)

	// Status should remain as first set
	if w.Status() != http.StatusCreated {
		t.Errorf("Status() = %d, want %d", w.Status(), http.StatusCreated)
	}
}

func TestResponseWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	w.Write([]byte("test"))
	w.Flush()

	if !w.Written() {
		t.Error("Written() should be true after Flush()")
	}
}

// Property-based test: For any byte slice written, Size() should equal the length of bytes written
// Feature: rue-framework, Property 3: Context Response Tracking
// Validates: Requirements 2.6, 2.7
func TestResponseWriter_Property_SizeMatchesWrittenBytes(t *testing.T) {
	f := func(data []byte) bool {
		rec := httptest.NewRecorder()
		w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		n, err := w.Write(data)
		if err != nil {
			return false
		}

		// Property: Size() should equal bytes written
		return w.Size() == n && w.Size() == len(data)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: For any status code, Status() should return the set status
// Feature: rue-framework, Property 3: Context Response Tracking
// Validates: Requirements 2.6, 2.7
func TestResponseWriter_Property_StatusTracking(t *testing.T) {
	f := func(code uint16) bool {
		// Limit to valid HTTP status codes (100-599)
		statusCode := int(code%500) + 100

		rec := httptest.NewRecorder()
		w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		w.WriteHeader(statusCode)

		// Property: Status() should return the set status code
		return w.Status() == statusCode
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Multiple writes should accumulate size correctly
// Feature: rue-framework, Property 3: Context Response Tracking
// Validates: Requirements 2.6, 2.7
func TestResponseWriter_Property_AccumulatedSize(t *testing.T) {
	f := func(chunks [][]byte) bool {
		if len(chunks) == 0 {
			return true
		}

		rec := httptest.NewRecorder()
		w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		expectedSize := 0
		for _, chunk := range chunks {
			n, err := w.Write(chunk)
			if err != nil {
				return false
			}
			expectedSize += n
		}

		// Property: Size() should equal total bytes written
		return w.Size() == expectedSize
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
