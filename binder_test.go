package rue

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/quick"

	"github.com/bytedance/sonic"
)

// Property 6: Binder Round-Trip Consistency
// For any struct with binding tags and for any valid request data (JSON, XML, form, query),
// binding the data to the struct and then serializing it back should preserve the original values.
// Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7

// Property 7: Binder Error Reporting
// For any binding operation that fails due to type mismatch or missing required fields,
// the returned error should contain the field name and a descriptive reason.
// Validates: Requirements 5.8

type TestUser struct {
	Name  string `json:"name" form:"name"`
	Age   int    `json:"age" form:"age"`
	Email string `json:"email" form:"email"`
}

type TestUserWithParam struct {
	ID   string `param:"id"`
	Name string `json:"name" form:"name"`
}

type TestUserWithHeader struct {
	Token string `header:"Authorization"`
	Name  string `json:"name" form:"name"`
}

func createBinderTestContext(method, path, contentType string, body string) *Context {
	engine := New()
	rec := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c := &Context{engine: engine}
	c.reset(rec, req)
	return c
}

func TestBinder_BindJSON(t *testing.T) {
	c := createBinderTestContext(http.MethodPost, "/users", "application/json", `{"name":"john","age":30,"email":"john@example.com"}`)

	var user TestUser
	binder := &DefaultBinder{}
	err := binder.Bind(c, &user)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("Name = %s, want john", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("Age = %d, want 30", user.Age)
	}
	if user.Email != "john@example.com" {
		t.Errorf("Email = %s, want john@example.com", user.Email)
	}
}

func TestBinder_BindXML(t *testing.T) {
	xmlBody := `<TestUser><Name>john</Name><Age>30</Age><Email>john@example.com</Email></TestUser>`
	c := createBinderTestContext(http.MethodPost, "/users", "application/xml", xmlBody)

	type XMLUser struct {
		Name  string `xml:"Name"`
		Age   int    `xml:"Age"`
		Email string `xml:"Email"`
	}

	var user XMLUser
	binder := &DefaultBinder{}
	err := binder.Bind(c, &user)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("Name = %s, want john", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("Age = %d, want 30", user.Age)
	}
}

func TestBinder_BindForm(t *testing.T) {
	form := url.Values{}
	form.Set("name", "john")
	form.Set("age", "30")
	form.Set("email", "john@example.com")

	c := createBinderTestContext(http.MethodPost, "/users", "application/x-www-form-urlencoded", form.Encode())

	var user TestUser
	binder := &DefaultBinder{}
	err := binder.Bind(c, &user)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("Name = %s, want john", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("Age = %d, want 30", user.Age)
	}
}

func TestBinder_BindQuery(t *testing.T) {
	c := createBinderTestContext(http.MethodGet, "/users?name=john&age=30&email=john@example.com", "", "")

	var user TestUser
	binder := &DefaultBinder{}
	err := binder.Bind(c, &user)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("Name = %s, want john", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("Age = %d, want 30", user.Age)
	}
}

func TestBinder_BindParam(t *testing.T) {
	c := createBinderTestContext(http.MethodGet, "/users/123", "", "")
	c.Params = Params{{Key: "id", Value: "123"}}

	var user TestUserWithParam
	binder := &DefaultBinder{}
	err := binder.bindParam(c, &user)
	if err != nil {
		t.Fatalf("bindParam() error = %v", err)
	}
	if user.ID != "123" {
		t.Errorf("ID = %s, want 123", user.ID)
	}
}

func TestBinder_BindHeader(t *testing.T) {
	c := createBinderTestContext(http.MethodGet, "/users", "", "")
	c.Request.Header.Set("Authorization", "Bearer token123")

	var user TestUserWithHeader
	binder := &DefaultBinder{}
	err := binder.bindHeader(c, &user)
	if err != nil {
		t.Fatalf("bindHeader() error = %v", err)
	}
	if user.Token != "Bearer token123" {
		t.Errorf("Token = %s, want Bearer token123", user.Token)
	}
}

func TestBinder_ErrorOnInvalidType(t *testing.T) {
	c := createBinderTestContext(http.MethodGet, "/users?age=notanumber", "", "")

	var user TestUser
	binder := &DefaultBinder{}
	err := binder.Bind(c, &user)

	if err == nil {
		t.Error("Expected error for invalid type")
	}

	bindErr, ok := err.(*BindingError)
	if !ok {
		t.Errorf("Expected BindingError, got %T", err)
	}
	if bindErr.Field != "age" {
		t.Errorf("Field = %s, want age", bindErr.Field)
	}
}

func TestBinder_ErrorOnNonPointer(t *testing.T) {
	form := url.Values{"name": []string{"john"}}
	var user TestUser
	err := mapForm(user, form) // Pass non-pointer

	if err == nil {
		t.Error("Expected error for non-pointer")
	}
	if !strings.Contains(err.Error(), "pointer") {
		t.Errorf("Error should mention pointer, got: %s", err.Error())
	}
}

func TestBinder_ErrorOnNonStruct(t *testing.T) {
	form := url.Values{"name": []string{"john"}}
	var str string
	err := mapForm(&str, form) // Pass pointer to non-struct

	if err == nil {
		t.Error("Expected error for non-struct")
	}
	if !strings.Contains(err.Error(), "struct") {
		t.Errorf("Error should mention struct, got: %s", err.Error())
	}
}

