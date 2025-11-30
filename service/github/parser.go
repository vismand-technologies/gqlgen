package github

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
)

// TypeInfo represents extracted type information from Go source
type TypeInfo struct {
	Name       string
	Package    string
	ImportPath string
	Kind       TypeKind
	Fields     []FieldInfo    // for structs
	Methods    []MethodInfo   // methods with receivers
	Underlying string         // for type aliases
}

// TypeKind represents the kind of Go type
type TypeKind int

const (
	TypeKindStruct TypeKind = iota
	TypeKindInterface
	TypeKindAlias
	TypeKindBasic
)

// FieldInfo represents a struct field
type FieldInfo struct {
	Name     string
	Type     string
	Tag      string
	Embedded bool
}

// MethodInfo represents a method
type MethodInfo struct {
	Name       string
	Params     []ParamInfo
	Results    []ParamInfo
	HasContext bool // first param is context.Context
}

// ParamInfo represents a function parameter or result
type ParamInfo struct {
	Name string
	Type string
}

// PackageTypes holds all types extracted from a package
type PackageTypes struct {
	Name       string
	ImportPath string
	Types      map[string]*TypeInfo
}

// Parser parses Go source files and extracts type information
type Parser struct {
	fset *token.FileSet
}

// NewParser creates a new Go source parser
func NewParser() *Parser {
	return &Parser{
		fset: token.NewFileSet(),
	}
}

// ParsePackage parses Go source files and extracts type information
func (p *Parser) ParsePackage(importPath string, files []FileContent) (*PackageTypes, error) {
	pkg := &PackageTypes{
		ImportPath: importPath,
		Types:      make(map[string]*TypeInfo),
	}

	for _, file := range files {
		f, err := parser.ParseFile(p.fset, file.Path, file.Content, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file.Path, err)
		}

		if pkg.Name == "" {
			pkg.Name = f.Name.Name
		}

		p.extractTypes(f, importPath, pkg)
	}

	return pkg, nil
}

// extractTypes extracts type declarations from an AST file
func (p *Parser) extractTypes(f *ast.File, importPath string, pkg *PackageTypes) {
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Skip unexported types
			if !ast.IsExported(typeSpec.Name.Name) {
				continue
			}

			info := &TypeInfo{
				Name:       typeSpec.Name.Name,
				Package:    pkg.Name,
				ImportPath: importPath,
			}

			switch t := typeSpec.Type.(type) {
			case *ast.StructType:
				info.Kind = TypeKindStruct
				info.Fields = p.extractFields(t)
			case *ast.InterfaceType:
				info.Kind = TypeKindInterface
				info.Methods = p.extractInterfaceMethods(t)
			case *ast.Ident:
				info.Kind = TypeKindAlias
				info.Underlying = t.Name
			case *ast.SelectorExpr:
				info.Kind = TypeKindAlias
				info.Underlying = p.exprToString(t)
			default:
				info.Kind = TypeKindBasic
				info.Underlying = p.exprToString(typeSpec.Type)
			}

			pkg.Types[info.Name] = info
		}
	}

	// Extract methods with receivers
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		// Get receiver type name
		recvType := p.getReceiverTypeName(funcDecl.Recv)
		if recvType == "" {
			continue
		}

		// Skip unexported methods
		if !ast.IsExported(funcDecl.Name.Name) {
			continue
		}

		typeInfo, exists := pkg.Types[recvType]
		if !exists {
			continue
		}

		method := p.extractMethod(funcDecl)
		typeInfo.Methods = append(typeInfo.Methods, method)
	}
}

// extractFields extracts fields from a struct type
func (p *Parser) extractFields(st *ast.StructType) []FieldInfo {
	var fields []FieldInfo

	if st.Fields == nil {
		return fields
	}

	for _, field := range st.Fields.List {
		typeStr := p.exprToString(field.Type)

		var tag string
		if field.Tag != nil {
			tag = strings.Trim(field.Tag.Value, "`")
		}

		if len(field.Names) == 0 {
			// Embedded field
			fields = append(fields, FieldInfo{
				Name:     typeStr,
				Type:     typeStr,
				Tag:      tag,
				Embedded: true,
			})
		} else {
			for _, name := range field.Names {
				if !ast.IsExported(name.Name) {
					continue
				}
				fields = append(fields, FieldInfo{
					Name: name.Name,
					Type: typeStr,
					Tag:  tag,
				})
			}
		}
	}

	return fields
}

// extractInterfaceMethods extracts methods from an interface type
func (p *Parser) extractInterfaceMethods(it *ast.InterfaceType) []MethodInfo {
	var methods []MethodInfo

	if it.Methods == nil {
		return methods
	}

	for _, method := range it.Methods.List {
		if len(method.Names) == 0 {
			continue // embedded interface
		}

		funcType, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}

		for _, name := range method.Names {
			if !ast.IsExported(name.Name) {
				continue
			}

			m := MethodInfo{
				Name:    name.Name,
				Params:  p.extractParams(funcType.Params),
				Results: p.extractParams(funcType.Results),
			}

			// Check if first param is context.Context
			if len(m.Params) > 0 && (m.Params[0].Type == "context.Context" || m.Params[0].Type == "Context") {
				m.HasContext = true
			}

			methods = append(methods, m)
		}
	}

	return methods
}

