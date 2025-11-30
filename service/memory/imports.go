package memory

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
	"sync"
)

// MemoryImports manages import statements for in-memory code generation
type MemoryImports struct {
	mu       sync.RWMutex
	reserved map[string]string // import path -> alias
	aliases  map[string]string // alias -> import path
	counter  int               // for generating unique aliases
}

// NewImports creates a new memory imports manager
func NewImports() *MemoryImports {
	return &MemoryImports{
		reserved: make(map[string]string),
		aliases:  make(map[string]string),
	}
}

// Reserve reserves an import path and returns its alias
func (m *MemoryImports) Reserve(importPath string, aliases ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already reserved
	if alias, ok := m.reserved[importPath]; ok {
		return alias, nil
	}

	// Determine alias
	var alias string
	if len(aliases) > 0 && aliases[0] != "" {
		alias = aliases[0]
	} else {
		// Extract package name from import path
		parts := strings.Split(importPath, "/")
		alias = parts[len(parts)-1]

		// Remove common suffixes
		alias = strings.TrimSuffix(alias, "-go")
		alias = strings.TrimSuffix(alias, ".go")

		// Handle version suffixes (e.g., v2, v3)
		if strings.HasPrefix(alias, "v") && len(alias) > 1 {
			if _, err := fmt.Sscanf(alias, "v%d", new(int)); err == nil {
				// This is a version suffix, use parent directory name
				if len(parts) > 1 {
					alias = parts[len(parts)-2]
				}
			}
		}
	}

	// Ensure alias is unique
	originalAlias := alias
	for {
		if _, exists := m.aliases[alias]; !exists {
			break
		}
		m.counter++
		alias = fmt.Sprintf("%s%d", originalAlias, m.counter)
	}

	// Reserve the import
	m.reserved[importPath] = alias
	m.aliases[alias] = importPath

	return alias, nil
}

// Lookup returns the alias for an import path
func (m *MemoryImports) Lookup(importPath string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if alias, ok := m.reserved[importPath]; ok {
		return alias
	}
	return ""
}

// LookupType returns the string representation of a type with proper imports
func (m *MemoryImports) LookupType(t types.Type) string {
	return m.formatType(t)
}

// formatType formats a type with proper package prefixes
func (m *MemoryImports) formatType(t types.Type) string {
	switch typ := t.(type) {
	case *types.Named:
		pkg := typ.Obj().Pkg()
		if pkg == nil {
			return typ.Obj().Name()
		}

		alias := m.Lookup(pkg.Path())
		if alias == "" {
			// Auto-reserve if not already reserved
			alias, _ = m.Reserve(pkg.Path())
		}

		if alias != "" {
			return alias + "." + typ.Obj().Name()
		}
		return typ.Obj().Name()

	case *types.Pointer:
		return "*" + m.formatType(typ.Elem())

	case *types.Slice:
		return "[]" + m.formatType(typ.Elem())

	case *types.Array:
		return fmt.Sprintf("[%d]%s", typ.Len(), m.formatType(typ.Elem()))

	case *types.Map:
		return fmt.Sprintf("map[%s]%s",
			m.formatType(typ.Key()),
			m.formatType(typ.Elem()))

	case *types.Chan:
		switch typ.Dir() {
		case types.SendRecv:
			return "chan " + m.formatType(typ.Elem())
		case types.SendOnly:
			return "chan<- " + m.formatType(typ.Elem())
		case types.RecvOnly:
			return "<-chan " + m.formatType(typ.Elem())
		}

	case *types.Basic:
		return typ.Name()

	case *types.Interface:
		if typ.Empty() {
			return "interface{}"
		}
		return "interface{...}"

	case *types.Struct:
		return "struct{...}"
	}

	return fmt.Sprintf("%T", t)
}

// String generates the import block as a string
func (m *MemoryImports) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.reserved) == 0 {
		return ""
	}

	// Sort imports for consistent output
	type importPair struct {
		path  string
		alias string
	}

	imports := make([]importPair, 0, len(m.reserved))
	for path, alias := range m.reserved {
		imports = append(imports, importPair{path, alias})
	}

	sort.Slice(imports, func(i, j int) bool {
		return imports[i].path < imports[j].path
	})

	var buf strings.Builder
	for _, imp := range imports {
		// Extract package name from path
		parts := strings.Split(imp.path, "/")
		pkgName := parts[len(parts)-1]

		// Only write alias if it differs from package name
		if imp.alias != pkgName {
			buf.WriteString("\t")
			buf.WriteString(imp.alias)
			buf.WriteString(" ")
		} else {
			buf.WriteString("\t")
		}

		buf.WriteString(`"`)
		buf.WriteString(imp.path)
		buf.WriteString(`"`)
		buf.WriteString("\n")
	}

	return buf.String()
}

// GetImports returns all reserved imports
func (m *MemoryImports) GetImports() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.reserved))
	for path, alias := range m.reserved {
		result[path] = alias
	}
	return result
}

// Clear removes all imports
func (m *MemoryImports) Clear() {
	m.mu.Lock()
	m.reserved = make(map[string]string)
	m.aliases = make(map[string]string)
	m.counter = 0
	m.mu.Unlock()
}

// Count returns the number of imports
func (m *MemoryImports) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.reserved)
}

// Has checks if an import path is reserved
func (m *MemoryImports) Has(importPath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.reserved[importPath]
	return ok
}

// Clone creates a copy of the imports manager
func (m *MemoryImports) Clone() *MemoryImports {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := NewImports()
	for path, alias := range m.reserved {
		clone.reserved[path] = alias
		clone.aliases[alias] = path
	}
	clone.counter = m.counter

	return clone
}