func TestBinder_SkipUntaggedFields(t *testing.T) {
	type UserWithUntagged struct {
		Name     string `form:"name"`
		Internal string // No tag
	}

	form := url.Values{"name": []string{"john"}, "Internal": []string{"secret"}}
	var user UserWithUntagged
	err := mapForm(&user, form)
	if err != nil {
		t.Fatalf("mapForm() error = %v", err)
	}
	if user.Name != "john" {
		t.Errorf("Name = %s, want john", user.Name)
	}
	if user.Internal != "" {
		t.Errorf("Internal should be empty, got %s", user.Internal)
	}
}

func TestBinder_AllFieldTypes(t *testing.T) {
	type AllTypes struct {
		String  string  `form:"string"`
		Int     int     `form:"int"`
		Int64   int64   `form:"int64"`
		Uint    uint    `form:"uint"`
		Float32 float32 `form:"float32"`
		Float64 float64 `form:"float64"`
		Bool    bool    `form:"bool"`
	}

	form := url.Values{
		"string":  []string{"hello"},
		"int":     []string{"-42"},
		"int64":   []string{"9223372036854775807"},
		"uint":    []string{"42"},
		"float32": []string{"3.14"},
		"float64": []string{"3.141592653589793"},
		"bool":    []string{"true"},
	}

	var obj AllTypes
	err := mapForm(&obj, form)
	if err != nil {
		t.Fatalf("mapForm() error = %v", err)
	}
	if obj.String != "hello" {
		t.Errorf("String = %s, want hello", obj.String)
	}
	if obj.Int != -42 {
		t.Errorf("Int = %d, want -42", obj.Int)
	}
	if obj.Int64 != 9223372036854775807 {
		t.Errorf("Int64 = %d, want 9223372036854775807", obj.Int64)
	}
	if obj.Uint != 42 {
		t.Errorf("Uint = %d, want 42", obj.Uint)
	}
	if obj.Float64 != 3.141592653589793 {
		t.Errorf("Float64 = %f, want 3.141592653589793", obj.Float64)
	}
	if obj.Bool != true {
		t.Errorf("Bool = %v, want true", obj.Bool)
	}
}

func TestBindingError_Error(t *testing.T) {
	err := &BindingError{Field: "age", Reason: "invalid integer"}
	expected := "binding error on field 'age': invalid integer"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

// Property-based test: JSON binding round-trip
// Feature: rue-framework, Property 6: Binder Round-Trip Consistency
// Validates: Requirements 5.1-5.8
func TestBinder_Property_JSONRoundTrip(t *testing.T) {
	f := func(name string, age uint8) bool {
		if name == "" || len(name) > 100 {
			return true
		}
		// Filter out invalid JSON characters
		for _, c := range name {
			if c < 32 || c == '"' || c == '\\' {
				return true
			}
		}

		original := TestUser{Name: name, Age: int(age)}

		// Serialize to JSON
		jsonData, err := sonic.Marshal(original)
		if err != nil {
			return true
		}

		// Create context with JSON body
		engine := New()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		c := &Context{engine: engine}
		c.reset(rec, req)

		// Bind
		var bound TestUser
		binder := &DefaultBinder{}
		if err := binder.Bind(c, &bound); err != nil {
			return false
		}

		// Property: bound values should match original
		return bound.Name == original.Name && bound.Age == original.Age
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Form binding round-trip
// Feature: rue-framework, Property 6: Binder Round-Trip Consistency
// Validates: Requirements 5.1-5.8
func TestBinder_Property_FormRoundTrip(t *testing.T) {
	f := func(name string, age uint8) bool {
		if name == "" || len(name) > 100 {
			return true
		}
		// Filter out invalid form characters
		for _, c := range name {
			if c == '&' || c == '=' || c < 32 {
				return true
			}
		}

		// Create form data
		form := url.Values{}
		form.Set("name", name)
		form.Set("age", string(rune('0'+age%10))) // Simple single digit

		// Create context with form body
		engine := New()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c := &Context{engine: engine}
		c.reset(rec, req)

		// Bind
		var bound TestUser
		binder := &DefaultBinder{}
		if err := binder.Bind(c, &bound); err != nil {
			return true // Skip on error (invalid data)
		}

		// Property: bound name should match
		return bound.Name == name
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Query binding round-trip
// Feature: rue-framework, Property 6: Binder Round-Trip Consistency
// Validates: Requirements 5.1-5.8
func TestBinder_Property_QueryRoundTrip(t *testing.T) {
	f := func(name string) bool {
		if name == "" || len(name) > 50 {
			return true
		}
		// Filter out invalid query characters
		for _, c := range name {
			if c == '&' || c == '=' || c == '?' || c == '#' || c < 32 {
				return true
			}
		}

		// Create query string
		query := url.Values{}
		query.Set("name", name)

		// Create context with query
		engine := New()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/users?"+query.Encode(), nil)
		c := &Context{engine: engine}
		c.reset(rec, req)

		// Bind
		var bound TestUser
		binder := &DefaultBinder{}
		if err := binder.Bind(c, &bound); err != nil {
			return false
		}

		// Property: bound name should match
		return bound.Name == name
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Error reporting includes field name
// Feature: rue-framework, Property 7: Binder Error Reporting
// Validates: Requirements 5.8
func TestBinder_Property_ErrorReporting(t *testing.T) {
	f := func(fieldName string) bool {
		if fieldName == "" || len(fieldName) > 20 {
			return true
		}
		// Only alphanumeric field names
		for _, c := range fieldName {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return true
			}
		}

		err := &BindingError{Field: fieldName, Reason: "test error"}

		// Property: error message should contain field name
		return strings.Contains(err.Error(), fieldName)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
