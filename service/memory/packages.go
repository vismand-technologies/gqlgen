package memory

import (
	"fmt"
	"go/types"
	"sync"
)

// VirtualPackages provides type resolution without filesystem access
type VirtualPackages struct {
	mu       sync.RWMutex
	packages map[string]*types.Package
	objects  map[string]types.Object
	nameMap  map[string]string // import path -> package name
}

// NewVirtualPackages creates a new virtual package system
func NewVirtualPackages() *VirtualPackages {
	vp := &VirtualPackages{
		packages: make(map[string]*types.Package),
		objects:  make(map[string]types.Object),
		nameMap:  make(map[string]string),
	}

	// Pre-populate with common types
	vp.initBuiltinTypes()

	return vp
}

// initBuiltinTypes initializes common Go builtin types
func (vp *VirtualPackages) initBuiltinTypes() {
	// Create builtin package
	builtinPkg := types.NewPackage("", "")

	// Add basic types
	basicTypes := []struct {
		name string
		kind types.BasicKind
	}{
		{"bool", types.Bool},
		{"int", types.Int},
		{"int8", types.Int8},
		{"int16", types.Int16},
		{"int32", types.Int32},
		{"int64", types.Int64},
		{"uint", types.Uint},
		{"uint8", types.Uint8},
		{"uint16", types.Uint16},
		{"uint32", types.Uint32},
		{"uint64", types.Uint64},
		{"uintptr", types.Uintptr},
		{"float32", types.Float32},
		{"float64", types.Float64},
		{"complex64", types.Complex64},
		{"complex128", types.Complex128},
		{"string", types.String},
		{"byte", types.Byte},
		{"rune", types.Rune},
	}

	for _, bt := range basicTypes {
		typ := types.Typ[bt.kind]
		obj := types.NewTypeName(0, builtinPkg, bt.name, typ)
		vp.objects[bt.name] = obj
	}

	// Add interface{} / any
	emptyInterface := types.NewInterfaceType(nil, nil)
	anyObj := types.NewTypeName(0, builtinPkg, "any", emptyInterface)
	vp.objects["any"] = anyObj
	vp.objects["interface{}"] = anyObj

	vp.packages[""] = builtinPkg
}

// RegisterPackage registers a package with the virtual system
func (vp *VirtualPackages) RegisterPackage(importPath, name string) *types.Package {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if pkg, ok := vp.packages[importPath]; ok {
		return pkg
	}

	pkg := types.NewPackage(importPath, name)
	vp.packages[importPath] = pkg
	return pkg
}

// RegisterType registers a type in a package. If wrapNamed is true, wraps in Named type.
func (vp *VirtualPackages) RegisterType(importPath, typeName string, typ types.Type) error {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	pkg, ok := vp.packages[importPath]
	if !ok {
		return fmt.Errorf("package %s not registered", importPath)
	}

	// For basic types (scalars), don't wrap in Named - just use the type directly
	// This prevents infinite recursion in the binder when it looks up marshalers
	_, isBasic := typ.(*types.Basic)
	if isBasic {
		obj := types.NewTypeName(0, pkg, typeName, typ)
		key := importPath + "." + typeName
		vp.objects[key] = obj
		return nil
	}

	// If already a Named type, extract the TypeName object
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		key := importPath + "." + typeName
		vp.objects[key] = obj
		return nil
	}

	// For struct/interface types, wrap with Named to ensure obj.Type() returns *types.Named
	obj := types.NewTypeName(0, pkg, typeName, nil)
	_ = types.NewNamed(obj, typ, nil) // This sets obj's type to the Named type
	
	key := importPath + "." + typeName
	vp.objects[key] = obj

	return nil
}

// LookupType looks up a type by import path and name
func (vp *VirtualPackages) LookupType(importPath, typeName string) (types.Type, error) {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	// Check builtin types first
	if importPath == "" {
		if obj, ok := vp.objects[typeName]; ok {
			return obj.Type(), nil
		}
	}

	// Check registered types
	key := importPath + "." + typeName
	if obj, ok := vp.objects[key]; ok {
		return obj.Type(), nil
	}

	return nil, fmt.Errorf("type %s.%s not found", importPath, typeName)
}

