package rue

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is the framework error type
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	Err     error  `json:"-"`
}

// NewError creates a new Error
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code=%d, message=%s, error=%v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is implements errors.Is interface for error comparison
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithDetails adds details to the error and returns a new Error
func (e *Error) WithDetails(details any) *Error {
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Err:     e.Err,
	}
}

// WithError wraps an error and returns a new Error
func (e *Error) WithError(err error) *Error {
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
		Err:     err,
	}
}

// WithMessage creates a new Error with a custom message
func (e *Error) WithMessage(message string) *Error {
	return &Error{
		Code:    e.Code,
		Message: message,
		Details: e.Details,
		Err:     e.Err,
	}
}

// StatusCode returns the HTTP status code
func (e *Error) StatusCode() int {
	return e.Code
}

// Predefined errors
var (
	ErrBadRequest          = NewError(http.StatusBadRequest, "Bad Request")
	ErrUnauthorized        = NewError(http.StatusUnauthorized, "Unauthorized")
	ErrForbidden           = NewError(http.StatusForbidden, "Forbidden")
	ErrNotFound            = NewError(http.StatusNotFound, "Not Found")
	ErrMethodNotAllowed    = NewError(http.StatusMethodNotAllowed, "Method Not Allowed")
	ErrConflict            = NewError(http.StatusConflict, "Conflict")
	ErrUnprocessableEntity = NewError(http.StatusUnprocessableEntity, "Unprocessable Entity")
	ErrTooManyRequests     = NewError(http.StatusTooManyRequests, "Too Many Requests")
	ErrInternalServerError = NewError(http.StatusInternalServerError, "Internal Server Error")
	ErrServiceUnavailable  = NewError(http.StatusServiceUnavailable, "Service Unavailable")
	ErrGatewayTimeout      = NewError(http.StatusGatewayTimeout, "Gateway Timeout")
	ErrRequestTimeout      = NewError(http.StatusRequestTimeout, "Request Timeout")
	ErrPayloadTooLarge     = NewError(http.StatusRequestEntityTooLarge, "Payload Too Large")
	ErrUnsupportedMedia    = NewError(http.StatusUnsupportedMediaType, "Unsupported Media Type")
)

// ErrorHandlerFunc is the error handler function type
type ErrorHandlerFunc func(c *Context, err error)

// DefaultErrorHandler is the default error handler
func DefaultErrorHandler(c *Context, err error) {
	if c.Writer.Written() {
		return
	}

	// Check if it's a rue Error
	var rueErr *Error
	if errors.As(err, &rueErr) {
		response := H{
			"code":    rueErr.Code,
			"message": rueErr.Message,
		}
		if rueErr.Details != nil {
			response["details"] = rueErr.Details
		}
		c.JSON(rueErr.Code, response)
		return
	}

	// Check if it's ValidationErrors
	var validationErrs ValidationErrors
	if errors.As(err, &validationErrs) {
		c.JSON(http.StatusBadRequest, H{
			"code":    http.StatusBadRequest,
			"message": "Validation failed",
			"errors":  validationErrs,
		})
		return
	}

	// Default to internal server error
	c.JSON(http.StatusInternalServerError, H{
		"code":    http.StatusInternalServerError,
		"message": err.Error(),
	})
}

// JSONErrorHandler returns errors in JSON format with optional debug info
func JSONErrorHandler(debug bool) ErrorHandlerFunc {
	return func(c *Context, err error) {
		if c.Writer.Written() {
			return
		}

		var rueErr *Error
		if errors.As(err, &rueErr) {
			response := H{
				"code":    rueErr.Code,
				"message": rueErr.Message,
			}
			if rueErr.Details != nil {
				response["details"] = rueErr.Details
			}
			if debug && rueErr.Err != nil {
				response["error"] = rueErr.Err.Error()
			}
			c.JSON(rueErr.Code, response)
			return
		}

		var validationErrs ValidationErrors
		if errors.As(err, &validationErrs) {
			c.JSON(http.StatusBadRequest, H{
				"code":    http.StatusBadRequest,
				"message": "Validation failed",
				"errors":  validationErrs,
			})
			return
		}

		response := H{
			"code":    http.StatusInternalServerError,
			"message": "Internal Server Error",
		}
		if debug {
			response["error"] = err.Error()
		}
		c.JSON(http.StatusInternalServerError, response)
	}
}

// AbortWithError aborts the request with an error
func (c *Context) AbortWithError(code int, err error) {
	c.Abort()
	c.Error(err)
	if c.engine != nil && c.engine.ErrorHandler != nil {
		c.engine.ErrorHandler(c, err)
	} else {
		DefaultErrorHandler(c, err)
	}
}

// Error adds an error to the context
func (c *Context) Error(err error) {
	if err == nil {
		return
	}
	c.Errors = append(c.Errors, err)
}
