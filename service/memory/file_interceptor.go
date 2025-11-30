package memory

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/99designs/gqlgen/codegen/templates"
)

// FileInterceptor captures file writes from templates.Render
type FileInterceptor struct {
	mu             sync.RWMutex
	capturedFiles  map[string][]byte
	originalRender func(templates.Options) error
	active         bool
}

// NewFileInterceptor creates a new file interceptor
func NewFileInterceptor() *FileInterceptor {
	return &FileInterceptor{
		capturedFiles: make(map[string][]byte),
	}
}

// Start begins intercepting template renders
func (fi *FileInterceptor) Start() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.active {
		return nil // Already active
	}

	// Save the original Render function
	fi.originalRender = templates.Render

	// Replace with our intercepting version
	templates.Render = func(opts templates.Options) error {
		return fi.interceptRender(opts)
	}

	fi.active = true
	return nil
}

// Stop restores the original template render function
func (fi *FileInterceptor) Stop() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if !fi.active {
		return nil // Not active
	}

	// Restore original Render function
	if fi.originalRender != nil {
		templates.Render = fi.originalRender
	}

	fi.active = false
	return nil
}

// interceptRender captures the rendered output instead of writing to disk
func (fi *FileInterceptor) interceptRender(opts templates.Options) error {
	// Call the original render to a temp location and read it back
	tmpFile := filepath.Join(os.TempDir(), filepath.Base(opts.Filename))

	// Temporarily modify filename to write to temp
	originalFilename := opts.Filename
	opts.Filename = tmpFile

	// Call original render
	err := fi.originalRender(opts)
	if err != nil {
		return err
	}

	// Read the temp file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		return err
	}

	// Clean up temp file
	os.Remove(tmpFile)

	// Convert absolute path to relative path
	relPath := originalFilename
	if filepath.IsAbs(originalFilename) {
		if wd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(wd, originalFilename); err == nil {
				relPath = rel
			}
		}
	}

	// Store in our map
	fi.mu.Lock()
	fi.capturedFiles[relPath] = content
	fi.mu.Unlock()

	return nil
}

// GetFiles returns all captured files
func (fi *FileInterceptor) GetFiles() map[string][]byte {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	// Return a copy
	result := make(map[string][]byte, len(fi.capturedFiles))
	for k, v := range fi.capturedFiles {
		result[k] = v
	}
	return result
}

// Clear removes all captured files
func (fi *FileInterceptor) Clear() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.capturedFiles = make(map[string][]byte)
}

// IsActive returns whether interception is currently active
func (fi *FileInterceptor) IsActive() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.active
}
