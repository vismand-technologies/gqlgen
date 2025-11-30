package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	gen := NewGeneratorService(os.TempDir())
	
	req := &GenerateRequest{
		Schema: `
type Query {
	hello: String!
}
`,
	}
	
	result, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}
	
	// Check for expected files
	expectedFiles := []string{
		"generated/generated.go",
		"generated/models_gen.go",
		"resolver.go",
	}
	
	for _, expected := range expectedFiles {
		found := false
		for path := range result.Files {
			if filepath.ToSlash(path) == expected || filepath.Base(path) == filepath.Base(expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in generated files", expected)
		}
	}
}