// GetObject returns an object by package path and type name
func (vp *VirtualPackages) GetObject(pkgPath, typeName string) types.Object {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	// Check builtin types first
	if pkgPath == "" {
		if obj, ok := vp.objects[typeName]; ok {
			return obj
		}
	}

	// Check registered types
	key := pkgPath + "." + typeName
	if obj, ok := vp.objects[key]; ok {
		return obj
	}

	return nil
}

// GetPackage returns a package by import path
func (vp *VirtualPackages) GetPackage(importPath string) *types.Package {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	pkg, ok := vp.packages[importPath]
	if !ok {
		return nil
	}
	return pkg
}

// CreateNamedType creates a new named type
func (vp *VirtualPackages) CreateNamedType(importPath, typeName string, underlying types.Type) types.Type {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	pkg, ok := vp.packages[importPath]
	if !ok {
		// Auto-create package if it doesn't exist
		pkg = types.NewPackage(importPath, extractPackageName(importPath))
		vp.packages[importPath] = pkg
	}

	// Create the TypeName with nil type first
	obj := types.NewTypeName(0, pkg, typeName, nil)
	// Create the Named type which sets obj's type to the named type
	named := types.NewNamed(obj, underlying, nil)

	// Store the TypeName object - its Type() method will now return the Named type
	key := importPath + "." + typeName
	vp.objects[key] = obj

	// Verify the type is correctly set
	_ = named

	return named
}

// CreateStruct creates a new struct type
func (vp *VirtualPackages) CreateStruct(fields []*types.Var) *types.Struct {
	return types.NewStruct(fields, nil)
}

// CreateInterface creates a new interface type
func (vp *VirtualPackages) CreateInterface(methods []*types.Func) *types.Interface {
	return types.NewInterfaceType(methods, nil)
}

// CreatePointer creates a pointer type
func (vp *VirtualPackages) CreatePointer(elem types.Type) *types.Pointer {
	return types.NewPointer(elem)
}

// CreateSlice creates a slice type
func (vp *VirtualPackages) CreateSlice(elem types.Type) *types.Slice {
	return types.NewSlice(elem)
}

// CreateMap creates a map type
func (vp *VirtualPackages) CreateMap(key, value types.Type) *types.Map {
	return types.NewMap(key, value)
}

// AddName adds a package name mapping for an import path
func (vp *VirtualPackages) AddName(importPath, name string) {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	vp.nameMap[importPath] = name
}

// GetName returns the package name for an import path
func (vp *VirtualPackages) GetName(importPath string) string {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	if name, ok := vp.nameMap[importPath]; ok {
		return name
	}
	return extractPackageName(importPath)
}

// ListPackages returns all registered packages
func (vp *VirtualPackages) ListPackages() []string {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	paths := make([]string, 0, len(vp.packages))
	for path := range vp.packages {
		if path != "" { // Skip builtin package
			paths = append(paths, path)
		}
	}
	return paths
}

// Clear removes all registered packages and types (except builtins)
func (vp *VirtualPackages) Clear() {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	vp.packages = make(map[string]*types.Package)
	vp.objects = make(map[string]types.Object)
	vp.nameMap = make(map[string]string)

	// Re-initialize builtins
	vp.mu.Unlock() // Unlock before calling initBuiltinTypes
	vp.initBuiltinTypes()
	vp.mu.Lock()
}

// extractPackageName extracts the package name from an import path
func extractPackageName(importPath string) string {
	// Simple extraction - take last component
	parts := []rune(importPath)
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == '/' {
			return string(parts[i+1:])
		}
	}
	return importPath
}

// GetBasicType returns a basic type by name
func (vp *VirtualPackages) GetBasicType(name string) (types.Type, error) {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	if obj, ok := vp.objects[name]; ok {
		return obj.Type(), nil
	}

	return nil, fmt.Errorf("basic type %s not found", name)
}

// RegisterCommonTypes registers commonly used types from standard library
func (vp *VirtualPackages) RegisterCommonTypes() {
	// Register context.Context
	contextPkg := vp.RegisterPackage("context", "context")
	contextInterface := types.NewInterfaceType(nil, nil)
	vp.RegisterType("context", "Context", contextInterface)

	// Register error
	errorInterface := types.NewInterfaceType(nil, nil)
	vp.RegisterType("", "error", errorInterface)

	// Register time.Time
	timePkg := vp.RegisterPackage("time", "time")
	timeStruct := types.NewStruct(nil, nil)
	vp.RegisterType("time", "Time", timeStruct)

	_ = contextPkg
	_ = timePkg
}
