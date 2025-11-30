package memory

import (
	"fmt"
)

// InMemoryGenerator generates all code in memory
type InMemoryGenerator struct {
	configBuilder *ConfigBuilder
	renderer      *MemoryRenderer
}

// NewInMemoryGenerator creates a new in-memory generator
func NewInMemoryGenerator() *InMemoryGenerator {
	return &InMemoryGenerator{
		configBuilder: NewConfigBuilder(),
		renderer:      NewRenderer(),
	}
}

// GetVirtualPackages returns the virtual packages for external type registration
func (g *InMemoryGenerator) GetVirtualPackages() *VirtualPackages {
	return g.configBuilder.virtualPackages
}

// Generate performs complete code generation in memory using real gqlgen plugins
func (g *InMemoryGenerator) Generate(opts ConfigOptions) (map[string][]byte, error) {
	// Build config
	cfg, err := g.configBuilder.BuildConfigWithOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Use PluginRunner to execute real gqlgen plugins with file interception
	pluginRunner := NewPluginRunner(cfg)
	files, err := pluginRunner.RunPlugins()
	if err != nil {
		return nil, fmt.Errorf("plugin execution failed: %w", err)
	}

	// Add configuration files (schema, gqlgen.yml, go.mod)
	g.addConfigFiles(files, opts.Schema, opts.ModuleName)

	return files, nil
}

// addConfigFiles adds configuration files to the output
func (g *InMemoryGenerator) addConfigFiles(files map[string][]byte, schema string, moduleName string) {
	// Add schema file
	files["schema.graphqls"] = []byte(schema)

	// Add gqlgen.yml
	gqlgenYML := `schema:
  - schema.graphqls

exec:
  filename: generated/generated.go
  package: generated

model:
  filename: generated/models_gen.go
  package: generated

resolver:
  filename: resolver.go
  package: main
  type: Resolver
`
	files["gqlgen.yml"] = []byte(gqlgenYML)

	// Add go.mod
	goMod := fmt.Sprintf(`module %s

go 1.24

require github.com/99designs/gqlgen v0.17.0
`, moduleName)
	files["go.mod"] = []byte(goMod)
}

// Clear clears the renderer
func (g *InMemoryGenerator) Clear() {
	g.renderer.Clear()
}
