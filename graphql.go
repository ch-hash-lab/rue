package rue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// ============== GraphQL Types ==============

// GraphQLType represents a GraphQL type
type GraphQLType interface {
	Name() string
	Description() string
}

// ScalarType represents a GraphQL scalar type
type ScalarType struct {
	name        string
	description string
}

func (s *ScalarType) Name() string        { return s.name }
func (s *ScalarType) Description() string { return s.description }

// Built-in scalar types
var (
	GraphQLString  = &ScalarType{name: "String", description: "A UTF-8 string"}
	GraphQLInt     = &ScalarType{name: "Int", description: "A 32-bit integer"}
	GraphQLFloat   = &ScalarType{name: "Float", description: "A floating point number"}
	GraphQLBoolean = &ScalarType{name: "Boolean", description: "A boolean value"}
	GraphQLID      = &ScalarType{name: "ID", description: "A unique identifier"}
)

// ObjectType represents a GraphQL object type
type ObjectType struct {
	name        string
	description string
	fields      map[string]*Field
}

func (o *ObjectType) Name() string        { return o.name }
func (o *ObjectType) Description() string { return o.description }

// NewObjectType creates a new object type
func NewObjectType(name, description string) *ObjectType {
	return &ObjectType{
		name:        name,
		description: description,
		fields:      make(map[string]*Field),
	}
}

// AddField adds a field to the object type
func (o *ObjectType) AddField(field *Field) {
	o.fields[field.Name] = field
}

// Field represents a GraphQL field
type Field struct {
	Name        string
	Description string
	Type        GraphQLType
	Args        []*Argument
	Resolve     ResolveFunc
}

// Argument represents a GraphQL argument
type Argument struct {
	Name         string
	Description  string
	Type         GraphQLType
	DefaultValue any
}

// ResolveFunc is the resolver function type
type ResolveFunc func(ctx *ResolveContext) (any, error)

// ResolveContext contains context for field resolution
type ResolveContext struct {
	Context *Context
	Source  any
	Args    map[string]any
	Info    *ResolveInfo
}

// ResolveInfo contains information about the current resolution
type ResolveInfo struct {
	FieldName  string
	ReturnType GraphQLType
	ParentType GraphQLType
}

// ============== GraphQL Schema ==============

// Schema represents a GraphQL schema
type Schema struct {
	Query    *ObjectType
	Mutation *ObjectType
}

// NewSchema creates a new GraphQL schema
func NewSchema() *Schema {
	return &Schema{}
}

// SetQuery sets the query type
func (s *Schema) SetQuery(query *ObjectType) {
	s.Query = query
}

// SetMutation sets the mutation type
func (s *Schema) SetMutation(mutation *ObjectType) {
	s.Mutation = mutation
}

