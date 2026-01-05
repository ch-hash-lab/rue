package rue

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGraphQL_ScalarTypes(t *testing.T) {
	if GraphQLString.Name() != "String" {
		t.Errorf("GraphQLString.Name() = %s, want String", GraphQLString.Name())
	}
	if GraphQLInt.Name() != "Int" {
		t.Errorf("GraphQLInt.Name() = %s, want Int", GraphQLInt.Name())
	}
	if GraphQLFloat.Name() != "Float" {
		t.Errorf("GraphQLFloat.Name() = %s, want Float", GraphQLFloat.Name())
	}
	if GraphQLBoolean.Name() != "Boolean" {
		t.Errorf("GraphQLBoolean.Name() = %s, want Boolean", GraphQLBoolean.Name())
	}
	if GraphQLID.Name() != "ID" {
		t.Errorf("GraphQLID.Name() = %s, want ID", GraphQLID.Name())
	}
}

func TestGraphQL_ObjectType(t *testing.T) {
	userType := NewObjectType("User", "A user in the system")

	if userType.Name() != "User" {
		t.Errorf("Name() = %s, want User", userType.Name())
	}
	if userType.Description() != "A user in the system" {
		t.Errorf("Description() = %s, want 'A user in the system'", userType.Description())
	}

	userType.AddField(&Field{
		Name: "id",
		Type: GraphQLID,
	})
	userType.AddField(&Field{
		Name: "name",
		Type: GraphQLString,
	})

	if len(userType.fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(userType.fields))
	}
}

func TestGraphQL_Schema(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query type")
	mutationType := NewObjectType("Mutation", "Root mutation type")

	schema.SetQuery(queryType)
	schema.SetMutation(mutationType)

	if schema.Query != queryType {
		t.Error("Query type not set correctly")
	}
	if schema.Mutation != mutationType {
		t.Error("Mutation type not set correctly")
	}
}

func TestGraphQL_ParseSimpleQuery(t *testing.T) {
	query := `{ hello }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	if op.Type != "query" {
		t.Errorf("Type = %s, want query", op.Type)
	}

	if len(op.Selections) != 1 {
		t.Fatalf("Selections count = %d, want 1", len(op.Selections))
	}

	if op.Selections[0].Name != "hello" {
		t.Errorf("Selection name = %s, want hello", op.Selections[0].Name)
	}
}

func TestGraphQL_ParseQueryWithName(t *testing.T) {
	query := `query GetUser { user { id name } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	if op.Type != "query" {
		t.Errorf("Type = %s, want query", op.Type)
	}

	if op.Name != "GetUser" {
		t.Errorf("Name = %s, want GetUser", op.Name)
	}
}

func TestGraphQL_ParseMutation(t *testing.T) {
	query := `mutation { createUser(name: "John") { id } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	if op.Type != "mutation" {
		t.Errorf("Type = %s, want mutation", op.Type)
	}
}

func TestGraphQL_ParseArguments(t *testing.T) {
	query := `{ user(id: 123) { name } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	if len(op.Selections) != 1 {
		t.Fatalf("Selections count = %d, want 1", len(op.Selections))
	}

	sel := op.Selections[0]
	if sel.Name != "user" {
		t.Errorf("Selection name = %s, want user", sel.Name)
	}

	if sel.Args["id"] != 123 {
		t.Errorf("Arg id = %v, want 123", sel.Args["id"])
	}
}

func TestGraphQL_ParseStringArgument(t *testing.T) {
	query := `{ user(name: "John") { id } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	sel := op.Selections[0]
	if sel.Args["name"] != "John" {
		t.Errorf("Arg name = %v, want John", sel.Args["name"])
	}
}

func TestGraphQL_ParseBooleanArgument(t *testing.T) {
	query := `{ users(active: true) { id } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	sel := op.Selections[0]
	if sel.Args["active"] != true {
		t.Errorf("Arg active = %v, want true", sel.Args["active"])
	}
}

func TestGraphQL_ParseNestedSelections(t *testing.T) {
	query := `{ user { id name posts { title } } }`

	op, err := parseQuery(query)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	if len(op.Selections) != 1 {
		t.Fatalf("Selections count = %d, want 1", len(op.Selections))
	}

	userSel := op.Selections[0]
	if userSel.Name != "user" {
		t.Errorf("Selection name = %s, want user", userSel.Name)
	}

	if len(userSel.Selections) != 3 {
		t.Errorf("Nested selections count = %d, want 3", len(userSel.Selections))
	}
}

func TestGraphQL_Execute(t *testing.T) {
	// Create schema
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "hello",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return "world", nil
		},
	})

	schema.SetQuery(queryType)

	// Create executor
	executor := NewExecutor(schema)

	// Execute query
	req := &GraphQLRequest{
		Query: `{ hello }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data is not a map")
	}

	if data["hello"] != "world" {
		t.Errorf("hello = %v, want world", data["hello"])
	}
}

func TestGraphQL_ExecuteWithArgs(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "greet",
		Type: GraphQLString,
		Args: []*Argument{
			{Name: "name", Type: GraphQLString},
		},
		Resolve: func(ctx *ResolveContext) (any, error) {
			name := ctx.Args["name"].(string)
			return "Hello, " + name, nil
		},
	})

	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `{ greet(name: "John") }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	if data["greet"] != "Hello, John" {
		t.Errorf("greet = %v, want 'Hello, John'", data["greet"])
	}
}

func TestGraphQL_ExecuteWithVariables(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "greet",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			name := ctx.Args["name"].(string)
			return "Hello, " + name, nil
		},
	})

	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query:     `{ greet(name: $name) }`,
		Variables: map[string]any{"name": "Jane"},
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	if data["greet"] != "Hello, Jane" {
		t.Errorf("greet = %v, want 'Hello, Jane'", data["greet"])
	}
}

