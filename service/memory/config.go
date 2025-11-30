package memory

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/internal/code"
)

// ConfigBuilder builds in-memory configurations
type ConfigBuilder struct {
	virtualPackages *VirtualPackages
}

// NewConfigBuilder creates a new config builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		virtualPackages: NewVirtualPackages(),
	}
}

// BuildConfig creates a config from a schema string
func (cb *ConfigBuilder) BuildConfig(schema string, moduleName string) (*config.Config, error) {
	// Parse schema
	source := &ast.Source{
		Name:  "schema.graphqls",
		Input: schema,
	}

	parsedSchema, err := gqlparser.LoadSchema(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Create config
	cfg := config.DefaultConfig()
	cfg.Schema = parsedSchema
	cfg.Sources = []*ast.Source{source}

	// Set module name
	if moduleName == "" {
		moduleName = "generated"
	}

	// Get current working directory for absolute paths
	wd, _ := os.Getwd()

	// Configure exec
	cfg.Exec.Filename = filepath.Join(wd, "generated/generated.go")
	cfg.Exec.Package = "generated"
	cfg.Exec.Layout = config.ExecLayoutSingleFile

	// Configure model
	cfg.Model.Filename = filepath.Join(wd, "generated/models_gen.go")
	cfg.Model.Package = "generated"

	// Configure resolver
	cfg.Resolver.Filename = filepath.Join(wd, "resolver.go")
	cfg.Resolver.Package = "main"
	cfg.Resolver.Type = "Resolver"
	cfg.Resolver.Layout = config.LayoutSingleFile

	// Skip filesystem operations
	cfg.SkipValidation = true
	cfg.SkipModTidy = true
	cfg.SkipPackageLoading = true
	cfg.SkipExistingResolvers = true
	cfg.Model = config.PackageConfig{
		Filename:   "generated/models_gen.go",
		Package:    "generated",
		ImportPath: moduleName + "/generated",
	}
	cfg.Exec = config.ExecConfig{
		Filename:   "generated/generated.go",
		Package:    "generated",
		ImportPath: moduleName + "/generated",
	}

	if err := cfg.LoadSchema(); err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	// Initialize packages to avoid disk I/O
	cfg.Packages = &code.Packages{}

	// Pre-populate common packages to avoid loading them from disk
	// Map import path -> package name
	commonPackages := map[string]string{
		"fmt":     "fmt",
		"io":      "io",
		"strconv": "strconv",
		"time":    "time",
		"sync":    "sync",
		"context": "context",
		"errors":  "errors",
		"bytes":   "bytes",
		"github.com/vektah/gqlparser/v2":                    "gqlparser",
		"github.com/vektah/gqlparser/v2/ast":                "ast",
		"github.com/99designs/gqlgen/graphql":               "graphql",
		"github.com/99designs/gqlgen/graphql/introspection": "introspection",
	}

	for importPath, pkgName := range commonPackages {
		cfg.Packages.AddName(importPath, pkgName)
	}

	// Add the module itself
	cfg.Packages.AddName(moduleName, moduleName)

	// Ensure generated packages are known
	cfg.Packages.AddName(cfg.Model.GetImportPath(), cfg.Model.Package)
	if cfg.Model.GetImportPath() != cfg.Exec.GetImportPath() {
		cfg.Packages.AddName(cfg.Exec.GetImportPath(), cfg.Exec.Package)
	}

	if err := cfg.Init(); err != nil {
		return nil, fmt.Errorf("failed to init config: %w", err)
	}

	// Register common types in virtual package system
	cb.virtualPackages.RegisterCommonTypes()

	// Register gqlgen types
	cb.registerGqlgenTypes()

	// Register generated package
	cb.virtualPackages.RegisterPackage(cfg.Exec.GetImportPath(), cfg.Exec.Package)

	// Populate Models for scalars
	cfg.Models.Add("String", "github.com/99designs/gqlgen/graphql.String")
	cfg.Models.Add("Int", "github.com/99designs/gqlgen/graphql.Int")
	cfg.Models.Add("Int32", "github.com/99designs/gqlgen/graphql.Int32")
	cfg.Models.Add("Int64", "github.com/99designs/gqlgen/graphql.Int64")
	cfg.Models.Add("Float", "github.com/99designs/gqlgen/graphql.Float")
	cfg.Models.Add("Boolean", "github.com/99designs/gqlgen/graphql.Boolean")
	cfg.Models.Add("ID", "github.com/99designs/gqlgen/graphql.ID")
	cfg.Models.Add("Any", "map[string]interface{}")
	cfg.Models.Add("Map", "map[string]interface{}")
	cfg.Models.Add("Time", "github.com/99designs/gqlgen/graphql.Time")
	cfg.Models.Add("Upload", "github.com/99designs/gqlgen/graphql.Upload")

	// Populate Models for introspection types - these bind to the real gqlgen introspection package
	cfg.Models.Add("__Directive", "github.com/99designs/gqlgen/graphql/introspection.Directive")
	cfg.Models.Add("__DirectiveLocation", "github.com/99designs/gqlgen/graphql.String")
	cfg.Models.Add("__EnumValue", "github.com/99designs/gqlgen/graphql/introspection.EnumValue")
	cfg.Models.Add("__Field", "github.com/99designs/gqlgen/graphql/introspection.Field")
	cfg.Models.Add("__InputValue", "github.com/99designs/gqlgen/graphql/introspection.InputValue")
	cfg.Models.Add("__Schema", "github.com/99designs/gqlgen/graphql/introspection.Schema")
	cfg.Models.Add("__Type", "github.com/99designs/gqlgen/graphql/introspection.Type")
	cfg.Models.Add("__TypeKind", "github.com/99designs/gqlgen/graphql.String")

	// Populate Models for all schema types
	for _, schemaType := range cfg.Schema.Types {
		if _, ok := cfg.Models[schemaType.Name]; ok {
			continue
		}

		// Register the type in virtual packages so Binder can find it later
		// But DO NOT add to cfg.Models yet, otherwise modelgen will skip generating it

		if schemaType.Kind == ast.Object || schemaType.Kind == ast.InputObject || schemaType.Kind == ast.Enum {
			// Register the type in virtual packages
			// Use a dummy struct type since we are generating it
			cb.virtualPackages.CreateNamedType(cfg.Model.GetImportPath(), templates.ToGoModelName(schemaType.Name), types.NewStruct(nil, nil))
		} else if schemaType.Kind == ast.Interface {
			// Register as an interface type
			interfaceType := types.NewInterfaceType(nil, nil)
			cb.virtualPackages.CreateNamedType(cfg.Model.GetImportPath(), templates.ToGoModelName(schemaType.Name), interfaceType)
		} else if schemaType.Kind == ast.Union {
			// Register as an interface type
			unionInterface := types.NewInterfaceType(nil, nil)
			cb.virtualPackages.CreateNamedType(cfg.Model.GetImportPath(), templates.ToGoModelName(schemaType.Name), unionInterface)
		}
	}

	// Set the PackageLoader to our virtual packages for type resolution
	cfg.PackageLoader = cb.virtualPackages

	return cfg, nil
}

// registerGqlgenTypes registers common gqlgen types
func (cb *ConfigBuilder) registerGqlgenTypes() {
	// Register graphql package types
	graphqlPkg := cb.virtualPackages.RegisterPackage("github.com/99designs/gqlgen/graphql", "graphql")

	// Common scalar types
	scalarTypes := []string{
		"String", "Int", "Int32", "Int64", "Float", "Boolean",
		"ID", "IntID", "Time", "Map", "Upload", "Any",
	}

	for _, typeName := range scalarTypes {
		// Create a basic string type as underlying type
		stringType, _ := cb.virtualPackages.GetBasicType("string")
		cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql", typeName, stringType)
	}

	// Register introspection types with their actual fields
	// These must match the real introspection package structs
	introspectionPkg := cb.virtualPackages.RegisterPackage(
		"github.com/99designs/gqlgen/graphql/introspection",
		"introspection",
	)

	stringType := types.Typ[types.String]
	boolType := types.Typ[types.Bool]

	// Create Type struct first (needed by Field and InputValue)
	typeStruct := cb.virtualPackages.CreateStruct([]*types.Var{
		types.NewVar(0, introspectionPkg, "Name", stringType),
	})
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "Type", typeStruct)

	// Create InputValue struct
	inputValueStruct := cb.virtualPackages.CreateStruct([]*types.Var{
		types.NewVar(0, introspectionPkg, "Name", stringType),
		types.NewVar(0, introspectionPkg, "DefaultValue", types.NewPointer(stringType)),
	})
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "InputValue", inputValueStruct)

	// Create Field struct
	fieldStruct := cb.virtualPackages.CreateStruct([]*types.Var{
		types.NewVar(0, introspectionPkg, "Name", stringType),
	})
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "Field", fieldStruct)

	// Create EnumValue struct
	enumValueStruct := cb.virtualPackages.CreateStruct([]*types.Var{
		types.NewVar(0, introspectionPkg, "Name", stringType),
	})
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "EnumValue", enumValueStruct)

	// Create Directive struct
	directiveStruct := cb.virtualPackages.CreateStruct([]*types.Var{
		types.NewVar(0, introspectionPkg, "Name", stringType),
		types.NewVar(0, introspectionPkg, "IsRepeatable", boolType),
	})
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "Directive", directiveStruct)

	// Create Schema struct
	schemaStruct := cb.virtualPackages.CreateStruct(nil)
	cb.virtualPackages.RegisterType("github.com/99designs/gqlgen/graphql/introspection", "Schema", schemaStruct)

	_ = graphqlPkg
}

