package rue

import (
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// Property 8: Validator Rule Enforcement
// For any struct with validation tags and for any field value that violates a validation rule,
// the Validator should report that violation. For any valid struct, validation should pass.
// Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7

func TestValidator_Required(t *testing.T) {
	type User struct {
		Name string `validate:"required"`
	}

	v := NewValidator()

	// Valid
	user := User{Name: "john"}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Valid user should pass, got error: %v", err)
	}

	// Invalid - empty string
	user = User{Name: ""}
	if err := v.Validate(&user); err == nil {
		t.Error("Empty name should fail required validation")
	}

	// Invalid - whitespace only
	user = User{Name: "   "}
	if err := v.Validate(&user); err == nil {
		t.Error("Whitespace-only name should fail required validation")
	}
}

func TestValidator_Min(t *testing.T) {
	type User struct {
		Age int `validate:"min=18"`
	}

	v := NewValidator()

	// Valid
	user := User{Age: 18}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Age 18 should pass min=18, got error: %v", err)
	}

	user = User{Age: 25}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Age 25 should pass min=18, got error: %v", err)
	}

	// Invalid
	user = User{Age: 17}
	if err := v.Validate(&user); err == nil {
		t.Error("Age 17 should fail min=18 validation")
	}
}

func TestValidator_Max(t *testing.T) {
	type User struct {
		Age int `validate:"max=100"`
	}

	v := NewValidator()

	// Valid
	user := User{Age: 100}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Age 100 should pass max=100, got error: %v", err)
	}

	// Invalid
	user = User{Age: 101}
	if err := v.Validate(&user); err == nil {
		t.Error("Age 101 should fail max=100 validation")
	}
}

func TestValidator_Len(t *testing.T) {
	type Code struct {
		Value string `validate:"len=6"`
	}

	v := NewValidator()

	// Valid
	code := Code{Value: "123456"}
	if err := v.Validate(&code); err != nil {
		t.Errorf("6-char code should pass len=6, got error: %v", err)
	}

	// Invalid - too short
	code = Code{Value: "12345"}
	if err := v.Validate(&code); err == nil {
		t.Error("5-char code should fail len=6 validation")
	}

	// Invalid - too long
	code = Code{Value: "1234567"}
	if err := v.Validate(&code); err == nil {
		t.Error("7-char code should fail len=6 validation")
	}
}

func TestValidator_Email(t *testing.T) {
	type User struct {
		Email string `validate:"email"`
	}

	v := NewValidator()

	// Valid emails
	validEmails := []string{
		"test@example.com",
		"user.name@domain.org",
		"user+tag@example.co.uk",
	}

	for _, email := range validEmails {
		user := User{Email: email}
		if err := v.Validate(&user); err != nil {
			t.Errorf("Email %s should be valid, got error: %v", email, err)
		}
	}

	// Invalid emails
	invalidEmails := []string{
		"notanemail",
		"@nodomain.com",
		"noat.com",
		"spaces in@email.com",
	}

	for _, email := range invalidEmails {
		user := User{Email: email}
		if err := v.Validate(&user); err == nil {
			t.Errorf("Email %s should be invalid", email)
		}
	}
}

func TestValidator_URL(t *testing.T) {
	type Link struct {
		URL string `validate:"url"`
	}

	v := NewValidator()

	// Valid URLs
	validURLs := []string{
		"http://example.com",
		"https://example.com/path",
		"https://sub.domain.com/path?query=1",
	}

	for _, url := range validURLs {
		link := Link{URL: url}
		if err := v.Validate(&link); err != nil {
			t.Errorf("URL %s should be valid, got error: %v", url, err)
		}
	}

	// Invalid URLs
	invalidURLs := []string{
		"notaurl",
		"ftp://example.com",
		"example.com",
	}

	for _, url := range invalidURLs {
		link := Link{URL: url}
		if err := v.Validate(&link); err == nil {
			t.Errorf("URL %s should be invalid", url)
		}
	}
}

