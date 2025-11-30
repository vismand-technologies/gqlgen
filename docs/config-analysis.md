# gqlgen YAML Configuration Analysis

## Overview

This document analyzes how gqlgen YAML configuration files reference generated code components and identifies challenges for the REST API implementation.

## How gqlgen YAML Configuration Works

### 1. Import Path Resolution

gqlgen heavily relies on **Go import paths** to reference types and packages:

```yaml
models:
  User:
    model: github.com/myorg/myapp/models.User
  
autobind:
  - github.com/myorg/myapp/models
```

**Key Mechanism:**
- Uses `code.ImportPathForDir()` to determine import paths from file system directories
- Requires a valid Go module context (`go.mod` file)
- Import paths are absolute and based on the module name

### 2. Package Configuration Structure

From `codegen/config/config.go`:

```go
type Config struct {
    SchemaFilename  StringList      // Schema files
    Exec            ExecConfig      // Generated executable code
    Model           PackageConfig   // Generated models
    Resolver        ResolverConfig  // Resolver implementations
    AutoBind        []string        // Auto-bind import paths
    Models          TypeMap         // Type mappings
}
```

Each package config has:
- `Filename` - Where to generate the file
- `Package` - Go package name
- `ImportPath()` - Computed from directory structure

### 3. Type Mapping

gqlgen supports three ways to map GraphQL types to Go types:

**a) Explicit Model Mapping:**
```yaml
models:
  User:
    model: github.com/myorg/myapp/models.User
```

**b) AutoBind:**
```yaml
autobind:
  - github.com/myorg/myapp/models
```
Automatically binds types by name if found in the package.

**c) Generated Models:**
If no mapping exists, gqlgen generates the type.

---

## Critical Challenges for REST API

### Challenge 1: **Import Path Dependencies** 🔴 CRITICAL

**Problem:**
- gqlgen expects import paths like `github.com/user/repo/package`
- Our REST API generates code in temporary directories
- Temporary directories don't have meaningful import paths
- The generated `go.mod` uses `module generated` which creates import path `generated/package`

**Current Implementation:**
```go
// service/generator.go
goModContent := `module generated

go 1.24

require github.com/99designs/gqlgen v0.17.0
`
```

**Impact:**
- Generated code has import path `generated` instead of user's actual module
- If users want to use autobind or custom models, they can't reference their own packages
- Generated code won't work if copied to a real project without fixing imports

**Example Issue:**
```yaml
# User wants this in their config:
models:
  User:
    model: github.com/mycompany/api/models.User

# But our temp directory has module name "generated"
# So the import path becomes: generated.User (wrong!)
```

---

### Challenge 2: **Autobind Requires Real Packages** 🔴 CRITICAL

**Problem:**
- Autobind feature loads Go packages using `packages.Load()`
- Requires packages to exist on disk with proper `go.mod`
- Can't reference user's existing types in temporary workspace

**Code Reference:**
```go
// config.go:810
func (c *Config) autobind() error {
    ps := c.Packages.LoadAll(c.AutoBind...)
    
    for _, p := range ps {
        if p == nil || p.Module == nil {
            return fmt.Errorf(
                "unable to load %s - make sure you're using an import path to a package that exists",
                c.AutoBind[i],
            )
        }
    }
}
```

**Impact:**
- Users can't use their existing model types
- Must rely on generated models only
- Loses one of gqlgen's key features

---

### Challenge 3: **Resolver Package References** 🟡 MODERATE

**Problem:**
- Generated resolvers reference the exec and model packages by import path
- Import paths are hardcoded based on directory structure
- Moving generated code requires updating all imports

**Example:**
```go
// Generated resolver.go
import (
    "generated/generated"  // exec package
    "generated/models"     // model package
)
```

If user moves this to their project, imports break.

---

### Challenge 4: **Federation and Multi-Package Setups** 🟡 MODERATE

**Problem:**
- Federation requires specific package structure
- Multiple services need to reference each other
- Our single-zip approach doesn't handle this well

**Example Federation Config:**
```yaml
exec:
  filename: graph/generated.go
  package: graph

federation:
  filename: graph/federation.go
  package: graph

model:
  filename: graph/model/models_gen.go
  package: model
```

---

## Recommended Solutions

### Solution 1: **Allow Custom Module Name** ✅ RECOMMENDED

Add configuration option for users to specify their module name:

```go
type GenerateConfig struct {
    PackageName     string `json:"package_name,omitempty"`
    ModuleName      string `json:"module_name,omitempty"`  // NEW
    ModulePath      string `json:"module_path,omitempty"`  // NEW
}
```

