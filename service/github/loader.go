package github

import (
	"context"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"strings"
	"sync"
)

// TypeRegistry is an interface for registering types
type TypeRegistry interface {
	RegisterPackage(importPath, name string) *types.Package
	RegisterType(importPath, typeName string, typ types.Type) error
	AddName(importPath, name string)
}

// PackageLoader loads Go packages from GitHub and registers them with a TypeRegistry
type PackageLoader struct {
	fetcher  *Fetcher
	parser   *Parser
	registry TypeRegistry
	cache    map[string]*PackageTypes
	mu       sync.RWMutex
	ref      string // git ref (branch/tag/commit)
}

// NewPackageLoader creates a new GitHub package loader
func NewPackageLoader(token string, registry TypeRegistry, ref string) *PackageLoader {
	return &PackageLoader{
		fetcher:  NewFetcher(token),
		parser:   NewParser(),
		registry: registry,
		cache:    make(map[string]*PackageTypes),
		ref:      ref,
	}
}

// LoadPackage fetches and registers a package from GitHub
func (l *PackageLoader) LoadPackage(ctx context.Context, importPath string) (*PackageTypes, error) {
	// Check cache first
	l.mu.RLock()
	if pkg, ok := l.cache[importPath]; ok {
		l.mu.RUnlock()
		return pkg, nil
	}
	l.mu.RUnlock()

	// Fetch from GitHub
	files, err := l.fetcher.FetchPackage(ctx, importPath, l.ref)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package %s: %w", importPath, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no Go files found in package %s", importPath)
	}

	// Parse the files
	pkg, err := l.parser.ParsePackage(importPath, files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package %s: %w", importPath, err)
	}

	// Register types with VirtualPackages
	l.registerTypes(pkg)

	// Cache the result
	l.mu.Lock()
	l.cache[importPath] = pkg
	l.mu.Unlock()

	log.Printf("Loaded %d types from GitHub package %s", len(pkg.Types), importPath)

	return pkg, nil
}

// LoadPackages loads multiple packages from GitHub
func (l *PackageLoader) LoadPackages(ctx context.Context, importPaths []string) error {
	for _, path := range importPaths {
		if _, err := l.LoadPackage(ctx, path); err != nil {
			return err
		}
	}
	return nil
}

// registerTypes registers all types from a package with the TypeRegistry
func (l *PackageLoader) registerTypes(pkg *PackageTypes) {
	// First register the package
	l.registry.RegisterPackage(pkg.ImportPath, pkg.Name)

	for _, typeInfo := range pkg.Types {
		goType := l.createGoType(typeInfo, pkg)
		if goType != nil {
			l.registry.RegisterType(pkg.ImportPath, typeInfo.Name, goType)
		}
	}

	// Also register the package name mapping
	l.registry.AddName(pkg.ImportPath, pkg.Name)
}

// createGoType creates a go/types.Type from TypeInfo
func (l *PackageLoader) createGoType(info *TypeInfo, pkg *PackageTypes) types.Type {
	// Create a package for the type
	typesPkg := types.NewPackage(pkg.ImportPath, pkg.Name)

	switch info.Kind {
	case TypeKindStruct:
		return l.createStructType(info, typesPkg, pkg)
	case TypeKindInterface:
		return l.createInterfaceType(info, typesPkg)
	default:
		// For aliases, try to resolve the underlying type
		return l.createAliasType(info, typesPkg)
	}
}

// createStructType creates a types.Struct with all fields
func (l *PackageLoader) createStructType(info *TypeInfo, pkg *types.Package, pkgTypes *PackageTypes) types.Type {
	var fields []*types.Var
	var tags []string

	for _, f := range info.Fields {
		fieldType := l.resolveType(f.Type, pkg, pkgTypes)
		if fieldType == nil {
			fieldType = types.Typ[types.String] // fallback
		}

		field := types.NewField(token.NoPos, pkg, f.Name, fieldType, f.Embedded)
		fields = append(fields, field)
		tags = append(tags, f.Tag)
	}

	structType := types.NewStruct(fields, tags)

	// Create a named type
	typeName := types.NewTypeName(token.NoPos, pkg, info.Name, nil)
	named := types.NewNamed(typeName, structType, nil)

	// Add methods
	for _, m := range info.Methods {
		sig := l.createMethodSignature(m, pkg, pkgTypes, named)
		fn := types.NewFunc(token.NoPos, pkg, m.Name, sig)
		named.AddMethod(fn)
	}

	return named
}

// createInterfaceType creates a types.Interface
func (l *PackageLoader) createInterfaceType(info *TypeInfo, pkg *types.Package) types.Type {
	var methods []*types.Func

	for _, m := range info.Methods {
		sig := l.createSignature(m, pkg)
		fn := types.NewFunc(token.NoPos, pkg, m.Name, sig)
		methods = append(methods, fn)
	}

	iface := types.NewInterfaceType(methods, nil)
	iface.Complete()

	// Create a named type
	typeName := types.NewTypeName(token.NoPos, pkg, info.Name, nil)
	return types.NewNamed(typeName, iface, nil)
}

