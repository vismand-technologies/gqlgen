package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/99designs/gqlgen/service/github"
	"github.com/99designs/gqlgen/service/memory"
	"github.com/99designs/gqlgen/service/testgen"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// GeneratorService handles GraphQL code generation
type GeneratorService struct {
	memoryGenerator *memory.InMemoryGenerator
}

// NewGeneratorService creates a new generator service
func NewGeneratorService(tempDir string) *GeneratorService {
	return &GeneratorService{
		memoryGenerator: memory.NewInMemoryGenerator(),
	}
}

// GenerateRequest represents a code generation request
type GenerateRequest struct {
	Schema          string            `json:"schema"`
	Config          *GenerateConfig   `json:"config,omitempty"`
	AdditionalFiles map[string]string `json:"additional_files,omitempty"` // filename -> content
}

// GenerateConfig allows customization of the generation process
type GenerateConfig struct {
	ModuleName               string `json:"module_name,omitempty"`
	PackageName              string `json:"package_name,omitempty"`
	ModelPackage             string `json:"model_package,omitempty"`
	ResolverPackage          string `json:"resolver_package,omitempty"`
	SkipValidation           bool   `json:"skip_validation,omitempty"`
	OmitSliceElementPointers bool   `json:"omit_slice_element_pointers,omitempty"`
	OmitGetters              bool   `json:"omit_getters,omitempty"`

	// GitHub integration for AutoBind and Custom Models
	GitHubToken string   `json:"github_token,omitempty"` // Optional: for private repos
	GitHubRef   string   `json:"github_ref,omitempty"`   // Optional: branch/tag/commit (default: "main")
	AutoBind    []string `json:"autobind,omitempty"`     // Package paths to autobind

	// Custom model mappings: GraphQL type name -> Go type path
	// e.g., {"User": "github.com/user/repo/models.User"}
	Models map[string]string `json:"models,omitempty"`

	// Generate integration tests using gqlgen client
	GenerateTests bool `json:"generate_tests,omitempty"`
}

// GenerateResult contains the generated files
type GenerateResult struct {
	Files  map[string][]byte `json:"files"`
	Errors []string          `json:"errors,omitempty"`
}

// Generate performs code generation from a schema
func (s *GeneratorService) Generate(req *GenerateRequest) (*GenerateResult, error) {
	// Prepare options
	opts := memory.ConfigOptions{
		Schema:     req.Schema,
		ModuleName: "generated", // Default
	}

	if req.Config != nil {
		if req.Config.ModuleName != "" {
			opts.ModuleName = req.Config.ModuleName
		}
		opts.ExecPackage = req.Config.PackageName
		opts.ModelPackage = req.Config.ModelPackage
		opts.ResolverPackage = req.Config.ResolverPackage
		opts.OmitSliceElementPointers = req.Config.OmitSliceElementPointers
		opts.OmitGetters = req.Config.OmitGetters

		// GitHub integration
		opts.GitHubToken = req.Config.GitHubToken
		opts.GitHubRef = req.Config.GitHubRef
		opts.AutoBind = req.Config.AutoBind
		opts.Models = req.Config.Models
	}

	// Load GitHub packages if AutoBind or Models are specified
	if req.Config != nil && (len(req.Config.AutoBind) > 0 || len(req.Config.Models) > 0) {
		if err := s.loadGitHubPackages(req.Config, s.memoryGenerator.GetVirtualPackages()); err != nil {
			log.Printf("Warning: failed to load GitHub packages: %v", err)
			// Continue anyway - types will be generated instead of bound
		}
	}

	// Generate code in memory
	files, err := s.memoryGenerator.Generate(opts)
	if err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	// Generate integration tests if requested
	if req.Config != nil && req.Config.GenerateTests {
		testFiles, err := s.generateTests(req.Schema, opts.ModuleName)
		if err != nil {
			log.Printf("Warning: failed to generate tests: %v", err)
		} else {
			for name, content := range testFiles {
				files["tests/"+name] = content
			}
		}
	}

	// Add additional files if provided
	if req.AdditionalFiles != nil {
		for filename, content := range req.AdditionalFiles {
			files[filename] = []byte(content)
		}
	}

	return &GenerateResult{
		Files: files,
	}, nil
}

// generateTests generates integration test files
func (s *GeneratorService) generateTests(schemaStr, moduleName string) (map[string][]byte, error) {
	// Parse schema
	source := &ast.Source{
		Name:  "schema.graphqls",
		Input: schemaStr,
	}

	schema, err := gqlparser.LoadSchema(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Generate tests
	gen := testgen.NewGenerator()
	return gen.GenerateTests(schema, moduleName)
}

// loadGitHubPackages loads packages from GitHub for AutoBind and custom Models
func (s *GeneratorService) loadGitHubPackages(config *GenerateConfig, registry github.TypeRegistry) error {
	// Pass empty ref to let fetcher resolve default branch per-repo
	loader := github.NewPackageLoader(config.GitHubToken, registry, config.GitHubRef)
	ctx := context.Background()

	// Collect all unique package paths to load
	packagesToLoad := make(map[string]bool)

	// Add AutoBind packages
	for _, pkg := range config.AutoBind {
		if strings.HasPrefix(pkg, "github.com/") {
			packagesToLoad[pkg] = true
		}
	}

	// Add packages from custom Models
	for _, modelPath := range config.Models {
		// Extract package path from "github.com/user/repo/pkg.TypeName"
		if strings.HasPrefix(modelPath, "github.com/") {
			if idx := strings.LastIndex(modelPath, "."); idx > 0 {
				pkgPath := modelPath[:idx]
				packagesToLoad[pkgPath] = true
			}
		}
	}

	// Load all packages
	for pkgPath := range packagesToLoad {
		refInfo := config.GitHubRef
		if refInfo == "" {
			refInfo = "default"
		}
		log.Printf("Loading GitHub package: %s (ref: %s)", pkgPath, refInfo)
		if _, err := loader.LoadPackage(ctx, pkgPath); err != nil {
			return fmt.Errorf("failed to load package %s: %w", pkgPath, err)
		}
	}

	return nil
}