// ============== GraphQL Request/Response ==============

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   any            `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message   string     `json:"message"`
	Locations []Location `json:"locations,omitempty"`
	Path      []any      `json:"path,omitempty"`
}

// Location represents a location in the query
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// ============== GraphQL Parser (Simplified) ==============

// Operation represents a GraphQL operation
type Operation struct {
	Type       string // "query" or "mutation"
	Name       string
	Selections []Selection
}

// Selection represents a field selection
type Selection struct {
	Name       string
	Alias      string
	Args       map[string]any
	Selections []Selection
}

// parseQuery parses a simplified GraphQL query
func parseQuery(query string) (*Operation, error) {
	query = strings.TrimSpace(query)

	op := &Operation{
		Type: "query",
	}

	// Detect operation type
	if strings.HasPrefix(query, "mutation") {
		op.Type = "mutation"
		query = strings.TrimPrefix(query, "mutation")
	} else if strings.HasPrefix(query, "query") {
		query = strings.TrimPrefix(query, "query")
	}

	// Find operation name (if any)
	query = strings.TrimSpace(query)
	if !strings.HasPrefix(query, "{") {
		// Has operation name
		idx := strings.Index(query, "{")
		if idx == -1 {
			return nil, fmt.Errorf("invalid query: missing opening brace")
		}
		namePart := strings.TrimSpace(query[:idx])
		// Remove variables part if present
		if parenIdx := strings.Index(namePart, "("); parenIdx != -1 {
			op.Name = strings.TrimSpace(namePart[:parenIdx])
		} else {
			op.Name = namePart
		}
		query = query[idx:]
	}

	// Parse selections
	selections, err := parseSelections(query)
	if err != nil {
		return nil, err
	}
	op.Selections = selections

	return op, nil
}

// parseSelections parses field selections
func parseSelections(query string) ([]Selection, error) {
	query = strings.TrimSpace(query)
	if !strings.HasPrefix(query, "{") {
		return nil, fmt.Errorf("expected '{'")
	}
	query = query[1:] // Remove opening brace

	var selections []Selection
	depth := 0

	for len(query) > 0 {
		query = strings.TrimSpace(query)

		if strings.HasPrefix(query, "}") {
			break
		}

		// Parse field name
		var fieldName string
		var alias string
		var args map[string]any

		// Check for alias
		colonIdx := strings.Index(query, ":")
		spaceIdx := strings.Index(query, " ")
		braceIdx := strings.Index(query, "{")
		parenIdx := strings.Index(query, "(")
		newlineIdx := strings.Index(query, "\n")

		// Find end of field name
		endIdx := len(query)
		for _, idx := range []int{colonIdx, spaceIdx, braceIdx, parenIdx, newlineIdx} {
			if idx > 0 && idx < endIdx {
				endIdx = idx
			}
		}

		fieldName = strings.TrimSpace(query[:endIdx])
		query = query[endIdx:]

		// Check for alias
		query = strings.TrimSpace(query)
		if strings.HasPrefix(query, ":") {
			alias = fieldName
			query = query[1:]
			query = strings.TrimSpace(query)

			// Get actual field name
			endIdx = len(query)
			for _, idx := range []int{
				strings.Index(query, " "),
				strings.Index(query, "{"),
				strings.Index(query, "("),
				strings.Index(query, "\n"),
			} {
				if idx > 0 && idx < endIdx {
					endIdx = idx
				}
			}
			fieldName = strings.TrimSpace(query[:endIdx])
			query = query[endIdx:]
		}

		// Parse arguments
		query = strings.TrimSpace(query)
		if strings.HasPrefix(query, "(") {
			args, query = parseArguments(query)
		}

		// Parse nested selections
		var nestedSelections []Selection
		query = strings.TrimSpace(query)
		if strings.HasPrefix(query, "{") {
			// Find matching closing brace
			depth = 1
			endIdx := 1
			for i := 1; i < len(query) && depth > 0; i++ {
				if query[i] == '{' {
					depth++
				} else if query[i] == '}' {
					depth--
				}
				endIdx = i + 1
			}

			nestedQuery := query[:endIdx]
			var err error
			nestedSelections, err = parseSelections(nestedQuery)
			if err != nil {
				return nil, err
			}
			query = query[endIdx:]
		}

		if fieldName != "" {
			selections = append(selections, Selection{
				Name:       fieldName,
				Alias:      alias,
				Args:       args,
				Selections: nestedSelections,
			})
		}
	}

	return selections, nil
}

// parseArguments parses field arguments
func parseArguments(query string) (map[string]any, string) {
	args := make(map[string]any)

	if !strings.HasPrefix(query, "(") {
		return args, query
	}

	// Find closing paren
	depth := 1
	endIdx := 1
	for i := 1; i < len(query) && depth > 0; i++ {
		if query[i] == '(' {
			depth++
		} else if query[i] == ')' {
			depth--
		}
		endIdx = i + 1
	}

	argsStr := query[1 : endIdx-1]
	query = query[endIdx:]

	// Parse key-value pairs
	pairs := strings.Split(argsStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse value
		args[key] = parseValue(value)
	}

	return args, query
}

// parseValue parses a GraphQL value
func parseValue(value string) any {
	value = strings.TrimSpace(value)

	// String
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1 : len(value)-1]
	}

	// Boolean
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Null
	if value == "null" {
		return nil
	}

	// Number
	var num float64
	if _, err := fmt.Sscanf(value, "%f", &num); err == nil {
		// Check if it's an integer
		if num == float64(int(num)) {
			return int(num)
		}
		return num
	}

	return value
}

// ============== GraphQL Executor ==============

// Executor executes GraphQL operations
type Executor struct {
	schema *Schema
}

// NewExecutor creates a new executor
func NewExecutor(schema *Schema) *Executor {
	return &Executor{schema: schema}
}

// Execute executes a GraphQL request
func (e *Executor) Execute(ctx *Context, req *GraphQLRequest) *GraphQLResponse {
	// Parse query
	op, err := parseQuery(req.Query)
	if err != nil {
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: err.Error()}},
		}
	}

	// Get root type
	var rootType *ObjectType
	if op.Type == "mutation" {
		rootType = e.schema.Mutation
	} else {
		rootType = e.schema.Query
	}

	if rootType == nil {
		return &GraphQLResponse{
			Errors: []GraphQLError{{Message: fmt.Sprintf("%s type not defined", op.Type)}},
		}
	}

	// Execute selections
	data, errs := e.executeSelections(ctx, rootType, nil, op.Selections, req.Variables)

	var errors []GraphQLError
	for _, err := range errs {
		errors = append(errors, GraphQLError{Message: err.Error()})
	}

	return &GraphQLResponse{
		Data:   data,
		Errors: errors,
	}
}

// executeSelections executes field selections
func (e *Executor) executeSelections(ctx *Context, objectType *ObjectType, source any, selections []Selection, variables map[string]any) (map[string]any, []error) {
	result := make(map[string]any)
	var errors []error

	for _, sel := range selections {
		field, ok := objectType.fields[sel.Name]
		if !ok {
			errors = append(errors, fmt.Errorf("field '%s' not found on type '%s'", sel.Name, objectType.Name()))
			continue
		}

		// Resolve arguments
		args := make(map[string]any)
		for k, v := range sel.Args {
			// Check for variable reference
			if str, ok := v.(string); ok && strings.HasPrefix(str, "$") {
				varName := str[1:]
				if varValue, ok := variables[varName]; ok {
					args[k] = varValue
				}
			} else {
				args[k] = v
			}
		}

		// Create resolve context
		resolveCtx := &ResolveContext{
			Context: ctx,
			Source:  source,
			Args:    args,
			Info: &ResolveInfo{
				FieldName:  sel.Name,
				ReturnType: field.Type,
				ParentType: objectType,
			},
		}

		// Resolve field
		var value any
		var err error
		if field.Resolve != nil {
			value, err = field.Resolve(resolveCtx)
		} else if source != nil {
			// Default resolver: get field from source
			value = getFieldValue(source, sel.Name)
		}

		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Handle nested selections
		if len(sel.Selections) > 0 {
			if nestedType, ok := field.Type.(*ObjectType); ok {
				valueType := reflect.TypeOf(value)
				if valueType != nil {
					switch valueType.Kind() {
					case reflect.Slice:
						slice := reflect.ValueOf(value)
						var items []map[string]any
						for i := 0; i < slice.Len(); i++ {
							item, itemErrs := e.executeSelections(ctx, nestedType, slice.Index(i).Interface(), sel.Selections, variables)
							items = append(items, item)
							errors = append(errors, itemErrs...)
						}
						value = items
					case reflect.Map:
						// Handle map[string]any
						value, _ = e.executeSelectionsFromMap(ctx, nestedType, value, sel.Selections, variables)
					default:
						value, _ = e.executeSelections(ctx, nestedType, value, sel.Selections, variables)
					}
				}
			}
		}

		// Use alias if provided
		key := sel.Name
		if sel.Alias != "" {
			key = sel.Alias
		}
		result[key] = value
	}

	return result, errors
}

// executeSelectionsFromMap handles map[string]any sources
func (e *Executor) executeSelectionsFromMap(ctx *Context, objectType *ObjectType, source any, selections []Selection, variables map[string]any) (map[string]any, []error) {
	result := make(map[string]any)
	var errors []error

	sourceMap, ok := source.(map[string]any)
	if !ok {
		return result, errors
	}

	for _, sel := range selections {
		// Get value from map
		value, exists := sourceMap[sel.Name]
		if !exists {
			// Try case-insensitive match
			for k, v := range sourceMap {
				if strings.EqualFold(k, sel.Name) {
					value = v
					exists = true
					break
				}
			}
		}

		// Handle nested selections
		if len(sel.Selections) > 0 {
			if nestedType, ok := objectType.fields[sel.Name]; ok {
				if nestedObjType, ok := nestedType.Type.(*ObjectType); ok {
					valueType := reflect.TypeOf(value)
					if valueType != nil && valueType.Kind() == reflect.Slice {
						slice := reflect.ValueOf(value)
						var items []map[string]any
						for i := 0; i < slice.Len(); i++ {
							item, itemErrs := e.executeSelectionsFromMap(ctx, nestedObjType, slice.Index(i).Interface(), sel.Selections, variables)
							items = append(items, item)
							errors = append(errors, itemErrs...)
						}
						value = items
					} else if valueType != nil && valueType.Kind() == reflect.Map {
						value, _ = e.executeSelectionsFromMap(ctx, nestedObjType, value, sel.Selections, variables)
					}
				}
			}
		}

		// Use alias if provided
		key := sel.Name
		if sel.Alias != "" {
			key = sel.Alias
		}
		result[key] = value
	}

	return result, errors
}

// getFieldValue gets a field value from a struct
func getFieldValue(source any, fieldName string) any {
	val := reflect.ValueOf(source)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	// Try exact match
	field := val.FieldByName(fieldName)
	if field.IsValid() {
		return field.Interface()
	}

	// Try case-insensitive match
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		if strings.EqualFold(typ.Field(i).Name, fieldName) {
			return val.Field(i).Interface()
		}
	}

	return nil
}

// ============== GraphQL Handler ==============

// GraphQLConfig defines GraphQL handler configuration
type GraphQLConfig struct {
	Schema     *Schema
	Playground bool
}

// GraphQL returns a GraphQL handler
func GraphQL(schema *Schema) HandlerFunc {
	return GraphQLWithConfig(GraphQLConfig{
		Schema:     schema,
		Playground: true,
	})
}

// GraphQLWithConfig returns a GraphQL handler with custom config
func GraphQLWithConfig(config GraphQLConfig) HandlerFunc {
	executor := NewExecutor(config.Schema)

	return func(c *Context) {
		// Serve playground for GET requests
		if c.Request.Method == "GET" && config.Playground {
			servePlayground(c)
			return
		}

		// Parse request
		var req GraphQLRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, H{
				"errors": []GraphQLError{{Message: "Invalid request body"}},
			})
			return
		}

		// Execute query
		resp := executor.Execute(c, &req)

		// Return response
		c.JSON(http.StatusOK, resp)
	}
}

// servePlayground serves the GraphQL Playground HTML
func servePlayground(c *Context) {
	html := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: window.location.href
      })
    })
  </script>
</body>
</html>`

	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	c.Writer.Write([]byte(html))
}

// ============== Helper Functions ==============

// MarshalJSON marshals a GraphQL response to JSON
func (r *GraphQLResponse) MarshalJSON() ([]byte, error) {
	type Alias GraphQLResponse
	return json.Marshal((*Alias)(r))
}
