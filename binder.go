package rue

import (
	"encoding/xml"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
)

// BindingError represents a binding error
type BindingError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (e *BindingError) Error() string {
	return "binding error on field '" + e.Field + "': " + e.Reason
}

// DefaultBinder is the default implementation of Binder
type DefaultBinder struct{}

// Bind binds the request data to obj based on Content-Type
func (b *DefaultBinder) Bind(c *Context, obj any) error {
	contentType := c.ContentType()

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return b.bindJSON(c, obj)
	case strings.HasPrefix(contentType, "application/xml"), strings.HasPrefix(contentType, "text/xml"):
		return b.bindXML(c, obj)
	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
		return b.bindForm(c, obj)
	case strings.HasPrefix(contentType, "multipart/form-data"):
		return b.bindMultipartForm(c, obj)
	default:
		// Try to bind query parameters
		return b.bindQuery(c, obj)
	}
}

// bindJSON binds JSON request body
func (b *DefaultBinder) bindJSON(c *Context, obj any) error {
	if c.Request.Body == nil {
		return errors.New("request body is empty")
	}
	return sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(obj)
}

// bindXML binds XML request body
func (b *DefaultBinder) bindXML(c *Context, obj any) error {
	if c.Request.Body == nil {
		return errors.New("request body is empty")
	}
	return xml.NewDecoder(c.Request.Body).Decode(obj)
}

// bindForm binds form data
func (b *DefaultBinder) bindForm(c *Context, obj any) error {
	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	return mapForm(obj, c.Request.PostForm)
}

// bindMultipartForm binds multipart form data
func (b *DefaultBinder) bindMultipartForm(c *Context, obj any) error {
	if err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
		return err
	}
	return mapForm(obj, c.Request.PostForm)
}

// bindQuery binds query parameters
func (b *DefaultBinder) bindQuery(c *Context, obj any) error {
	return mapForm(obj, c.Request.URL.Query())
}

// bindHeader binds request headers
func (b *DefaultBinder) bindHeader(c *Context, obj any) error {
	return mapHeader(obj, c.Request.Header)
}

// bindParam binds path parameters
func (b *DefaultBinder) bindParam(c *Context, obj any) error {
	return mapParams(obj, c.Params)
}

// mapForm maps form values to a struct
func mapForm(ptr any, form url.Values) error {
	return mapFormByTag(ptr, form, "form")
}

// mapFormByTag maps form values to a struct using a specific tag
func mapFormByTag(ptr any, form url.Values, tag string) error {
	typ := reflect.TypeOf(ptr)
	val := reflect.ValueOf(ptr)

	if typ.Kind() != reflect.Ptr {
		return errors.New("binding element must be a pointer")
	}

	typ = typ.Elem()
	val = val.Elem()

	if typ.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		tagValue := field.Tag.Get(tag)
		if tagValue == "" || tagValue == "-" {
			// Try json tag as fallback
			tagValue = field.Tag.Get("json")
			if tagValue == "" || tagValue == "-" {
				continue
			}
		}

		// Handle tag options like `form:"name,omitempty"`
		tagName := strings.Split(tagValue, ",")[0]

		formValue := form.Get(tagName)
		if formValue == "" {
			continue
		}

		if err := setFieldValue(fieldVal, formValue); err != nil {
			return &BindingError{Field: tagName, Reason: err.Error()}
		}
	}

	return nil
}

// mapHeader maps header values to a struct
func mapHeader(ptr any, header map[string][]string) error {
	typ := reflect.TypeOf(ptr)
	val := reflect.ValueOf(ptr)

	if typ.Kind() != reflect.Ptr {
		return errors.New("binding element must be a pointer")
	}

	typ = typ.Elem()
	val = val.Elem()

	if typ.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		tagValue := field.Tag.Get("header")
		if tagValue == "" || tagValue == "-" {
			continue
		}

		tagName := strings.Split(tagValue, ",")[0]
		headerValues := header[tagName]
		if len(headerValues) == 0 {
			continue
		}
		headerValue := headerValues[0]

		if err := setFieldValue(fieldVal, headerValue); err != nil {
			return &BindingError{Field: tagName, Reason: err.Error()}
		}
	}

	return nil
}

// mapParams maps path parameters to a struct
func mapParams(ptr any, params Params) error {
	typ := reflect.TypeOf(ptr)
	val := reflect.ValueOf(ptr)

	if typ.Kind() != reflect.Ptr {
		return errors.New("binding element must be a pointer")
	}

	typ = typ.Elem()
	val = val.Elem()

	if typ.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		tagValue := field.Tag.Get("param")
		if tagValue == "" || tagValue == "-" {
			continue
		}

		tagName := strings.Split(tagValue, ",")[0]
		paramValue := params.ByName(tagName)
		if paramValue == "" {
			continue
		}

		if err := setFieldValue(fieldVal, paramValue); err != nil {
			return &BindingError{Field: tagName, Reason: err.Error()}
		}
	}

	return nil
}

// setFieldValue sets a field value from a string
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	default:
		return errors.New("unsupported field type: " + field.Kind().String())
	}
	return nil
}