// GetVirtualPackages returns the virtual packages system
func (cb *ConfigBuilder) GetVirtualPackages() *VirtualPackages {
	return cb.virtualPackages
}

// BuildConfigWithOptions creates a config with custom options
func (cb *ConfigBuilder) BuildConfigWithOptions(opts ConfigOptions) (*config.Config, error) {
	cfg, err := cb.BuildConfig(opts.Schema, opts.ModuleName)
	if err != nil {
		return nil, err
	}

	// Apply custom package names
	if opts.ExecPackage != "" {
		cfg.Exec.Package = opts.ExecPackage
	}
	if opts.ModelPackage != "" {
		cfg.Model.Package = opts.ModelPackage
	}
	if opts.ResolverPackage != "" {
		cfg.Resolver.Package = opts.ResolverPackage
	}

	// Apply custom filenames
	if opts.ExecFilename != "" {
		cfg.Exec.Filename = opts.ExecFilename
	}
	if opts.ModelFilename != "" {
		cfg.Model.Filename = opts.ModelFilename
	}
	if opts.ResolverFilename != "" {
		cfg.Resolver.Filename = opts.ResolverFilename
	}

	// Apply other options
	if opts.OmitSliceElementPointers {
		cfg.OmitSliceElementPointers = true
	}
	if opts.OmitGetters {
		cfg.OmitGetters = true
	}
	if opts.StructFieldsAlwaysPointers {
		cfg.StructFieldsAlwaysPointers = true
	}

	// Apply AutoBind packages
	if len(opts.AutoBind) > 0 {
		cfg.AutoBind = opts.AutoBind
	}

	// Apply custom model mappings
	if len(opts.Models) > 0 {
		for typeName, modelPath := range opts.Models {
			cfg.Models.Add(typeName, modelPath)
		}
	}

	return cfg, nil
}

// ConfigOptions contains options for building a config
type ConfigOptions struct {
	Schema     string
	ModuleName string

	// Package names
	ExecPackage     string
	ModelPackage    string
	ResolverPackage string

	// Filenames
	ExecFilename     string
	ModelFilename    string
	ResolverFilename string

	// Generation options
	OmitSliceElementPointers   bool
	OmitGetters                bool
	StructFieldsAlwaysPointers bool

	// GitHub integration for AutoBind and Custom Models
	GitHubToken string   // Optional: for private repos
	GitHubRef   string   // Optional: branch/tag/commit (default: "main")
	AutoBind    []string // Package paths to autobind (e.g., "github.com/user/repo/models")

	// Custom model mappings: GraphQL type name -> Go type path
	// e.g., {"User": "github.com/user/repo/models.User"}
	Models map[string]string
}