// createAliasType creates a type alias
func (l *PackageLoader) createAliasType(info *TypeInfo, pkg *types.Package) types.Type {
	underlying := l.parseBasicType(info.Underlying)
	if underlying == nil {
		underlying = types.Typ[types.String]
	}

	typeName := types.NewTypeName(token.NoPos, pkg, info.Name, nil)
	return types.NewNamed(typeName, underlying, nil)
}

// createMethodSignature creates a method signature with receiver
func (l *PackageLoader) createMethodSignature(m MethodInfo, pkg *types.Package, pkgTypes *PackageTypes, recv *types.Named) *types.Signature {
	var params []*types.Var
	var results []*types.Var

	for _, param := range m.Params {
		paramType := l.resolveType(param.Type, pkg, pkgTypes)
		if paramType == nil {
			paramType = types.Typ[types.String]
		}
		params = append(params, types.NewParam(token.NoPos, pkg, param.Name, paramType))
	}

	for _, result := range m.Results {
		resultType := l.resolveType(result.Type, pkg, pkgTypes)
		if resultType == nil {
			resultType = types.Typ[types.String]
		}
		results = append(results, types.NewParam(token.NoPos, pkg, result.Name, resultType))
	}

	// Create receiver
	recvVar := types.NewParam(token.NoPos, pkg, "", types.NewPointer(recv))

	return types.NewSignature(recvVar, types.NewTuple(params...), types.NewTuple(results...), false)
}

// createSignature creates a function signature without receiver
func (l *PackageLoader) createSignature(m MethodInfo, pkg *types.Package) *types.Signature {
	var params []*types.Var
	var results []*types.Var

	for _, param := range m.Params {
		paramType := l.parseBasicType(param.Type)
		if paramType == nil {
			paramType = types.Typ[types.String]
		}
		params = append(params, types.NewParam(token.NoPos, pkg, param.Name, paramType))
	}

	for _, result := range m.Results {
		resultType := l.parseBasicType(result.Type)
		if resultType == nil {
			resultType = types.Typ[types.String]
		}
		results = append(results, types.NewParam(token.NoPos, pkg, result.Name, resultType))
	}

	return types.NewSignature(nil, types.NewTuple(params...), types.NewTuple(results...), false)
}

// resolveType resolves a type string to a types.Type, checking local package types first
func (l *PackageLoader) resolveType(typeStr string, pkg *types.Package, pkgTypes *PackageTypes) types.Type {
	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		elem := l.resolveType(typeStr[1:], pkg, pkgTypes)
		if elem != nil {
			return types.NewPointer(elem)
		}
		return nil
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		elem := l.resolveType(typeStr[2:], pkg, pkgTypes)
		if elem != nil {
			return types.NewSlice(elem)
		}
		return nil
	}

	// Check if it's a type from the same package
	if typeInfo, ok := pkgTypes.Types[typeStr]; ok {
		// Return a forward reference to avoid infinite recursion
		typeName := types.NewTypeName(token.NoPos, pkg, typeInfo.Name, nil)
		return types.NewNamed(typeName, types.Typ[types.Invalid], nil)
	}

	// Try basic types
	return l.parseBasicType(typeStr)
}

// parseBasicType parses basic Go types
func (l *PackageLoader) parseBasicType(typeStr string) types.Type {
	switch typeStr {
	case "string":
		return types.Typ[types.String]
	case "int":
		return types.Typ[types.Int]
	case "int8":
		return types.Typ[types.Int8]
	case "int16":
		return types.Typ[types.Int16]
	case "int32":
		return types.Typ[types.Int32]
	case "int64":
		return types.Typ[types.Int64]
	case "uint":
		return types.Typ[types.Uint]
	case "uint8":
		return types.Typ[types.Uint8]
	case "uint16":
		return types.Typ[types.Uint16]
	case "uint32":
		return types.Typ[types.Uint32]
	case "uint64":
		return types.Typ[types.Uint64]
	case "float32":
		return types.Typ[types.Float32]
	case "float64":
		return types.Typ[types.Float64]
	case "bool":
		return types.Typ[types.Bool]
	case "byte":
		return types.Typ[types.Byte]
	case "rune":
		return types.Typ[types.Rune]
	case "error":
		return types.Universe.Lookup("error").Type()
	case "any", "interface{}":
		return types.Universe.Lookup("any").Type()
	case "context.Context", "Context":
		return types.Universe.Lookup("any").Type()
	case "time.Time", "Time":
		return types.Typ[types.String] // Treat as string for now
	}
	return nil
}

// GetCachedPackage returns a cached package if available
func (l *PackageLoader) GetCachedPackage(importPath string) *PackageTypes {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cache[importPath]
}

// HasType checks if a type exists in any loaded package
func (l *PackageLoader) HasType(importPath, typeName string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if pkg, ok := l.cache[importPath]; ok {
		_, exists := pkg.Types[typeName]
		return exists
	}
	return false
}