func TestValidator_Regex(t *testing.T) {
	type Code struct {
		Value string `validate:"regex=^[A-Z]{3}[0-9]{3}$"`
	}

	v := NewValidator()

	// Valid
	code := Code{Value: "ABC123"}
	if err := v.Validate(&code); err != nil {
		t.Errorf("ABC123 should match pattern, got error: %v", err)
	}

	// Invalid
	code = Code{Value: "abc123"}
	if err := v.Validate(&code); err == nil {
		t.Error("abc123 should not match pattern (lowercase)")
	}

	code = Code{Value: "ABCD123"}
	if err := v.Validate(&code); err == nil {
		t.Error("ABCD123 should not match pattern (4 letters)")
	}
}

func TestValidator_OneOf(t *testing.T) {
	type Status struct {
		Value string `validate:"oneof=active inactive pending"`
	}

	v := NewValidator()

	// Valid
	for _, val := range []string{"active", "inactive", "pending"} {
		status := Status{Value: val}
		if err := v.Validate(&status); err != nil {
			t.Errorf("%s should be valid, got error: %v", val, err)
		}
	}

	// Invalid
	status := Status{Value: "unknown"}
	if err := v.Validate(&status); err == nil {
		t.Error("unknown should not be in oneof")
	}
}

func TestValidator_GT_GTE_LT_LTE(t *testing.T) {
	type Numbers struct {
		GT  int `validate:"gt=0"`
		GTE int `validate:"gte=0"`
		LT  int `validate:"lt=100"`
		LTE int `validate:"lte=100"`
	}

	v := NewValidator()

	// Valid
	nums := Numbers{GT: 1, GTE: 0, LT: 99, LTE: 100}
	if err := v.Validate(&nums); err != nil {
		t.Errorf("Valid numbers should pass, got error: %v", err)
	}

	// Invalid GT
	nums = Numbers{GT: 0, GTE: 0, LT: 99, LTE: 100}
	if err := v.Validate(&nums); err == nil {
		t.Error("GT=0 should fail gt=0 validation")
	}

	// Invalid LT
	nums = Numbers{GT: 1, GTE: 0, LT: 100, LTE: 100}
	if err := v.Validate(&nums); err == nil {
		t.Error("LT=100 should fail lt=100 validation")
	}
}

func TestValidator_MultipleRules(t *testing.T) {
	type User struct {
		Name string `validate:"required,min=2,max=50"`
		Age  int    `validate:"required,min=0,max=150"`
	}

	v := NewValidator()

	// Valid
	user := User{Name: "John", Age: 30}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Valid user should pass, got error: %v", err)
	}

	// Invalid - name too short
	user = User{Name: "J", Age: 30}
	if err := v.Validate(&user); err == nil {
		t.Error("Name 'J' should fail min=2 validation")
	}
}

func TestValidator_NestedStruct(t *testing.T) {
	type Address struct {
		City string `validate:"required"`
	}
	type User struct {
		Name    string  `validate:"required"`
		Address Address // Nested struct
	}

	v := NewValidator()

	// Valid
	user := User{Name: "John", Address: Address{City: "NYC"}}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Valid user should pass, got error: %v", err)
	}

	// Invalid - nested field empty
	user = User{Name: "John", Address: Address{City: ""}}
	if err := v.Validate(&user); err == nil {
		t.Error("Empty city should fail required validation")
	}
}

func TestValidator_CustomValidation(t *testing.T) {
	type User struct {
		Username string `validate:"username"`
	}

	v := NewValidator()
	v.RegisterValidation("username", func(field reflect.Value, param string) bool {
		s := field.String()
		// Username must start with letter and contain only alphanumeric
		if len(s) == 0 {
			return false
		}
		if s[0] < 'a' || s[0] > 'z' {
			return false
		}
		for _, c := range s {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				return false
			}
		}
		return true
	})

	// Valid
	user := User{Username: "john123"}
	if err := v.Validate(&user); err != nil {
		t.Errorf("Valid username should pass, got error: %v", err)
	}

	// Invalid - starts with number
	user = User{Username: "123john"}
	if err := v.Validate(&user); err == nil {
		t.Error("Username starting with number should fail")
	}
}