func TestGraphQL_ExecuteNestedQuery(t *testing.T) {
	// Create types
	postType := NewObjectType("Post", "A blog post")
	postType.AddField(&Field{Name: "id", Type: GraphQLID})
	postType.AddField(&Field{Name: "title", Type: GraphQLString})

	userType := NewObjectType("User", "A user")
	userType.AddField(&Field{Name: "id", Type: GraphQLID})
	userType.AddField(&Field{Name: "name", Type: GraphQLString})
	userType.AddField(&Field{
		Name: "posts",
		Type: postType,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return []map[string]any{
				{"id": "1", "title": "First Post"},
				{"id": "2", "title": "Second Post"},
			}, nil
		},
	})

	schema := NewSchema()
	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "user",
		Type: userType,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return map[string]any{
				"id":   "123",
				"name": "John",
			}, nil
		},
	})
	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `{ user { id name } }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	user := data["user"].(map[string]any)

	if user["id"] != "123" {
		t.Errorf("user.id = %v, want 123", user["id"])
	}
	if user["name"] != "John" {
		t.Errorf("user.name = %v, want John", user["name"])
	}
}

func TestGraphQL_Handler(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "hello",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return "world", nil
		},
	})
	schema.SetQuery(queryType)

	engine := New()
	engine.POST("/graphql", GraphQL(schema))

	// Test query
	reqBody := `{"query": "{ hello }"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	if data["hello"] != "world" {
		t.Errorf("hello = %v, want world", data["hello"])
	}
}

func TestGraphQL_Playground(t *testing.T) {
	schema := NewSchema()
	queryType := NewObjectType("Query", "Root query")
	schema.SetQuery(queryType)

	engine := New()
	engine.GET("/graphql", GraphQL(schema))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/graphql", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %s, want text/html", w.Header().Get("Content-Type"))
	}

	if !strings.Contains(w.Body.String(), "GraphQL Playground") {
		t.Error("Response should contain GraphQL Playground")
	}
}

func TestGraphQL_InvalidRequest(t *testing.T) {
	schema := NewSchema()
	queryType := NewObjectType("Query", "Root query")
	schema.SetQuery(queryType)

	engine := New()
	engine.POST("/graphql", GraphQL(schema))

	// Invalid JSON
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/graphql", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGraphQL_FieldNotFound(t *testing.T) {
	schema := NewSchema()
	queryType := NewObjectType("Query", "Root query")
	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `{ nonexistent }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) == 0 {
		t.Error("Expected error for nonexistent field")
	}
}

func TestGraphQL_MutationNotDefined(t *testing.T) {
	schema := NewSchema()
	queryType := NewObjectType("Query", "Root query")
	schema.SetQuery(queryType)
	// No mutation type set

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `mutation { createUser }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) == 0 {
		t.Error("Expected error for undefined mutation type")
	}
}

func TestGraphQL_ParseValue(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{`"hello"`, "hello"},
		{`123`, 123},
		{`12.5`, 12.5},
		{`true`, true},
		{`false`, false},
		{`null`, nil},
	}

	for _, tt := range tests {
		result := parseValue(tt.input)
		if result != tt.expected {
			t.Errorf("parseValue(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestGraphQL_GetFieldValue(t *testing.T) {
	type User struct {
		ID   string
		Name string
	}

	user := User{ID: "123", Name: "John"}

	// Exact match
	if getFieldValue(user, "ID") != "123" {
		t.Error("Failed to get ID field")
	}

	// Case-insensitive match
	if getFieldValue(user, "name") != "John" {
		t.Error("Failed to get name field (case-insensitive)")
	}

	// Non-existent field
	if getFieldValue(user, "nonexistent") != nil {
		t.Error("Should return nil for non-existent field")
	}

	// Pointer to struct
	if getFieldValue(&user, "ID") != "123" {
		t.Error("Failed to get field from pointer")
	}
}

func TestGraphQL_Alias(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "hello",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return "world", nil
		},
	})
	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `{ greeting: hello }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	if data["greeting"] != "world" {
		t.Errorf("greeting = %v, want world", data["greeting"])
	}
}

func TestGraphQL_MultipleFields(t *testing.T) {
	schema := NewSchema()

	queryType := NewObjectType("Query", "Root query")
	queryType.AddField(&Field{
		Name: "hello",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return "world", nil
		},
	})
	queryType.AddField(&Field{
		Name: "goodbye",
		Type: GraphQLString,
		Resolve: func(ctx *ResolveContext) (any, error) {
			return "farewell", nil
		},
	})
	schema.SetQuery(queryType)

	executor := NewExecutor(schema)

	req := &GraphQLRequest{
		Query: `{ hello goodbye }`,
	}

	resp := executor.Execute(nil, req)

	if len(resp.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", resp.Errors)
	}

	data := resp.Data.(map[string]any)
	if data["hello"] != "world" {
		t.Errorf("hello = %v, want world", data["hello"])
	}
	if data["goodbye"] != "farewell" {
		t.Errorf("goodbye = %v, want farewell", data["goodbye"])
	}
}
