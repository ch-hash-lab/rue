package rue

import (
	"io"
)

// Binder is the interface for request data binding
type Binder interface {
	Bind(c *Context, obj any) error
}

// Validator is the interface for data validation
type Validator interface {
	Validate(obj any) error
}

// Renderer is the interface for response rendering
type Renderer interface {
	Render(w io.Writer, name string, data any) error
}

// ErrorHandler is the function type for handling errors
type ErrorHandler func(*Context, error)