**Implementation:**
```go
func (s *GeneratorService) initGoModule(workDir string, req *GenerateRequest) error {
    moduleName := "generated"
    if req.Config != nil && req.Config.ModuleName != "" {
        moduleName = req.Config.ModuleName
    }
    
    goModContent := fmt.Sprintf(`module %s

go 1.24

require github.com/99designs/gqlgen v0.17.0
`, moduleName)
    
    return os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goModContent), 0644)
}
```

**Benefits:**
- Generated code has correct import paths
- Can be dropped into user's project
- Supports custom model references

---

### Solution 2: **Post-Generation Import Rewriting** ✅ RECOMMENDED

Add a post-processing step to rewrite imports:

```go
func RewriteImports(files map[string][]byte, oldModule, newModule string) map[string][]byte {
    for path, content := range files {
        if strings.HasSuffix(path, ".go") {
            updated := strings.ReplaceAll(
                string(content),
                fmt.Sprintf("import \"%s", oldModule),
                fmt.Sprintf("import \"%s", newModule),
            )
            files[path] = []byte(updated)
        }
    }
    return files
}
```

---

### Solution 3: **Include README with Instructions** ✅ EASY WIN

Add a README.md to generated zip explaining:
- How to integrate into existing project
- Import path considerations
- Required dependencies

---

### Solution 4: **Support Custom Model Upload** 🔄 FUTURE ENHANCEMENT

Allow users to upload their existing model files:

```json
{
  "schema": "type Query { user: User }",
  "additional_files": {
    "models/user.go": "package models\n\ntype User struct {...}"
  },
  "config": {
    "module_name": "github.com/user/api"
  }
}
```

---

## Current State Assessment

### What Works ✅
- Basic code generation
- Simple schemas without custom types
- Self-contained generated code
- Zip file delivery

### What Needs Improvement 🔧
- Module name configuration
- Import path handling
- Custom model support
- Autobind functionality
- Documentation for integration

### What Doesn't Work ❌
- Autobind with user's existing packages
- Custom model references in config
- Federation multi-package setups
- Direct integration into existing projects

---

## Priority Recommendations

### High Priority (Implement Now)
1. **Add `module_name` configuration option**
2. **Generate README.md with integration instructions**
3. **Update API documentation with import path warnings**

### Medium Priority (Next Phase)
4. **Implement import rewriting utility**
5. **Support additional_files for custom models**
6. **Add validation for module names**

### Low Priority (Future)
7. **Support for complex federation setups**
8. **Interactive import path configuration**
9. **Template-based code generation**

---

## Code Changes Required

### 1. Update GenerateConfig

```go
// service/generator.go
type GenerateConfig struct {
    PackageName              string `json:"package_name,omitempty"`
    ModelPackage             string `json:"model_package,omitempty"`
    ResolverPackage          string `json:"resolver_package,omitempty"`
    ModuleName               string `json:"module_name,omitempty"`      // NEW
    SkipValidation           bool   `json:"skip_validation,omitempty"`
    OmitSliceElementPointers bool   `json:"omit_slice_element_pointers,omitempty"`
}
```

### 2. Update initGoModule

```go
func (s *GeneratorService) initGoModule(workDir string, req *GenerateRequest) error {
    moduleName := "generated"
    if req.Config != nil && req.Config.ModuleName != "" {
        moduleName = req.Config.ModuleName
    }
    
    goModContent := fmt.Sprintf(`module %s

go 1.24

require github.com/99designs/gqlgen v0.17.0
`, moduleName)
    
    return os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goModContent), 0644)
}
```

### 3. Add README Generation

```go
func (s *GeneratorService) generateReadme(req *GenerateRequest) string {
    moduleName := "generated"
    if req.Config != nil && req.Config.ModuleName != "" {
        moduleName = req.Config.ModuleName
    }
    
    return fmt.Sprintf(`# Generated GraphQL Server

## Integration Instructions

This code was generated with module name: %s

### To integrate into your project:

1. Extract files to your project directory
2. Update import paths if needed
3. Run: go mod tidy
4. Implement resolver functions in resolver.go

### Import Paths

All imports use: %s

If you need different import paths, use find-and-replace:
- Find: "import \"%s"
- Replace: "import \"your/module/path"

## Dependencies

- github.com/99designs/gqlgen

Run: go get github.com/99designs/gqlgen
`, moduleName, moduleName, moduleName)
}
```

---

## Conclusion

The main challenge is that **gqlgen was designed for local development** where:
- Code is generated in the user's project
- Import paths are known and stable
- Go module context exists

Our REST API generates code **in isolation**, which breaks assumptions about:
- Import path resolution
- Package references
- Type binding

**Recommended Immediate Action:**
Implement the `module_name` configuration option and generate a README with integration instructions. This provides a good balance between functionality and complexity.
