package testgen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser/v2/ast"
)

// Generator generates integration tests for GraphQL schemas
type Generator struct{}

// NewGenerator creates a new test generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateTests generates integration test files for a schema
func (g *Generator) GenerateTests(schema *ast.Schema, moduleName string) (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Generate main test file
	mainTest, err := g.generateMainTest(schema, moduleName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main test: %w", err)
	}
	files["integration_test.go"] = mainTest

	// Generate query tests
	if schema.Query != nil {
		queryTests, err := g.generateQueryTests(schema, moduleName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate query tests: %w", err)
		}
		files["query_test.go"] = queryTests
	}

	// Generate mutation tests
	if schema.Mutation != nil {
		mutationTests, err := g.generateMutationTests(schema, moduleName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate mutation tests: %w", err)
		}
		files["mutation_test.go"] = mutationTests
	}

	return files, nil
}

// generateMainTest generates the main test setup file
func (g *Generator) generateMainTest(schema *ast.Schema, moduleName string) ([]byte, error) {
	tmpl := `package integration_test

import (
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"{{.ModuleName}}/generated"
)

// TestServer creates a test server for integration tests
func TestServer(t *testing.T) *client.Client {
	t.Helper()

	// Create resolver - you need to implement this
	resolver := &Resolver{}

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: resolver,
	}))

	return client.New(httptest.NewServer(srv).Client(), httptest.NewServer(srv).URL)
}

// Resolver implements the resolver interface for tests
// TODO: Implement mock resolvers for your schema
type Resolver struct{}
`

	data := struct {
		ModuleName string
	}{
		ModuleName: moduleName,
	}

	return g.executeTemplate("main_test", tmpl, data)
}

// generateQueryTests generates tests for query operations
func (g *Generator) generateQueryTests(schema *ast.Schema, moduleName string) ([]byte, error) {
	tmpl := `package integration_test

import (
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/stretchr/testify/require"
)

{{range .Fields}}{{if not (.Name | isIntrospection)}}
func Test_Query_{{.Name | title}}(t *testing.T) {
	c := TestServer(t)

	var resp struct {
		{{.Name | title}} {{.Type | goType}}
	}

	{{if .Arguments}}
	// Query with arguments
	err := c.Post(` + "`" + `
		query {{.Name | title}}({{range $i, $arg := .Arguments}}{{if $i}}, {{end}}${{$arg.Name}}: {{$arg.Type}}{{end}}) {
			{{.Name}}({{range $i, $arg := .Arguments}}{{if $i}}, {{end}}{{$arg.Name}}: ${{$arg.Name}}{{end}}) {{if .Type | isObject}}{
				# TODO: Add fields to select
				__typename
			}{{end}}
		}
	` + "`" + `, &resp, client.Var("{{(index .Arguments 0).Name}}", {{(index .Arguments 0).Type | sampleValue}}))
	{{else}}
	// Query without arguments
	err := c.Post(` + "`" + `
		query {
			{{.Name}} {{if .Type | isObject}}{
				# TODO: Add fields to select
				__typename
			}{{end}}
		}
	` + "`" + `, &resp)
	{{end}}

	require.NoError(t, err)
	// TODO: Add assertions for {{.Name}}
}
{{end}}{{end}}
`

	data := struct {
		Fields []*ast.FieldDefinition
	}{
		Fields: schema.Query.Fields,
	}

	return g.executeTemplate("query_test", tmpl, data)
}

// generateMutationTests generates tests for mutation operations
func (g *Generator) generateMutationTests(schema *ast.Schema, moduleName string) ([]byte, error) {
	tmpl := `package integration_test

import (
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/stretchr/testify/require"
)

{{range .Fields}}
func Test_Mutation_{{.Name | title}}(t *testing.T) {
	c := TestServer(t)

	var resp struct {
		{{.Name | title}} {{.Type | goType}}
	}

	{{if .Arguments}}
	// Mutation with arguments
	err := c.Post(` + "`" + `
		mutation {{.Name | title}}({{range $i, $arg := .Arguments}}{{if $i}}, {{end}}${{$arg.Name}}: {{$arg.Type}}{{end}}) {
			{{.Name}}({{range $i, $arg := .Arguments}}{{if $i}}, {{end}}{{$arg.Name}}: ${{$arg.Name}}{{end}}) {{if .Type | isObject}}{
				# TODO: Add fields to select
				__typename
			}{{end}}
		}
	` + "`" + `, &resp{{range .Arguments}}, client.Var("{{.Name}}", {{.Type | sampleValue}}){{end}})
	{{else}}
	// Mutation without arguments
	err := c.Post(` + "`" + `
		mutation {
			{{.Name}} {{if .Type | isObject}}{
				# TODO: Add fields to select
				__typename
			}{{end}}
		}
	` + "`" + `, &resp)
	{{end}}

	require.NoError(t, err)
	// TODO: Add assertions for {{.Name}}
}
{{end}}
`

	data := struct {
		Fields []*ast.FieldDefinition
	}{
		Fields: schema.Mutation.Fields,
	}

	return g.executeTemplate("mutation_test", tmpl, data)
}

func (g *Generator) executeTemplate(name, tmplStr string, data any) ([]byte, error) {
	funcMap := template.FuncMap{
		"title": toTitle,
		"lower": strings.ToLower,
		"goType": func(t *ast.Type) string {
			return graphqlTypeToGo(t)
		},
		"isObject": func(t *ast.Type) bool {
			return isObjectType(t)
		},
		"sampleValue": func(t *ast.Type) string {
			return sampleValueForType(t)
		},
		"isIntrospection": func(name string) bool {
			return strings.HasPrefix(name, "__")
		},
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// graphqlTypeToGo converts GraphQL type to Go type string
func graphqlTypeToGo(t *ast.Type) string {
	if t == nil {
		return "any"
	}

	var result string
	if t.Elem != nil {
		// It's a list
		result = "[]" + graphqlTypeToGo(t.Elem)
	} else {
		switch t.NamedType {
		case "String":
			result = "string"
		case "Int":
			result = "int"
		case "Float":
			result = "float64"
		case "Boolean":
			result = "bool"
		case "ID":
			result = "string"
		default:
			result = "map[string]any" // For custom types
		}
	}

	if !t.NonNull {
		result = "*" + result
	}

	return result
}

// isObjectType checks if a type is an object (not scalar)
func isObjectType(t *ast.Type) bool {
	if t == nil {
		return false
	}
	if t.Elem != nil {
		return isObjectType(t.Elem)
	}
	switch t.NamedType {
	case "String", "Int", "Float", "Boolean", "ID":
		return false
	default:
		return true
	}
}

// sampleValueForType returns a sample value for testing
func sampleValueForType(t *ast.Type) string {
	if t == nil {
		return "nil"
	}
	if t.Elem != nil {
		return "[]any{}"
	}
	switch t.NamedType {
	case "String":
		return `"test"`
	case "Int":
		return "1"
	case "Float":
		return "1.0"
	case "Boolean":
		return "true"
	case "ID":
		return `"1"`
	default:
		return "map[string]any{}"
	}
}

// toTitle converts a string to title case (first letter uppercase)
func toTitle(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