// extractMethod extracts method info from a function declaration
func (p *Parser) extractMethod(funcDecl *ast.FuncDecl) MethodInfo {
	m := MethodInfo{
		Name:    funcDecl.Name.Name,
		Params:  p.extractParams(funcDecl.Type.Params),
		Results: p.extractParams(funcDecl.Type.Results),
	}

	// Check if first param is context.Context
	if len(m.Params) > 0 && (m.Params[0].Type == "context.Context" || m.Params[0].Type == "Context") {
		m.HasContext = true
	}

	return m
}

// extractParams extracts parameters from a field list
func (p *Parser) extractParams(fl *ast.FieldList) []ParamInfo {
	var params []ParamInfo

	if fl == nil {
		return params
	}

	for _, field := range fl.List {
		typeStr := p.exprToString(field.Type)

		if len(field.Names) == 0 {
			params = append(params, ParamInfo{Type: typeStr})
		} else {
			for _, name := range field.Names {
				params = append(params, ParamInfo{
					Name: name.Name,
					Type: typeStr,
				})
			}
		}
	}

	return params
}

// getReceiverTypeName extracts the type name from a method receiver
func (p *Parser) getReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	recvType := recv.List[0].Type

	// Handle pointer receiver
	if star, ok := recvType.(*ast.StarExpr); ok {
		recvType = star.X
	}

	if ident, ok := recvType.(*ast.Ident); ok {
		return ident.Name
	}

	return ""
}

// exprToString converts an AST expression to a string representation
func (p *Parser) exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return p.exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + p.exprToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.exprToString(t.Elt)
		}
		return fmt.Sprintf("[%s]%s", p.exprToString(t.Len), p.exprToString(t.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", p.exprToString(t.Key), p.exprToString(t.Value))
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + p.exprToString(t.Value)
	case *ast.BasicLit:
		return t.Value
	case *ast.Ellipsis:
		return "..." + p.exprToString(t.Elt)
	default:
		return "unknown"
	}
}

// ConvertToGoType converts TypeInfo to go/types.Type for use with gqlgen binder
func (p *Parser) ConvertToGoType(info *TypeInfo, pkg *types.Package) types.Type {
	switch info.Kind {
	case TypeKindStruct:
		return p.createStructType(info, pkg)
	case TypeKindInterface:
		return p.createInterfaceType(info, pkg)
	default:
		// For aliases and basic types, return a named type wrapping the underlying
		return nil
	}
}

// createStructType creates a types.Struct from TypeInfo
func (p *Parser) createStructType(info *TypeInfo, pkg *types.Package) *types.Struct {
	var fields []*types.Var
	var tags []string

	for _, f := range info.Fields {
		fieldType := p.parseTypeString(f.Type, pkg)
		if fieldType == nil {
			fieldType = types.Typ[types.String] // fallback
		}

		field := types.NewField(token.NoPos, pkg, f.Name, fieldType, f.Embedded)
		fields = append(fields, field)
		tags = append(tags, f.Tag)
	}

	return types.NewStruct(fields, tags)
}

// createInterfaceType creates a types.Interface from TypeInfo
func (p *Parser) createInterfaceType(info *TypeInfo, pkg *types.Package) *types.Interface {
	var methods []*types.Func

	for _, m := range info.Methods {
		sig := p.createSignature(m, pkg)
		fn := types.NewFunc(token.NoPos, pkg, m.Name, sig)
		methods = append(methods, fn)
	}

	iface := types.NewInterfaceType(methods, nil)
	iface.Complete()
	return iface
}

// createSignature creates a types.Signature from MethodInfo
func (p *Parser) createSignature(m MethodInfo, pkg *types.Package) *types.Signature {
	var params []*types.Var
	var results []*types.Var

	for _, param := range m.Params {
		paramType := p.parseTypeString(param.Type, pkg)
		if paramType == nil {
			paramType = types.Typ[types.String]
		}
		params = append(params, types.NewParam(token.NoPos, pkg, param.Name, paramType))
	}

	for _, result := range m.Results {
		resultType := p.parseTypeString(result.Type, pkg)
		if resultType == nil {
			resultType = types.Typ[types.String]
		}
		results = append(results, types.NewParam(token.NoPos, pkg, result.Name, resultType))
	}

	return types.NewSignature(nil, types.NewTuple(params...), types.NewTuple(results...), false)
}

// parseTypeString parses a type string into a types.Type
func (p *Parser) parseTypeString(typeStr string, pkg *types.Package) types.Type {
	// Handle common types
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
		// Return interface{} as placeholder for context.Context
		return types.Universe.Lookup("any").Type()
	}

	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		elem := p.parseTypeString(typeStr[1:], pkg)
		if elem != nil {
			return types.NewPointer(elem)
		}
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		elem := p.parseTypeString(typeStr[2:], pkg)
		if elem != nil {
			return types.NewSlice(elem)
		}
	}

	// Handle map types
	if strings.HasPrefix(typeStr, "map[") {
		// Simple parsing for map[string]T
		if strings.HasPrefix(typeStr, "map[string]") {
			valueType := p.parseTypeString(typeStr[11:], pkg)
			if valueType != nil {
				return types.NewMap(types.Typ[types.String], valueType)
			}
		}
	}

	// For unknown types, return nil (caller should handle)
	return nil
}