func TestValidator_ValidationErrors(t *testing.T) {
	type User struct {
		Name  string `validate:"required"`
		Email string `validate:"email"`
		Age   int    `validate:"min=18"`
	}

	v := NewValidator()

	// Multiple validation errors
	user := User{Name: "", Email: "invalid", Age: 10}
	err := v.Validate(&user)

	if err == nil {
		t.Fatal("Expected validation errors")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors, got %T", err)
	}

	if len(errs) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(errs))
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{Field: "name", Tag: "required", Message: "field is required"}
	expected := "validation failed on field 'name': field is required"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errs := ValidationErrors{
		{Field: "name", Message: "required"},
		{Field: "age", Message: "min"},
	}

	errStr := errs.Error()
	if !strings.Contains(errStr, "name") || !strings.Contains(errStr, "age") {
		t.Errorf("Error() should contain all field names, got: %s", errStr)
	}
}

func TestValidator_NonStruct(t *testing.T) {
	v := NewValidator()

	// Non-struct should pass (no validation)
	var str string = "test"
	if err := v.Validate(&str); err != nil {
		t.Errorf("Non-struct should pass validation, got error: %v", err)
	}
}

// Property-based test: Required validation should fail for empty strings
// Feature: rue-framework, Property 8: Validator Rule Enforcement
// Validates: Requirements 6.1-6.7
func TestValidator_Property_RequiredFailsEmpty(t *testing.T) {
	type User struct {
		Name string `validate:"required"`
	}

	v := NewValidator()

	f := func(name string) bool {
		user := User{Name: name}
		err := v.Validate(&user)

		isEmpty := strings.TrimSpace(name) == ""

		// Property: required should fail for empty/whitespace strings
		if isEmpty {
			return err != nil
		}
		return err == nil
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Min validation should enforce minimum value
// Feature: rue-framework, Property 8: Validator Rule Enforcement
// Validates: Requirements 6.1-6.7
func TestValidator_Property_MinEnforcement(t *testing.T) {
	type User struct {
		Age int `validate:"min=18"`
	}

	v := NewValidator()

	f := func(age int8) bool {
		user := User{Age: int(age)}
		err := v.Validate(&user)

		// Property: min=18 should fail for age < 18
		if int(age) < 18 {
			return err != nil
		}
		return err == nil
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Max validation should enforce maximum value
// Feature: rue-framework, Property 8: Validator Rule Enforcement
// Validates: Requirements 6.1-6.7
func TestValidator_Property_MaxEnforcement(t *testing.T) {
	type User struct {
		Age int `validate:"max=100"`
	}

	v := NewValidator()

	f := func(age int8) bool {
		user := User{Age: int(age)}
		err := v.Validate(&user)

		// Property: max=100 should fail for age > 100
		if int(age) > 100 {
			return err != nil
		}
		return err == nil
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Validation errors should contain field name
// Feature: rue-framework, Property 8: Validator Rule Enforcement
// Validates: Requirements 6.1-6.7
func TestValidator_Property_ErrorContainsFieldName(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	v := NewValidator()

	f := func() bool {
		user := User{Name: ""} // Will fail validation
		err := v.Validate(&user)

		if err == nil {
			return false
		}

		errs, ok := err.(ValidationErrors)
		if !ok {
			return false
		}

		// Property: error should contain field name
		for _, e := range errs {
			if e.Field == "name" || e.Field == "Name" {
				return true
			}
		}
		return false
	}

	// Run multiple times
	for i := 0; i < 100; i++ {
		if !f() {
			t.Error("Validation error should contain field name")
			break
		}
	}
}
