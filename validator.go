package rue

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// ValidationFunc is a custom validation function
type ValidationFunc func(field reflect.Value, param string) bool

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on field '%s': %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, err := range e {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// DefaultValidator is the default validator implementation
type DefaultValidator struct {
	tagName    string
	customFunc map[string]ValidationFunc
}

// NewValidator creates a new DefaultValidator
func NewValidator() *DefaultValidator {
	return &DefaultValidator{
		tagName:    "validate",
		customFunc: make(map[string]ValidationFunc),
	}
}

// RegisterValidation registers a custom validation function
func (v *DefaultValidator) RegisterValidation(tag string, fn ValidationFunc) {
	v.customFunc[tag] = fn
}

// Validate validates a struct
func (v *DefaultValidator) Validate(obj any) error {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	var errs ValidationErrors
	v.validateStruct(val, "", &errs)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (v *DefaultValidator) validateStruct(val reflect.Value, prefix string, errs *ValidationErrors) {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanInterface() {
			continue
		}

		fieldName := fieldType.Name
		if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			fieldName = strings.Split(jsonTag, ",")[0]
		}
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			v.validateStruct(field, fieldName, errs)
			continue
		}

		// Handle pointer to struct
		if field.Kind() == reflect.Ptr && !field.IsNil() && field.Elem().Kind() == reflect.Struct {
			v.validateStruct(field.Elem(), fieldName, errs)
			continue
		}

		// Get validation tag
		tagValue := fieldType.Tag.Get(v.tagName)
		if tagValue == "" || tagValue == "-" {
			continue
		}

		// Parse and validate each rule
		rules := strings.Split(tagValue, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}

			var tag, param string
			if idx := strings.Index(rule, "="); idx > 0 {
				tag = rule[:idx]
				param = rule[idx+1:]
			} else {
				tag = rule
			}

			if !v.validateField(field, tag, param) {
				*errs = append(*errs, ValidationError{
					Field:   fieldName,
					Tag:     tag,
					Value:   field.Interface(),
					Message: v.getErrorMessage(tag, param),
				})
			}
		}
	}
}

func (v *DefaultValidator) validateField(field reflect.Value, tag, param string) bool {
	// Check custom validators first
	if fn, ok := v.customFunc[tag]; ok {
		return fn(field, param)
	}

	// Built-in validators
	switch tag {
	case "required":
		return v.validateRequired(field)
	case "min":
		return v.validateMin(field, param)
	case "max":
		return v.validateMax(field, param)
	case "len":
		return v.validateLen(field, param)
	case "email":
		return v.validateEmail(field)
	case "url":
		return v.validateURL(field)
	case "regex":
		return v.validateRegex(field, param)
	case "oneof":
		return v.validateOneOf(field, param)
	case "gt":
		return v.validateGT(field, param)
	case "gte":
		return v.validateGTE(field, param)
	case "lt":
		return v.validateLT(field, param)
	case "lte":
		return v.validateLTE(field, param)
	default:
		return true
	}
}

func (v *DefaultValidator) validateRequired(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.String:
		return strings.TrimSpace(field.String()) != ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return field.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !field.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return field.Float() != 0
	case reflect.Bool:
		return field.Bool()
	default:
		return !field.IsZero()
	}
}

func (v *DefaultValidator) validateMin(field reflect.Value, param string) bool {
	minVal, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.String:
		return float64(len(field.String())) >= minVal
	case reflect.Slice, reflect.Map, reflect.Array:
		return float64(field.Len()) >= minVal
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) >= minVal
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) >= minVal
	case reflect.Float32, reflect.Float64:
		return field.Float() >= minVal
	default:
		return false
	}
}

func (v *DefaultValidator) validateMax(field reflect.Value, param string) bool {
	maxVal, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.String:
		return float64(len(field.String())) <= maxVal
	case reflect.Slice, reflect.Map, reflect.Array:
		return float64(field.Len()) <= maxVal
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) <= maxVal
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) <= maxVal
	case reflect.Float32, reflect.Float64:
		return field.Float() <= maxVal
	default:
		return false
	}
}

func (v *DefaultValidator) validateLen(field reflect.Value, param string) bool {
	lenVal, err := strconv.Atoi(param)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.String:
		return len(field.String()) == lenVal
	case reflect.Slice, reflect.Map, reflect.Array:
		return field.Len() == lenVal
	default:
		return false
	}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func (v *DefaultValidator) validateEmail(field reflect.Value) bool {
	if field.Kind() != reflect.String {
		return false
	}
	return emailRegex.MatchString(field.String())
}

var urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

func (v *DefaultValidator) validateURL(field reflect.Value) bool {
	if field.Kind() != reflect.String {
		return false
	}
	return urlRegex.MatchString(field.String())
}

func (v *DefaultValidator) validateRegex(field reflect.Value, param string) bool {
	if field.Kind() != reflect.String {
		return false
	}
	re, err := regexp.Compile(param)
	if err != nil {
		return false
	}
	return re.MatchString(field.String())
}

func (v *DefaultValidator) validateOneOf(field reflect.Value, param string) bool {
	values := strings.Split(param, " ")
	fieldStr := fmt.Sprintf("%v", field.Interface())
	for _, val := range values {
		if fieldStr == val {
			return true
		}
	}
	return false
}

func (v *DefaultValidator) validateGT(field reflect.Value, param string) bool {
	val, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) > val
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) > val
	case reflect.Float32, reflect.Float64:
		return field.Float() > val
	default:
		return false
	}
}

func (v *DefaultValidator) validateGTE(field reflect.Value, param string) bool {
	val, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) >= val
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) >= val
	case reflect.Float32, reflect.Float64:
		return field.Float() >= val
	default:
		return false
	}
}

func (v *DefaultValidator) validateLT(field reflect.Value, param string) bool {
	val, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) < val
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) < val
	case reflect.Float32, reflect.Float64:
		return field.Float() < val
	default:
		return false
	}
}

func (v *DefaultValidator) validateLTE(field reflect.Value, param string) bool {
	val, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return false
	}

	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(field.Int()) <= val
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(field.Uint()) <= val
	case reflect.Float32, reflect.Float64:
		return field.Float() <= val
	default:
		return false
	}
}

func (v *DefaultValidator) getErrorMessage(tag, param string) string {
	switch tag {
	case "required":
		return "field is required"
	case "min":
		return fmt.Sprintf("value must be at least %s", param)
	case "max":
		return fmt.Sprintf("value must be at most %s", param)
	case "len":
		return fmt.Sprintf("length must be exactly %s", param)
	case "email":
		return "invalid email format"
	case "url":
		return "invalid URL format"
	case "regex":
		return fmt.Sprintf("value must match pattern %s", param)
	case "oneof":
		return fmt.Sprintf("value must be one of: %s", param)
	case "gt":
		return fmt.Sprintf("value must be greater than %s", param)
	case "gte":
		return fmt.Sprintf("value must be greater than or equal to %s", param)
	case "lt":
		return fmt.Sprintf("value must be less than %s", param)
	case "lte":
		return fmt.Sprintf("value must be less than or equal to %s", param)
	default:
		return "validation failed"
	}
}
