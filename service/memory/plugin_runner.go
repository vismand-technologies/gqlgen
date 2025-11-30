package memory

import (
	"fmt"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/99designs/gqlgen/plugin/resolvergen"
)

// PluginRunner orchestrates running gqlgen plugins with file interception
type PluginRunner struct {
	interceptor *FileInterceptor
	config      *config.Config
}

// NewPluginRunner creates a new plugin runner
func NewPluginRunner(cfg *config.Config) *PluginRunner {
	return &PluginRunner{
		interceptor: NewFileInterceptor(),
		config:      cfg,
	}
}

// RunPlugins executes gqlgen plugins and captures their output
func (pr *PluginRunner) RunPlugins() (map[string][]byte, error) {
	// Start file interception
	if err := pr.interceptor.Start(); err != nil {
		return nil, fmt.Errorf("failed to start file interception: %w", err)
	}

	// Ensure we stop interception even if there's an error
	defer pr.interceptor.Stop()

	// Initialize plugins
	modelgenPlugin := modelgen.New()
	resolvergenPlugin := resolvergen.New()

	plugins := []plugin.Plugin{modelgenPlugin, resolvergenPlugin}

	// 1. MutateConfig
	for _, p := range plugins {
		if mut, ok := p.(plugin.ConfigMutator); ok {
			if err := mut.MutateConfig(pr.config); err != nil {
				return nil, fmt.Errorf("%s failed: %w", p.Name(), err)
			}
		}
	}

	// 2. Build Data
	// We need to pass plugins to BuildData so it can use them for type resolution if needed
	dataPlugins := make([]interface{}, len(plugins))
	for i, p := range plugins {
		dataPlugins[i] = p
	}

	data, err := codegen.BuildData(pr.config, dataPlugins...)
	if err != nil {
		return nil, fmt.Errorf("build data failed: %w", err)
	}

	// 3. Generate Code (Plugins)
	for _, p := range plugins {
		if gen, ok := p.(plugin.CodeGenerator); ok {
			if err := gen.GenerateCode(data); err != nil {
				return nil, fmt.Errorf("%s generate code failed: %w", p.Name(), err)
			}
		}
	}

	// 4. Generate Core (Exec)
	if err := codegen.GenerateCode(data); err != nil {
		return nil, fmt.Errorf("codegen generate failed: %w", err)
	}

	// Get captured files
	files := pr.interceptor.GetFiles()

	return files, nil
}

// GetInterceptor returns the file interceptor (for testing)
func (pr *PluginRunner) GetInterceptor() *FileInterceptor {
	return pr.interceptor
}
