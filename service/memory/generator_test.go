package memory

import (
	"strings"
	"testing"
)

func TestInMemoryGenerator_Generate(t *testing.T) {
	gen := NewInMemoryGenerator()

	tests := []struct {
		name          string
		opts          ConfigOptions
		wantFiles     []string
		wantErr       bool
		checkContents map[string][]string // filename -> strings that should be present
	}{
		{
			name: "simple query schema",
			opts: ConfigOptions{
				Schema: `
type Query {
	hello: String!
}
`,
				ModuleName: "example.com/test",
			},
			wantFiles: []string{
				"schema.graphqls",
				"gqlgen.yml",
				"go.mod",
			},
			wantErr: false,
			checkContents: map[string][]string{
				"schema.graphqls": {"type Query", "hello: String!"},
				"go.mod":          {"module example.com/test"},
			},
		},
		{
			name: "schema with mutation",
			opts: ConfigOptions{
				Schema: `
type Query {
	hello: String!
}

type Mutation {
	setMessage(msg: String!): String!
}
`,
				ModuleName: "example.com/myapp",
			},
			wantFiles: []string{
				"schema.graphqls",
				"gqlgen.yml",
				"go.mod",
			},
			wantErr: false,
			checkContents: map[string][]string{
				"schema.graphqls": {"type Query", "type Mutation"},
				"go.mod":          {"module example.com/myapp"},
			},
		},
		{
			name: "invalid schema",
			opts: ConfigOptions{
				Schema:     "invalid { schema",
				ModuleName: "test",
			},
			wantErr: true,
		},
		{
			name: "empty schema generates minimal output",
			opts: ConfigOptions{
				Schema:     "",
				ModuleName: "test",
			},
			// Empty schema still generates config files
			wantFiles: []string{
				"schema.graphqls",
				"gqlgen.yml",
				"go.mod",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := gen.Generate(tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check expected files exist
			for _, wantFile := range tt.wantFiles {
				found := false
				for path := range files {
					if strings.HasSuffix(path, wantFile) || path == wantFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %s not found in generated files: %v", wantFile, listKeys(files))
				}
			}

			// Check file contents
			for filename, expectedStrings := range tt.checkContents {
				var content []byte
				var found bool
				for path, c := range files {
					if strings.HasSuffix(path, filename) || path == filename {
						content = c
						found = true
						break
					}
				}
				if !found {
					t.Errorf("file %s not found for content check", filename)
					continue
				}

				contentStr := string(content)
				for _, expected := range expectedStrings {
					if !strings.Contains(contentStr, expected) {
						t.Errorf("file %s missing expected content: %s", filename, expected)
					}
				}
			}
		})
	}
}

func TestConfigBuilder_BuildConfig(t *testing.T) {
	cb := NewConfigBuilder()

	tests := []struct {
		name       string
		schema     string
		moduleName string
		wantErr    bool
	}{
		{
			name: "valid simple schema",
			schema: `
type Query {
	hello: String!
}
`,
			moduleName: "example.com/test",
			wantErr:    false,
		},
		{
			name: "schema with enum",
			schema: `
type Query {
	status: Status!
}

enum Status {
	ACTIVE
	INACTIVE
	PENDING
}
`,
			moduleName: "example.com/test",
			wantErr:    false,
		},
		{
			name: "schema with interface",
			schema: `
type Query {
	node(id: ID!): Node
}

interface Node {
	id: ID!
}

type User implements Node {
	id: ID!
	name: String!
}
`,
			moduleName: "example.com/test",
			wantErr:    false,
		},
		{
			name:       "invalid schema",
			schema:     "not valid graphql",
			moduleName: "test",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := cb.BuildConfig(tt.schema, tt.moduleName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg == nil {
				t.Fatal("expected config but got nil")
			}

			if cfg.Schema == nil {
				t.Error("expected schema to be parsed")
			}
		})
	}
}

func TestConfigBuilder_BuildConfigWithOptions(t *testing.T) {
	cb := NewConfigBuilder()

	schema := `
type Query {
	hello: String!
}
`

	opts := ConfigOptions{
		Schema:                   schema,
		ModuleName:               "example.com/custom",
		ExecPackage:              "exec",
		ModelPackage:             "model",
		ResolverPackage:          "resolver",
		OmitSliceElementPointers: true,
		OmitGetters:              true,
	}

	cfg, err := cb.BuildConfigWithOptions(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Exec.Package != "exec" {
		t.Errorf("expected exec package 'exec', got '%s'", cfg.Exec.Package)
	}

	if cfg.Model.Package != "model" {
		t.Errorf("expected model package 'model', got '%s'", cfg.Model.Package)
	}

	if cfg.Resolver.Package != "resolver" {
		t.Errorf("expected resolver package 'resolver', got '%s'", cfg.Resolver.Package)
	}

	if !cfg.OmitSliceElementPointers {
		t.Error("expected OmitSliceElementPointers to be true")
	}

	if !cfg.OmitGetters {
		t.Error("expected OmitGetters to be true")
	}
}

func TestVirtualPackages(t *testing.T) {
	vp := NewVirtualPackages()

	t.Run("basic types", func(t *testing.T) {
		typ, err := vp.GetBasicType("string")
		if err != nil {
			t.Fatalf("failed to get string type: %v", err)
		}
		if typ == nil {
			t.Error("expected string type but got nil")
		}

		typ, err = vp.GetBasicType("int")
		if err != nil {
			t.Fatalf("failed to get int type: %v", err)
		}
		if typ == nil {
			t.Error("expected int type but got nil")
		}
	})

	t.Run("register package", func(t *testing.T) {
		pkg := vp.RegisterPackage("example.com/test", "test")
		if pkg == nil {
			t.Error("expected package but got nil")
		}
		if pkg.Path() != "example.com/test" {
			t.Errorf("expected path 'example.com/test', got '%s'", pkg.Path())
		}
		if pkg.Name() != "test" {
			t.Errorf("expected name 'test', got '%s'", pkg.Name())
		}
	})

	t.Run("register and lookup type", func(t *testing.T) {
		vp.RegisterPackage("example.com/types", "types")

		stringType, _ := vp.GetBasicType("string")
		err := vp.RegisterType("example.com/types", "MyString", stringType)
		if err != nil {
			t.Fatalf("failed to register type: %v", err)
		}

		typ, err := vp.LookupType("example.com/types", "MyString")
		if err != nil {
			t.Fatalf("failed to lookup type: %v", err)
		}
		if typ == nil {
			t.Error("expected type but got nil")
		}
	})

	t.Run("create named type", func(t *testing.T) {
		stringType, _ := vp.GetBasicType("string")
		named := vp.CreateNamedType("example.com/models", "User", stringType)
		if named == nil {
			t.Error("expected named type but got nil")
		}
	})
}

func TestMemoryRenderer(t *testing.T) {
	r := NewRenderer()

	t.Run("add and get file", func(t *testing.T) {
		content := []byte("package main\n\nfunc main() {}\n")
		r.AddFile("main.go", content)

		got, ok := r.GetFile("main.go")
		if !ok {
			t.Error("expected file to exist")
		}
		if string(got) != string(content) {
			t.Errorf("content mismatch: got %s, want %s", got, content)
		}
	})

	t.Run("file count", func(t *testing.T) {
		r.Clear()
		r.AddFile("a.go", []byte("a"))
		r.AddFile("b.go", []byte("b"))
		r.AddFile("c.go", []byte("c"))

		if r.FileCount() != 3 {
			t.Errorf("expected 3 files, got %d", r.FileCount())
		}
	})

	t.Run("has file", func(t *testing.T) {
		r.Clear()
		r.AddFile("exists.go", []byte("exists"))

		if !r.HasFile("exists.go") {
			t.Error("expected file to exist")
		}
		if r.HasFile("notexists.go") {
			t.Error("expected file to not exist")
		}
	})

	t.Run("list files", func(t *testing.T) {
		r.Clear()
		r.AddFile("a.go", []byte("a"))
		r.AddFile("b.go", []byte("b"))

		files := r.ListFiles()
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("remove file", func(t *testing.T) {
		r.Clear()
		r.AddFile("remove.go", []byte("remove"))
		r.RemoveFile("remove.go")

		if r.HasFile("remove.go") {
			t.Error("expected file to be removed")
		}
	})

	t.Run("clear", func(t *testing.T) {
		r.AddFile("a.go", []byte("a"))
		r.AddFile("b.go", []byte("b"))
		r.Clear()

		if r.FileCount() != 0 {
			t.Errorf("expected 0 files after clear, got %d", r.FileCount())
		}
	})
}

func listKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
